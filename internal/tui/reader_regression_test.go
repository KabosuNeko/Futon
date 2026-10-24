package tui

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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

func TestReaderFrameRendersFooterInStepRead(t *testing.T) {
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
	rendered, err := r.RenderInBox(buf.Bytes(), 0, 0)
	if err != nil {
		t.Fatalf("render image: %v", err)
	}

	m.imageData = [][]byte{buf.Bytes(), buf.Bytes(), buf.Bytes()}
	m.setCached(m.currentIdx, rendered)

	// Any message triggers the frame wrapper; an unhandled key keeps state.
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	frame := collectRawPayloads(cmd)
	if frame == "" {
		t.Fatal("expected the reader frame as raw output")
	}
	if !strings.Contains(frame, "\x1b[H\x1b[2J") {
		t.Errorf("expected frame to clear the screen, got:\n%q", frame)
	}
	if !strings.Contains(frame, rendered.EscapeSequence) {
		t.Errorf("expected frame to embed the image escape sequence, got:\n%q", frame)
	}
	if !strings.Contains(frame, "Trang 2/3") {
		t.Errorf("expected footer with page info, got:\n%q", frame)
	}
	if !strings.Contains(frame, "[ctrl+d] Lưu ảnh") {
		t.Errorf("expected reader footer hint, got:\n%q", frame)
	}
	if !strings.Contains(frame, "[ctrl+e] Xuất CBZ") {
		t.Errorf("expected reader footer cbz hint, got:\n%q", frame)
	}
	if !strings.Contains(frame, "[↑/k] [↓/j] Chap") {
		t.Errorf("expected reader footer chapter-jump hint, got:\n%q", frame)
	}
	if !strings.Contains(frame, "[←/h] [→/l] Trang") {
		t.Errorf("expected matching page arrows in reader footer, got:\n%q", frame)
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

func TestReaderExportLabelFallsBackToChapterID(t *testing.T) {
	m := NewReaderModel("m1", "Title", "https://example.com/ch/1", "12", nil, 0, -1, nil)
	if got := m.exportLabel(); got != "12" {
		t.Errorf("exportLabel = %q, want the chapter number", got)
	}

	m.chapterNumber = ""
	if got := m.exportLabel(); got != "https://example.com/ch/1" {
		t.Errorf("exportLabel = %q, want the chapter ID fallback", got)
	}
}

func TestReaderCtrlHTogglesFooterHints(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.step = stepRead
	m.total = 1
	m.currentIdx = 0
	m.imageData = [][]byte{{1}}
	rendered := imgrender.RenderedImage{EscapeSequence: "\x1b_Gimg\x1b\\", WidthPx: 80, HeightPx: 80}
	m.setCached(0, rendered)

	if !strings.Contains(m.frameContent(), "[esc] Thoát") {
		t.Fatal("expected footer hints while the footer is visible")
	}

	hidden, cmd := m.update(tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl})
	if !hidden.hideFooter {
		t.Fatal("ctrl+h did not hide the footer")
	}
	if cmd == nil {
		t.Fatal("hiding the footer must re-render the current page")
	}
	if len(hidden.imageCache) != 0 {
		t.Error("layout toggle must drop rendered pages")
	}
	if !hidden.isLoading {
		t.Error("expected the reader to show the loading state after a re-render")
	}

	hidden.setCached(0, rendered)
	hidden.isLoading = false
	frame := hidden.frameContent()
	for _, hint := range []string{"[esc] Thoát", "[ctrl+d]", "[ctrl+h]"} {
		if strings.Contains(frame, hint) {
			t.Errorf("hidden footer still contains %q", hint)
		}
	}
	if !strings.Contains(frame, rendered.EscapeSequence) {
		t.Error("image missing from the hidden-footer frame")
	}

	shown, _ := hidden.update(tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl})
	if shown.hideFooter {
		t.Fatal("second ctrl+h did not show the footer")
	}
	shown.setCached(0, rendered)
	shown.isLoading = false
	if !strings.Contains(shown.frameContent(), "[esc] Thoát") {
		t.Error("footer hints did not come back")
	}
}
