package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/PuerkitoBio/goquery"
)

type MangaSearchResultMsg struct {
	Manga          []models.Manga
	Err            error
	ProviderCounts map[string]int
	ProviderErrors map[string]string
}

type ChapterListMsg struct {
	Chapters []models.Chapter
	Err      error
}

type ChapterImagesMsg struct {
	URLs []string
	Err  error
}

const defaultUserAgent = "Futon-App/1.0"

const providerTimeout = 10 * time.Second
const providerMaxRetries = 2

type FilterOptions struct {
	Status int // 0: All, 1: Ongoing, 2: Completed
	Sort   int // 0: Updated, 1: Popular, 2: Rating, 3: Alphabetical
	Genre  int // 0: All, 1: Action, 2: Adventure, 3: Comedy, 4: Drama, 5: Fantasy, 6: Isekai, 7: Romance, 8: Shounen, 9: Webtoons / Manhwa
	Page   int
}

// MangaProvider is the contract every source must sign.
type MangaProvider interface {
	Name() string
	Search(keyword string) ([]models.Manga, error)
	FetchLatest(page int) ([]models.Manga, error)
	Filter(opts FilterOptions) ([]models.Manga, error)
	FetchChapters(mangaID string) ([]models.Chapter, error)
	FetchPages(chapterID string) ([]string, error)
}

func resolveURL(baseURL, path string) string {
	if strings.HasPrefix(path, "http") {
		return path
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimPrefix(path, "/")
}

func ensureClient(client *http.Client) *http.Client {
	if client == nil {
		return &http.Client{Timeout: providerTimeout}
	}
	if client.Timeout == 0 {
		c := *client
		c.Timeout = providerTimeout
		return &c
	}
	return client
}

func httpGet(client *http.Client, endpoint, userAgent string) (*http.Response, error) {
	client = ensureClient(client)
	var lastErr error
	for attempt := 0; attempt <= providerMaxRetries; attempt++ {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("tạo request: %w", err)
		}
		req.Header.Set("User-Agent", userAgent)
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

// Sites lazy-load images — check data attributes before falling back to src.
func imageSrc(s *goquery.Selection) (string, bool) {
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

// FetchCoverBytes downloads cover image bytes with headers tailored for manga providers.
func FetchCoverBytes(coverURL, provider string) ([]byte, error) {
	if coverURL == "" {
		return nil, fmt.Errorf("cover URL trống")
	}

	client := &http.Client{Timeout: providerTimeout}
	req, err := http.NewRequest(http.MethodGet, coverURL, nil)
	if err != nil {
		return nil, fmt.Errorf("tạo request tải ảnh: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")

	pLower := strings.ToLower(provider)
	switch {
	case strings.Contains(pLower, "truyenqq") || strings.Contains(coverURL, "hinhhinh.com"):
		req.Header.Set("Referer", "https://truyenqqko.com")
	case strings.Contains(pLower, "foxtruyen") || strings.Contains(coverURL, "hinhgg.com") || strings.Contains(coverURL, "foxtruyen"):
		req.Header.Set("Referer", "https://foxtruyen2.com")
	case strings.Contains(pLower, "baotangtruyen") || strings.Contains(coverURL, "baotangtruyen"):
		req.Header.Set("Referer", "https://www.baotangtruyen.vip")
	case strings.Contains(pLower, "mangadex") || strings.Contains(coverURL, "mangadex.org"):
		req.Header.Set("Referer", "https://mangadex.org")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tải ảnh: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d khi tải ảnh", resp.StatusCode)
	}

	var buf strings.Builder
	_ = buf
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("đọc dữ liệu ảnh: %w", err)
	}
	return data, nil
}

func reverseChapters(chapters []models.Chapter) {
	for i, j := 0, len(chapters)-1; i < j; i, j = i+1, j-1 {
		chapters[i], chapters[j] = chapters[j], chapters[i]
	}
}
