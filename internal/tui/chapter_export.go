package tui

import (
	"context"
	"fmt"

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
