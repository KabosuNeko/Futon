package export

import (
	"archive/zip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportImagesToCBZ(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "futon_export_test_*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fakeImg1 := []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00")
	fakeImg2 := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01")
	images := [][]byte{fakeImg1, fakeImg2}

	targetPath, err := ExportImagesToCBZ("One / Piece: Test", "100.5", images, tempDir)
	if err != nil {
		t.Fatalf("ExportImagesToCBZ error: %v", err)
	}

	if !strings.HasSuffix(targetPath, "One - Piece- Test_Chap_100.5.cbz") {
		t.Errorf("unexpected filename: %s", targetPath)
	}

	// Verify zip contents
	zr, err := zip.OpenReader(targetPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()

	foundEntries := make(map[string]bool)
	var comicInfoContent string
	for _, f := range zr.File {
		foundEntries[f.Name] = true
		if f.Name == "ComicInfo.xml" {
			rc, err := f.Open()
			if err == nil {
				b, _ := io.ReadAll(rc)
				comicInfoContent = string(b)
				rc.Close()
			}
		}
	}

	if !foundEntries["ComicInfo.xml"] {
		t.Errorf("missing ComicInfo.xml in cbz")
	}
	if !foundEntries["001.jpg"] {
		t.Errorf("missing 001.jpg in cbz")
	}
	if !foundEntries["002.png"] {
		t.Errorf("missing 002.png in cbz")
	}

	if !strings.Contains(comicInfoContent, "<Series>One - Piece- Test</Series>") {
		t.Errorf("ComicInfo missing series title, got:\n%s", comicInfoContent)
	}
	if !strings.Contains(comicInfoContent, "<PageCount>2</PageCount>") {
		t.Errorf("ComicInfo missing page count 2, got:\n%s", comicInfoContent)
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Manga/Title: Episode 1", "Manga-Title- Episode 1"},
		{"<Illegal> | * \" ? \\", "-Illegal- - - - - -"},
		{"", "Untitled"},
		{"  Spaces  ", "Spaces"},
	}

	for _, c := range cases {
		if got := SanitizeFilename(c.input); got != c.want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestGetDefaultExportDir(t *testing.T) {
	dir, err := GetDefaultExportDir()
	if err != nil {
		t.Fatalf("GetDefaultExportDir error: %v", err)
	}
	if !strings.HasSuffix(dir, filepath.Join("Downloads", "Futon")) {
		t.Errorf("unexpected export dir: %s", dir)
	}
}

func TestDetectImageExt(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"jpeg", []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01"), "jpg"},
		{"png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), "png"},
		{"gif", []byte("GIF89a\x01\x00\x01\x00"), "gif"},
		{"webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), "webp"},
		{"unknown", []byte("not an image"), "jpg"},
	}

	for _, c := range cases {
		if got := detectImageExt(c.data); got != c.want {
			t.Errorf("detectImageExt(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestExportEscapesXMLMetadata(t *testing.T) {
	dir := t.TempDir()
	images := [][]byte{[]byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01")}

	path, err := ExportImagesToCBZ("Tom & Jerry", "1&2", images, dir)
	if err != nil {
		t.Fatalf("ExportImagesToCBZ error: %v", err)
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()

	var comicInfo string
	for _, f := range zr.File {
		if f.Name != "ComicInfo.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open ComicInfo.xml: %v", err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read ComicInfo.xml: %v", err)
		}
		comicInfo = string(data)
	}

	for _, want := range []string{
		"<Series>Tom &amp; Jerry</Series>",
		"<Number>1&amp;2</Number>",
	} {
		if !strings.Contains(comicInfo, want) {
			t.Errorf("ComicInfo.xml missing %q:\n%s", want, comicInfo)
		}
	}
}

func TestImageClientHasTimeout(t *testing.T) {
	if imageClient.Timeout <= 0 {
		t.Fatal("image client has no timeout; a stalled CDN would hang an export")
	}
}

func TestDownloadImageMangaDexHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != mangaDexImageUserAgent {
			t.Errorf("User-Agent = %q, want %q", got, mangaDexImageUserAgent)
		}
		if got := r.Header.Get("Referer"); got != "" {
			t.Errorf("MangaDex Referer = %q, want none", got)
		}
		w.Write([]byte("image-bytes"))
	}))
	defer srv.Close()

	data, err := DownloadImage(srv.URL, "MangaDex", "https://mangadex.org/chapter/123")
	if err != nil {
		t.Fatalf("DownloadImage error: %v", err)
	}
	if string(data) != "image-bytes" {
		t.Errorf("data = %q", data)
	}
}

func TestDownloadImageUsesChapterReferer(t *testing.T) {
	const chapterURL = "https://truyenqqko.com/truyen/one-piece/chuong-1.html"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Referer"); got != chapterURL {
			t.Errorf("Referer = %q, want %q", got, chapterURL)
		}
		if got := r.Header.Get("User-Agent"); got != defaultImageUserAgent {
			t.Errorf("User-Agent = %q, want %q", got, defaultImageUserAgent)
		}
		w.Write([]byte("image-bytes"))
	}))
	defer srv.Close()

	if _, err := DownloadImage(srv.URL, "TruyenQQ", chapterURL); err != nil {
		t.Fatalf("DownloadImage error: %v", err)
	}
}

func TestExportChapterURLsSendsProviderHeaders(t *testing.T) {
	const chapterURL = "https://truyenggvn.com/truyen-tranh/one-piece-chap-1"
	jpg := []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Referer"); got != chapterURL {
			http.Error(w, "missing referer", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(jpg)
	}))
	defer srv.Close()

	path, err := ExportChapterURLsToCBZ("Title", "1", []string{srv.URL}, t.TempDir(), "TruyenGG", chapterURL)
	if err != nil {
		t.Fatalf("ExportChapterURLsToCBZ error: %v", err)
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()
	if len(zr.File) != 2 {
		t.Fatalf("zip entries = %d, want ComicInfo.xml + 1 page", len(zr.File))
	}
}

func TestExportChapterURLsFailsWhenProviderRejects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := ExportChapterURLsToCBZ("Title", "1", []string{srv.URL}, t.TempDir(), "TruyenQQ", "https://truyenqqko.com/x")
	if err == nil {
		t.Fatal("expected an error when every page download is rejected")
	}
}
