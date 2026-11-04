package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/PuerkitoBio/goquery"
)

// cuutruyenBaseURL is cuutruyen.moe, a copy of CuuTruyen that serves plain
// HTML and unscrambled, lightly compressed images (CuuTruyen itself
// scrambles page images and is DNS-blocked on some networks).
var cuutruyenBaseURL = "https://cuutruyen.moe"

// cuutruyenPassword is the site's default access password. The site sometimes
// puts pages behind a Livewire "enter secret" form that accepts it.
const cuutruyenPassword = "5"

// cuutruyenGenres are the site's genre IDs in FilterGenres order. The site
// has English and Vietnamese tags for most genres; each ID here is the one
// with more titles.
var cuutruyenGenres = []string{"", "33", "39", "34", "20", "38", "30", "17", "12", "16"}

type CuuTruyenProvider struct {
	httpClient *http.Client
}

func NewCuuTruyenProvider() *CuuTruyenProvider {
	// The password gate is unlocked per session, so cookies must persist.
	jar, _ := cookiejar.New(nil)
	return &CuuTruyenProvider{httpClient: &http.Client{Timeout: providerTimeout, Jar: jar}}
}

func (p *CuuTruyenProvider) Name() string { return "CuuTruyen" }

var (
	cuutruyenBgImage      = regexp.MustCompile(`background-image:\s*url\(['"]?(.*?)['"]?\)`)
	cuutruyenWireData     = regexp.MustCompile(`wire:initial-data="([^"]+)"`)
	cuutruyenLivewireCSRF = regexp.MustCompile(`livewire_token\s*=\s*'([^']+)'`)
)

// fetchDoc GETs a site page, unlocking the password gate if it appears.
func (p *CuuTruyenProvider) fetchDoc(ctx context.Context, endpoint string) (*goquery.Document, error) {
	body, err := p.fetchBody(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	if isCuuTruyenGate(body) {
		if err := p.submitPassword(ctx, body); err != nil {
			return nil, fmt.Errorf("mở khóa cổng mật khẩu: %w", err)
		}
		if body, err = p.fetchBody(ctx, endpoint); err != nil {
			return nil, err
		}
		if isCuuTruyenGate(body) {
			return nil, fmt.Errorf("cổng mật khẩu không chấp nhận mật khẩu mặc định")
		}
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}
	return doc, nil
}

func (p *CuuTruyenProvider) fetchBody(ctx context.Context, endpoint string) ([]byte, error) {
	resp, err := httpGet(ctx, p.httpClient, endpoint, browserUserAgent)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func isCuuTruyenGate(body []byte) bool {
	return bytes.Contains(body, []byte("wire:initial-data")) && bytes.Contains(body, []byte("enter-secret"))
}

// submitPassword fills the Livewire password form the way the browser does:
// sync the input, then call submit, echoing the component's state back.
func (p *CuuTruyenProvider) submitPassword(ctx context.Context, gate []byte) error {
	wire := cuutruyenWireData.FindSubmatch(gate)
	csrf := cuutruyenLivewireCSRF.FindSubmatch(gate)
	if wire == nil || csrf == nil {
		return fmt.Errorf("không tìm thấy dữ liệu form")
	}
	var component struct {
		Fingerprint json.RawMessage `json:"fingerprint"`
		ServerMemo  json.RawMessage `json:"serverMemo"`
	}
	if err := json.Unmarshal([]byte(html.UnescapeString(string(wire[1]))), &component); err != nil {
		return fmt.Errorf("parse wire:initial-data: %w", err)
	}
	payload, err := json.Marshal(map[string]any{
		"fingerprint": component.Fingerprint,
		"serverMemo":  component.ServerMemo,
		"updates": []map[string]any{
			{"type": "syncInput", "payload": map[string]any{"id": "s1", "name": "password", "value": cuutruyenPassword}},
			{"type": "callMethod", "payload": map[string]any{"id": "c1", "method": "submit", "params": []any{}}},
		},
	})
	if err != nil {
		return err
	}

	resp, err := httpDo(ctx, p.httpClient, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, cuutruyenBaseURL+"/livewire/message/enter-secret", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", browserUserAgent)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/html, application/xhtml+xml")
		req.Header.Set("X-CSRF-TOKEN", string(csrf[1]))
		req.Header.Set("X-Livewire", "true")
		req.Header.Set("Referer", cuutruyenBaseURL+"/")
		return req, nil
	})
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func parseCuuTruyenList(doc *goquery.Document) []models.Manga {
	var mangas []models.Manga
	doc.Find("div.manga-vertical").Each(func(_ int, s *goquery.Selection) {
		link := s.Find("div.p-2 a").First()
		href, ok := link.Attr("href")
		title := strings.TrimSpace(link.Text())
		if !ok || href == "" || title == "" {
			return
		}
		cover := ""
		if style, ok := s.Find("div.cover").First().Attr("style"); ok {
			if m := cuutruyenBgImage.FindStringSubmatch(style); m != nil {
				cover = m[1]
			}
		}
		mangas = append(mangas, models.Manga{ID: cuutruyenPath(href), Title: title, CoverURL: cover})
	})
	return mangas
}

// cuutruyenPath keeps only the path of a site link, the provider's ID format.
func cuutruyenPath(href string) string {
	if u, err := url.Parse(strings.TrimSpace(href)); err == nil && u.Host != "" {
		return u.Path
	}
	return strings.TrimSpace(href)
}

func (p *CuuTruyenProvider) search(ctx context.Context, q url.Values) ([]models.Manga, error) {
	doc, err := p.fetchDoc(ctx, cuutruyenBaseURL+"/tim-kiem?"+q.Encode())
	if err != nil {
		return nil, err
	}
	return parseCuuTruyenList(doc), nil
}

// Search filters by name; the site's own "keyword" parameter currently makes
// its server answer HTTP 500 for every query.
func (p *CuuTruyenProvider) Search(ctx context.Context, keyword string) ([]models.Manga, error) {
	return p.search(ctx, url.Values{"filter[name]": {keyword}, "page": {"1"}})
}

func (p *CuuTruyenProvider) FetchLatest(ctx context.Context, page int) ([]models.Manga, error) {
	page = max(1, page)
	mangas, err := p.search(ctx, url.Values{
		"sort":           {"-updated_at"},
		"filter[status]": {"2,1"},
		"page":           {fmt.Sprint(page)},
	})
	if err != nil {
		return nil, err
	}
	if len(mangas) == 0 && page == 1 {
		return nil, errEmptyList
	}
	return mangas, nil
}

// Filter maps onto /tim-kiem, which combines genre, status and sort. The site
// has no rating sort; "rating" uses most viewed.
func (p *CuuTruyenProvider) Filter(ctx context.Context, opts FilterOptions) ([]models.Manga, error) {
	genre, err := genreSlug(cuutruyenGenres, opts.Genre)
	if err != nil {
		return nil, err
	}
	q := url.Values{"page": {fmt.Sprint(max(1, opts.Page))}}
	switch opts.Status {
	case 1:
		q.Set("filter[status]", "2") // the site's code for ongoing
	case 2:
		q.Set("filter[status]", "1") // and for completed
	default:
		q.Set("filter[status]", "2,1")
	}
	switch opts.Sort {
	case 1, 2:
		q.Set("sort", "-views")
	case 3:
		q.Set("sort", "name")
	default:
		q.Set("sort", "-updated_at")
	}
	if genre != "" {
		q.Set("filter[accept_genres]", genre)
	}
	return p.search(ctx, q)
}

func (p *CuuTruyenProvider) FetchChapters(ctx context.Context, mangaID string) ([]models.Chapter, error) {
	doc, err := p.fetchDoc(ctx, resolveURL(cuutruyenBaseURL, mangaID))
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var chapters []models.Chapter
	doc.Find("ul.overflow-y-auto a[href*='/truyen/']").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		title := strings.TrimSpace(s.Find("div.grow span.text-ellipsis, div.grow span.truncate").First().Text())
		id := cuutruyenPath(href)
		if id == "" || title == "" || seen[id] {
			return
		}
		seen[id] = true
		chapters = append(chapters, models.Chapter{ID: id, Title: title})
	})

	slices.Reverse(chapters) // the site lists newest first
	return chapters, nil
}

func (p *CuuTruyenProvider) FetchPages(ctx context.Context, chapterID string) ([]string, error) {
	doc, err := p.fetchDoc(ctx, resolveURL(cuutruyenBaseURL, chapterID))
	if err != nil {
		return nil, err
	}
	var urls []string
	doc.Find("div.text-center > img.max-w-full").Each(func(_ int, s *goquery.Selection) {
		if src, ok := imageSrc(s); ok {
			urls = append(urls, resolveURL(cuutruyenBaseURL, src))
		}
	})
	return urls, nil
}

func (p *CuuTruyenProvider) coverReferer() string { return cuutruyenBaseURL + "/" }

func (p *CuuTruyenProvider) pageReferer(chapterID string) string {
	return resolveURL(cuutruyenBaseURL, chapterID)
}
