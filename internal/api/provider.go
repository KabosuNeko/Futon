package api

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/PuerkitoBio/goquery"
)

const defaultUserAgent = "Futon-App/1.0"

// browserUserAgent is sent to the HTML providers and image CDNs that reject
// non-browser clients.
const browserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36"

const providerTimeout = 10 * time.Second
const providerMaxRetries = 2

type FilterOptions struct {
	Status int // 0: All, 1: Ongoing, 2: Completed
	Sort   int // 0: Updated, 1: Popular, 2: Rating, 3: Alphabetical
	Genre  int // index into FilterGenres; 0 means all genres
	Page   int
}

// FilterGenres lists the genres the filter screen offers. Index 0 is "all";
// every provider keeps a slug table with exactly this length and order.
var FilterGenres = []string{"Tất cả", "Action", "Adventure", "Comedy", "Drama", "Fantasy", "Isekai", "Romance", "Shounen", "Webtoons / Manhwa"}

// genreSlug returns the provider slug for opts.Genre, or "" for "all".
func genreSlug(slugs []string, genre int) (string, error) {
	if genre <= 0 {
		return "", nil
	}
	if genre >= len(slugs) || slugs[genre] == "" {
		return "", fmt.Errorf("thể loại không hỗ trợ: %d", genre)
	}
	return slugs[genre], nil
}

// MangaProvider is implemented by every manga source. The context cancels a
// request that the UI no longer needs (e.g. a superseded search).
type MangaProvider interface {
	Name() string
	Search(ctx context.Context, keyword string) ([]models.Manga, error)
	FetchLatest(ctx context.Context, page int) ([]models.Manga, error)
	Filter(ctx context.Context, opts FilterOptions) ([]models.Manga, error)
	FetchChapters(ctx context.Context, mangaID string) ([]models.Chapter, error)
	FetchPages(ctx context.Context, chapterID string) ([]string, error)
}

// errEmptyList reports a list page that parsed to nothing. A latest-updates
// page is never legitimately empty, so this means the markup changed.
var errEmptyList = fmt.Errorf("không đọc được danh sách truyện (trang có thể đã đổi giao diện)")

func resolveURL(baseURL, path string) string {
	if strings.HasPrefix(path, "http") {
		return path
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimPrefix(path, "/")
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
