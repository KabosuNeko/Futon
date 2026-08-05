package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// writeOldFile writes a file under ~/.config/futon in the test HOME.
func writeOldFile(t *testing.T, name, content string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "futon")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestMigrateOldDataMergesAndRemovesOldFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	writeOldFile(t, "favorites.json", `[
		{"manga_id": "m1", "title": "One Piece", "provider": "OTruyen"},
		{"manga_id": "m2", "title": "Naruto"}
	]`)
	writeOldFile(t, "sources.json", `["OTruyen"]`)

	ud, err := LoadUserData()
	if err != nil {
		t.Fatalf("LoadUserData error: %v", err)
	}
	if ud == nil {
		t.Fatal("expected non-nil UserData after migration")
	}

	if len(ud.Sources) != 1 || ud.Sources[0] != "OTruyen" {
		t.Errorf("expected sources [OTruyen], got %v", ud.Sources)
	}
	if len(ud.Favorites) != 2 {
		t.Fatalf("expected 2 favorites, got %d", len(ud.Favorites))
	}
	if ud.Favorites[0].MangaID != "m1" || ud.Favorites[0].Title != "One Piece" || ud.Favorites[0].Provider != "OTruyen" {
		t.Errorf("unexpected first favorite: %+v", ud.Favorites[0])
	}
	if ud.Favorites[1].MangaID != "m2" || ud.Favorites[1].Title != "Naruto" {
		t.Errorf("unexpected second favorite: %+v", ud.Favorites[1])
	}

	// userdata.json must now exist.
	udPath := filepath.Join(os.Getenv("HOME"), ".config", "futon", "userdata.json")
	if _, err := os.Stat(udPath); err != nil {
		t.Errorf("expected userdata.json to exist after migration: %v", err)
	}

	// Old files must be removed from disk.
	for _, name := range []string{"favorites.json", "sources.json"} {
		oldPath := filepath.Join(os.Getenv("HOME"), ".config", "futon", name)
		if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed, stat err=%v", name, err)
		}
	}
}

func TestMigrateOldDataSkipsWhenUserDataExists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Pre-existing userdata.json with its own content.
	writeOldFile(t, "userdata.json", `{"sources": ["MangaDex"], "favorites": [{"manga_id": "m9", "title": "Existing"}]}`)
	// Old favorites.json also present.
	writeOldFile(t, "favorites.json", `[{"manga_id": "m1", "title": "Old"}]`)

	ud, err := LoadUserData()
	if err != nil {
		t.Fatalf("LoadUserData error: %v", err)
	}
	if ud == nil {
		t.Fatal("expected non-nil UserData")
	}

	// Data must come from userdata.json, not the old file.
	if len(ud.Sources) != 1 || ud.Sources[0] != "MangaDex" {
		t.Errorf("expected sources [MangaDex], got %v", ud.Sources)
	}
	if len(ud.Favorites) != 1 || ud.Favorites[0].MangaID != "m9" {
		t.Errorf("expected favorite m9, got %+v", ud.Favorites)
	}

	// Old favorites.json must still exist (migration did not run).
	oldPath := filepath.Join(os.Getenv("HOME"), ".config", "futon", "favorites.json")
	if _, err := os.Stat(oldPath); err != nil {
		t.Errorf("expected favorites.json to remain untouched, stat err=%v", err)
	}
}

func TestMigrateOldDataNoFilesReturnsNil(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	ud, err := LoadUserData()
	if err != nil {
		t.Fatalf("LoadUserData error: %v", err)
	}
	if ud != nil {
		t.Errorf("expected nil UserData, got %+v", ud)
	}

	// No userdata.json should be created.
	udPath := filepath.Join(os.Getenv("HOME"), ".config", "futon", "userdata.json")
	if _, err := os.Stat(udPath); !os.IsNotExist(err) {
		t.Errorf("expected no userdata.json, stat err=%v", err)
	}
}

func TestMigrateOldDataIgnoresCorruptFavorites(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	// Corrupt favorites.json, valid sources.json.
	writeOldFile(t, "favorites.json", `{invalid`)
	writeOldFile(t, "sources.json", `["OTruyen"]`)

	ud, err := LoadUserData()
	if err != nil {
		t.Fatalf("LoadUserData error: %v", err)
	}
	if ud == nil {
		t.Fatal("expected non-nil UserData")
	}

	// Sources migrated; corrupt favorites dropped (empty slice).
	if len(ud.Sources) != 1 || ud.Sources[0] != "OTruyen" {
		t.Errorf("expected sources [OTruyen], got %v", ud.Sources)
	}
	if len(ud.Favorites) != 0 {
		t.Errorf("expected 0 favorites (corrupt dropped), got %d", len(ud.Favorites))
	}

	// Old files removed after successful migration.
	for _, name := range []string{"favorites.json", "sources.json"} {
		oldPath := filepath.Join(os.Getenv("HOME"), ".config", "futon", name)
		if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed, stat err=%v", name, err)
		}
	}
}
