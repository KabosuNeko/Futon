package storage

import "testing"

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
