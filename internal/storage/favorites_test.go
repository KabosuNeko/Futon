package storage

import (
	"fmt"
	"sync"
	"testing"
)

func TestFavoritesAddLoadRemove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := AddFavorite(FavoriteManga{MangaID: "m1", Title: "One Piece"}); err != nil {
		t.Fatalf("AddFavorite error: %v", err)
	}
	if err := AddFavorite(FavoriteManga{MangaID: "m2", Title: "Naruto"}); err != nil {
		t.Fatalf("AddFavorite error: %v", err)
	}
	// Duplicate should be ignored.
	if err := AddFavorite(FavoriteManga{MangaID: "m1", Title: "One Piece"}); err != nil {
		t.Fatalf("duplicate AddFavorite error: %v", err)
	}

	favs, err := LoadFavorites()
	if err != nil {
		t.Fatalf("LoadFavorites error: %v", err)
	}
	if len(favs) != 2 {
		t.Fatalf("expected 2 favorites, got %d", len(favs))
	}

	if err := RemoveFavorite("m1", ""); err != nil {
		t.Fatalf("RemoveFavorite error: %v", err)
	}
	favs, err = LoadFavorites()
	if err != nil {
		t.Fatalf("LoadFavorites after remove error: %v", err)
	}
	if len(favs) != 1 {
		t.Fatalf("expected 1 favorite after remove, got %d", len(favs))
	}
	if favs[0].MangaID != "m2" {
		t.Errorf("expected remaining favorite m2, got %s", favs[0].MangaID)
	}
}

func TestRemoveFavoriteNotFound(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := RemoveFavorite("missing", ""); err != nil {
		t.Fatalf("RemoveFavorite missing should not error: %v", err)
	}
}

func TestFavoritesAreKeyedByProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := AddFavorite(FavoriteManga{MangaID: "x", Title: "A", Provider: "P1"}); err != nil {
		t.Fatal(err)
	}
	if err := AddFavorite(FavoriteManga{MangaID: "x", Title: "B", Provider: "P2"}); err != nil {
		t.Fatal(err)
	}
	favs, _ := LoadFavorites()
	if len(favs) != 2 {
		t.Fatalf("favorites = %d, want 2 (same ID on two providers)", len(favs))
	}

	if err := RemoveFavorite("x", "P1"); err != nil {
		t.Fatal(err)
	}
	favs, _ = LoadFavorites()
	if len(favs) != 1 || favs[0].Provider != "P2" {
		t.Errorf("after removing P1: %+v, want only P2", favs)
	}
}

func TestConcurrentUserDataUpdatesAreNotLost(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = AddFavorite(FavoriteManga{MangaID: fmt.Sprintf("m%d", i), Provider: "P"})
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = SaveSources([]string{"P"})
	}()
	wg.Wait()

	ud, err := LoadUserData()
	if err != nil {
		t.Fatal(err)
	}
	if len(ud.Favorites) != 20 || len(ud.Sources) != 1 {
		t.Errorf("favorites = %d, sources = %d; want 20 and 1 (no lost updates)", len(ud.Favorites), len(ud.Sources))
	}
}

func TestChapterLangPersists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if lang, _ := LoadChapterLang(); lang != "" {
		t.Fatalf("default lang = %q, want empty", lang)
	}
	if err := SaveChapterLang("en"); err != nil {
		t.Fatal(err)
	}
	if lang, _ := LoadChapterLang(); lang != "en" {
		t.Errorf("lang = %q, want en", lang)
	}
}

func TestReaderFitWidthPersists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if on, _ := LoadReaderFitWidth(); on {
		t.Fatal("fit-width must default to off")
	}
	if err := SaveReaderFitWidth(true); err != nil {
		t.Fatal(err)
	}
	if on, _ := LoadReaderFitWidth(); !on {
		t.Error("fit-width setting not persisted")
	}
}
