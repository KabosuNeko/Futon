package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/KabosuNeko/Futon/internal/models"
)

func newMangaDexTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	old := mangadexBaseURL
	mangadexBaseURL = srv.URL
	t.Cleanup(func() { mangadexBaseURL = old })
	return srv
}

func mangaJSON(t *testing.T, items int, total int) []byte {
	t.Helper()
	resp := models.MangaSearchResponse{Total: total}
	for i := 0; i < items; i++ {
		resp.Data = append(resp.Data, models.MangaData{
			ID: fmt.Sprintf("id-%d", i),
			Attributes: models.MangaAttributes{
				Title: map[string]string{"en": fmt.Sprintf("Manga %d", i)},
			},
		})
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("mangaJSON marshal: %v", err)
	}
	return b
}

func TestMangaDexSearchPaginates(t *testing.T) {
	var mu sync.Mutex
	var offsets []int

	newMangaDexTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "100" {
			t.Errorf("limit = %q, want 100", r.URL.Query().Get("limit"))
		}
		offset := 0
		fmt.Sscanf(r.URL.Query().Get("offset"), "%d", &offset)
		mu.Lock()
		offsets = append(offsets, offset)
		mu.Unlock()

		// Page 1: full page (100). Page 2: partial (25). Total = 125.
		items := 100
		if offset > 0 {
			items = 25
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mangaJSON(t, items, 125))
	})

	m := NewMangaDexProvider()
	results, err := m.Search("naruto")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 125 {
		t.Errorf("len(results) = %d, want 125", len(results))
	}
	mu.Lock()
	defer mu.Unlock()
	if len(offsets) != 2 || offsets[0] != 0 || offsets[1] != 100 {
		t.Errorf("offsets = %v, want [0 100]", offsets)
	}
}

func TestMangaDexSearchSinglePage(t *testing.T) {
	newMangaDexTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(mangaJSON(t, 3, 3))
	})

	m := NewMangaDexProvider()
	results, err := m.Search("one piece")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 3 {
		t.Errorf("len(results) = %d, want 3", len(results))
	}
	if results[0].Title != "Manga 0" {
		t.Errorf("results[0].Title = %q, want %q", results[0].Title, "Manga 0")
	}
}

func TestMangaDexSearchStopsAtTotal(t *testing.T) {
	// Full page returned with total == items: loop must stop without an extra request.
	requests := 0
	newMangaDexTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests > 1 {
			t.Fatalf("expected 1 request, got %d", requests)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(mangaJSON(t, 100, 100))
	})

	m := NewMangaDexProvider()
	results, err := m.Search("naruto")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 100 {
		t.Errorf("len(results) = %d, want 100", len(results))
	}
}

func TestMangaDexSearchHTTPError(t *testing.T) {
	newMangaDexTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	m := NewMangaDexProvider()
	if _, err := m.Search("naruto"); err == nil {
		t.Fatal("Search() error = nil, want HTTP 500 error")
	}
}
