package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	// Tests isolate data through HOME; XDG paths would point at the real ones.
	os.Unsetenv("XDG_CONFIG_HOME")
	os.Unsetenv("XDG_DOWNLOAD_DIR")
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

func TestSaveAndGetHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := SaveHistory("m1", "Title", "otruyen", "", "c1", "1", 5); err != nil {
		t.Fatalf("SaveHistory error: %v", err)
	}

	h, ok := GetHistory("m1", "otruyen")
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
	if err := SaveHistory("m1", "Original", "otruyen", "https://x/cover.jpg", "c1", "1", 0); err != nil {
		t.Fatalf("SaveHistory error: %v", err)
	}
	if err := SaveHistory("m1", "", "otruyen", "", "c2", "2", 3); err != nil {
		t.Fatalf("SaveHistory error: %v", err)
	}

	h, ok := GetHistory("m1", "otruyen")
	if !ok {
		t.Fatalf("expected history for m1")
	}
	if h.MangaTitle != "Original" {
		t.Errorf("expected old title preserved, got %s", h.MangaTitle)
	}
	if h.CoverURL != "https://x/cover.jpg" {
		t.Errorf("expected old cover preserved, got %q", h.CoverURL)
	}
	if h.ChapterNumber != "2" {
		t.Errorf("expected new chapter number, got %s", h.ChapterNumber)
	}
}

func TestDeleteHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SaveHistory("m1", "Title", "otruyen", "", "c1", "1", 0); err != nil {
		t.Fatalf("SaveHistory error: %v", err)
	}
	if err := DeleteHistory("m1", "otruyen"); err != nil {
		t.Fatalf("DeleteHistory error: %v", err)
	}
	if _, ok := GetHistory("m1", "otruyen"); ok {
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
	if err := SaveHistory("m1", "Title", "otruyen", "", "c1", "1", 5); err != nil {
		t.Fatalf("SaveHistory error: %v", err)
	}

	h, ok := GetHistory("m1", "otruyen")
	if !ok {
		t.Fatalf("expected history for m1")
	}
	h.PageIndex = 99

	again, ok := GetHistory("m1", "otruyen")
	if !ok {
		t.Fatalf("expected history for m1")
	}
	if again.PageIndex != 5 {
		t.Errorf("mutating returned history changed the cache: PageIndex = %d, want 5", again.PageIndex)
	}
}

func TestLoadAllHistoryReturnsCopy(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SaveHistory("m1", "Title", "otruyen", "", "c1", "1", 5); err != nil {
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

func resetHistoryForTest() {
	historyMu.Lock()
	historyCache = nil
	historyLoaded = false
	historyDeleted = map[string]bool{}
	historyMu.Unlock()
}

func TestFlushHistoryLoadsExistingFileBeforeWriting(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetHistoryForTest()
	defer resetHistoryForTest()

	dir := filepath.Join(os.Getenv("HOME"), ".config", "futon")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "history.json")
	existing := `[{"manga_id":"existing-1","chapter_id":"c1","page_index":4,"updated_at":1}]`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatalf("write history: %v", err)
	}

	if err := FlushHistory(); err != nil {
		t.Fatalf("FlushHistory error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	if !strings.Contains(string(data), "existing-1") {
		t.Fatalf("flush before load wiped existing history: %s", data)
	}
}

func TestHistoryIsKeyedByProvider(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetHistoryForTest()
	defer resetHistoryForTest()

	if err := SaveHistory("same-id", "A", "ProviderA", "", "a1", "1", 1); err != nil {
		t.Fatal(err)
	}
	if err := SaveHistory("same-id", "B", "ProviderB", "", "b7", "7", 3); err != nil {
		t.Fatal(err)
	}

	a, ok := GetHistory("same-id", "ProviderA")
	if !ok || a.ChapterID != "a1" {
		t.Fatalf("ProviderA history = %+v, %v; want chapter a1", a, ok)
	}
	b, ok := GetHistory("same-id", "ProviderB")
	if !ok || b.ChapterID != "b7" {
		t.Fatalf("ProviderB history = %+v, %v; want chapter b7", b, ok)
	}

	if err := DeleteHistory("same-id", "ProviderA"); err != nil {
		t.Fatal(err)
	}
	if _, ok := GetHistory("same-id", "ProviderA"); ok {
		t.Error("ProviderA entry still present after delete")
	}
	if _, ok := GetHistory("same-id", "ProviderB"); !ok {
		t.Error("deleting ProviderA removed ProviderB's entry")
	}
}

func TestLegacyHistoryWithoutProviderIsFoundAndRekeyed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetHistoryForTest()
	defer resetHistoryForTest()

	dir := filepath.Join(os.Getenv("HOME"), ".config", "futon")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `[{"manga_id":"old","manga_title":"Old","chapter_id":"c1","page_index":2,"updated_at":1}]`
	if err := os.WriteFile(filepath.Join(dir, "history.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	h, ok := GetHistory("old", "OTruyen")
	if !ok || h.ChapterID != "c1" {
		t.Fatalf("legacy entry not found via provider lookup: %+v, %v", h, ok)
	}

	if err := SaveHistory("old", "", "OTruyen", "", "c2", "", 0); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadAllHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Provider != "OTruyen" || entries[0].MangaTitle != "Old" {
		t.Errorf("legacy entry not re-keyed in place: %+v", entries)
	}
}

// writeOtherWindow replaces history.json as another Futon window would.
func writeOtherWindow(t *testing.T, entries string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("HOME"), ".config", "futon")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "history.json"), []byte(entries), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFlushKeepsEntriesWrittenByAnotherWindow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetHistoryForTest()
	defer resetHistoryForTest()

	if err := SaveHistory("a", "A", "P", "", "a1", "1", 0); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	writeOtherWindow(t, fmt.Sprintf(`[
		{"manga_id":"b","provider":"P","chapter_id":"b1","page_index":2,"updated_at":%d},
		{"manga_id":"a","provider":"P","chapter_id":"a-old","page_index":9,"updated_at":%d}
	]`, now, now-100))

	if err := FlushHistory(); err != nil {
		t.Fatal(err)
	}
	if h, ok := GetHistory("b", "P"); !ok || h.ChapterID != "b1" {
		t.Errorf("the other window's entry was lost: %+v %v", h, ok)
	}
	if h, _ := GetHistory("a", "P"); h.ChapterID != "a1" {
		t.Errorf("an older entry from disk replaced this window's newer one: %+v", h)
	}
}

func TestFlushDoesNotResurrectDeletedEntries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetHistoryForTest()
	defer resetHistoryForTest()

	if err := SaveHistory("a", "A", "P", "", "a1", "1", 0); err != nil {
		t.Fatal(err)
	}
	if err := FlushHistory(); err != nil {
		t.Fatal(err)
	}
	if err := DeleteHistory("a", "P"); err != nil {
		t.Fatal(err)
	}
	if err := FlushHistory(); err != nil {
		t.Fatal(err)
	}
	if _, ok := GetHistory("a", "P"); ok {
		t.Error("a deleted entry came back from the file on the next flush")
	}
}

func TestRekeyedLegacyEntryIsNotDuplicatedOnFlush(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetHistoryForTest()
	defer resetHistoryForTest()

	writeOtherWindow(t, `[{"manga_id":"old","chapter_id":"c1","page_index":2,"updated_at":1}]`)
	if err := SaveHistory("old", "", "OTruyen", "", "c2", "", 0); err != nil {
		t.Fatal(err)
	}
	if err := FlushHistory(); err != nil {
		t.Fatal(err)
	}
	if all, _ := LoadAllHistory(); len(all) != 1 {
		t.Errorf("entries = %+v, want only the re-keyed one", all)
	}
}

func TestSetHistoryCoverKeepsPosition(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	resetHistoryForTest()
	if err := SaveHistory("m1", "T", "P", "", "c3", "3", 7); err != nil {
		t.Fatal(err)
	}
	before, _ := GetHistory("m1", "P")
	if err := SetHistoryCover("m1", "P", "https://x/c.jpg"); err != nil {
		t.Fatal(err)
	}
	h, _ := GetHistory("m1", "P")
	if h.CoverURL != "https://x/c.jpg" || h.ChapterID != "c3" || h.PageIndex != 7 || h.UpdatedAt != before.UpdatedAt {
		t.Errorf("entry = %+v, want the cover added and nothing else changed", h)
	}
}
