package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestTruyenQQConstructorDoesNotTouchNetwork(t *testing.T) {
	old := http.DefaultTransport
	probes := 0
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		probes++
		return nil, errors.New("no network in test")
	})
	defer func() { http.DefaultTransport = old }()

	_ = NewTruyenQQProvider()
	if probes != 0 {
		t.Fatalf("constructor made %d network requests, want 0", probes)
	}
}

func TestTruyenQQDomainProbeIsLazyAndRunsOnce(t *testing.T) {
	old := http.DefaultTransport
	probes := 0
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		probes++
		return nil, errors.New("no network in test")
	})
	defer func() { http.DefaultTransport = old }()

	p := NewTruyenQQProvider()
	if p.baseURL != truyenqqPrimaryURL {
		t.Fatalf("baseURL = %q before first request, want %q", p.baseURL, truyenqqPrimaryURL)
	}

	p.resolveDomain()
	if probes != 2 {
		t.Fatalf("probe requests = %d, want primary + fallback (2)", probes)
	}
	if p.baseURL != truyenqqPrimaryURL {
		t.Fatalf("baseURL = %q after failed probes, want primary", p.baseURL)
	}

	p.resolveDomain()
	if probes != 2 {
		t.Fatalf("probe requests = %d after second call, want 2 (sync.Once)", probes)
	}
}

func newTruyenQQTestProvider(t *testing.T, handler http.HandlerFunc) *TruyenQQProvider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	p := NewTruyenQQProvider()
	p.baseURL = srv.URL
	p.domainOnce.Do(func() {}) // skip the real-domain probe
	return p
}

func TestTruyenQQUsesTruyenGGAsMirror(t *testing.T) {
	p := NewTruyenQQProvider()
	if p.Name() != "TruyenQQ" || p.baseURL != truyenqqPrimaryURL {
		t.Errorf("provider = %s @ %s", p.Name(), p.baseURL)
	}
	if len(p.fallbacks) != 1 || p.fallbacks[0] != "https://truyenggvn.com" {
		t.Errorf("fallbacks = %v, want the TruyenGG mirror", p.fallbacks)
	}
}

func TestTruyenQQPathIsMirrorIndependent(t *testing.T) {
	for in, want := range map[string]string{
		"https://truyenggvn.com/truyen-tranh/futari-bus-22373":             "/truyen-tranh/futari-bus-22373",
		"https://truyenggvn.com/truyen-tranh/futari-bus-22373-chap-1.html": "/truyen-tranh/futari-bus-22373-chap-1",
		"https://truyenqqko.com/truyen-tranh/futari-bus-22373-chap-1":      "/truyen-tranh/futari-bus-22373-chap-1",
		"/truyen-tranh/futari-bus-22373":                                   "/truyen-tranh/futari-bus-22373",
		"":                                                                 "",
	} {
		if got := TruyenQQPath(in); got != want {
			t.Errorf("TruyenQQPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTruyenQQPageRefererUsesCurrentMirror(t *testing.T) {
	p := NewTruyenQQProvider()
	p.baseURL = "https://mirror.example"
	p.domainOnce.Do(func() {})
	if got := pageRefererFor(p, "/truyen-tranh/x-chap-1"); got != "https://mirror.example/truyen-tranh/x-chap-1" {
		t.Errorf("PageReferer = %q", got)
	}
}

func TestTruyenQQSearch(t *testing.T) {
	p := newTruyenQQTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
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

	mangas, err := p.Search(context.Background(), "one piece")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(mangas) != 1 {
		t.Fatalf("mangas = %d, want 1", len(mangas))
	}
	if mangas[0].ID != "/truyen-tranh/one-piece-1" || mangas[0].Title != "One Piece" || mangas[0].CoverURL != "https://img.example/cover.jpg" {
		t.Errorf("unexpected manga: %+v", mangas[0])
	}
}

func TestTruyenQQLatestChaptersAndPages(t *testing.T) {
	p := newTruyenQQTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/truyen-moi-cap-nhat":
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
		case "/truyen-tranh/one-piece-chap-1":
			w.Write([]byte(`
				<div class="page-chapter"><img data-original="https://img.example/1.jpg"></div>
				<div class="page-chapter"><img data-original="https://img.example/2.jpg"></div>`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})

	mangas, err := p.FetchLatest(context.Background(), 1)
	if err != nil {
		t.Fatalf("FetchLatest error: %v", err)
	}
	if len(mangas) != 1 || mangas[0].ID != "/truyen-tranh/one-piece" || mangas[0].Title != "One Piece" {
		t.Fatalf("unexpected latest: %+v", mangas)
	}

	chapters, err := p.FetchChapters(context.Background(), "/truyen-tranh/one-piece")
	if err != nil {
		t.Fatalf("FetchChapters error: %v", err)
	}
	if len(chapters) != 2 || chapters[0].Title != "Chương 1" || chapters[1].Title != "Chương 2" {
		t.Fatalf("chapters not ascending: %+v", chapters)
	}
	if chapters[0].ID != "/truyen-tranh/one-piece-chap-1" {
		t.Errorf("chapter ID = %q, want a mirror-independent path without .html", chapters[0].ID)
	}

	urls, err := p.FetchPages(context.Background(), chapters[0].ID)
	if err != nil {
		t.Fatalf("FetchPages error: %v", err)
	}
	if len(urls) != 2 || urls[0] != "https://img.example/1.jpg" || urls[1] != "https://img.example/2.jpg" {
		t.Fatalf("unexpected pages: %v", urls)
	}
}

func TestTruyenQQProbeGivesUpQuickly(t *testing.T) {
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // a mirror that never answers
	}))
	defer hang.Close()

	p := newTruyenQQProvider(hang.URL)
	start := time.Now()
	if p.tryDomain(hang.URL) {
		t.Fatal("a hanging mirror was accepted")
	}
	if d := time.Since(start); d > probeTimeout+time.Second {
		t.Errorf("probe took %v, want about %v", d, probeTimeout)
	}
}
