package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/KabosuNeko/Futon/internal/models"
)

type MangaDexProvider struct {
	mu         sync.RWMutex
	lang       string
	httpClient *http.Client
}

// mangadexBaseURL is overridable in tests (see mangadex_test.go).
var mangadexBaseURL = "https://api.mangadex.org"

// mangadexSearchPageLimit is the number of results fetched per search request
// (the API maximum).
const mangadexSearchPageLimit = 100

// mangadexSearchMaxPages caps search pagination. A broad query ("the") can
// match thousands of titles; walking them all is slow, trips the rate limit,
// and MangaDex rejects offset+limit > 10000 outright.
const mangadexSearchMaxPages = 3

// mangadexGenreTags are MangaDex tag UUIDs in FilterGenres order.
var mangadexGenreTags = []string{
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

func NewMangaDexProvider() *MangaDexProvider {
	return &MangaDexProvider{lang: "vi", httpClient: &http.Client{Timeout: providerTimeout}}
}

func (m *MangaDexProvider) Name() string {
	return "MangaDex"
}

// mangadexImageUserAgent identifies the client to MangaDex@Home; MangaDex
// asks for a descriptive UA and serves placeholders when a Referer is sent,
// so page downloads send no Referer.
const mangadexImageUserAgent = "Futon-App/1.0 (https://github.com/KabosuNeko/Futon)"

func (m *MangaDexProvider) imageUserAgent() string { return mangadexImageUserAgent }
func (m *MangaDexProvider) coverReferer() string   { return "https://mangadex.org" }

func (m *MangaDexProvider) SetLang(lang string) {
	m.mu.Lock()
	m.lang = lang
	m.mu.Unlock()
}

func (m *MangaDexProvider) Lang() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lang
}

func mangadexList(resp *http.Response) ([]models.Manga, int, error) {
	defer resp.Body.Close()

	var result mangadexListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, fmt.Errorf("parse JSON: %w", err)
	}

	all := make([]models.Manga, 0, len(result.Data))
	for _, data := range result.Data {
		all = append(all, data.toManga())
	}
	return all, result.Total, nil
}

// listParams returns the query shared by every manga list request. Only
// titles with chapters in the configured language are listed, since the
// chapter feed is filtered by that language too.
func (m *MangaDexProvider) listParams(limit, offset int) url.Values {
	params := url.Values{}
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("offset", fmt.Sprintf("%d", offset))
	params.Add("includes[]", "cover_art")
	params.Add("includes[]", "author")
	params.Add("includes[]", "artist")
	params.Add("availableTranslatedLanguage[]", m.Lang())
	return params
}

func (m *MangaDexProvider) list(ctx context.Context, params url.Values) ([]models.Manga, int, error) {
	endpoint := fmt.Sprintf("%s/manga?%s", mangadexBaseURL, params.Encode())
	resp, err := httpGet(ctx, m.httpClient, endpoint, defaultUserAgent)
	if err != nil {
		return nil, 0, err
	}
	return mangadexList(resp)
}

func (m *MangaDexProvider) Search(ctx context.Context, query string) ([]models.Manga, error) {
	var all []models.Manga

	for page := 0; page < mangadexSearchMaxPages; page++ {
		params := m.listParams(mangadexSearchPageLimit, page*mangadexSearchPageLimit)
		params.Set("title", query)

		batch, total, err := m.list(ctx, params)
		if err != nil {
			// Later pages are a bonus; keep what the first ones returned.
			if len(all) > 0 && ctx.Err() == nil {
				return all, nil
			}
			return nil, err
		}
		all = append(all, batch...)

		if len(batch) < mangadexSearchPageLimit {
			break
		}
		if total > 0 && len(all) >= total {
			break
		}
	}

	return all, nil
}

func (m *MangaDexProvider) FetchLatest(ctx context.Context, page int) ([]models.Manga, error) {
	return m.Filter(ctx, FilterOptions{Page: page})
}

func (m *MangaDexProvider) Filter(ctx context.Context, opts FilterOptions) ([]models.Manga, error) {
	page := max(1, opts.Page)
	params := m.listParams(20, (page-1)*20)

	switch opts.Status {
	case 1:
		params.Add("status[]", "ongoing")
	case 2:
		params.Add("status[]", "completed")
	}

	switch opts.Sort {
	case 1:
		params.Set("order[followedCount]", "desc")
	case 2:
		params.Set("order[rating]", "desc")
	case 3:
		params.Set("order[title]", "asc")
	default:
		params.Set("order[updatedAt]", "desc")
	}

	tag, err := genreSlug(mangadexGenreTags, opts.Genre)
	if err != nil {
		return nil, err
	}
	if tag != "" {
		params.Add("includedTags[]", tag)
	}

	mangas, _, err := m.list(ctx, params)
	return mangas, err
}

func (m *MangaDexProvider) FetchChapters(ctx context.Context, mangaID string) ([]models.Chapter, error) {
	const limit = 500
	var all []models.Chapter
	seen := make(map[string]bool)
	offset := 0
	lang := m.Lang()

	for {
		endpoint := fmt.Sprintf(
			"%s/manga/%s/feed?translatedLanguage[]=%s&order[chapter]=asc&includeExternalUrl=0&limit=%d&offset=%d",
			mangadexBaseURL, url.PathEscape(mangaID), url.QueryEscape(lang), limit, offset,
		)

		resp, err := httpGet(ctx, m.httpClient, endpoint, defaultUserAgent)
		if err != nil {
			return nil, err
		}

		var result mangadexFeedResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		resp.Body.Close()

		for _, data := range result.Data {
			// External chapters live on another site and have no pages here.
			if data.Attributes.ExternalURL != "" {
				continue
			}
			// Several scanlation groups often upload the same chapter; keep
			// the first so the list has one entry per chapter number.
			if n := data.Attributes.Chapter; n != "" {
				if seen[n] {
					continue
				}
				seen[n] = true
			}
			all = append(all, data.toChapter())
		}

		if len(result.Data) < limit {
			break
		}
		offset += limit
		if result.Total > 0 && offset >= result.Total {
			break
		}
	}

	return all, nil
}

func (m *MangaDexProvider) FetchPages(ctx context.Context, chapterID string) ([]string, error) {
	endpoint := fmt.Sprintf(
		"%s/at-home/server/%s",
		mangadexBaseURL,
		url.PathEscape(chapterID),
	)

	resp, err := httpGet(ctx, m.httpClient, endpoint, defaultUserAgent)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result mangadexAtHomeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	urls := make([]string, len(result.Chapter.Data))
	for i, filename := range result.Chapter.Data {
		urls[i] = fmt.Sprintf("%s/data/%s/%s", result.BaseURL, result.Chapter.Hash, filename)
	}
	return urls, nil
}

// ---- MangaDex API response types ----

type mangadexListResponse struct {
	Data  []mangadexManga `json:"data"`
	Total int             `json:"total"`
}

type mangadexManga struct {
	ID            string                  `json:"id"`
	Attributes    mangadexMangaAttributes `json:"attributes"`
	Relationships []mangadexRelationship  `json:"relationships"`
}

type mangadexRelationship struct {
	Type       string                 `json:"type"`
	Attributes *mangadexRelAttributes `json:"attributes"`
}

type mangadexRelAttributes struct {
	FileName string `json:"fileName"`
	Name     string `json:"name"`
}

type mangadexTag struct {
	Attributes struct {
		Name map[string]string `json:"name"`
	} `json:"attributes"`
}

type mangadexMangaAttributes struct {
	Title  map[string]string `json:"title"`
	Year   *int              `json:"year"`
	Status string            `json:"status"`
	Tags   []mangadexTag     `json:"tags"`
}

func (d mangadexManga) toManga() models.Manga {
	var coverURL string
	var author string
	for _, rel := range d.Relationships {
		if rel.Type == "cover_art" && rel.Attributes != nil && rel.Attributes.FileName != "" {
			coverURL = "https://uploads.mangadex.org/covers/" + d.ID + "/" + rel.Attributes.FileName + ".256.jpg"
		}
		if (rel.Type == "author" || rel.Type == "artist") && rel.Attributes != nil && rel.Attributes.Name != "" {
			if author == "" {
				author = rel.Attributes.Name
			}
		}
	}

	var year string
	if d.Attributes.Year != nil && *d.Attributes.Year > 0 {
		year = strconv.Itoa(*d.Attributes.Year)
	}

	var genres []string
	for _, tag := range d.Attributes.Tags {
		if name, ok := tag.Attributes.Name["en"]; ok && name != "" {
			genres = append(genres, name)
		}
	}

	return models.Manga{
		ID:       d.ID,
		Title:    mangadexTitle(d.Attributes.Title),
		CoverURL: coverURL,
		Author:   author,
		Year:     year,
		Status:   d.Attributes.Status,
		Genres:   genres,
	}
}

func mangadexTitle(titles map[string]string) string {
	if t, ok := titles["vi"]; ok && t != "" {
		return t
	}
	if t, ok := titles["en"]; ok && t != "" {
		return t
	}
	if t, ok := titles["ja-ro"]; ok && t != "" {
		return t
	}
	for _, t := range titles {
		if t != "" {
			return t
		}
	}
	return "Không rõ tiêu đề"
}

type mangadexFeedResponse struct {
	Data  []mangadexChapter `json:"data"`
	Total int               `json:"total"`
}

type mangadexChapter struct {
	ID         string                    `json:"id"`
	Attributes mangadexChapterAttributes `json:"attributes"`
}

type mangadexChapterAttributes struct {
	Chapter     string `json:"chapter"`
	Title       string `json:"title"`
	ExternalURL string `json:"externalUrl"`
}

func (d mangadexChapter) toChapter() models.Chapter {
	return models.Chapter{
		ID:     d.ID,
		Number: d.Attributes.Chapter,
		Title:  d.Attributes.Title,
	}
}

type mangadexAtHomeResponse struct {
	BaseURL string                `json:"baseUrl"`
	Chapter mangadexAtHomeChapter `json:"chapter"`
}

type mangadexAtHomeChapter struct {
	Hash string   `json:"hash"`
	Data []string `json:"data"`
}
