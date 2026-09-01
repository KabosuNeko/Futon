package export

import (
	"archive/zip"
	"bytes"
	"fmt"
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

	// Write ComicInfo.xml
	comicInfoXML := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<ComicInfo xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <Title>Chapter %s</Title>
  <Series>%s</Series>
  <Number>%s</Number>
  <PageCount>%d</PageCount>
</ComicInfo>`, cleanChap, cleanTitle, cleanChap, len(images))

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

// ExportChapterURLsToCBZ downloads images from urls and packages them into a .cbz file.
func ExportChapterURLsToCBZ(mangaTitle, chapterNumber string, urls []string, referer, userAgent, destDir string) (string, error) {
	if len(urls) == 0 {
		return "", fmt.Errorf("danh sách URL ảnh trống")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	var images [][]byte

	for _, u := range urls {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			continue
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

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err == nil && len(data) > 0 {
			images = append(images, data)
		}
	}

	if len(images) == 0 {
		return "", fmt.Errorf("không tải được ảnh nào từ chapter")
	}

	return ExportImagesToCBZ(mangaTitle, chapterNumber, images, destDir)
}

func detectImageExt(data []byte) string {
	if len(data) > 4 {
		if bytes.HasPrefix(data, []byte("\xff\xd8\xff")) {
			return "jpg"
		}
		if bytes.HasPrefix(data, []byte("\x89PNG")) {
			return "png"
		}
		if bytes.HasPrefix(data, []byte("GIF8")) {
			return "gif"
		}
		if len(data) > 12 && string(data[8:12]) == "WEBP" {
			return "webp"
		}
	}
	return "jpg"
}
