package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mergeGG folds "GG" entries into "QQ" and strips the mirror host from IDs.
func mergeGG(provider, mangaID, chapterID string) (string, string, string) {
	if provider != "GG" && provider != "QQ" {
		return provider, mangaID, chapterID
	}
	strip := func(s string) string {
		return strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(s, "https://gg.example"), "https://qq.example"), ".html")
	}
	return "QQ", strip(mangaID), strip(chapterID)
}

func TestMigrateEntriesMergesProviders(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetHistoryForTest()
	defer resetHistoryForTest()

	dir := filepath.Join(os.Getenv("HOME"), ".config", "futon")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	history := `[
		{"manga_id":"https://gg.example/m-1","provider":"GG","chapter_id":"https://gg.example/m-1-chap-3.html","page_index":4,"updated_at":200},
		{"manga_id":"/m-1","provider":"QQ","chapter_id":"https://qq.example/m-1-chap-1","page_index":0,"updated_at":100},
		{"manga_id":"x","provider":"Other","chapter_id":"c","page_index":1,"updated_at":50}
	]`
	if err := os.WriteFile(filepath.Join(dir, "history.json"), []byte(history), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AddFavorite(FavoriteManga{MangaID: "https://gg.example/m-1", Provider: "GG"}); err != nil {
		t.Fatal(err)
	}
	if err := AddFavorite(FavoriteManga{MangaID: "/m-1", Provider: "QQ"}); err != nil {
		t.Fatal(err)
	}

	if err := MigrateEntries(mergeGG); err != nil {
		t.Fatalf("MigrateEntries: %v", err)
	}

	h, ok := GetHistory("/m-1", "QQ")
	if !ok || h.ChapterID != "/m-1-chap-3" || h.PageIndex != 4 {
		t.Errorf("merged history = %+v, %v; want the newer GG entry re-keyed", h, ok)
	}
	if _, ok := GetHistory("x", "Other"); !ok {
		t.Error("unrelated history entry lost")
	}
	all, _ := LoadAllHistory()
	if len(all) != 2 {
		t.Errorf("history entries = %d, want 2", len(all))
	}
	favs, _ := LoadFavorites()
	if len(favs) != 1 || favs[0].Provider != "QQ" || favs[0].MangaID != "/m-1" {
		t.Errorf("favorites = %+v, want one merged QQ entry", favs)
	}

	// Idempotent: a second run changes nothing.
	before, _ := os.ReadFile(filepath.Join(dir, "history.json"))
	if err := MigrateEntries(mergeGG); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "history.json"))
	if string(before) != string(after) {
		t.Error("second migration rewrote history")
	}
}
