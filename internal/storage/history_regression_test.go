package storage

import (
	"testing"
)

func TestSaveAndGetHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := SaveHistory("m1", "Title", "c1", "1", 5); err != nil {
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
	if err := SaveHistory("m1", "Original", "c1", "1", 0); err != nil {
		t.Fatalf("SaveHistory error: %v", err)
	}
	if err := SaveHistory("m1", "", "c2", "2", 3); err != nil {
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
	if err := SaveHistory("m1", "Title", "c1", "1", 0); err != nil {
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
