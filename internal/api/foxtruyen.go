package api

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/PuerkitoBio/goquery"
)

const foxtruyenBaseURL = "https://foxtruyen2.com"

var foxtruyenBrowserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36"

type FoxTruyenProvider struct {
	baseURL    string
	httpClient *http.Client
}

func NewFoxTruyenProvider() *FoxTruyenProvider {
	return &FoxTruyenProvider{
		baseURL:    strings.TrimRight(foxtruyenBaseURL, "/"),
		httpClient: &http.Client{Timeout: providerTimeout},
	}
}

func (p *FoxTruyenProvider) Name() string {
	return "FoxTruyen"
}

func parseFoxList(doc *goquery.Document) []models.Manga {
	var mangas []models.Manga
	doc.Find(".row.list_item_home > .item_home, .list_item_home .item_home, .list-stories .item").Each(func(i int, s *goquery.Selection) {
		mangaLink := s.Find("a.thumbblock")
		if mangaLink.Length() == 0 {
			mangaLink = s.Find("a.book_name")
		}
		href, exists := mangaLink.Attr("href")
		if !exists || href == "" {
			return
		}

		title := strings.TrimSpace(s.Find("a.book_name").Text())
		if title == "" {
			title = strings.TrimSpace(s.Find("h3 a").Text())
		}
		if title == "" {
			return
		}

		cover := ""
		s.Find(".image-cover img, img").Each(func(_ int, img *goquery.Selection) {
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

func (p *FoxTruyenProvider) Search(keyword string) ([]models.Manga, error) {
	endpoint := p.baseURL + "/tim-kiem?q=" + url.QueryEscape(keyword)

	resp, err := httpGet(p.httpClient, endpoint, foxtruyenBrowserUA)
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

func (p *FoxTruyenProvider) FetchLatest(page int) ([]models.Manga, error) {
	if page < 1 {
		page = 1
	}
	endpoint := p.baseURL + fmt.Sprintf("/danh-sach/truyen-moi?page=%d", page)

	resp, err := httpGet(p.httpClient, endpoint, foxtruyenBrowserUA)
	if err != nil {
		resp, err = httpGet(p.httpClient, p.baseURL, foxtruyenBrowserUA)
		if err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	mangas := parseFoxList(doc)
	if len(mangas) == 0 {
		return p.Search("")
	}
	return mangas, nil
}

func (p *FoxTruyenProvider) Filter(opts FilterOptions) ([]models.Manga, error) {
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
		endpoint = p.baseURL + fmt.Sprintf("/danh-sach/hoan-thanh?page=%d", page)
	} else if opts.Sort == 1 || opts.Sort == 2 {
		endpoint = p.baseURL + fmt.Sprintf("/danh-sach/truyen-hot?page=%d", page)
	} else {
		return p.FetchLatest(page)
	}

	resp, err := httpGet(p.httpClient, endpoint, foxtruyenBrowserUA)
	if err != nil {
		return p.FetchLatest(page)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	mangas := parseFoxList(doc)
	if len(mangas) == 0 {
		return p.FetchLatest(page)
	}
	return mangas, nil
}

func (p *FoxTruyenProvider) FetchChapters(mangaURL string) ([]models.Chapter, error) {
	endpoint := resolveURL(p.baseURL, mangaURL)

	resp, err := httpGet(p.httpClient, endpoint, foxtruyenBrowserUA)
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

	slices.Reverse(chapters)
	return chapters, nil
}

func (p *FoxTruyenProvider) FetchPages(chapterID string) ([]string, error) {
	endpoint := resolveURL(p.baseURL, chapterID)

	resp, err := httpGet(p.httpClient, endpoint, foxtruyenBrowserUA)
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
		src, ok := imageSrc(s)
		if !ok || src == "" {
			return
		}
		urls = append(urls, src)
	})

	return urls, nil
}
