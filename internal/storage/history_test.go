package storage

import (
	"os"
	"testing"
)

// TestMain redirects HOME to a temp dir for the whole test binary.
//
// SaveHistory (history.go) debounces disk writes on a 2s timer. A test that
// saves history can schedule the flush and finish before it fires; the late
// timer would then write test data into the developer's real ~/.config/futon/.
// Pinning HOME here keeps every delayed flush inside a throwaway temp dir.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "futon-storage-test-home-*")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", home)
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

func TestSaveAndGetHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := SaveHistory("m1", "Title", "otruyen", "c1", "1", 5); err != nil {
		t.Fatalf("SaveHistory error: %v", err)
	}

	h, ok := GetHistory("m1")
	if !ok {
		t.Fatalf("expected history for m1")
	}
	if h.PageIndex != 5 {
		t.Errorf("expected PageIndex 5, got %d", h.PageIndex)
	}
	if h.MangaTitle != "Title" {
		t.Errorf("expected MangaTitle Title, got %s", h.MangaTitle)
	}
}

func TestSaveHistoryKeepsOldTitle(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SaveHistory("m1", "Original", "otruyen", "c1", "1", 0); err != nil {
		t.Fatalf("SaveHistory error: %v", err)
	}
	if err := SaveHistory("m1", "", "otruyen", "c2", "2", 3); err != nil {
		t.Fatalf("SaveHistory error: %v", err)
	}

	h, ok := GetHistory("m1")
	if !ok {
		t.Fatalf("expected history for m1")
	}
	if h.MangaTitle != "Original" {
		t.Errorf("expected old title preserved, got %s", h.MangaTitle)
	}
	if h.ChapterNumber != "2" {
		t.Errorf("expected new chapter number, got %s", h.ChapterNumber)
	}
}

func TestDeleteHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SaveHistory("m1", "Title", "otruyen", "c1", "1", 0); err != nil {
		t.Fatalf("SaveHistory error: %v", err)
	}
	if err := DeleteHistory("m1"); err != nil {
		t.Fatalf("DeleteHistory error: %v", err)
	}
	if _, ok := GetHistory("m1"); ok {
		t.Errorf("expected history deleted")
	}
}

func TestLoadAllHistoryEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	entries, err := LoadAllHistory()
	if err != nil {
		t.Fatalf("LoadAllHistory error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty history, got %d", len(entries))
	}
}

func TestLoadAllHistorySortsByUpdatedAt(t *testing.T) {
	historyCache = map[string]ReadHistory{
		"m1": {MangaID: "m1", UpdatedAt: 100},
		"m2": {MangaID: "m2", UpdatedAt: 300},
		"m3": {MangaID: "m3", UpdatedAt: 200},
	}
	historyLoaded = true
	defer func() {
		historyCache = nil
		historyLoaded = false
	}()

	entries, err := LoadAllHistory()
	if err != nil {
		t.Fatalf("LoadAllHistory error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	want := []string{"m2", "m3", "m1"}
	for i, e := range entries {
		if e.MangaID != want[i] {
			t.Errorf("entry[%d].MangaID = %s, want %s", i, e.MangaID, want[i])
		}
	}
}

// The accessors return copies; mutating them must not touch the RAM cache.
func TestGetHistoryReturnsCopy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SaveHistory("m1", "Title", "otruyen", "c1", "1", 5); err != nil {
		t.Fatalf("SaveHistory error: %v", err)
	}

	h, ok := GetHistory("m1")
	if !ok {
		t.Fatalf("expected history for m1")
	}
	h.PageIndex = 99

	again, ok := GetHistory("m1")
	if !ok {
		t.Fatalf("expected history for m1")
	}
	if again.PageIndex != 5 {
		t.Errorf("mutating returned history changed the cache: PageIndex = %d, want 5", again.PageIndex)
	}
}

func TestLoadAllHistoryReturnsCopy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SaveHistory("m1", "Title", "otruyen", "c1", "1", 5); err != nil {
		t.Fatalf("SaveHistory error: %v", err)
	}

	entries, err := LoadAllHistory()
	if err != nil {
		t.Fatalf("LoadAllHistory error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	entries[0].PageIndex = 99

	again, err := LoadAllHistory()
	if err != nil {
		t.Fatalf("LoadAllHistory error: %v", err)
	}
	if again[0].PageIndex != 5 {
		t.Errorf("mutating returned entries changed the cache: PageIndex = %d, want 5", again[0].PageIndex)
	}
}
