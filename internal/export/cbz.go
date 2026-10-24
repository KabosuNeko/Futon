package export

import (
	"archive/zip"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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

// SanitizeFilename cleans manga or chapter titles for safe file system usage.
func SanitizeFilename(name string) string {
	s := filenameReplacer.Replace(name)
	s = strings.TrimSpace(s)
	if s == "" {
		return "Untitled"
	}
	return s
}

// GetDefaultExportDir returns the ~/Downloads/Futon directory, creating it if necessary.
func GetDefaultExportDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, "Downloads", "Futon")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ".", err
	}
	return dir, nil
}

// ExportImagesToCBZ packs a list of already-downloaded image byte slices into a .cbz file.
func ExportImagesToCBZ(mangaTitle, chapterNumber string, images [][]byte, destDir string) (string, error) {
	if len(images) == 0 {
		return "", fmt.Errorf("không có ảnh nào để xuất")
	}

	if destDir == "" {
		var err error
		destDir, err = GetDefaultExportDir()
		if err != nil {
			destDir = "."
		}
	}

	cleanTitle := SanitizeFilename(mangaTitle)
	cleanChap := SanitizeFilename(chapterNumber)
	filename := fmt.Sprintf("%s_Chap_%s.cbz", cleanTitle, cleanChap)
	targetPath := filepath.Join(destDir, filename)

	f, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("tạo file cbz: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)

	// Write ComicInfo.xml. Values are sanitized for the file system, not for
	// XML: titles such as "Tom & Jerry" must be escaped here.
	comicInfoXML := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<ComicInfo xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <Title>Chapter %s</Title>
  <Series>%s</Series>
  <Number>%s</Number>
  <PageCount>%d</PageCount>
</ComicInfo>`, html.EscapeString(cleanChap), html.EscapeString(cleanTitle), html.EscapeString(cleanChap), len(images))

	xmlEntry, err := zw.Create("ComicInfo.xml")
	if err == nil {
		_, _ = xmlEntry.Write([]byte(comicInfoXML))
	}

	// Write images
	for i, imgBytes := range images {
		if len(imgBytes) == 0 {
			continue
		}
		ext := detectImageExt(imgBytes)
		entryName := fmt.Sprintf("%03d.%s", i+1, ext)

		entry, err := zw.Create(entryName)
		if err != nil {
			continue
		}
		_, _ = entry.Write(imgBytes)
	}

	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("hoàn tất ghi file cbz: %w", err)
	}

	return targetPath, nil
}

const (
	defaultImageUserAgent  = "Futon-App/1.0"
	mangaDexImageUserAgent = "Futon-App/1.0 (https://github.com/KabosuNeko/Futon)"
)

// imageClient bounds page downloads; a stalled CDN must not hang an export.
var imageClient = &http.Client{Timeout: 15 * time.Second}

// DownloadImage fetches a page image with the headers its provider expects.
func DownloadImage(rawURL, providerName, chapterURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	userAgent := defaultImageUserAgent
	referer := ""
	if strings.EqualFold(providerName, "MangaDex") {
		// MangaDex serves placeholders when a Referer is present.
		userAgent = mangaDexImageUserAgent
	} else if strings.HasPrefix(chapterURL, "http://") || strings.HasPrefix(chapterURL, "https://") {
		referer = chapterURL
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := imageClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// ExportChapterURLsToCBZ downloads images from urls and packages them into a .cbz file.
func ExportChapterURLsToCBZ(mangaTitle, chapterNumber string, urls []string, destDir, providerName, chapterURL string) (string, error) {
	if len(urls) == 0 {
		return "", fmt.Errorf("danh sách URL ảnh trống")
	}

	var images [][]byte
	for _, u := range urls {
		data, err := DownloadImage(u, providerName, chapterURL)
		if err != nil || len(data) == 0 {
			continue
		}
		images = append(images, data)
	}

	if len(images) == 0 {
		return "", fmt.Errorf("không tải được ảnh nào từ chapter")
	}

	return ExportImagesToCBZ(mangaTitle, chapterNumber, images, destDir)
}

func detectImageExt(data []byte) string {
	switch http.DetectContentType(data) {
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	default:
		return "jpg"
	}
}
