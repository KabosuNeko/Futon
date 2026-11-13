package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type ReadHistory struct {
	MangaID       string `json:"manga_id"`
	MangaTitle    string `json:"manga_title,omitempty"`
	Provider      string `json:"provider,omitempty"`
	ChapterID     string `json:"chapter_id"`
	ChapterNumber string `json:"chapter_number,omitempty"`
	PageIndex     int    `json:"page_index"`
	UpdatedAt     int64  `json:"updated_at"`
}

var (
	historyMu     sync.RWMutex
	historyCache  map[string]ReadHistory // keyed by historyKey
	historyLoaded bool
	flushTimer    *time.Timer
	// historyDeleted holds keys removed in this session, so merging the file
	// written by another Futon window does not bring them back.
	historyDeleted = map[string]bool{}
	// flushMu serializes flushes: the debounce timer and an explicit flush
	// may run together, and an older snapshot must not be renamed in last.
	flushMu sync.Mutex
)

const flushDelay = 2 * time.Second

// historyKey identifies a manga across sources: manga IDs are only unique
// within one provider. Entries written before the provider was recorded have
// an empty provider and are found through the legacy key.
func historyKey(mangaID, provider string) string {
	return provider + "\x00" + mangaID
}

func historyPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history.json"), nil
}

func loadHistory() error {
	if historyLoaded {
		return nil
	}

	path, err := historyPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			historyCache = make(map[string]ReadHistory)
			historyLoaded = true
			return nil
		}
		return fmt.Errorf("đọc file history: %w", err)
	}

	var entries []ReadHistory
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("parse history JSON: %w", err)
	}

	historyCache = make(map[string]ReadHistory, len(entries))
	for _, e := range entries {
		historyCache[historyKey(e.MangaID, e.Provider)] = e
	}
	historyLoaded = true
	return nil
}

// SaveHistory updates reading progress in memory and schedules background persistence.
func SaveHistory(mangaID, mangaTitle, provider, chapterID, chapterNumber string, pageIndex int) error {
	if mangaID == "" || chapterID == "" {
		return nil
	}

	historyMu.Lock()
	defer historyMu.Unlock()

	if err := loadHistory(); err != nil {
		return err
	}

	if old, ok := lookupHistory(mangaID, provider); ok {
		if mangaTitle == "" {
			mangaTitle = old.MangaTitle
		}
		if chapterNumber == "" {
			chapterNumber = old.ChapterNumber
		}
	}
	// Re-key a legacy provider-less entry now that the provider is known.
	forgetHistory(historyKey(mangaID, ""))
	delete(historyDeleted, historyKey(mangaID, provider))

	historyCache[historyKey(mangaID, provider)] = ReadHistory{
		MangaID:       mangaID,
		MangaTitle:    mangaTitle,
		Provider:      provider,
		ChapterID:     chapterID,
		ChapterNumber: chapterNumber,
		PageIndex:     pageIndex,
		UpdatedAt:     time.Now().Unix(),
	}

	scheduleFlush()
	return nil
}

// lookupHistory finds the entry for (mangaID, provider), falling back to a
// legacy entry saved without a provider. Callers hold historyMu.
func lookupHistory(mangaID, provider string) (ReadHistory, bool) {
	if h, ok := historyCache[historyKey(mangaID, provider)]; ok {
		return h, true
	}
	h, ok := historyCache[historyKey(mangaID, "")]
	return h, ok
}

func GetHistory(mangaID, provider string) (*ReadHistory, bool) {
	historyMu.Lock()
	defer historyMu.Unlock()

	if err := loadHistory(); err != nil {
		return nil, false
	}

	h, ok := lookupHistory(mangaID, provider)
	if !ok {
		return nil, false
	}

	return &h, true
}

// LoadAllHistory returns all reading history records sorted newest first.
func LoadAllHistory() ([]ReadHistory, error) {
	historyMu.Lock()
	defer historyMu.Unlock()

	if err := loadHistory(); err != nil {
		return nil, err
	}

	return sortedHistory(), nil
}

// sortedHistory returns the cache newest first, with a stable tie-break so
// the file on disk does not reshuffle on every flush. Callers hold historyMu.
func sortedHistory() []ReadHistory {
	entries := make([]ReadHistory, 0, len(historyCache))
	for _, e := range historyCache {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].UpdatedAt != entries[j].UpdatedAt {
			return entries[i].UpdatedAt > entries[j].UpdatedAt
		}
		return historyKey(entries[i].MangaID, entries[i].Provider) < historyKey(entries[j].MangaID, entries[j].Provider)
	})
	return entries
}

// forgetHistory removes an entry for good. Callers hold historyMu.
func forgetHistory(key string) {
	delete(historyCache, key)
	historyDeleted[key] = true
}

// mergeHistoryFromDisk takes in entries that another Futon window wrote since
// this one loaded the file: missing ones, and newer ones for the same manga.
// A missing or unreadable file leaves the cache as it is. Callers hold
// historyMu.
func mergeHistoryFromDisk() {
	path, err := historyPath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var entries []ReadHistory
	if json.Unmarshal(data, &entries) != nil {
		return
	}
	for _, e := range entries {
		key := historyKey(e.MangaID, e.Provider)
		if historyDeleted[key] {
			continue
		}
		if cur, ok := historyCache[key]; !ok || e.UpdatedAt > cur.UpdatedAt {
			historyCache[key] = e
		}
	}
}

func FlushHistory() error {
	flushMu.Lock()
	defer flushMu.Unlock()

	historyMu.Lock()
	// Flushing before anything has read the file must not overwrite it with a
	// snapshot of the still-empty cache.
	if err := loadHistory(); err != nil {
		historyMu.Unlock()
		return err
	}
	mergeHistoryFromDisk()
	snapshot := sortedHistory()
	historyMu.Unlock()

	path, err := historyPath()
	if err != nil {
		return err
	}

	if err := writeJSON(path, snapshot); err != nil {
		return fmt.Errorf("ghi file history: %w", err)
	}
	return nil
}

func DeleteHistory(mangaID, provider string) error {
	if mangaID == "" {
		return nil
	}

	historyMu.Lock()
	if err := loadHistory(); err != nil {
		historyMu.Unlock()
		return err
	}

	forgetHistory(historyKey(mangaID, provider))
	historyMu.Unlock()

	return FlushHistory()
}

// Debounce disk writes: restart timer on every call, write 2s after the last update.
func scheduleFlush() {
	if flushTimer != nil {
		flushTimer.Stop()
	}
	flushTimer = time.AfterFunc(flushDelay, func() {
		if err := FlushHistory(); err != nil {
			fmt.Fprintf(os.Stderr, "futon: flush history: %v\n", err)
		}
	})
}
