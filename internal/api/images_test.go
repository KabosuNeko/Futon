package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// testQQ returns a TruyenQQ provider pointed at base, without domain probing.
func testQQ(base string) *TruyenQQProvider {
	p := NewTruyenQQProvider()
	p.baseURL = base
	p.domainOnce.Do(func() {})
	return p
}

func TestFetchCoverUsesProviderReferer(t *testing.T) {
	cases := []struct {
		provider MangaProvider
		want     string
	}{
		{testQQ("https://qq.example"), "https://qq.example"},
		{NewCuuTruyenProvider(), cuutruyenBaseURL + "/"},
		{NewMangaDexProvider(), "https://mangadex.org"},
		{NewBaoTangTruyenProvider(), baotangtruyenBaseURL},
	}
	for _, c := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Referer"); got != c.want {
				t.Errorf("%s cover Referer = %q, want %q", c.provider.Name(), got, c.want)
			}
			if got := r.Header.Get("User-Agent"); got != browserUserAgent {
				t.Errorf("%s cover UA = %q", c.provider.Name(), got)
			}
			w.Write([]byte("cover"))
		}))
		data, err := FetchCover(context.Background(), c.provider, srv.URL)
		srv.Close()
		if err != nil || string(data) != "cover" {
			t.Errorf("%s FetchCover = %q, %v", c.provider.Name(), data, err)
		}
	}
}

func TestImageClientHasTimeout(t *testing.T) {
	if imageClient.Timeout <= 0 {
		t.Fatal("image client has no timeout; a stalled CDN would hang a page or an export")
	}
}

func TestDownloadPageMangaDexHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != mangadexImageUserAgent {
			t.Errorf("User-Agent = %q, want %q", got, mangadexImageUserAgent)
		}
		if got := r.Header.Get("Referer"); got != "" {
			t.Errorf("MangaDex Referer = %q, want none", got)
		}
		w.Write([]byte("image-bytes"))
	}))
	defer srv.Close()

	data, err := DownloadPage(context.Background(), NewMangaDexProvider(), "chapter-uuid", srv.URL)
	if err != nil || string(data) != "image-bytes" {
		t.Fatalf("DownloadPage = %q, %v", data, err)
	}
}

func TestDownloadPageUsesChapterReferer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Referer"); got != "https://qq.example/truyen-tranh/x-chap-1" {
			t.Errorf("Referer = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != defaultImageUserAgent {
			t.Errorf("User-Agent = %q, want %q", got, defaultImageUserAgent)
		}
		w.Write([]byte("image-bytes"))
	}))
	defer srv.Close()

	if _, err := DownloadPage(context.Background(), testQQ("https://qq.example"), "/truyen-tranh/x-chap-1", srv.URL); err != nil {
		t.Fatalf("DownloadPage error: %v", err)
	}
}

func TestDownloadPageWithoutProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "" {
			t.Errorf("unexpected Referer %q", r.Header.Get("Referer"))
		}
		w.Write([]byte("x"))
	}))
	defer srv.Close()
	if _, err := DownloadPage(context.Background(), nil, "c", srv.URL); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadPageRetriesTransientErrors(t *testing.T) {
	old := imageRetryDelay
	imageRetryDelay = time.Millisecond
	defer func() { imageRetryDelay = old }()

	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("image-bytes"))
	}))
	defer srv.Close()

	if _, err := DownloadPage(context.Background(), nil, "", srv.URL); err != nil {
		t.Fatalf("DownloadPage error after transient failures: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestDownloadPageDoesNotRetryClientErrors(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	if _, err := DownloadPage(context.Background(), nil, "", srv.URL); err == nil {
		t.Fatal("expected an error")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (403 is not transient)", attempts)
	}
}

func TestDownloadChapterPagesFailsOnAnyMissingPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/2" {
			http.Error(w, "gone", http.StatusNotFound)
			return
		}
		w.Write([]byte("jpg"))
	}))
	defer srv.Close()

	urls := []string{srv.URL + "/1", srv.URL + "/2", srv.URL + "/3"}
	_, err := DownloadChapterPages(context.Background(), nil, "", urls)
	if err == nil || !strings.Contains(err.Error(), "trang 2/3") {
		t.Fatalf("err = %v, want it to name page 2/3", err)
	}
}

func TestDownloadChapterPagesKeepsOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.URL.Path))
	}))
	defer srv.Close()

	var urls []string
	for i := 0; i < 10; i++ {
		urls = append(urls, fmt.Sprintf("%s/p%d", srv.URL, i))
	}
	images, err := DownloadChapterPages(context.Background(), nil, "", urls)
	if err != nil {
		t.Fatal(err)
	}
	for i, img := range images {
		if want := fmt.Sprintf("/p%d", i); string(img) != want {
			t.Errorf("images[%d] = %q, want %q", i, img, want)
		}
	}
}

func TestDownloadChapterPagesRejectsEmptyList(t *testing.T) {
	if _, err := DownloadChapterPages(context.Background(), nil, "", nil); err == nil {
		t.Fatal("expected an error for a chapter without pages")
	}
}

func TestDownloadPageRejectsOversizedImage(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write(make([]byte, maxImageBytes+1))
	}))
	defer srv.Close()

	if _, err := DownloadPage(context.Background(), nil, "", srv.URL); err == nil || !strings.Contains(err.Error(), "quá lớn") {
		t.Fatalf("err = %v, want an oversized-image error", err)
	}
	if n := hits.Load(); n != 1 {
		t.Errorf("requests = %d, want 1 (too large is not transient)", n)
	}
}

func TestLookupCoverOnEverySource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manga/md-1":
			fmt.Fprint(w, `{"data":{"id":"md-1","relationships":[{"type":"cover_art","attributes":{"fileName":"c.jpg"}}]}}`)
		case "/truyen-tranh/qq-1", "/truyen/ct-1":
			fmt.Fprint(w, `<meta property="og:image" content="https://cdn.example/og.jpg">`)
		case "/truyen-tranh/bt-1":
			fmt.Fprint(w, `<meta property="og:image" content="https://bt.example/logo.png">
				<div class="col-image"><img src="https://bt.example/cover.jpg"></div>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	oldMD, oldCT := mangadexBaseURL, cuutruyenBaseURL
	mangadexBaseURL, cuutruyenBaseURL = srv.URL, srv.URL
	defer func() { mangadexBaseURL, cuutruyenBaseURL = oldMD, oldCT }()
	bt := NewBaoTangTruyenProvider()
	bt.baseURL = srv.URL

	cases := []struct {
		p    MangaProvider
		id   string
		want string
	}{
		{NewMangaDexProvider(), "md-1", "https://uploads.mangadex.org/covers/md-1/c.jpg.256.jpg"},
		{testQQ(srv.URL), "/truyen-tranh/qq-1", "https://cdn.example/og.jpg"},
		{bt, "/truyen-tranh/bt-1", "https://bt.example/cover.jpg"}, // not the og:image logo
		{NewCuuTruyenProvider(), "/truyen/ct-1", "https://cdn.example/og.jpg"},
	}
	for _, c := range cases {
		if got, err := LookupCover(context.Background(), c.p, c.id); err != nil || got != c.want {
			t.Errorf("%s: cover = %q, %v; want %q", c.p.Name(), got, err, c.want)
		}
	}
	if _, err := LookupCover(context.Background(), testQQ(srv.URL), "/truyen-tranh/missing"); err == nil {
		t.Error("a missing page must be an error, not an empty cover")
	}
}

func TestBaoTangCoverNeverFallsBackToTheLogo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<meta property="og:image" content="https://bt.example/logo.png"><div class="new-layout"></div>`)
	}))
	defer srv.Close()
	bt := NewBaoTangTruyenProvider()
	bt.baseURL = srv.URL
	if got, err := LookupCover(context.Background(), bt, "/truyen-tranh/x"); err == nil {
		t.Errorf("cover = %q, want an error instead of the og:image logo", got)
	}
}
