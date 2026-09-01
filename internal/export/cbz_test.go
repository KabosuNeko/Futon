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
