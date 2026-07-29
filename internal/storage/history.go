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

	tea "github.com/charmbracelet/bubbletea"
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

type HistorySavedMsg struct {
	Err error
}

var (
	historyMu     sync.RWMutex
	historyCache  map[string]ReadHistory
	historyLoaded bool
	flushTimer    *time.Timer
)

const flushDelay = 2 * time.Second

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
		historyCache[e.MangaID] = e
	}
	historyLoaded = true
	return nil
}

// SaveHistory memoizes in RAM, flushes to disk later (async — no UI blocking).
func SaveHistory(mangaID, mangaTitle, provider, chapterID, chapterNumber string, pageIndex int) error {
	if mangaID == "" || chapterID == "" {
		return nil
	}

	historyMu.Lock()
	defer historyMu.Unlock()

	if err := loadHistory(); err != nil {
		return err
	}

	// Keep old title/number if caller forgot to tell us (happens more than you'd think).
	if old, ok := historyCache[mangaID]; ok {
		if mangaTitle == "" {
			mangaTitle = old.MangaTitle
		}
		if chapterNumber == "" {
			chapterNumber = old.ChapterNumber
		}
	}

	historyCache[mangaID] = ReadHistory{
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

func SaveHistoryCmd(mangaID, mangaTitle, provider, chapterID, chapterNumber string, pageIndex int) tea.Cmd {
	return func() tea.Msg {
		return HistorySavedMsg{Err: SaveHistory(mangaID, mangaTitle, provider, chapterID, chapterNumber, pageIndex)}
	}
}

func GetHistory(mangaID string) (*ReadHistory, bool) {
	historyMu.Lock()
	defer historyMu.Unlock()

	if err := loadHistory(); err != nil {
		return nil, false
	}

	h, ok := historyCache[mangaID]
	if !ok {
		return nil, false
	}

	// Return a copy so callers can't poke the cache directly.
	return &ReadHistory{
		MangaID:       h.MangaID,
		MangaTitle:    h.MangaTitle,
		Provider:      h.Provider,
		ChapterID:     h.ChapterID,
		ChapterNumber: h.ChapterNumber,
		PageIndex:     h.PageIndex,
		UpdatedAt:     h.UpdatedAt,
	}, true
}

// LoadAllHistory — sort newest first, because nobody cares about what they read last year.
func LoadAllHistory() ([]ReadHistory, error) {
	historyMu.Lock()
	defer historyMu.Unlock()

	if err := loadHistory(); err != nil {
		return nil, err
	}

	entries := make([]ReadHistory, 0, len(historyCache))
	for _, e := range historyCache {
		entries = append(entries, ReadHistory{
			MangaID:       e.MangaID,
			MangaTitle:    e.MangaTitle,
			Provider:      e.Provider,
			ChapterID:     e.ChapterID,
			ChapterNumber: e.ChapterNumber,
			PageIndex:     e.PageIndex,
			UpdatedAt:     e.UpdatedAt,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].UpdatedAt > entries[j].UpdatedAt
	})

	return entries, nil
}

func FlushHistory() error {
	historyMu.RLock()
	snapshot := make([]ReadHistory, 0, len(historyCache))
	for _, e := range historyCache {
		snapshot = append(snapshot, e)
	}
	historyMu.RUnlock()

	path, err := historyPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode history: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("ghi file history: %w", err)
	}
	return nil
}

func FlushHistoryCmd() tea.Cmd {
	return func() tea.Msg {
		return HistorySavedMsg{Err: FlushHistory()}
	}
}

func DeleteHistory(mangaID string) error {
	if mangaID == "" {
		return nil
	}

	historyMu.Lock()
	if err := loadHistory(); err != nil {
		historyMu.Unlock()
		return err
	}

	delete(historyCache, mangaID)
	historyMu.Unlock()

	return FlushHistory()
}

func DeleteHistoryCmd(mangaID string) tea.Cmd {
	return func() tea.Msg {
		return HistorySavedMsg{Err: DeleteHistory(mangaID)}
	}
}

// Debounce disk writes: restart timer on every call, write 2s after the last update.
func scheduleFlush() {
	if flushTimer != nil {
		flushTimer.Stop()
	}
	flushTimer = time.AfterFunc(flushDelay, func() {
		_ = FlushHistory()
	})
}
