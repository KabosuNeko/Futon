package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

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

func (p *BaoTangTruyenProvider) baotangtruyenGet(endpoint string) (*http.Response, error) {
	client := ensureClient(p.httpClient)
	var lastErr error
	for attempt := 0; attempt <= providerMaxRetries; attempt++ {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("tạo request: %w", err)
		}
		req.Header.Set("User-Agent", baotangtruyenBrowserUA)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("gọi HTTP: %w", err)
			if attempt < providerMaxRetries {
				time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
				continue
			}
			return nil, lastErr
		}
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			if attempt < providerMaxRetries {
				time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
				continue
			}
			return nil, lastErr
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return resp, nil
	}
	return nil, lastErr
}

func (p *BaoTangTruyenProvider) resolveURL(path string) string {
	if strings.HasPrefix(path, "http") {
		return path
	}
	base := strings.TrimRight(p.baseURL, "/")
	path = strings.TrimPrefix(path, "/")
	return base + "/" + path
}

func (p *BaoTangTruyenProvider) Search(keyword string) ([]models.Manga, error) {
	endpoint := p.baseURL + "/tim-truyen?keyword=" + url.QueryEscape(keyword)

	resp, err := p.baotangtruyenGet(endpoint)
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
			if v, ok := foxImgAttr(img); ok {
				cover = v
			}
		})

		mangas = append(mangas, models.Manga{
			ID:       href,
			Title:    title,
			CoverURL: cover,
		})
	})

	if mangas == nil {
		mangas = []models.Manga{}
	}
	return mangas, nil
}

func (p *BaoTangTruyenProvider) FetchLatest(page int) ([]models.Manga, error) {
	if page < 1 {
		page = 1
	}
	endpoint := p.baseURL + fmt.Sprintf("/truyen-moi?page=%d", page)

	resp, err := p.baotangtruyenGet(endpoint)
	if err != nil {
		resp, err = p.baotangtruyenGet(p.baseURL)
		if err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

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
			if v, ok := foxImgAttr(img); ok && cover == "" {
				cover = v
			}
		})

		mangas = append(mangas, models.Manga{
			ID:       href,
			Title:    title,
			CoverURL: cover,
		})
	})

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

	resp, err := p.baotangtruyenGet(endpoint)
	if err != nil {
		return p.FetchLatest(page)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

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
			if v, ok := foxImgAttr(img); ok && cover == "" {
				cover = v
			}
		})

		mangas = append(mangas, models.Manga{
			ID:       href,
			Title:    title,
			CoverURL: cover,
		})
	})

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
	endpoint := p.resolveURL(mangaURL)

	resp, err := p.baotangtruyenGet(endpoint)
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
	for i, j := 0, len(chapters)-1; i < j; i, j = i+1, j-1 {
		chapters[i], chapters[j] = chapters[j], chapters[i]
	}

	if chapters == nil {
		chapters = []models.Chapter{}
	}
	return chapters, nil
}

func (p *BaoTangTruyenProvider) FetchPages(chapterID string) ([]string, error) {
	endpoint := p.resolveURL(chapterID)

	resp, err := p.baotangtruyenGet(endpoint)
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
		src, ok := foxImgAttr(s)
		if !ok || src == "" {
			return
		}
		urls = append(urls, src)
	})

	if urls == nil {
		urls = []string{}
	}
	return urls, nil
}
