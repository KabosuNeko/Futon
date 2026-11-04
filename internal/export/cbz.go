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

// ChapterFileLabel names a chapter in file names: "Chap_12" for a bare
// number, or the label itself ("Chương 12", "Chap 150") when it already says
// what it is, so names never read "Chap_Chương 12".
func ChapterFileLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return "unknown"
	}
	if r := label[0]; r >= '0' && r <= '9' {
		return "Chap_" + label
	}
	return label
}

// ShortPath shows a path with the home directory as "~", for messages.
func ShortPath(path string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if rel, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.Join("~", rel)
		}
	}
	return path
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

// ExportImagesToCBZ packs already-downloaded page images into a .cbz file.
// Every page must be present: a CBZ with silently missing pages is worse than
// an error. The archive is written to a temp file and renamed into place, so a
// failed export never leaves a truncated .cbz behind.
func ExportImagesToCBZ(mangaTitle, chapterNumber string, images [][]byte, destDir string) (string, error) {
	if len(images) == 0 {
		return "", fmt.Errorf("không có ảnh nào để xuất")
	}
	for i, img := range images {
		if len(img) == 0 {
			return "", fmt.Errorf("thiếu ảnh trang %d", i+1)
		}
	}

	if destDir == "" {
		var err error
		destDir, err = GetDefaultExportDir()
		if err != nil {
			destDir = "."
		}
	}

	filename := fmt.Sprintf("%s_%s.cbz", SanitizeFilename(mangaTitle), SanitizeFilename(ChapterFileLabel(chapterNumber)))
	targetPath := filepath.Join(destDir, filename)

	tmp, err := os.CreateTemp(destDir, filename+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("tạo file cbz: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op after a successful rename

	if err := writeCBZ(tmp, mangaTitle, chapterNumber, images); err != nil {
		tmp.Close()
		return "", fmt.Errorf("ghi file cbz: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("ghi file cbz: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o644); err != nil {
		return "", fmt.Errorf("ghi file cbz: %w", err)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return "", fmt.Errorf("ghi file cbz: %w", err)
	}
	return targetPath, nil
}

func writeCBZ(w io.Writer, mangaTitle, chapterNumber string, images [][]byte) error {
	zw := zip.NewWriter(w)

	// Metadata keeps the original titles; only the file name is sanitized.
	comicInfoXML := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<ComicInfo xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <Title>Chapter %s</Title>
  <Series>%s</Series>
  <Number>%s</Number>
  <PageCount>%d</PageCount>
</ComicInfo>`, html.EscapeString(chapterNumber), html.EscapeString(mangaTitle), html.EscapeString(chapterNumber), len(images))

	entry, err := zw.Create("ComicInfo.xml")
	if err != nil {
		return err
	}
	if _, err := entry.Write([]byte(comicInfoXML)); err != nil {
		return err
	}

	for i, imgBytes := range images {
		entry, err := zw.Create(fmt.Sprintf("%03d.%s", i+1, DetectImageExt(imgBytes)))
		if err != nil {
			return err
		}
		if _, err := entry.Write(imgBytes); err != nil {
			return err
		}
	}
	return zw.Close()
}

// DetectImageExt returns the file extension matching the image bytes.
func DetectImageExt(data []byte) string {
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
