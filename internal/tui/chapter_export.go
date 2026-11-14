package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/export"
	"github.com/KabosuNeko/Futon/internal/models"
)

type cbzExportedMsg struct {
	path string
	err  error
}

// chapterLabel names a chapter for display, history and file names. HTML
// providers omit chapter numbers (titles only); without this every chapter
// of theirs would export to the same "unknown" file.
func chapterLabel(ch models.Chapter) string {
	if ch.Number != "" {
		return ch.Number
	}
	if ch.Title != "" {
		return ch.Title
	}
	return "unknown"
}

// exportChapterCBZ downloads every page of a chapter and writes it as a CBZ
// into destDir ("" for the default export directory).
func exportChapterCBZ(ctx context.Context, provider api.MangaProvider, chapterID, mangaTitle, label, destDir string) (string, error) {
	urls, err := provider.FetchPages(ctx, chapterID)
	if err != nil {
		return "", fmt.Errorf("lấy link ảnh: %w", err)
	}
	images, err := api.DownloadChapterPages(ctx, provider, chapterID, urls)
	if err != nil {
		return "", err
	}
	return export.ExportImagesToCBZ(mangaTitle, label, images, destDir)
}

func exportChapterCmd(provider api.MangaProvider, chapterID, mangaTitle, chapterNumber string) tea.Cmd {
	return func() tea.Msg {
		path, err := exportChapterCBZ(context.Background(), provider, chapterID, mangaTitle, chapterNumber, "")
		return cbzExportedMsg{path: path, err: err}
	}
}

// batchExportMsg reports a ctrl+a export, one message per chapter; next waits
// for the following one and is nil on the final message.
type batchExportMsg struct {
	mangaID  string
	done     int
	failed   []string
	total    int
	destDir  string
	err      error
	canceled bool
	final    bool
	next     tea.Cmd
}

// exportAllChaptersCmd exports every chapter into ~/Downloads/Futon/{Title}/,
// streaming one batchExportMsg per chapter. Cancelling ctx stops it after the
// chapter in progress.
func exportAllChaptersCmd(ctx context.Context, mangaID string, provider api.MangaProvider, mangaTitle string, chapters []models.Chapter) tea.Cmd {
	return func() tea.Msg {
		baseDir, err := export.GetDefaultExportDir()
		if err != nil {
			baseDir = "."
		}
		mangaDir := filepath.Join(baseDir, export.SanitizeFilename(mangaTitle))
		if err := os.MkdirAll(mangaDir, 0o755); err != nil {
			return batchExportMsg{mangaID: mangaID, final: true, err: fmt.Errorf("tạo thư mục truyện: %w", err)}
		}

		// Buffered for every message, so an abandoned export never blocks.
		updates := make(chan batchExportMsg, len(chapters)+1)
		go func() {
			defer close(updates)
			progress := batchExportMsg{mangaID: mangaID, total: len(chapters), destDir: mangaDir}
			// Chapters with the same label (oneshots without number or title)
			// would otherwise overwrite each other's file.
			used := make(map[string]bool, len(chapters))
			for _, ch := range chapters {
				if ctx.Err() != nil {
					break
				}
				label := chapterLabel(ch)
				for n := 2; used[label]; n++ {
					label = fmt.Sprintf("%s_%d", chapterLabel(ch), n)
				}
				used[label] = true
				_, err := exportChapterCBZ(ctx, provider, ch.ID, mangaTitle, label, mangaDir)
				if ctx.Err() != nil {
					break
				}
				if err != nil {
					progress.failed = append(append([]string(nil), progress.failed...), label)
				} else {
					progress.done++
				}
				updates <- progress
			}
			progress.canceled = ctx.Err() != nil
			progress.final = true
			updates <- progress
		}()
		return waitBatchExport(updates)()
	}
}

func waitBatchExport(updates <-chan batchExportMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-updates
		if !ok {
			return nil
		}
		if !msg.final {
			msg.next = waitBatchExport(updates)
		}
		return msg
	}
}

func (m ChapterListModel) handleBatchExport(msg batchExportMsg) (tea.Model, tea.Cmd) {
	if msg.mangaID != m.mangaID {
		return m, msg.next // an export of a manga no longer shown; let it drain
	}
	if !msg.final {
		m.flashMsg = fmt.Sprintf("Đang xuất %d/%d chapters ra CBZ... [esc] Hủy", msg.done+len(msg.failed), msg.total)
		return m, msg.next
	}

	m.cancelExport = nil
	dir := export.ShortPath(msg.destDir)
	switch {
	case msg.err != nil:
		m.flashMsg = fmt.Sprintf("Lỗi xuất toàn bộ CBZ: %v", msg.err)
	case msg.canceled:
		m.flashMsg = fmt.Sprintf("Đã hủy xuất CBZ (%d/%d chapters xong)", msg.done, msg.total)
	case len(msg.failed) > 0:
		// Offline, every chapter fails; a full list would push the screen apart.
		failed := strings.Join(msg.failed[:min(5, len(msg.failed))], ", ")
		if extra := len(msg.failed) - 5; extra > 0 {
			failed += fmt.Sprintf(" +%d", extra)
		}
		m.flashMsg = fmt.Sprintf("Đã xuất %d/%d chapters vào %s; lỗi: %s", msg.done, msg.total, dir, failed)
	default:
		m.flashMsg = fmt.Sprintf("Đã xuất %d/%d chapters vào %s", msg.done, msg.total, dir)
	}
	return m, clearFlashAfter(5 * time.Second)
}
