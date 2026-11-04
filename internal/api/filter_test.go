package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestGenreTablesMatchFilterGenres(t *testing.T) {
	tables := map[string][]string{
		"MangaDex":      mangadexGenreTags,
		"TruyenQQ":      truyenqqGenres,
		"CuuTruyen":     cuutruyenGenres,
		"BaoTangTruyen": baotangGenres,
	}
	for name, table := range tables {
		if len(table) != len(FilterGenres) {
			t.Errorf("%s genre table has %d entries, want %d (FilterGenres)", name, len(table), len(FilterGenres))
		}
	}
}

func TestGenreSlugRejectsUnknownGenre(t *testing.T) {
	if _, err := genreSlug(baotangGenres, len(baotangGenres)); err == nil {
		t.Error("out-of-range genre should be an error, not a silent fallback")
	}
	if slug, err := genreSlug(baotangGenres, 0); err != nil || slug != "" {
		t.Errorf("genre 0 = %q, %v; want all genres", slug, err)
	}
}

// recordPaths serves an HTML list with one item and records request URIs.
func recordPaths(t *testing.T, body string) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.RequestURI())
		mu.Unlock()
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), paths...)
	}
}

const truyenqqListBody = `<ul class="list_grid"><li><h3><a href="/truyen-tranh/x">X</a></h3></li></ul>`

func TestTruyenQQFilterURLs(t *testing.T) {
	cases := []struct {
		name string
		opts FilterOptions
		want string
	}{
		{"genre+status+sort", FilterOptions{Genre: 8, Status: 2, Sort: 1, Page: 2}, "/the-loai/shounen-31/trang-2?sort=4&status=2"},
		{"genre ongoing", FilterOptions{Genre: 9, Status: 1}, "/the-loai/manhwa-49?sort=2&status=0"},
		{"completed", FilterOptions{Status: 2}, "/truyen-hoan-thanh"},
		{"top month", FilterOptions{Sort: 1, Page: 3}, "/top-thang/trang-3"},
		{"liked", FilterOptions{Sort: 2}, "/truyen-yeu-thich"},
		{"latest", FilterOptions{}, "/truyen-moi-cap-nhat"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, paths := recordPaths(t, truyenqqListBody)
			p := newTruyenQQProvider(srv.URL)
			p.domainOnce.Do(func() {})
			if _, err := p.Filter(context.Background(), tc.opts); err != nil {
				t.Fatalf("Filter error: %v", err)
			}
			if got := paths(); len(got) != 1 || got[0] != tc.want {
				t.Errorf("requested %v, want [%s]", got, tc.want)
			}
		})
	}
}

func TestTruyenQQLatestEmptyIsError(t *testing.T) {
	srv, _ := recordPaths(t, `<html><body>maintenance</body></html>`)
	p := newTruyenQQProvider(srv.URL, "")
	p.domainOnce.Do(func() {})
	if _, err := p.FetchLatest(context.Background(), 1); !errors.Is(err, errEmptyList) {
		t.Fatalf("err = %v, want errEmptyList", err)
	}
}

func TestTruyenQQFilterErrorIsNotMaskedByLatest(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.NotFound(w, r)
	}))
	defer srv.Close()
	p := newTruyenQQProvider(srv.URL, "")
	p.domainOnce.Do(func() {})
	if _, err := p.Filter(context.Background(), FilterOptions{Genre: 1}); err == nil {
		t.Fatal("Filter error = nil, want the 404 to surface")
	}
	if requests != 1 {
		t.Errorf("requests = %d, want 1 (no fallback to latest)", requests)
	}
}

func TestBaoTangFilterURL(t *testing.T) {
	srv, paths := recordPaths(t, `<div class="items"><div class="row"><div class="item"><h3><a href="/x">X</a></h3></div></div></div>`)
	p := NewBaoTangTruyenProvider()
	p.baseURL = srv.URL

	if _, err := p.Filter(context.Background(), FilterOptions{Genre: 6, Status: 1, Sort: 2, Page: 2}); err != nil {
		t.Fatalf("Filter error: %v", err)
	}
	want := "/tim-truyen/chuyen-sinh?page=2&sort=20&status=1"
	if got := paths(); len(got) != 1 || got[0] != want {
		t.Errorf("requested %v, want [%s]", got, want)
	}
}

func TestMangaDexSearchCapsPages(t *testing.T) {
	requests := 0
	newMangaDexTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		w.Write(mangaJSON(t, 100, 5000))
	})

	results, err := NewMangaDexProvider().Search(context.Background(), "the")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if requests != mangadexSearchMaxPages {
		t.Errorf("requests = %d, want %d", requests, mangadexSearchMaxPages)
	}
	if len(results) != mangadexSearchMaxPages*mangadexSearchPageLimit {
		t.Errorf("results = %d, want %d", len(results), mangadexSearchMaxPages*mangadexSearchPageLimit)
	}
}

func TestMangaDexSearchKeepsResultsWhenLaterPageFails(t *testing.T) {
	newMangaDexTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("offset") != "0" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mangaJSON(t, 100, 5000))
	})

	results, err := NewMangaDexProvider().Search(context.Background(), "the")
	if err != nil {
		t.Fatalf("Search error = %v, want first page kept", err)
	}
	if len(results) != 100 {
		t.Errorf("results = %d, want 100", len(results))
	}
}

func TestMangaDexListsFilterByLanguage(t *testing.T) {
	newMangaDexTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query()["availableTranslatedLanguage[]"]; len(got) != 1 || got[0] != "en" {
			t.Errorf("availableTranslatedLanguage[] = %v, want [en]", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mangaJSON(t, 1, 1))
	})

	p := NewMangaDexProvider()
	p.SetLang("en")
	ctx := context.Background()
	if _, err := p.Search(ctx, "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.FetchLatest(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Filter(ctx, FilterOptions{Genre: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestMangaDexFetchChaptersDedupesAndSkipsExternal(t *testing.T) {
	newMangaDexTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("includeExternalUrl") != "0" {
			t.Errorf("includeExternalUrl = %q, want 0", r.URL.Query().Get("includeExternalUrl"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"total": 5, "data": [
			{"id": "a", "attributes": {"chapter": "1"}},
			{"id": "b", "attributes": {"chapter": "1"}},
			{"id": "c", "attributes": {"chapter": "2", "externalUrl": "https://elsewhere"}},
			{"id": "d", "attributes": {"chapter": ""}},
			{"id": "e", "attributes": {"chapter": ""}}
		]}`)
	})

	chapters, err := NewMangaDexProvider().FetchChapters(context.Background(), "m")
	if err != nil {
		t.Fatalf("FetchChapters error: %v", err)
	}
	var ids []string
	for _, c := range chapters {
		ids = append(ids, c.ID)
	}
	if fmt.Sprint(ids) != "[a d e]" {
		t.Errorf("chapter IDs = %v, want [a d e] (dup and external dropped, oneshots kept)", ids)
	}
}
