package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/PuerkitoBio/goquery"
)

const (
	truyenqqPrimaryURL  = "https://truyenqqko.com"
	truyenqqFallbackURL = "https://metruyenqq.net"
)

// TruyenQQ blocks anything that doesn't look like a real browser.
// So we lie. Ethically questionable, functionally necessary.
const truyenqqBrowserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36"

type TruyenQQProvider struct {
	baseURL    string
	httpClient *http.Client
}

func NewTruyenQQProvider() *TruyenQQProvider {
	p := &TruyenQQProvider{
		httpClient: &http.Client{},
		baseURL:    truyenqqPrimaryURL,
	}
	_ = p.tryDomain(truyenqqPrimaryURL) ||
		p.tryDomain(truyenqqFallbackURL)
	return p
}

func (p *TruyenQQProvider) tryDomain(domain string) bool {
	resp, err := http.Get(domain)
	if err != nil {
		return false
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		p.baseURL = domain
		return true
	}
	return false
}

func (p *TruyenQQProvider) Name() string {
	return "TruyenQQ"
}

func (p *TruyenQQProvider) Search(keyword string) ([]models.Manga, error) {
	endpoint := resolveURL(p.baseURL, "/frontend/search/search")

	form := url.Values{
		"search": {keyword},
		"type":   {"0"},
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("tạo request: %w", err)
	}
	req.Header.Set("User-Agent", truyenqqBrowserUA)
	// TruyenQQ's /frontend/search/search requires these exact headers to return HTML.
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", p.baseURL)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gọi HTTP: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	mangas := make([]models.Manga, 0)
	doc.Find("li").Each(func(i int, s *goquery.Selection) {
		if s.HasClass("no_result") {
			return
		}
		a := s.Find("a")
		href, exists := a.Attr("href")
		if !exists || href == "" {
			return
		}
		name := strings.TrimSpace(s.Find(".search_info .name").Text())
		if name == "" {
			return
		}
		cover, _ := s.Find(".search_avatar img").Attr("src")
		mangas = append(mangas, models.Manga{
			ID:       href,
			Title:    name,
			CoverURL: cover,
		})
	})

	return mangas, nil
}

func (p *TruyenQQProvider) FetchChapters(mangaURL string) ([]models.Chapter, error) {
	endpoint := resolveURL(p.baseURL, mangaURL)

	resp, err := httpGet(p.httpClient, endpoint, truyenqqBrowserUA)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	chapters := make([]models.Chapter, 0)
	doc.Find(".works-chapter-list .works-chapter-item a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		title := strings.TrimSpace(s.Text())
		if !exists || title == "" {
			return
		}
		chapters = append(chapters, models.Chapter{
			ID:    resolveURL(p.baseURL, strings.TrimSpace(href)),
			Title: title,
		})
	})

	reverseChapters(chapters)
	return chapters, nil
}

func (p *TruyenQQProvider) FetchPages(chapterID string) ([]string, error) {
	endpoint := resolveURL(p.baseURL, chapterID)

	resp, err := httpGet(p.httpClient, endpoint, truyenqqBrowserUA)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	urls := make([]string, 0)
	doc.Find(".page-chapter img").Each(func(i int, s *goquery.Selection) {
		src, ok := imageSrc(s)
		if !ok || src == "" {
			return
		}
		urls = append(urls, src)
	})

	return urls, nil
}
