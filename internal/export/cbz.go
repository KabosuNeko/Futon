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
	"unicode/utf8"
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

// maxNameBytes keeps names under the 255-byte file name limit of Linux and
// macOS file systems, with room for an extension and a temp-file suffix.
// Vietnamese letters take 2-3 bytes each, so long titles do hit it.
const maxNameBytes = 200

// SanitizeFilename cleans manga or chapter titles for safe file system usage.
func SanitizeFilename(name string) string {
	// No leading dots: ".." would point at the parent folder and ".x" hides the file.
	s := strings.TrimLeft(strings.TrimSpace(truncateBytes(filenameReplacer.Replace(name), maxNameBytes)), ". ")
	if s == "" {
		return "Untitled"
	}
	return s
}

// truncateBytes cuts s to at most n bytes without splitting a character.
func truncateBytes(s string, n int) string {
	for len(s) > n {
		_, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
	}
	return s
}

// FileStem names a chapter's files: "{Title}_{Chapter}". Some sources repeat
// the title in the chapter name, so that prefix is dropped. When the name is
// too long the title is shortened, never the chapter: two chapters must not
// end up with the same name and overwrite each other.
func FileStem(mangaTitle, chapterLabel string) string {
	title := strings.TrimSpace(mangaTitle)
	label := strings.TrimSpace(chapterLabel)
	if title != "" && len(label) > len(title) && strings.HasPrefix(label, title) &&
		strings.ContainsRune(" -:", rune(label[len(title)])) { // "One" must not cut "Oneshot"
		label = strings.TrimLeft(label[len(title):], " -:")
	}
	chapter := truncateBytes(SanitizeFilename(ChapterFileLabel(label)), maxNameBytes/2)
	return SanitizeFilename(truncateBytes(SanitizeFilename(title), maxNameBytes-1-len(chapter))) + "_" + chapter
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

// GetDefaultExportDir returns {Downloads}/Futon, creating it if necessary.
func GetDefaultExportDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(downloadsDir(home), "Futon")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ".", err
	}
	return dir, nil
}

// downloadsDir is the user's download folder: XDG_DOWNLOAD_DIR from the
// environment or from user-dirs.dirs (localized, e.g. "~/Tải xuống" on a
// Vietnamese desktop), else ~/Downloads.
func downloadsDir(home string) string {
	if dir := os.Getenv("XDG_DOWNLOAD_DIR"); filepath.IsAbs(dir) {
		return dir
	}
	if cfg, err := os.UserConfigDir(); err == nil {
		if data, err := os.ReadFile(filepath.Join(cfg, "user-dirs.dirs")); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				v, ok := strings.CutPrefix(strings.TrimSpace(line), "XDG_DOWNLOAD_DIR=")
				if !ok {
					continue
				}
				v = filepath.Clean(strings.Replace(strings.Trim(v, `"`), "$HOME", home, 1))
				// xdg-user-dirs points a disabled folder at $HOME itself.
				if filepath.IsAbs(v) && v != filepath.Clean(home) {
					return v
				}
			}
		}
	}
	return filepath.Join(home, "Downloads")
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

	filename := FileStem(mangaTitle, chapterNumber) + ".cbz"
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
