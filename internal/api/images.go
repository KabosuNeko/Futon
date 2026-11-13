package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Providers describe the headers their image CDNs expect by implementing any
// of these optional interfaces; the image functions below apply them.
type (
	// coverReferrer names the Referer for cover images.
	coverReferrer interface{ coverReferer() string }
	// pageReferrer names the Referer for a chapter's page images, typically
	// the chapter's absolute URL (chapter IDs may be paths).
	pageReferrer interface {
		pageReferer(chapterID string) string
	}
	// imageAgent overrides the User-Agent for page images.
	imageAgent interface{ imageUserAgent() string }
)

const defaultImageUserAgent = "Futon-App/1.0"

// imageClient bounds image downloads; a stalled CDN must not hang a reader
// page or an export.
var imageClient = &http.Client{Timeout: 15 * time.Second}

// imageMaxAttempts covers the transient CDN failures (resets, 5xx, 429) that
// otherwise turn one flaky page into a failed chapter.
const imageMaxAttempts = 3

// imageRetryDelay is the base backoff between attempts; tests shorten it.
var imageRetryDelay = 500 * time.Millisecond

// maxImageBytes caps one downloaded image; real pages are a few MB at most.
const maxImageBytes = 32 << 20

// maxConcurrentPageDownloads bounds parallel page downloads for one chapter.
const maxConcurrentPageDownloads = 4

// pageReferer returns the Referer for chapterID's page images ("" if none).
func pageRefererFor(p MangaProvider, chapterID string) string {
	if r, ok := p.(pageReferrer); ok {
		return r.pageReferer(chapterID)
	}
	if strings.HasPrefix(chapterID, "http://") || strings.HasPrefix(chapterID, "https://") {
		return chapterID
	}
	return ""
}

// FetchCover downloads a cover image with the provider's cover headers.
func FetchCover(ctx context.Context, p MangaProvider, coverURL string) ([]byte, error) {
	if coverURL == "" {
		return nil, fmt.Errorf("cover URL trống")
	}
	referer := ""
	if r, ok := p.(coverReferrer); ok {
		referer = r.coverReferer()
	}
	data, _, err := fetchImage(ctx, coverURL, browserUserAgent, referer)
	return data, err
}

// DownloadPage fetches one page image of chapterID with the headers the
// provider's CDN expects, retrying transient failures. p may be nil.
func DownloadPage(ctx context.Context, p MangaProvider, chapterID, imageURL string) ([]byte, error) {
	userAgent := defaultImageUserAgent
	if a, ok := p.(imageAgent); ok {
		userAgent = a.imageUserAgent()
	}
	referer := ""
	if p != nil {
		referer = pageRefererFor(p, chapterID)
	}

	var lastErr error
	for attempt := 0; attempt < imageMaxAttempts; attempt++ {
		if attempt > 0 {
			if err := sleepCtx(ctx, time.Duration(attempt)*imageRetryDelay); err != nil {
				return nil, err
			}
		}
		data, retry, err := fetchImage(ctx, imageURL, userAgent, referer)
		if err == nil {
			return data, nil
		}
		if !retry || ctx.Err() != nil {
			return nil, err
		}
		lastErr = err
	}
	return nil, lastErr
}

// fetchImage performs one image request. retry reports whether the failure
// is transient.
func fetchImage(ctx context.Context, rawURL, userAgent, referer string) (data []byte, retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := imageClient.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("tải ảnh: %w", unwrapURLError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		transient := resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests
		return nil, transient, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	// One byte past the cap tells an oversized image from one exactly at it.
	data, err = io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, true, err
	}
	if len(data) > maxImageBytes {
		return nil, false, fmt.Errorf("ảnh quá lớn (> %d MB)", maxImageBytes>>20)
	}
	if len(data) == 0 {
		return nil, true, fmt.Errorf("ảnh rỗng")
	}
	return data, false, nil
}

// DownloadChapterPages downloads every page of a chapter a few at a time,
// preserving order. It fails if any page cannot be downloaded, naming it.
func DownloadChapterPages(ctx context.Context, p MangaProvider, chapterID string, urls []string) ([][]byte, error) {
	if len(urls) == 0 {
		return nil, fmt.Errorf("danh sách URL ảnh trống")
	}
	pageCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		images   = make([][]byte, len(urls))
		sem      = make(chan struct{}, maxConcurrentPageDownloads)
		wg       sync.WaitGroup
		failOnce sync.Once
		firstErr error
	)
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-pageCtx.Done():
				return
			}
			data, err := DownloadPage(pageCtx, p, chapterID, u)
			if err != nil {
				// The chapter fails anyway; record the first cause and stop the rest.
				failOnce.Do(func() {
					firstErr = fmt.Errorf("tải trang %d/%d: %w", i+1, len(urls), err)
					cancel()
				})
				return
			}
			images[i] = data
		}(i, u)
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return images, nil
}
