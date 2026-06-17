package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/KabosuNeko/Futon/internal/models"
)

type MangaDexProvider struct {
	lang string
}

func NewMangaDexProvider() *MangaDexProvider {
	return &MangaDexProvider{lang: "vi"}
}

func (m *MangaDexProvider) Name() string {
	return "MangaDex"
}

func (m *MangaDexProvider) SetLang(lang string) {
	m.lang = lang
}

func mangadexGet(endpoint string) (*http.Response, error) {
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

func (m *MangaDexProvider) Search(query string) ([]models.Manga, error) {
	endpoint := fmt.Sprintf(
		"https://api.mangadex.org/manga?title=%s&limit=5&includes[]=cover_art",
		url.QueryEscape(query),
	)

	resp, err := mangadexGet(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result models.MangaSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	mangas := make([]models.Manga, len(result.Data))
	for i, data := range result.Data {
		mangas[i] = data.ToManga()
	}
	return mangas, nil
}

func (m *MangaDexProvider) FetchChapters(mangaID string) ([]models.Chapter, error) {
	const limit = 500
	var all []models.Chapter
	offset := 0

	for {
		endpoint := fmt.Sprintf(
			"https://api.mangadex.org/manga/%s/feed?translatedLanguage[]=%s&order[chapter]=asc&limit=%d&offset=%d",
			url.PathEscape(mangaID), m.lang, limit, offset,
		)

		resp, err := mangadexGet(endpoint)
		if err != nil {
			return nil, err
		}

		var result models.ChapterFeedResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		resp.Body.Close()

		for _, data := range result.Data {
			all = append(all, data.ToChapter())
		}

		if len(result.Data) < limit {
			break
		}
		if result.Total > 0 && len(all) >= result.Total {
			break
		}
		offset += limit
	}

	return all, nil
}

func (m *MangaDexProvider) FetchPages(chapterID string) ([]string, error) {
	endpoint := fmt.Sprintf(
		"https://api.mangadex.org/at-home/server/%s",
		url.PathEscape(chapterID),
	)

	resp, err := mangadexGet(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result models.AtHomeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	urls := make([]string, len(result.Chapter.Data))
	for i, filename := range result.Chapter.Data {
		urls[i] = fmt.Sprintf("%s/data/%s/%s", result.BaseURL, result.Chapter.Hash, filename)
	}
	return urls, nil
}
