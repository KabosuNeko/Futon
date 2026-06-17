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

func otruyenGet(endpoint string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("tạo request: %w", err)
	}
	req.Header.Set("User-Agent", "Futon-App/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gọi API: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("API trả về HTTP %d", resp.StatusCode)
	}
	return resp, nil
}

func (o *OTruyenProvider) Search(query string) ([]models.Manga, error) {
	endpoint := fmt.Sprintf("%s/tim-kiem?keyword=%s", otruyenBaseURL, url.QueryEscape(query))

	resp, err := otruyenGet(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			Items []otruyenMangaItem `json:"items"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	mangas := make([]models.Manga, 0, len(result.Data.Items))
	for _, item := range result.Data.Items {
		mangas = append(mangas, item.toManga())
	}
	return mangas, nil
}

func (o *OTruyenProvider) FetchChapters(slug string) ([]models.Chapter, error) {
	endpoint := fmt.Sprintf("%s/truyen-tranh/%s", otruyenBaseURL, url.PathEscape(slug))

	resp, err := otruyenGet(endpoint)
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

	// Lấy server_data từ server đầu tiên.
	serverData := result.Data.Item.Chapters[0].ServerData
	chapters := make([]models.Chapter, 0, len(serverData))
	for _, ch := range serverData {
		chapters = append(chapters, ch.toChapter())
	}
	return chapters, nil
}

func (o *OTruyenProvider) FetchPages(chapterEndpoint string) ([]string, error) {
	resp, err := otruyenGet(chapterEndpoint)
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

type otruyenMangaItem struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (item otruyenMangaItem) toManga() models.Manga {
	return models.Manga{
		ID:    item.Slug,
		Title: item.Name,
	}
}

type otruyenMangaDetail struct {
	Name     string                 `json:"name"`
	Slug     string                 `json:"slug"`
	Chapters []otruyenChapterServer `json:"chapters"`
}

type otruyenChapterServer struct {
	ServerName string               `json:"server_name"`
	ServerData []otruyenChapterData `json:"server_data"`
}

type otruyenChapterData struct {
	Filename      string `json:"filename"`
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
