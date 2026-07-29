package tui

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

// MangaDex returns placeholder images unless you send this exact UA + no Referer.
const mangaDexImageUserAgent = "Futon-App/1.0 (https://github.com/KabosuNeko/Futon)"

const kittyClearSeq = "\x1b_Ga=d;\x1b\\"

var filenameReplacer = strings.NewReplacer(
	"/", "-",
	"\\", "-",
	":", "-",
	"*", "-",
	"?", "-",
	"\"", "-",
	"<", "-",
	">", "-",
	"|", "-",
)

func downloadImageBytes(url, referer, userAgent string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	} else {
		req.Header.Set("User-Agent", "Futon-App/1.0")
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func downloadOne(url string, index int, referer, userAgent string) tea.Cmd {
	return func() tea.Msg {
		data, err := downloadImageBytes(url, referer, userAgent)
		if err != nil {
			return downloadProgressMsg{index: index, err: fmt.Errorf("tải ảnh %d: %w", index+1, err)}
		}
		return downloadProgressMsg{data: data, index: index}
	}
}

func renderPage(r imgrender.Renderer, imgData []byte, index int) tea.Cmd {
	return func() tea.Msg {
		img, err := r.Render(imgData)
		if err != nil {
			return renderDoneMsg{index: index, err: fmt.Errorf("render trang %d: %w", index+1, err)}
		}
		return renderDoneMsg{index: index, img: img}
	}
}

func preloadNextChapter(nextID string, provider api.MangaProvider) tea.Cmd {
	return func() tea.Msg {
		urls, err := provider.FetchPages(nextID)
		if err != nil {
			return PreloadCompleteMsg{ChapID: nextID}
		}

		preloadCount := 2 // Preload 2 pages — enough for instant flip, light on bandwidth.
		if len(urls) < preloadCount {
			preloadCount = len(urls)
		}

		var referer, userAgent string
		if provider != nil && provider.Name() == "MangaDex" {
			userAgent = mangaDexImageUserAgent
		} else {
			referer = nextID
		}

		images := make([][]byte, 0, preloadCount)
		for i := 0; i < preloadCount && i < len(urls); i++ {
			data, err := downloadImageBytes(urls[i], referer, userAgent)
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

func chapterNavCmd(chapterID, mangaID, mangaTitle string, allChapterIDs []string, chapterIndex, startPage int) tea.Cmd {
	return func() tea.Msg {
		return ViewChapterMsg{
			MangaID:        mangaID,
			MangaTitle:     mangaTitle,
			ChapterID:      chapterID,
			ChapterNumber:  "",
			AllChapterIDs:  allChapterIDs,
			ChapterIndex:   chapterIndex,
			StartPageIndex: startPage,
		}
	}
}

func clearGraphicsCmd() tea.Cmd {
	return func() tea.Msg {
		fmt.Print(kittyClearSeq)
		return clearDoneMsg{}
	}
}

func clearScreenCmd() tea.Cmd {
	return func() tea.Msg {
		fmt.Print(kittyClearSeq)
		fmt.Print("\x1b[H\x1b[2J")
		return clearDoneMsg{}
	}
}

func saveImageCmd(data []byte, mangaTitle, chapterNumber string, pageNumber int) tea.Cmd {
	return func() tea.Msg {
		home, err := os.UserHomeDir()
		if err != nil {
			return imageSavedMsg{err: fmt.Errorf("lấy thư mục home: %w", err)}
		}
		dir := filepath.Join(home, "Downloads", "Futon_Downloads")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return imageSavedMsg{err: fmt.Errorf("tạo thư mục download: %w", err)}
		}

		safeTitle := sanitizeFilename(mangaTitle)
		if safeTitle == "" {
			safeTitle = "manga"
		}
		ch := sanitizeFilename(chapterNumber)
		if ch == "" {
			ch = "unknown"
		}
		filename := fmt.Sprintf("%s_Ch%s_Pg%d.jpg", safeTitle, ch, pageNumber)
		path := filepath.Join(dir, filename)

		// If filename exists, append suffix up to 999 before giving up.
		for suffix := 1; suffix < 1000; suffix++ {
			if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
				break
			}
			filename = fmt.Sprintf("%s_Ch%s_Pg%d_%d.jpg", safeTitle, ch, pageNumber, suffix)
			path = filepath.Join(dir, filename)
		}

		if err := os.WriteFile(path, data, 0o644); err != nil {
			return imageSavedMsg{err: fmt.Errorf("ghi file ảnh: %w", err)}
		}
		return imageSavedMsg{path: path}
	}
}

func sanitizeFilename(name string) string {
	return filenameReplacer.Replace(name)
}
