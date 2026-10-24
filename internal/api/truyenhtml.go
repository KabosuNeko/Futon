package api

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/PuerkitoBio/goquery"
)

// truyenHTMLBrowserUA is used to pass Cloudflare/bot protection headers.
const truyenHTMLBrowserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36"

// truyenHTMLProvider scrapes sites from the TruyenQQ/TruyenGG family, which
// share the same search, list, chapter, and image markup.
type truyenHTMLProvider struct {
	name       string
	baseURL    string
	fallbacks  []string
	httpClient *http.Client
	domainOnce sync.Once
}

func newTruyenHTMLProvider(name, baseURL string, fallbacks ...string) *truyenHTMLProvider {
	return &truyenHTMLProvider{
		name:       name,
		baseURL:    baseURL,
		fallbacks:  fallbacks,
		httpClient: &http.Client{Timeout: providerTimeout},
	}
}

// resolveDomain picks a reachable mirror on first use. Probing in the
// constructor blocked startup (and tests) on network calls with no timeout.
func (p *truyenHTMLProvider) resolveDomain() {
	p.domainOnce.Do(func() {
		domains := append([]string{p.baseURL}, p.fallbacks...)
		for _, domain := range domains {
			if p.tryDomain(domain) {
				return
			}
		}
	})
}

func (p *truyenHTMLProvider) tryDomain(domain string) bool {
	req, err := http.NewRequest(http.MethodGet, domain, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", truyenHTMLBrowserUA)
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

func (p *truyenHTMLProvider) Name() string {
	return p.name
}

func (p *truyenHTMLProvider) Search(keyword string) ([]models.Manga, error) {
	p.resolveDomain()
	endpoint := resolveURL(p.baseURL, "/frontend/search/search")

	form := url.Values{
		"search": {keyword},
		"type":   {"0"},
	}

	resp, err := httpDo(p.httpClient, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, fmt.Errorf("tạo request: %w", err)
		}
		req.Header.Set("User-Agent", truyenHTMLBrowserUA)
		// The HTML search endpoint requires these exact headers to return HTML.
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("Referer", p.baseURL)
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

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

func parseTruyenHTMLList(doc *goquery.Document) []models.Manga {
	mangas := make([]models.Manga, 0)
	doc.Find(".list_grid li, .list-stories li, .story-item").Each(func(i int, s *goquery.Selection) {
		a := s.Find("h3 a, .book_name a, .title a, a.qtip").First()
		if a.Length() == 0 {
			a = s.Find("a").First()
		}
		href, exists := a.Attr("href")
		if !exists || href == "" {
			return
		}
		name := strings.TrimSpace(a.Text())
		if name == "" {
			name = strings.TrimSpace(s.Find(".book_info h3").Text())
		}
		if name == "" {
			return
		}
		cover, _ := imageSrc(s.Find("img"))
		mangas = append(mangas, models.Manga{
			ID:       href,
			Title:    name,
			CoverURL: cover,
		})
	})
	return mangas
}

func (p *truyenHTMLProvider) FetchLatest(page int) ([]models.Manga, error) {
	p.resolveDomain()
	if page < 1 {
		page = 1
	}
	endpoint := resolveURL(p.baseURL, fmt.Sprintf("/truyen-moi-cap-nhat/trang-%d.html", page))
	if page == 1 {
		endpoint = resolveURL(p.baseURL, "/truyen-moi-cap-nhat.html")
	}

	resp, err := httpDo(p.httpClient, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("tạo request: %w", err)
		}
		req.Header.Set("User-Agent", truyenHTMLBrowserUA)
		req.Header.Set("Referer", p.baseURL)
		return req, nil
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	mangas := parseTruyenHTMLList(doc)
	if len(mangas) == 0 {
		return p.Search("")
	}

	return mangas, nil
}

func (p *truyenHTMLProvider) Filter(opts FilterOptions) ([]models.Manga, error) {
	p.resolveDomain()
	page := opts.Page
	if page < 1 {
		page = 1
	}

	var endpoint string
	if opts.Genre > 0 {
		genres := []string{"", "action-26", "adventure", "comedy-28", "drama-29", "fantasy-30", "isekai", "romance-36", "shounen-38", "manhwa-35"}
		if opts.Genre < len(genres) {
			endpoint = resolveURL(p.baseURL, fmt.Sprintf("/the-loai/%s.html", genres[opts.Genre]))
			if page > 1 {
				endpoint = resolveURL(p.baseURL, fmt.Sprintf("/the-loai/%s/trang-%d.html", genres[opts.Genre], page))
			}
		}
	} else if opts.Status == 2 {
		endpoint = resolveURL(p.baseURL, "/truyen-hoan-thanh.html")
		if page > 1 {
			endpoint = resolveURL(p.baseURL, fmt.Sprintf("/truyen-hoan-thanh/trang-%d.html", page))
		}
	} else if opts.Sort == 1 {
		endpoint = resolveURL(p.baseURL, "/truyen-top-thang.html")
		if page > 1 {
			endpoint = resolveURL(p.baseURL, fmt.Sprintf("/truyen-top-thang/trang-%d.html", page))
		}
	} else if opts.Sort == 2 {
		endpoint = resolveURL(p.baseURL, "/truyen-yeu-thich.html")
		if page > 1 {
			endpoint = resolveURL(p.baseURL, fmt.Sprintf("/truyen-yeu-thich/trang-%d.html", page))
		}
	} else {
		return p.FetchLatest(page)
	}

	resp, err := httpDo(p.httpClient, func() (*http.Request, error) {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", truyenHTMLBrowserUA)
		req.Header.Set("Referer", p.baseURL)
		return req, nil
	})
	if err != nil {
		return p.FetchLatest(page)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	mangas := parseTruyenHTMLList(doc)
	if len(mangas) == 0 {
		return p.FetchLatest(page)
	}
	return mangas, nil
}

func (p *truyenHTMLProvider) FetchChapters(mangaURL string) ([]models.Chapter, error) {
	p.resolveDomain()
	endpoint := resolveURL(p.baseURL, mangaURL)

	resp, err := httpGet(p.httpClient, endpoint, truyenHTMLBrowserUA)
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

	slices.Reverse(chapters)
	return chapters, nil
}

func (p *truyenHTMLProvider) FetchPages(chapterID string) ([]string, error) {
	p.resolveDomain()
	endpoint := resolveURL(p.baseURL, chapterID)

	resp, err := httpGet(p.httpClient, endpoint, truyenHTMLBrowserUA)
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
