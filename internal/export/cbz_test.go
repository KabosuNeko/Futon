package export

import (
	"archive/zip"
	"io"
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

	if !strings.Contains(comicInfoContent, "<Series>One / Piece: Test</Series>") {
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
		if got := DetectImageExt(c.data); got != c.want {
			t.Errorf("DetectImageExt(%s) = %q, want %q", c.name, got, c.want)
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

func TestExportImagesToCBZRejectsMissingPage(t *testing.T) {
	dir := t.TempDir()
	jpg := []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01")
	if _, err := ExportImagesToCBZ("T", "1", [][]byte{jpg, nil, jpg}, dir); err == nil {
		t.Fatal("expected an error for a missing page")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("failed export left files behind: %v", entries)
	}
}

func TestExportKeepsOriginalTitleInComicInfo(t *testing.T) {
	jpg := []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01")
	path, err := ExportImagesToCBZ("Re:Zero / Arc 1", "1", [][]byte{jpg}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(rc)
	rc.Close()
	if !strings.Contains(string(data), "<Series>Re:Zero / Arc 1</Series>") {
		t.Errorf("ComicInfo.xml lost the original title:\n%s", data)
	}
}

func TestChapterFileLabel(t *testing.T) {
	for in, want := range map[string]string{
		"12":        "Chap_12",
		"100.5":     "Chap_100.5",
		"Chương 12": "Chương 12",
		"Chap 150":  "Chap 150",
		"":          "unknown",
	} {
		if got := ChapterFileLabel(in); got != want {
			t.Errorf("ChapterFileLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCBZNameDoesNotRepeatChapterWord(t *testing.T) {
	path, err := ExportImagesToCBZ("Blue Lock", "Chương 1", [][]byte{[]byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01")}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "Blue Lock_Chương 1.cbz") {
		t.Errorf("path = %q, want Blue Lock_Chương 1.cbz", path)
	}
}

func TestShortPathUsesTilde(t *testing.T) {
	t.Setenv("HOME", "/home/someone")
	if got := ShortPath("/home/someone/Downloads/Futon/x.cbz"); got != "~/Downloads/Futon/x.cbz" {
		t.Errorf("ShortPath = %q", got)
	}
	if got := ShortPath("/tmp/x.cbz"); got != "/tmp/x.cbz" {
		t.Errorf("ShortPath outside home = %q", got)
	}
}
