package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/PuerkitoBio/goquery"
)

const (
	truyenqqPrimaryURL = "https://truyenqqko.com"
	// truyenqqFallbackURL is TruyenGG, a mirror serving the same catalog
	// under the same paths (it used to be listed as a separate source).
	truyenqqFallbackURL = "https://truyenggvn.com"
)

func NewTruyenQQProvider() *TruyenQQProvider {
	return newTruyenQQProvider(truyenqqPrimaryURL, truyenqqFallbackURL)
}

// truyenqqGenres are genre slugs in FilterGenres order (checked against the
// site menus of TruyenQQ and its TruyenGG mirror, which share them).
var truyenqqGenres = []string{"", "action-26", "adventure-27", "comedy-28", "drama-29", "fantasy-30", "isekai-85", "romance-36", "shounen-31", "manhwa-49"}

// TruyenQQProvider scrapes TruyenQQ. It probes its mirrors on first use and
// sticks to the first one that answers.
type TruyenQQProvider struct {
	baseURL    string
	fallbacks  []string
	httpClient *http.Client
	domainOnce sync.Once
}

func newTruyenQQProvider(baseURL string, fallbacks ...string) *TruyenQQProvider {
	return &TruyenQQProvider{
		baseURL:    baseURL,
		fallbacks:  fallbacks,
		httpClient: &http.Client{Timeout: providerTimeout},
	}
}

// resolveDomain picks a reachable mirror on first use. Probing in the
// constructor blocked startup (and tests) on network calls with no timeout.
func (p *TruyenQQProvider) resolveDomain() {
	p.domainOnce.Do(func() {
		domains := append([]string{p.baseURL}, p.fallbacks...)
		for _, domain := range domains {
			if p.tryDomain(domain) {
				return
			}
		}
	})
}

// probeTimeout bounds each mirror probe: offline, the old 10s per mirror
// stalled the first TruyenQQ request for 20s. The probe deliberately ignores
// the caller's context: a cancelled search must not settle the mirror choice.
const probeTimeout = 3 * time.Second

func (p *TruyenQQProvider) tryDomain(domain string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, domain, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", browserUserAgent)
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

func (p *TruyenQQProvider) Search(ctx context.Context, keyword string) ([]models.Manga, error) {
	p.resolveDomain()
	endpoint := resolveURL(p.baseURL, "/frontend/search/search")

	form := url.Values{
		"search": {keyword},
		"type":   {"0"},
	}

	resp, err := httpDo(ctx, p.httpClient, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, fmt.Errorf("tạo request: %w", err)
		}
		req.Header.Set("User-Agent", browserUserAgent)
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
			ID:       TruyenQQPath(href),
			Title:    name,
			CoverURL: cover,
		})
	})

	return mangas, nil
}

func parseTruyenQQList(doc *goquery.Document) []models.Manga {
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
			ID:       TruyenQQPath(href),
			Title:    name,
			CoverURL: cover,
		})
	})
	return mangas
}

// TruyenQQPath reduces a manga/chapter link to its path without ".html".
// IDs stay valid on every mirror, and the same manga gets the same ID whether
// it came from search (absolute links) or a list page (relative links).
// Also used to migrate entries saved as absolute URLs of either mirror.
func TruyenQQPath(href string) string {
	href = strings.TrimSpace(href)
	if u, err := url.Parse(href); err == nil && u.Host != "" {
		href = u.Path
	}
	return strings.TrimSuffix(href, ".html")
}

func (p *TruyenQQProvider) coverReferer() string {
	p.resolveDomain()
	return p.baseURL
}

// pageReferer resolves a chapter path against the mirror currently in use.
func (p *TruyenQQProvider) pageReferer(chapterID string) string {
	p.resolveDomain()
	return resolveURL(p.baseURL, chapterID)
}

// listURL builds a paginated list URL: /path[/trang-N][?query]. Both mirrors
// accept paths without ".html" (TruyenQQ even drops the query when
// redirecting a ".html" path).
func (p *TruyenQQProvider) listURL(path string, page int, query url.Values) string {
	u := path
	if page > 1 {
		u += fmt.Sprintf("/trang-%d", page)
	}
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return resolveURL(p.baseURL, u)
}

func (p *TruyenQQProvider) fetchList(ctx context.Context, endpoint string) ([]models.Manga, error) {
	resp, err := httpDo(ctx, p.httpClient, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("tạo request: %w", err)
		}
		req.Header.Set("User-Agent", browserUserAgent)
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
	return parseTruyenQQList(doc), nil
}

func (p *TruyenQQProvider) FetchLatest(ctx context.Context, page int) ([]models.Manga, error) {
	p.resolveDomain()
	page = max(1, page)
	mangas, err := p.fetchList(ctx, p.listURL("/truyen-moi-cap-nhat", page, nil))
	if err != nil {
		return nil, err
	}
	if len(mangas) == 0 && page == 1 {
		return nil, errEmptyList
	}
	return mangas, nil
}

// Filter uses the genre page when a genre is chosen, since it accepts status
// and sort parameters. Without a genre the sites only offer fixed lists, so
// status wins over sort, and unsupported combinations show the latest list.
func (p *TruyenQQProvider) Filter(ctx context.Context, opts FilterOptions) ([]models.Manga, error) {
	p.resolveDomain()
	page := max(1, opts.Page)
	genre, err := genreSlug(truyenqqGenres, opts.Genre)
	if err != nil {
		return nil, err
	}

	var endpoint string
	switch {
	case genre != "":
		q := url.Values{}
		switch opts.Status {
		case 1:
			q.Set("status", "0")
		case 2:
			q.Set("status", "2")
		}
		switch opts.Sort {
		case 1, 2:
			q.Set("sort", "4") // most viewed
		default:
			q.Set("sort", "2") // recently updated
		}
		endpoint = p.listURL("/the-loai/"+genre, page, q)
	case opts.Status == 2:
		endpoint = p.listURL("/truyen-hoan-thanh", page, nil)
	case opts.Sort == 1:
		endpoint = p.listURL("/top-thang", page, nil)
	case opts.Sort == 2:
		endpoint = p.listURL("/truyen-yeu-thich", page, nil)
	default:
		return p.FetchLatest(ctx, page)
	}
	return p.fetchList(ctx, endpoint)
}

func (p *TruyenQQProvider) FetchChapters(ctx context.Context, mangaURL string) ([]models.Chapter, error) {
	p.resolveDomain()
	endpoint := resolveURL(p.baseURL, mangaURL)

	resp, err := httpGet(ctx, p.httpClient, endpoint, browserUserAgent)
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
			ID:    TruyenQQPath(href),
			Title: title,
		})
	})

	slices.Reverse(chapters)
	return chapters, nil
}

func (p *TruyenQQProvider) lookupCover(ctx context.Context, mangaID string) (string, error) {
	p.resolveDomain()
	resp, err := httpGet(ctx, p.httpClient, resolveURL(p.baseURL, mangaID), browserUserAgent)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("parse HTML: %w", err)
	}
	return pageCover(doc, ""), nil
}

func (p *TruyenQQProvider) FetchPages(ctx context.Context, chapterID string) ([]string, error) {
	p.resolveDomain()
	endpoint := resolveURL(p.baseURL, chapterID)

	resp, err := httpGet(ctx, p.httpClient, endpoint, browserUserAgent)
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
