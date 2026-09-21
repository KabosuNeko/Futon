package api

import (
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

var baotangtruyenBrowserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36"

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

func parseBaoTangList(doc *goquery.Document) []models.Manga {
	var mangas []models.Manga
	doc.Find("div.items > div.row > div.item, .list-stories .item, .items .item").Each(func(i int, s *goquery.Selection) {
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

func (p *BaoTangTruyenProvider) Search(keyword string) ([]models.Manga, error) {
	endpoint := p.baseURL + "/tim-truyen?keyword=" + url.QueryEscape(keyword)

	resp, err := httpGet(p.httpClient, endpoint, baotangtruyenBrowserUA)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var mangas []models.Manga
	doc.Find("div.items > div.row > div.item").Each(func(i int, s *goquery.Selection) {
		linkEl := s.Find("figcaption h3 a")
		href, exists := linkEl.Attr("href")
		if !exists || href == "" {
			return
		}

		title := strings.TrimSpace(linkEl.Text())
		if title == "" {
			return
		}

		cover := ""
		s.Find(".image a img").Each(func(_ int, img *goquery.Selection) {
			if v, ok := imageSrc(img); ok {
				cover = v
			}
		})

		mangas = append(mangas, models.Manga{
			ID:       href,
			Title:    title,
			CoverURL: cover,
		})
	})

	return mangas, nil
}

func (p *BaoTangTruyenProvider) FetchLatest(page int) ([]models.Manga, error) {
	if page < 1 {
		page = 1
	}
	endpoint := p.baseURL + fmt.Sprintf("/truyen-moi?page=%d", page)

	resp, err := httpGet(p.httpClient, endpoint, baotangtruyenBrowserUA)
	if err != nil {
		resp, err = httpGet(p.httpClient, p.baseURL, baotangtruyenBrowserUA)
		if err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	mangas := parseBaoTangList(doc)
	if len(mangas) == 0 {
		return p.Search("")
	}
	return mangas, nil
}

func (p *BaoTangTruyenProvider) Filter(opts FilterOptions) ([]models.Manga, error) {
	page := opts.Page
	if page < 1 {
		page = 1
	}

	var endpoint string
	if opts.Genre > 0 {
		genres := []string{"", "action", "adventure", "comedy", "drama", "fantasy", "isekai", "romance", "shounen", "manhwa"}
		if opts.Genre < len(genres) {
			endpoint = p.baseURL + fmt.Sprintf("/the-loai/%s?page=%d", genres[opts.Genre], page)
		}
	} else if opts.Status == 2 {
		endpoint = p.baseURL + fmt.Sprintf("/truyen-hoan-thanh?page=%d", page)
	} else if opts.Sort == 1 || opts.Sort == 2 {
		endpoint = p.baseURL + fmt.Sprintf("/truyen-hot?page=%d", page)
	} else {
		return p.FetchLatest(page)
	}

	resp, err := httpGet(p.httpClient, endpoint, baotangtruyenBrowserUA)
	if err != nil {
		return p.FetchLatest(page)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	mangas := parseBaoTangList(doc)
	if len(mangas) == 0 {
		return p.FetchLatest(page)
	}
	return mangas, nil
}

type btChapterItem struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type btItemList struct {
	ItemListElement []btChapterItem `json:"itemListElement"`
}

func (p *BaoTangTruyenProvider) FetchChapters(mangaURL string) ([]models.Chapter, error) {
	endpoint := resolveURL(p.baseURL, mangaURL)

	resp, err := httpGet(p.httpClient, endpoint, baotangtruyenBrowserUA)
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
				ID:    strings.TrimSpace(item.URL),
				Title: strings.TrimSpace(item.Name),
			})
		}
	})

	// JSON-LD is newest-first (position 1 = newest), reverse so chapter-1 is first
	slices.Reverse(chapters)
	return chapters, nil
}

func (p *BaoTangTruyenProvider) FetchPages(chapterID string) ([]string, error) {
	endpoint := resolveURL(p.baseURL, chapterID)

	resp, err := httpGet(p.httpClient, endpoint, baotangtruyenBrowserUA)
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
