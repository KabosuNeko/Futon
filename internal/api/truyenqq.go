package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/KabosuNeko/Futon/internal/models"
)

const (
	truyenqqDiscoveryURL = "https://truyenqq.link/"
	truyenqqFallbackURL  = "https://metruyenqq.net"
)

var truyenqqBrowserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

type TruyenQQProvider struct {
	baseURL    string
	httpClient *http.Client
}

func NewTruyenQQProvider() *TruyenQQProvider {
	p := &TruyenQQProvider{
		httpClient: &http.Client{},
	}
	p.baseURL = p.discoverDomain()
	return p
}

func (p *TruyenQQProvider) discoverDomain() string {
	domain := p.discoverFromLink()
	if domain == "" || !p.checkAPI(domain) {
		if p.checkAPI(truyenqqFallbackURL) {
			return truyenqqFallbackURL
		}
		return truyenqqFallbackURL
	}
	return domain
}

func (p *TruyenQQProvider) discoverFromLink() string {
	req, err := http.NewRequest(http.MethodGet, truyenqqDiscoveryURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", truyenqqBrowserUA)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return ""
	}

	var found string
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		if found != "" {
			return
		}
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		hrefLower := strings.ToLower(href)
		text := strings.ToLower(s.Text())
		if strings.Contains(hrefLower, "metruyenqq") ||
			strings.Contains(hrefLower, "truyenqqto") ||
			strings.Contains(hrefLower, "truyenqqko") ||
			strings.Contains(hrefLower, "truyenqq.") {
			found = strings.TrimRight(href, "/")
		}
		if strings.Contains(text, "metruyenqq") ||
			strings.Contains(text, "truyenqqto") ||
			strings.Contains(text, "truyenqqko") {
			found = strings.TrimRight(href, "/")
		}
	})
	return found
}

func (p *TruyenQQProvider) checkAPI(domain string) bool {
	domain = strings.TrimRight(domain, "/")
	req, err := http.NewRequest(http.MethodGet, domain+"/api/search?q=test", nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", truyenqqBrowserUA)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
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
	endpoint := p.resolveURL("/api/search") + "?q=" + url.QueryEscape(keyword)

	resp, err := p.truyenqqGet(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var apiResult struct {
		Data []struct {
			Name  string `json:"name"`
			Slug  string `json:"slug"`
			Cover string `json:"cover"`
			URL   string `json:"url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	mangas := make([]models.Manga, 0, len(apiResult.Data))
	for _, item := range apiResult.Data {
		mangas = append(mangas, models.Manga{
			ID:       item.URL,
			Title:    item.Name,
			CoverURL: item.Cover,
		})
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
			ID:    strings.TrimSpace(href),
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
	doc.Find(".chapter-images img.chapter-img").Each(func(i int, s *goquery.Selection) {
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
