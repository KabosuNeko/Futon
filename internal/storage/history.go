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

// ReadHistory lưu tiến độ đọc gần nhất của một bộ truyện.
type ReadHistory struct {
	MangaID       string `json:"manga_id"`
	MangaTitle    string `json:"manga_title,omitempty"`
	ChapterID     string `json:"chapter_id"`
	ChapterNumber string `json:"chapter_number,omitempty"`
	PageIndex     int    `json:"page_index"`
	UpdatedAt     int64  `json:"updated_at"`
}

// HistorySavedMsg được gửi khi việc ghi lịch sử hoàn tất (thành công hoặc lỗi).
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

// SaveHistory cập nhật cache trong bộ nhớ và lên lịch flush ra đĩa.
// Không block UI vì Bubble Tea chạy tea.Cmd trong goroutine.
func SaveHistory(mangaID, mangaTitle, chapterID, chapterNumber string, pageIndex int) error {
	if mangaID == "" || chapterID == "" {
		return nil
	}

	historyMu.Lock()
	defer historyMu.Unlock()

	if err := loadHistory(); err != nil {
		return err
	}

	// Giữ lại title/chapterNumber cũ nếu caller không cung cấp.
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
		ChapterID:     chapterID,
		ChapterNumber: chapterNumber,
		PageIndex:     pageIndex,
		UpdatedAt:     time.Now().Unix(),
	}

	scheduleFlush()
	return nil
}

// SaveHistoryCmd trả về tea.Cmd để lưu lịch sử bất đồng bộ.
func SaveHistoryCmd(mangaID, mangaTitle, chapterID, chapterNumber string, pageIndex int) tea.Cmd {
	return func() tea.Msg {
		return HistorySavedMsg{Err: SaveHistory(mangaID, mangaTitle, chapterID, chapterNumber, pageIndex)}
	}
}

// GetHistory lấy lịch sử đọc của một bộ truyện.
func GetHistory(mangaID string) (*ReadHistory, bool) {
	historyMu.RLock()
	defer historyMu.RUnlock()

	if err := loadHistory(); err != nil {
		return nil, false
	}

	h, ok := historyCache[mangaID]
	if !ok {
		return nil, false
	}

	// Trả về bản sao để tránh caller sửa cache trực tiếp.
	return &ReadHistory{
		MangaID:       h.MangaID,
		MangaTitle:    h.MangaTitle,
		ChapterID:     h.ChapterID,
		ChapterNumber: h.ChapterNumber,
		PageIndex:     h.PageIndex,
		UpdatedAt:     h.UpdatedAt,
	}, true
}

// LoadAllHistory trả về toàn bộ lịch sử đọc, sắp xếp mới nhất lên đầu.
func LoadAllHistory() ([]ReadHistory, error) {
	historyMu.RLock()
	defer historyMu.RUnlock()

	if err := loadHistory(); err != nil {
		return nil, err
	}

	entries := make([]ReadHistory, 0, len(historyCache))
	for _, e := range historyCache {
		entries = append(entries, ReadHistory{
			MangaID:       e.MangaID,
			MangaTitle:    e.MangaTitle,
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

// FlushHistory ghi cache ra đĩa ngay lập tức.
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

// FlushHistoryCmd trả về tea.Cmd để flush lịch sử bất đồng bộ.
func FlushHistoryCmd() tea.Cmd {
	return func() tea.Msg {
		return HistorySavedMsg{Err: FlushHistory()}
	}
}

// DeleteHistory xóa lịch sử đọc của một bộ truyện.
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

	// Copy snapshot và ghi file ngoài lock để tránh deadlock (FlushHistory lấy RLock).
	snapshot := make([]ReadHistory, 0, len(historyCache))
	for _, e := range historyCache {
		snapshot = append(snapshot, e)
	}
	historyMu.Unlock()

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

// DeleteHistoryCmd trả về tea.Cmd để xóa lịch sử bất đồng bộ.
func DeleteHistoryCmd(mangaID string) tea.Cmd {
	return func() tea.Msg {
		return HistorySavedMsg{Err: DeleteHistory(mangaID)}
	}
}

func scheduleFlush() {
	if flushTimer != nil {
		flushTimer.Stop()
	}
	flushTimer = time.AfterFunc(flushDelay, func() {
		_ = FlushHistory()
	})
}
