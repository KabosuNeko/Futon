package storage

import (
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

	if err := RemoveFavorite("m1"); err != nil {
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
	if err := RemoveFavorite("missing"); err != nil {
		t.Fatalf("RemoveFavorite missing should not error: %v", err)
	}
}
