package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

func TestRightKeyBlockedWhenNotReady(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.total = 5

	// stepFetchURLs: should ignore right.
	m.step = stepFetchURLs
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("right")})
	rm := newM.(ReaderModel)
	if cmd != nil {
		t.Errorf("expected nil cmd in stepFetchURLs, got %v", cmd)
	}
	if rm.currentIdx != 0 {
		t.Errorf("expected currentIdx 0 in stepFetchURLs, got %d", rm.currentIdx)
	}

	// stepRead but rendering: should ignore right.
	m.step = stepRead
	m.isLoading = true
	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("right")})
	rm = newM.(ReaderModel)
	if cmd != nil {
		t.Errorf("expected nil cmd while loading, got %v", cmd)
	}
	if rm.currentIdx != 0 {
		t.Errorf("expected currentIdx 0 while loading, got %d", rm.currentIdx)
	}
}

func TestRightKeyClampsCurrentIndex(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.step = stepRead
	m.total = 3
	m.currentIdx = 10
	m.imageData = make([][]byte, 3)

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("right")})
	rm := newM.(ReaderModel)
	if cmd != nil {
		t.Errorf("expected nil cmd, got %v", cmd)
	}
	if rm.currentIdx != 2 {
		t.Errorf("expected currentIdx clamped to 2, got %d", rm.currentIdx)
	}
}

func TestLeftKeyClampsCurrentIndex(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.step = stepRead
	m.total = 3
	m.currentIdx = -5
	m.imageData = make([][]byte, 3)

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("left")})
	rm := newM.(ReaderModel)
	if cmd != nil {
		t.Errorf("expected nil cmd, got %v", cmd)
	}
	if rm.currentIdx != 0 {
		t.Errorf("expected currentIdx clamped to 0, got %d", rm.currentIdx)
	}
}

func TestDownloadProgressIgnoresStaleIndex(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.step = stepDownload
	m.total = 3
	m.imageData = make([][]byte, 3)
	m.downloadOrder = []int{0, 1, 2}
	m.downloadPos = 0
	m.currentIdx = 0

	msg := downloadProgressMsg{index: 5, data: []byte{1, 2, 3}}
	newM, cmd := m.Update(msg)
	rm := newM.(ReaderModel)
	if cmd != nil {
		t.Errorf("expected nil cmd for stale index, got %v", cmd)
	}
	if rm.downloaded != 0 {
		t.Errorf("expected downloaded unchanged, got %d", rm.downloaded)
	}
	for i, d := range rm.imageData {
		if len(d) != 0 {
			t.Errorf("expected imageData[%d] empty, got %d bytes", i, d)
		}
	}
}

func TestRenderDoneIgnoresStaleIndex(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.step = stepDownload
	m.total = 2
	m.imageData = make([][]byte, 2)
	m.currentIdx = 0
	m.isLoading = true

	msg := renderDoneMsg{index: 7, img: imgrender.RenderedImage{WidthPx: 1, HeightPx: 1}}
	newM, cmd := m.Update(msg)
	rm := newM.(ReaderModel)
	if cmd != nil {
		t.Errorf("expected nil cmd for stale render, got %v", cmd)
	}
	if _, ok := rm.imageCache[7]; !ok {
		t.Errorf("expected stale render to still be cached by index")
	}
	if !rm.isLoading {
		t.Errorf("expected isLoading to remain true for stale render")
	}
}

func TestWindowSizeNoPanicWithEmptyImageData(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.step = stepRead
	m.total = 5
	m.currentIdx = 0
	m.imageData = make([][]byte, 0)

	_, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd != nil {
		t.Errorf("expected nil cmd when no image data, got %v", cmd)
	}
}

func TestNextRenderIndexRespectsImageDataBounds(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.total = 5
	m.currentIdx = 0
	m.imageData = [][]byte{nil, {1}, nil, nil, nil}
	m.imageCache = map[int]imgrender.RenderedImage{}

	if got := m.nextRenderIndex(); got != 1 {
		t.Errorf("expected nextRenderIndex=1, got %d", got)
	}

	// total larger than imageData length must not panic.
	m.imageData = [][]byte{nil, {1}}
	if got := m.nextRenderIndex(); got != 1 {
		t.Errorf("expected nextRenderIndex=1 within bounds, got %d", got)
	}
}
