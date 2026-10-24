package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTruyenGGTestProvider(t *testing.T, handler http.HandlerFunc) *truyenHTMLProvider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	p := NewTruyenGGProvider()
	p.baseURL = srv.URL
	p.domainOnce.Do(func() {}) // skip the real-domain probe
	return p
}

func TestTruyenGGProviderName(t *testing.T) {
	p := NewTruyenGGProvider()
	if p.Name() != "TruyenGG" {
		t.Errorf("Name() = %q, want TruyenGG", p.Name())
	}
	if p.baseURL != truyenggBaseURL {
		t.Errorf("baseURL = %q, want %q", p.baseURL, truyenggBaseURL)
	}
}

func TestTruyenGGSearch(t *testing.T) {
	p := newTruyenGGTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/frontend/search/search" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Requested-With"); got != "XMLHttpRequest" {
			t.Errorf("X-Requested-With = %q", got)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		if got := r.Form.Get("search"); got != "one piece" {
			t.Errorf("search = %q, want one piece", got)
		}
		w.Write([]byte(`
			<li>
				<a href="https://truyenggvn.com/truyen-tranh/one-piece-1">
					<div class="search_avatar"><img src="https://img.example/cover.jpg"></div>
					<div class="search_info"><p class="name">One Piece</p></div>
				</a>
			</li>`))
	})

	mangas, err := p.Search("one piece")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(mangas) != 1 {
		t.Fatalf("mangas = %d, want 1", len(mangas))
	}
	if mangas[0].ID != "https://truyenggvn.com/truyen-tranh/one-piece-1" || mangas[0].Title != "One Piece" || mangas[0].CoverURL != "https://img.example/cover.jpg" {
		t.Errorf("unexpected manga: %+v", mangas[0])
	}
}

func TestTruyenGGLatestChaptersAndPages(t *testing.T) {
	p := newTruyenGGTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/truyen-moi-cap-nhat.html":
			w.Write([]byte(`
				<ul class="list_grid grid">
					<li>
						<div class="book_name"><h3><a href="/truyen-tranh/one-piece">One Piece</a></h3></div>
						<img data-original="https://img.example/cover.jpg">
					</li>
				</ul>`))
		case "/truyen-tranh/one-piece":
			w.Write([]byte(`
				<div class="works-chapter-list">
					<div class="works-chapter-item"><a href="/truyen-tranh/one-piece-chap-2.html">Chương 2</a></div>
					<div class="works-chapter-item"><a href="/truyen-tranh/one-piece-chap-1.html">Chương 1</a></div>
				</div>`))
		case "/truyen-tranh/one-piece-chap-1.html":
			w.Write([]byte(`
				<div class="page-chapter"><img data-original="https://img.example/1.jpg"></div>
				<div class="page-chapter"><img data-original="https://img.example/2.jpg"></div>`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})

	mangas, err := p.FetchLatest(1)
	if err != nil {
		t.Fatalf("FetchLatest error: %v", err)
	}
	if len(mangas) != 1 || mangas[0].ID != "/truyen-tranh/one-piece" || mangas[0].Title != "One Piece" {
		t.Fatalf("unexpected latest: %+v", mangas)
	}

	chapters, err := p.FetchChapters("/truyen-tranh/one-piece")
	if err != nil {
		t.Fatalf("FetchChapters error: %v", err)
	}
	if len(chapters) != 2 || chapters[0].Title != "Chương 1" || chapters[1].Title != "Chương 2" {
		t.Fatalf("chapters not ascending: %+v", chapters)
	}
	if !strings.HasPrefix(chapters[0].ID, p.baseURL) {
		t.Errorf("chapter ID is not absolute: %q", chapters[0].ID)
	}

	urls, err := p.FetchPages(chapters[0].ID)
	if err != nil {
		t.Fatalf("FetchPages error: %v", err)
	}
	if len(urls) != 2 || urls[0] != "https://img.example/1.jpg" || urls[1] != "https://img.example/2.jpg" {
		t.Fatalf("unexpected pages: %v", urls)
	}
}
