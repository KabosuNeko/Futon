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

func TestUpdateUserDataPersistsSettings(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if ud, _ := LoadUserData(); ud.ChapterLang != "" || ud.ReaderFitWidth || ud.ReaderHideFooter {
		t.Fatalf("defaults = %+v, want zero values", ud)
	}
	err := UpdateUserData(func(ud *UserData) {
		ud.ChapterLang = "en"
		ud.ReaderFitWidth = true
		ud.ReaderHideFooter = true
	})
	if err != nil {
		t.Fatal(err)
	}
	if ud, _ := LoadUserData(); ud.ChapterLang != "en" || !ud.ReaderFitWidth || !ud.ReaderHideFooter {
		t.Errorf("saved settings = %+v", ud)
	}
}

func TestSetFavoriteCoverFillsOldEntry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := AddFavorite(FavoriteManga{MangaID: "m1", Title: "A", Provider: "P"}); err != nil {
		t.Fatal(err)
	}
	if err := SetFavoriteCover("m1", "P", "https://x/a.jpg"); err != nil {
		t.Fatal(err)
	}
	if err := SetFavoriteCover("m1", "Other", "https://x/b.jpg"); err != nil { // not a favorite
		t.Fatal(err)
	}
	favs, err := LoadFavorites()
	if err != nil || len(favs) != 1 || favs[0].CoverURL != "https://x/a.jpg" {
		t.Fatalf("favorites = %+v, err %v", favs, err)
	}
}
