package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/KabosuNeko/Futon/internal/models"
)

const otruyenBaseURL = "https://otruyenapi.com/v1/api"

type OTruyenProvider struct{}

func NewOTruyenProvider() *OTruyenProvider {
	return &OTruyenProvider{}
}

func (o *OTruyenProvider) Name() string {
	return "OTruyen"
}

type otruyenListResponse struct {
	Data struct {
		AppDomainCDNImage string             `json:"APP_DOMAIN_CDN_IMAGE"`
		Items             []otruyenMangaItem `json:"items"`
	} `json:"data"`
}

func otruyenList(resp *http.Response) ([]models.Manga, error) {
	defer resp.Body.Close()

	var result otruyenListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	cdnDomain := result.Data.AppDomainCDNImage
	mangas := make([]models.Manga, 0, len(result.Data.Items))
	for _, item := range result.Data.Items {
		mangas = append(mangas, item.toManga(cdnDomain))
	}
	return mangas, nil
}

func (o *OTruyenProvider) Search(query string) ([]models.Manga, error) {
	endpoint := fmt.Sprintf("%s/tim-kiem?keyword=%s", otruyenBaseURL, url.QueryEscape(query))

	resp, err := httpGet(http.DefaultClient, endpoint, defaultUserAgent)
	if err != nil {
		return nil, err
	}
	return otruyenList(resp)
}

func (o *OTruyenProvider) FetchLatest(page int) ([]models.Manga, error) {
	if page < 1 {
		page = 1
	}
	endpoint := fmt.Sprintf("%s/danh-sach/truyen-moi?page=%d", otruyenBaseURL, page)

	resp, err := httpGet(http.DefaultClient, endpoint, defaultUserAgent)
	if err != nil {
		return nil, err
	}
	return otruyenList(resp)
}

func (o *OTruyenProvider) Filter(opts FilterOptions) ([]models.Manga, error) {
	page := opts.Page
	if page < 1 {
		page = 1
	}
	var endpoint string
	if opts.Genre > 0 {
		genres := []string{"", "action", "adventure", "comedy", "drama", "fantasy", "isekai", "romance", "shounen", "manhwa"}
		if opts.Genre < len(genres) {
			endpoint = fmt.Sprintf("%s/the-loai/%s?page=%d", otruyenBaseURL, genres[opts.Genre], page)
		}
	} else if opts.Status == 1 {
		endpoint = fmt.Sprintf("%s/danh-sach/dang-phat-hanh?page=%d", otruyenBaseURL, page)
	} else if opts.Status == 2 {
		endpoint = fmt.Sprintf("%s/danh-sach/hoan-thanh?page=%d", otruyenBaseURL, page)
	} else if opts.Sort == 1 || opts.Sort == 2 {
		endpoint = fmt.Sprintf("%s/danh-sach/dang-phat-hanh?page=%d", otruyenBaseURL, page)
	} else {
		endpoint = fmt.Sprintf("%s/danh-sach/truyen-moi?page=%d", otruyenBaseURL, page)
	}

	resp, err := httpGet(http.DefaultClient, endpoint, defaultUserAgent)
	if err != nil {
		return o.FetchLatest(page)
	}
	return otruyenList(resp)
}

func (o *OTruyenProvider) FetchChapters(slug string) ([]models.Chapter, error) {
	endpoint := fmt.Sprintf("%s/truyen-tranh/%s", otruyenBaseURL, url.PathEscape(slug))

	resp, err := httpGet(http.DefaultClient, endpoint, defaultUserAgent)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Item otruyenMangaDetail `json:"item"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	if len(result.Data.Item.Chapters) == 0 {
		return []models.Chapter{}, nil
	}

	// OTruyen always puts chapter data in the first server group; 1+ are mirrors.
	serverData := result.Data.Item.Chapters[0].ServerData
	chapters := make([]models.Chapter, 0, len(serverData))
	for _, ch := range serverData {
		chapters = append(chapters, ch.toChapter())
	}
	return chapters, nil
}

func (o *OTruyenProvider) FetchPages(chapterEndpoint string) ([]string, error) {
	resp, err := httpGet(http.DefaultClient, chapterEndpoint, defaultUserAgent)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			DomainCDN string `json:"domain_cdn"`
			Item      struct {
				ChapterPath  string                `json:"chapter_path"`
				ChapterImage []otruyenChapterImage `json:"chapter_image"`
			} `json:"item"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	domain := strings.TrimRight(result.Data.DomainCDN, "/")
	path := strings.Trim(result.Data.Item.ChapterPath, "/")

	images := result.Data.Item.ChapterImage
	sort.Slice(images, func(i, j int) bool {
		return images[i].ImagePage < images[j].ImagePage
	})

	urls := make([]string, len(images))
	for i, img := range images {
		urls[i] = fmt.Sprintf("%s/%s/%s", domain, path, img.ImageFile)
	}
	return urls, nil
}

type otruyenCategory struct {
	Name string `json:"name"`
}

type otruyenMangaItem struct {
	Name     string            `json:"name"`
	Slug     string            `json:"slug"`
	ThumbURL string            `json:"thumb_url"`
	Author   []string          `json:"author"`
	Category []otruyenCategory `json:"category"`
	Status   string            `json:"status"`
}

func (item otruyenMangaItem) toManga(cdnDomain string) models.Manga {
	var coverURL string
	if item.ThumbURL != "" {
		if strings.HasPrefix(item.ThumbURL, "http://") || strings.HasPrefix(item.ThumbURL, "https://") {
			coverURL = item.ThumbURL
		} else {
			if cdnDomain == "" {
				cdnDomain = "https://img.otruyenapi.com"
			}
			cdnDomain = strings.TrimRight(cdnDomain, "/")
			coverURL = fmt.Sprintf("%s/uploads/comics/%s", cdnDomain, item.ThumbURL)
		}
	}

	var author string
	if len(item.Author) > 0 {
		author = strings.Join(item.Author, ", ")
	}

	var genres []string
	for _, cat := range item.Category {
		if cat.Name != "" {
			genres = append(genres, cat.Name)
		}
	}

	return models.Manga{
		ID:       item.Slug,
		Title:    item.Name,
		CoverURL: coverURL,
		Author:   author,
		Status:   item.Status,
		Genres:   genres,
	}
}

type otruyenMangaDetail struct {
	Chapters []otruyenChapterServer `json:"chapters"`
}

type otruyenChapterServer struct {
	ServerData []otruyenChapterData `json:"server_data"`
}

type otruyenChapterData struct {
	ChapterName   string `json:"chapter_name"`
	ChapterTitle  string `json:"chapter_title"`
	ChapterAPIURL string `json:"chapter_api_data"`
}

func (ch otruyenChapterData) toChapter() models.Chapter {
	return models.Chapter{
		ID:     ch.ChapterAPIURL,
		Number: ch.ChapterName,
		Title:  ch.ChapterTitle,
	}
}

type otruyenChapterImage struct {
	ImagePage int    `json:"image_page"`
	ImageFile string `json:"image_file"`
}
