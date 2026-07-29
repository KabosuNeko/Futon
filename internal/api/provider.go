package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/KabosuNeko/Futon/internal/models"
)

type MangaSearchResultMsg struct {
	Manga []models.Manga
	Err   error
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

// MangaProvider is the contract every source must sign.
type MangaProvider interface {
	Name() string
	Search(keyword string) ([]models.Manga, error)
	FetchChapters(mangaID string) ([]models.Chapter, error)
	FetchPages(chapterID string) ([]string, error)
}

func resolveURL(baseURL, path string) string {
	if strings.HasPrefix(path, "http") {
		return path
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimPrefix(path, "/")
}

func httpGet(client *http.Client, endpoint, userAgent string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("tạo request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "vi-VN,vi;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gọi HTTP: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return resp, nil
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

func reverseChapters(chapters []models.Chapter) {
	for i, j := 0, len(chapters)-1; i < j; i, j = i+1, j-1 {
		chapters[i], chapters[j] = chapters[j], chapters[i]
	}
}
