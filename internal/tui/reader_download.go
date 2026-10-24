package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/export"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

const kittyClearSeq = "\x1b_Ga=d,q=2;\x1b\\"

func downloadOne(url string, index int, chapterID, providerName string) tea.Cmd {
	return func() tea.Msg {
		data, err := export.DownloadImage(url, providerName, chapterID)
		if err != nil {
			return downloadProgressMsg{index: index, chapterID: chapterID, err: fmt.Errorf("tải ảnh %d: %w", index+1, err)}
		}
		return downloadProgressMsg{data: data, index: index, chapterID: chapterID}
	}
}

func renderPage(r imgrender.Renderer, imgData []byte, index int, chapterID string, rows int) tea.Cmd {
	return func() tea.Msg {
		img, err := r.RenderInBox(imgData, 0, rows)
		if err != nil {
			return renderDoneMsg{index: index, chapterID: chapterID, err: fmt.Errorf("render trang %d: %w", index+1, err)}
		}
		return renderDoneMsg{index: index, chapterID: chapterID, img: img}
	}
}

// preloadNextChapter fetches the next chapter's pages before the user asks.
func preloadNextChapter(nextID string, provider api.MangaProvider) tea.Cmd {
	return func() tea.Msg {
		urls, err := provider.FetchPages(nextID)
		if err != nil {
			return PreloadCompleteMsg{ChapID: nextID}
		}

		// Preload 2 pages — enough for instant flip, light on bandwidth.
		preloadCount := min(2, len(urls))

		images := make([][]byte, 0, preloadCount)
		for i := 0; i < preloadCount; i++ {
			data, err := export.DownloadImage(urls[i], provider.Name(), nextID)
			if err != nil {
				break
			}
			images = append(images, data)
		}

		if len(images) == 0 {
			return PreloadCompleteMsg{ChapID: nextID}
		}
		return PreloadCompleteMsg{
			ChapID: nextID,
			URLs:   urls,
			Images: images,
		}
	}
}

func chapterNavCmd(chapterID, mangaID, mangaTitle string, allChapterIDs, allChapterNumbers []string, chapterIndex, startPage int) tea.Cmd {
	chapterNumber := ""
	if chapterIndex >= 0 && chapterIndex < len(allChapterNumbers) {
		chapterNumber = allChapterNumbers[chapterIndex]
	}
	return func() tea.Msg {
		return ViewChapterMsg{
			MangaID:           mangaID,
			MangaTitle:        mangaTitle,
			ChapterID:         chapterID,
			ChapterNumber:     chapterNumber,
			AllChapterIDs:     allChapterIDs,
			AllChapterNumbers: allChapterNumbers,
			ChapterIndex:      chapterIndex,
			StartPageIndex:    startPage,
		}
	}
}

// clearGraphicsCmd deletes every terminal image without touching text.
func clearGraphicsCmd() tea.Cmd {
	return tea.Raw(kittyClearSeq)
}

// clearScreenCmd deletes every terminal image and repaints the screen blank so
// the next rendered screen is not ghosted by stale image pixels.
func clearScreenCmd() tea.Cmd {
	return tea.Raw(kittyClearSeq + "\x1b[H\x1b[2J")
}

func saveImageCmd(data []byte, mangaTitle, chapterNumber string, pageNumber int) tea.Cmd {
	return func() tea.Msg {
		dir, err := export.GetDefaultExportDir()
		if err != nil {
			return imageSavedMsg{err: fmt.Errorf("tạo thư mục download: %w", err)}
		}

		safeTitle := export.SanitizeFilename(mangaTitle)
		if safeTitle == "Untitled" {
			safeTitle = "manga"
		}
		ch := export.SanitizeFilename(chapterNumber)
		if ch == "Untitled" {
			ch = "unknown"
		}
		path := filepath.Join(dir, fmt.Sprintf("%s_Ch%s_Pg%d.jpg", safeTitle, ch, pageNumber))

		// If filename exists, append suffix up to 999 before giving up.
		for suffix := 1; suffix < 1000; suffix++ {
			if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
				break
			}
			path = filepath.Join(dir, fmt.Sprintf("%s_Ch%s_Pg%d_%d.jpg", safeTitle, ch, pageNumber, suffix))
		}

		if err := os.WriteFile(path, data, 0o644); err != nil {
			return imageSavedMsg{err: fmt.Errorf("ghi file ảnh: %w", err)}
		}
		return imageSavedMsg{path: path}
	}
}
