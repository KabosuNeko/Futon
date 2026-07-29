package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/KabosuNeko/Futon/internal/models"
)

const foxtruyenBaseURL = "https://foxtruyen2.com"

var foxtruyenBrowserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

type FoxTruyenProvider struct {
	baseURL    string
	httpClient *http.Client
}

func NewFoxTruyenProvider() *FoxTruyenProvider {
	return &FoxTruyenProvider{
		baseURL:    strings.TrimRight(foxtruyenBaseURL, "/"),
		httpClient: &http.Client{},
	}
}

func (p *FoxTruyenProvider) Name() string {
	return "FoxTruyen"
}

func (p *FoxTruyenProvider) foxtruyenGet(endpoint string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("tạo request: %w", err)
	}
	req.Header.Set("User-Agent", foxtruyenBrowserUA)
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

func (p *FoxTruyenProvider) resolveURL(path string) string {
	if strings.HasPrefix(path, "http") {
		return path
	}
	base := strings.TrimRight(p.baseURL, "/")
	path = strings.TrimPrefix(path, "/")
	return base + "/" + path
}

func foxImgAttr(s *goquery.Selection) (string, bool) {
	for _, attr := range []string{"data-original", "data-src", "data-lazy-src"} {
		if v, exists := s.Attr(attr); exists && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), true
		}
	}
	src, exists := s.Attr("src")
	if exists && strings.TrimSpace(src) != "" {
		return strings.TrimSpace(src), true
	}
	return "", false
}

func (p *FoxTruyenProvider) Search(keyword string) ([]models.Manga, error) {
	endpoint := p.baseURL + "/tim-kiem?q=" + url.QueryEscape(keyword)

	resp, err := p.foxtruyenGet(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var mangas []models.Manga
	doc.Find(".row.list_item_home > .item_home").Each(func(i int, s *goquery.Selection) {
		mangaLink := s.Find("a.thumbblock")
		href, exists := mangaLink.Attr("href")
		if !exists || href == "" {
			return
		}

		title := strings.TrimSpace(s.Find("a.book_name").Text())
		if title == "" {
			return
		}

		cover := ""
		s.Find(".image-cover img").Each(func(_ int, img *goquery.Selection) {
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

func (p *FoxTruyenProvider) FetchChapters(mangaURL string) ([]models.Chapter, error) {
	endpoint := p.resolveURL(mangaURL)

	resp, err := p.foxtruyenGet(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var chapters []models.Chapter
	doc.Find("ul.fx-chap-list li.fx-chap-item a.fx-chap-item__name").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		title := strings.TrimSpace(s.Text())
		if !exists || href == "" || title == "" {
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

func (p *FoxTruyenProvider) FetchPages(chapterID string) ([]string, error) {
	endpoint := p.resolveURL(chapterID)

	resp, err := p.foxtruyenGet(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var urls []string
	doc.Find("div.content_detail_manga img").Each(func(i int, s *goquery.Selection) {
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
