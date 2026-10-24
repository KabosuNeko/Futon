package export

import (
	"archive/zip"
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"testing"

	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
	_ "golang.org/x/image/webp"
)

// TestLiveProviderExports is an opt-in integration test: it hits every provider
// and exports a real chapter. Run with:
//
//	FUTON_LIVE_EXPORT=1 go test ./internal/export -run TestLiveProviderExports -v -timeout 10m
//
// The default test run stays offline.
func TestLiveProviderExports(t *testing.T) {
	if os.Getenv("FUTON_LIVE_EXPORT") == "" {
		t.Skip("set FUTON_LIVE_EXPORT=1 to run live provider export tests")
	}

	providers := []api.MangaProvider{
		api.NewOTruyenProvider(),
		api.NewMangaDexProvider(),
		api.NewTruyenQQProvider(),
		api.NewTruyenGGProvider(),
		api.NewBaoTangTruyenProvider(),
	}
	for _, provider := range providers {
		t.Run(provider.Name(), func(t *testing.T) {
			liveExportOne(t, provider)
		})
	}
}

func liveExportOne(t *testing.T, provider api.MangaProvider) {
	t.Helper()

	var mangas []models.Manga
	var err error
	if md, ok := provider.(*api.MangaDexProvider); ok {
		// Latest manga are often untranslated; use a known title with chapters.
		md.SetLang("en")
		mangas, err = md.Search("one piece")
	} else {
		mangas, err = provider.FetchLatest(1)
	}
	if err != nil {
		t.Skipf("source unavailable: %v", err)
	}
	if len(mangas) == 0 {
		t.Skip("source returned no manga")
	}
	if len(mangas) > 3 {
		mangas = mangas[:3]
	}

	for _, manga := range mangas {
		chapters, err := provider.FetchChapters(manga.ID)
		if err != nil || len(chapters) == 0 {
			continue
		}
		for _, idx := range []int{0, len(chapters) - 1} {
			chapter := chapters[idx]
			urls, err := provider.FetchPages(chapter.ID)
			if err != nil || len(urls) == 0 {
				continue
			}
			pages := min(2, len(urls))
			label := chapter.Number
			if label == "" {
				label = chapter.Title
			}
			if label == "" {
				label = "unknown"
			}

			path, err := ExportChapterURLsToCBZ("Futon live test", label, urls[:pages], t.TempDir(), provider.Name(), chapter.ID)
			if err != nil {
				t.Fatalf("export failed: %v", err)
			}
			verifyLiveCBZ(t, path, pages)
			t.Logf("manga=%q chapter=%q urls=%d cbz=%s", manga.Title, label, len(urls), path)
			return
		}
	}
	t.Fatalf("no downloadable chapter found")
}

func verifyLiveCBZ(t *testing.T, path string, wantPages int) {
	t.Helper()

	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open cbz: %v", err)
	}
	defer zr.Close()

	imageEntries := 0
	for _, f := range zr.File {
		if f.Name == "ComicInfo.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}

		cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s is not a decodable image: %v (%d bytes)", f.Name, err, len(data))
		}
		if cfg.Width <= 0 || cfg.Height <= 0 {
			t.Fatalf("%s has invalid dimensions %dx%d", f.Name, cfg.Width, cfg.Height)
		}
		t.Logf("page %s: %s %dx%d, %d bytes", f.Name, format, cfg.Width, cfg.Height, len(data))
		imageEntries++
	}

	if imageEntries != wantPages {
		t.Fatalf("cbz has %d image entries, want %d", imageEntries, wantPages)
	}
}
