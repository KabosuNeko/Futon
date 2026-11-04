package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func newCuuTruyenTestServer(t *testing.T, handler http.HandlerFunc) *CuuTruyenProvider {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	old := cuutruyenBaseURL
	cuutruyenBaseURL = srv.URL
	t.Cleanup(func() { cuutruyenBaseURL = old })
	return NewCuuTruyenProvider()
}

const cuutruyenListHTML = `
<div class="manga-vertical">
  <div class="relative"><a href="/truyen/blue-lock"><div class="cover-frame">
    <div class="rounded-lg cover" style="background-image: url('https://ct.example/covers/bl.jpg')"></div>
  </div></a></div>
  <div class="p-2 w-full truncate"><a href="/truyen/blue-lock" class="text-ellipsis">Blue Lock</a></div>
</div>`

func TestCuuTruyenListAndFilter(t *testing.T) {
	var mu sync.Mutex
	var queries []string
	p := newCuuTruyenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		queries = append(queries, r.URL.RawQuery)
		mu.Unlock()
		fmt.Fprint(w, cuutruyenListHTML)
	})

	mangas, err := p.Search(context.Background(), "blue lock")
	if err != nil {
		t.Fatal(err)
	}
	if len(mangas) != 1 || mangas[0].ID != "/truyen/blue-lock" || mangas[0].Title != "Blue Lock" || mangas[0].CoverURL != "https://ct.example/covers/bl.jpg" {
		t.Fatalf("unexpected manga: %+v", mangas)
	}

	if _, err := p.Filter(context.Background(), FilterOptions{Genre: 1, Status: 2, Sort: 1, Page: 3}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if queries[0] != "filter%5Bname%5D=blue+lock&page=1" {
		t.Errorf("search query = %q", queries[0])
	}
	// Completed is "1" on this site; action is tag 33.
	if want := "filter%5Baccept_genres%5D=33&filter%5Bstatus%5D=1&page=3&sort=-views"; queries[1] != want {
		t.Errorf("filter query = %q, want %q", queries[1], want)
	}
}

func TestCuuTruyenChaptersAndPages(t *testing.T) {
	p := newCuuTruyenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/truyen/blue-lock":
			fmt.Fprint(w, `<ul class="overflow-y-auto">
				<a href="/truyen/blue-lock/2"><li><div class="grow truncate"><span class="text-ellipsis">Chap 2</span></div></li></a>
				<a href="/truyen/blue-lock/1"><li><div class="grow truncate"><span class="text-ellipsis">Chap 1</span></div></li></a>
				<a href="/truyen/blue-lock/1"><li><div class="grow truncate"><span class="text-ellipsis">Chap 1</span></div></li></a>
			</ul>`)
		case "/truyen/blue-lock/1":
			fmt.Fprint(w, `<div class="text-center">
				<img class="max-w-full my-0 mx-auto" src="https://ct.example/1/0.webp">
				<img class="max-w-full my-0 mx-auto" src="https://ct.example/1/1.webp">
			</div><img class="max-w-full" src="https://ct.example/ad.png">`)
		default:
			http.NotFound(w, r)
		}
	})

	chapters, err := p.FetchChapters(context.Background(), "/truyen/blue-lock")
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 2 || chapters[0].Title != "Chap 1" || chapters[1].ID != "/truyen/blue-lock/2" {
		t.Fatalf("chapters = %+v, want 2 ascending, deduplicated", chapters)
	}

	pages, err := p.FetchPages(context.Background(), chapters[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(pages, " ") != "https://ct.example/1/0.webp https://ct.example/1/1.webp" {
		t.Errorf("pages = %v", pages)
	}
	if got := pageRefererFor(p, chapters[0].ID); got != cuutruyenBaseURL+"/truyen/blue-lock/1" {
		t.Errorf("PageReferer = %q", got)
	}
}

func TestCuuTruyenUnlocksPasswordGate(t *testing.T) {
	wireData := html.EscapeString(`{"fingerprint":{"id":"abc","name":"enter-secret"},"serverMemo":{"checksum":"xyz"}}`)
	gate := `<div wire:id="abc" wire:initial-data="` + wireData + `"><form>enter-secret</form></div>
		<script>window.livewire_token = 'csrf-123';</script>`

	var posted map[string]json.RawMessage
	p := newCuuTruyenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/livewire/message/enter-secret" {
			if r.Header.Get("X-CSRF-TOKEN") != "csrf-123" || r.Header.Get("X-Livewire") != "true" {
				t.Errorf("missing Livewire headers: %v", r.Header)
			}
			if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
				t.Errorf("decode payload: %v", err)
			}
			http.SetCookie(w, &http.Cookie{Name: "unlocked", Value: "1", Path: "/"})
			return
		}
		if c, err := r.Cookie("unlocked"); err != nil || c.Value != "1" {
			fmt.Fprint(w, gate)
			return
		}
		fmt.Fprint(w, cuutruyenListHTML)
	})

	mangas, err := p.FetchLatest(context.Background(), 1)
	if err != nil {
		t.Fatalf("FetchLatest through gate: %v", err)
	}
	if len(mangas) != 1 {
		t.Fatalf("mangas = %d, want 1 after unlocking", len(mangas))
	}
	if !strings.Contains(string(posted["updates"]), `"value":"5"`) || !strings.Contains(string(posted["fingerprint"]), `"abc"`) {
		t.Errorf("unexpected Livewire payload: %s / %s", posted["updates"], posted["fingerprint"])
	}
}

func TestCuuTruyenGateRejectingPasswordIsAnError(t *testing.T) {
	gate := `<div wire:initial-data="{&quot;fingerprint&quot;:{},&quot;serverMemo&quot;:{}}">enter-secret</div>
		<script>livewire_token = 't';</script>`
	p := newCuuTruyenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, gate)
		}
	})
	if _, err := p.FetchLatest(context.Background(), 1); err == nil || !strings.Contains(err.Error(), "mật khẩu") {
		t.Fatalf("err = %v, want a password gate error", err)
	}
}
