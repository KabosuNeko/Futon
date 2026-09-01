package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/KabosuNeko/Futon/internal/models"
)

type MangaDexProvider struct {
	lang string
}

// mangadexBaseURL is overridable in tests (see mangadex_test.go).
var mangadexBaseURL = "https://api.mangadex.org"

// mangadexSearchPageLimit is the number of results fetched per search request.
// MangaDex API caps this at 100 per request; pagination walks the rest via offset.
const mangadexSearchPageLimit = 100

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
	client := ensureClient(http.DefaultClient)
	var lastErr error
	for attempt := 0; attempt <= providerMaxRetries; attempt++ {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("tạo request: %w", err)
		}
		req.Header.Set("User-Agent", defaultUserAgent)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("gọi API: %w", err)
			if attempt < providerMaxRetries {
				time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
				continue
			}
			return nil, lastErr
		}
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			resp.Body.Close()
			lastErr = fmt.Errorf("API trả về HTTP %d", resp.StatusCode)
			if attempt < providerMaxRetries {
				time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
				continue
			}
			return nil, lastErr
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("API trả về HTTP %d", resp.StatusCode)
		}
		return resp, nil
	}
	return nil, lastErr
}

func (m *MangaDexProvider) Search(query string) ([]models.Manga, error) {
	var all []models.Manga
	offset := 0

	for {
		endpoint := fmt.Sprintf(
			"%s/manga?title=%s&limit=%d&offset=%d&includes[]=cover_art&includes[]=author&includes[]=artist",
			mangadexBaseURL, url.QueryEscape(query), mangadexSearchPageLimit, offset,
		)

		resp, err := mangadexGet(endpoint)
		if err != nil {
			return nil, err
		}

		var result models.MangaSearchResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		resp.Body.Close()

		for _, data := range result.Data {
			all = append(all, data.ToManga())
		}

		if len(result.Data) < mangadexSearchPageLimit {
			break
		}
		if result.Total > 0 && len(all) >= result.Total {
			break
		}
		offset += mangadexSearchPageLimit
	}

	return all, nil
}

func (m *MangaDexProvider) FetchLatest(page int) ([]models.Manga, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * 20
	endpoint := fmt.Sprintf(
		"%s/manga?limit=20&offset=%d&order[updatedAt]=desc&includes[]=cover_art&includes[]=author&includes[]=artist",
		mangadexBaseURL, offset,
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

	all := make([]models.Manga, 0, len(result.Data))
	for _, data := range result.Data {
		all = append(all, data.ToManga())
	}
	return all, nil
}

func (m *MangaDexProvider) Filter(opts FilterOptions) ([]models.Manga, error) {
	page := opts.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * 20

	params := url.Values{}
	params.Set("limit", "20")
	params.Set("offset", fmt.Sprintf("%d", offset))
	params.Add("includes[]", "cover_art")
	params.Add("includes[]", "author")
	params.Add("includes[]", "artist")

	switch opts.Status {
	case 1:
		params.Add("status[]", "ongoing")
	case 2:
		params.Add("status[]", "completed")
	}

	switch opts.Sort {
	case 0:
		params.Set("order[updatedAt]", "desc")
	case 1:
		params.Set("order[followedCount]", "desc")
	case 2:
		params.Set("order[rating]", "desc")
	case 3:
		params.Set("order[title]", "asc")
	default:
		params.Set("order[updatedAt]", "desc")
	}

	genreUUIDs := []string{
		"",
		"391b0423-d847-456f-aa08-113e2d8472f3", // Action
		"87cc87cd-a395-47af-b27a-93258283bbc6", // Adventure
		"4d32cc48-9f00-4cca-9b5a-a839f0764984", // Comedy
		"b9af3a63-f058-46de-a9a0-e0c13906197a", // Drama
		"cdc58593-87dd-415e-bbc0-2ec27bf404cc", // Fantasy
		"ace04997-f6bd-436e-b261-779182193d3d", // Isekai
		"423e2eae-a7a2-4a8b-ac03-a8351462d71d", // Romance
		"2bd2e8d0-f146-434a-9b51-fc9ff2c5fe6a", // Shounen
		"e197df38-d0e7-43b5-9b09-2842d0c326dd", // Webtoons / Manhwa
	}
	if opts.Genre > 0 && opts.Genre < len(genreUUIDs) && genreUUIDs[opts.Genre] != "" {
		params.Add("includedTags[]", genreUUIDs[opts.Genre])
	}

	endpoint := fmt.Sprintf("%s/manga?%s", mangadexBaseURL, params.Encode())
	resp, err := mangadexGet(endpoint)
	if err != nil {
		return m.FetchLatest(page)
	}
	defer resp.Body.Close()

	var result models.MangaSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	all := make([]models.Manga, 0, len(result.Data))
	for _, data := range result.Data {
		all = append(all, data.ToManga())
	}
	return all, nil
}

func (m *MangaDexProvider) FetchChapters(mangaID string) ([]models.Chapter, error) {
	// MangaDex pagination: fetch up to 500 per page, loop until we have them all.
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
