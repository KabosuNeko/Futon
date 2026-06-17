package tui

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

func TestImageCacheLRUDiscardsOldest(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)

	for i := 0; i < maxImageCache+5; i++ {
		m.setCached(i, imgrender.RenderedImage{WidthPx: i, HeightPx: i})
	}

	if len(m.imageCache) != maxImageCache {
		t.Errorf("expected cache size %d, got %d", maxImageCache, len(m.imageCache))
	}
	// Oldest entries should have been evicted.
	for i := 0; i < 5; i++ {
		if _, ok := m.imageCache[i]; ok {
			t.Errorf("expected entry %d to be evicted", i)
		}
	}
	// Newest entries should still be present.
	for i := maxImageCache + 4; i >= maxImageCache; i-- {
		if _, ok := m.imageCache[i]; !ok {
			t.Errorf("expected entry %d to be cached", i)
		}
	}
}

func TestBuildDownloadOrderStartsAtCurrentPage(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.total = 5

	order := m.buildDownloadOrder(2)
	want := []int{2, 3, 4, 0, 1}
	if len(order) != len(want) {
		t.Fatalf("expected order length %d, got %d", len(want), len(order))
	}
	for i, v := range want {
		if order[i] != v {
			t.Errorf("order[%d] = %d, want %d", i, order[i], v)
		}
	}
}

func TestNextRenderIndexSkipsCachedAndEmpty(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.total = 5
	m.currentIdx = 0
	m.imageData = [][]byte{nil, {1}, nil, {2}, nil}
	m.imageCache = map[int]imgrender.RenderedImage{1: {WidthPx: 1}}

	if got := m.nextRenderIndex(); got != 3 {
		t.Errorf("expected nextRenderIndex 3, got %d", got)
	}
}

func TestApplyPreloadedChapterResetsState(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", []string{"c1", "c2"}, 0, -1, nil)
	m.preloadedChapID = "c2"
	m.preloadedURLs = []string{"u1", "u2"}
	m.preloadedImages = [][]byte{{1}, {2}}

	m.applyPreloadedChapter("c2")

	if m.chapterID != "c2" {
		t.Errorf("expected chapterID c2, got %s", m.chapterID)
	}
	if m.chapterIndex != 1 {
		t.Errorf("expected chapterIndex 1, got %d", m.chapterIndex)
	}
	if m.total != 2 {
		t.Errorf("expected total 2, got %d", m.total)
	}
	if len(m.imageData) != 2 || len(m.imageData[0]) != 1 {
		t.Errorf("expected preloaded images copied")
	}
	if m.preloadedChapID != "" {
		t.Errorf("expected preloaded state cleared")
	}
}

func TestReaderViewRendersFooterInStepRead(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.step = stepRead
	m.total = 3
	m.currentIdx = 1
	m.width = 80
	m.height = 24

	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	r := imgrender.New()
	rendered, err := r.Render(buf.Bytes(), m.width)
	if err != nil {
		t.Fatalf("render image: %v", err)
	}

	m.imageData = [][]byte{buf.Bytes(), buf.Bytes(), buf.Bytes()}
	m.setCached(m.currentIdx, rendered)

	view := m.View()
	if !strings.Contains(view, "Trang 2/3") {
		t.Errorf("expected footer with page info, got:\n%s", view)
	}
	if !strings.Contains(view, "ctrl+d: tải ảnh") {
		t.Errorf("expected reader footer hint, got:\n%s", view)
	}
}

func TestClampCurrentIndex(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.total = 3

	m.currentIdx = -10
	m.clampCurrentIndex()
	if m.currentIdx != 0 {
		t.Errorf("expected clamp to 0, got %d", m.currentIdx)
	}

	m.currentIdx = 10
	m.clampCurrentIndex()
	if m.currentIdx != 2 {
		t.Errorf("expected clamp to 2, got %d", m.currentIdx)
	}
}
