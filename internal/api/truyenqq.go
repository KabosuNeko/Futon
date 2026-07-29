package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/KabosuNeko/Futon/internal/models"
)

const (
	truyenqqPrimaryURL = "https://truyenqqko.com"
	truyenqqFallbackURL = "https://metruyenqq.net"
)

var truyenqqBrowserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

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
	req, err := http.NewRequest(http.MethodGet, domain, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", truyenqqBrowserUA)
	resp, err := p.httpClient.Do(req)
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

func (p *TruyenQQProvider) truyenqqGet(endpoint string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("tạo request: %w", err)
	}
	req.Header.Set("User-Agent", truyenqqBrowserUA)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gọi HTTP: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return resp, nil
}

func (p *TruyenQQProvider) resolveURL(path string) string {
	if strings.HasPrefix(path, "http") {
		return path
	}
	base := strings.TrimRight(p.baseURL, "/")
	path = strings.TrimPrefix(path, "/")
	return base + "/" + path
}

func (p *TruyenQQProvider) Search(keyword string) ([]models.Manga, error) {
	endpoint := p.resolveURL("/frontend/search/search")

	form := url.Values{
		"search": {keyword},
		"type":   {"0"},
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("tạo request: %w", err)
	}
	req.Header.Set("User-Agent", truyenqqBrowserUA)
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

	var mangas []models.Manga
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

	if mangas == nil {
		mangas = []models.Manga{}
	}
	return mangas, nil
}

func (p *TruyenQQProvider) FetchChapters(mangaURL string) ([]models.Chapter, error) {
	endpoint := p.resolveURL(mangaURL)

	resp, err := p.truyenqqGet(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var chapters []models.Chapter
	doc.Find(".works-chapter-list .works-chapter-item a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		title := strings.TrimSpace(s.Text())
		if !exists || title == "" {
			return
		}
		chapters = append(chapters, models.Chapter{
			ID:    p.resolveURL(strings.TrimSpace(href)),
			Title: title,
		})
	})

	for i, j := 0, len(chapters)-1; i < j; i, j = i+1, j-1 {
		chapters[i], chapters[j] = chapters[j], chapters[i]
	}
	if chapters == nil {
		chapters = []models.Chapter{}
	}
	return chapters, nil
}

func (p *TruyenQQProvider) FetchPages(chapterID string) ([]string, error) {
	endpoint := p.resolveURL(chapterID)

	resp, err := p.truyenqqGet(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var urls []string
	doc.Find(".page-chapter img").Each(func(i int, s *goquery.Selection) {
		src, exists := srcAttr(s)
		if !exists || src == "" {
			return
		}
		urls = append(urls, strings.TrimSpace(src))
	})

	if urls == nil {
		urls = []string{}
	}
	return urls, nil
}

func srcAttr(s *goquery.Selection) (string, bool) {
	src, exists := s.Attr("src")
	if exists && src != "" {
		return src, true
	}
	src, exists = s.Attr("data-original")
	if exists && src != "" {
		return src, true
	}
	return "", false
}
