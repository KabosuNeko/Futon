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

const baotangtruyenBaseURL = "https://www.baotangtruyen.vip"

var baotangtruyenBrowserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

type BaoTangTruyenProvider struct {
	baseURL    string
	httpClient *http.Client
}

func NewBaoTangTruyenProvider() *BaoTangTruyenProvider {
	return &BaoTangTruyenProvider{
		baseURL:    strings.TrimRight(baotangtruyenBaseURL, "/"),
		httpClient: &http.Client{},
	}
}

func (p *BaoTangTruyenProvider) Name() string {
	return "BaoTangTruyen"
}

func (p *BaoTangTruyenProvider) baotangtruyenGet(endpoint string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("tạo request: %w", err)
	}
	req.Header.Set("User-Agent", baotangtruyenBrowserUA)
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
