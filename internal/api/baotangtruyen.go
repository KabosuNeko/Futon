package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/PuerkitoBio/goquery"
)

const baotangtruyenBaseURL = "https://www.baotangtruyen.vip"

// baotangGenres are genre slugs under /tim-truyen/ in FilterGenres order
// (the site has no "isekai" tag; "chuyen-sinh" is its equivalent).
var baotangGenres = []string{"", "action", "adventure", "comedy", "drama", "fantasy", "chuyen-sinh", "romance", "shounen", "manhwa"}

const baotangListSelector = "div.items > div.row > div.item, .list-stories .item, .items .item"

type BaoTangTruyenProvider struct {
	baseURL    string
	httpClient *http.Client
}

func NewBaoTangTruyenProvider() *BaoTangTruyenProvider {
	return &BaoTangTruyenProvider{
		baseURL:    strings.TrimRight(baotangtruyenBaseURL, "/"),
		httpClient: &http.Client{Timeout: providerTimeout},
	}
}

func (p *BaoTangTruyenProvider) Name() string {
	return "BaoTangTruyen"
}

func (p *BaoTangTruyenProvider) coverReferer() string { return p.baseURL }

func (p *BaoTangTruyenProvider) pageReferer(chapterID string) string {
	return resolveURL(p.baseURL, chapterID)
}

func parseBaoTangList(doc *goquery.Document, selector string) []models.Manga {
	var mangas []models.Manga
	doc.Find(selector).Each(func(i int, s *goquery.Selection) {
		linkEl := s.Find("figcaption h3 a, h3 a, a.title").First()
		if linkEl.Length() == 0 {
			linkEl = s.Find("a").First()
		}
		href, exists := linkEl.Attr("href")
		if !exists || href == "" {
			return
		}

		title := strings.TrimSpace(linkEl.Text())
		if title == "" {
			title = strings.TrimSpace(s.Find(".title").Text())
		}
		if title == "" {
			return
		}

		cover := ""
		s.Find(".image a img, img").Each(func(_ int, img *goquery.Selection) {
			if v, ok := imageSrc(img); ok && cover == "" {
				cover = v
			}
		})

		mangas = append(mangas, models.Manga{
			ID:       href,
			Title:    title,
			CoverURL: cover,
		})
	})
	return mangas
}

func (p *BaoTangTruyenProvider) fetchList(ctx context.Context, endpoint, selector string) ([]models.Manga, error) {
	resp, err := httpGet(ctx, p.httpClient, endpoint, browserUserAgent)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}
	return parseBaoTangList(doc, selector), nil
}

func (p *BaoTangTruyenProvider) Search(ctx context.Context, keyword string) ([]models.Manga, error) {
	return p.fetchList(ctx, p.baseURL+"/tim-truyen?keyword="+url.QueryEscape(keyword), "div.items > div.row > div.item")
}

func (p *BaoTangTruyenProvider) FetchLatest(ctx context.Context, page int) ([]models.Manga, error) {
	page = max(1, page)
	mangas, err := p.fetchList(ctx, p.baseURL+fmt.Sprintf("/truyen-moi?page=%d", page), baotangListSelector)
	if err != nil {
		return nil, err
	}
	// Only an empty first page means broken parsing; later pages end the list.
	if len(mangas) == 0 && page == 1 {
		return nil, errEmptyList
	}
	return mangas, nil
}

// Filter uses the site's /tim-truyen[/genre] listing, which accepts genre,
// status and sort together. There is no alphabetical sort; it falls back to
// the default (recently updated) order.
func (p *BaoTangTruyenProvider) Filter(ctx context.Context, opts FilterOptions) ([]models.Manga, error) {
	genre, err := genreSlug(baotangGenres, opts.Genre)
	if err != nil {
		return nil, err
	}

	path := "/tim-truyen"
	if genre != "" {
		path += "/" + genre
	}

	q := url.Values{}
	switch opts.Status {
	case 1:
		q.Set("status", "1")
	case 2:
		q.Set("status", "2")
	default:
		q.Set("status", "-1")
	}
	switch opts.Sort {
	case 1:
		q.Set("sort", "10") // top all-time views
	case 2:
		q.Set("sort", "20") // most liked
	default:
		q.Set("sort", "0")
	}
	q.Set("page", fmt.Sprintf("%d", max(1, opts.Page)))

	return p.fetchList(ctx, p.baseURL+path+"?"+q.Encode(), baotangListSelector)
}

type btChapterItem struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type btItemList struct {
	ItemListElement []btChapterItem `json:"itemListElement"`
}

func (p *BaoTangTruyenProvider) FetchChapters(ctx context.Context, mangaURL string) ([]models.Chapter, error) {
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

	var chapters []models.Chapter
	doc.Find("script[type=\"application/ld+json\"]").Each(func(i int, s *goquery.Selection) {
		if len(chapters) > 0 {
			return
		}
		content := strings.TrimSpace(s.Text())
		if !strings.Contains(content, "ItemList") {
			return
		}

		var itemList btItemList
		if err := json.Unmarshal([]byte(content), &itemList); err != nil {
			return
		}

		for _, item := range itemList.ItemListElement {
			if item.Name == "" || item.URL == "" {
				continue
			}
			chapters = append(chapters, models.Chapter{
				ID:    resolveURL(p.baseURL, strings.TrimSpace(item.URL)),
				Title: strings.TrimSpace(item.Name),
			})
		}
	})

	// JSON-LD is newest-first (position 1 = newest), reverse so chapter-1 is first
	slices.Reverse(chapters)
	return chapters, nil
}

func (p *BaoTangTruyenProvider) FetchPages(ctx context.Context, chapterID string) ([]string, error) {
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

	var urls []string
	doc.Find(".reading-detail .page-chapter img").Each(func(i int, s *goquery.Selection) {
		src, ok := imageSrc(s)
		if !ok || src == "" {
			return
		}
		urls = append(urls, src)
	})

	return urls, nil
}
