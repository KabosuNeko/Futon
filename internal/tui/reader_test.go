package tui

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

// collectMessages runs a command tree and returns every non-nil message.
func collectMessages(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	v := reflect.ValueOf(msg)
	if v.Kind() == reflect.Slice {
		var out []tea.Msg
		for i := 0; i < v.Len(); i++ {
			if sub, ok := v.Index(i).Interface().(tea.Cmd); ok {
				out = append(out, collectMessages(sub)...)
			}
		}
		return out
	}
	return []tea.Msg{msg}
}

func TestRightKeyBlockedWhenNotReady(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.total = 5

	// stepFetchURLs: should ignore right.
	m.step = stepFetchURLs
	rm, cmd := m.update(tea.KeyPressMsg{Code: tea.KeyRight})
	if cmd != nil {
		t.Errorf("expected nil cmd in stepFetchURLs, got %v", cmd)
	}
	if rm.currentIdx != 0 {
		t.Errorf("expected currentIdx 0 in stepFetchURLs, got %d", rm.currentIdx)
	}

	// stepRead but rendering: should ignore right.
	m.step = stepRead
	m.isLoading = true
	rm, cmd = m.update(tea.KeyPressMsg{Code: tea.KeyRight})
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

	rm, cmd := m.update(tea.KeyPressMsg{Code: tea.KeyRight})
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

	rm, cmd := m.update(tea.KeyPressMsg{Code: tea.KeyLeft})
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

	msg := downloadProgressMsg{index: 5, chapterID: "c1", data: []byte{1, 2, 3}}
	rm, cmd := m.update(msg)
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

	msg := renderDoneMsg{index: 7, chapterID: "c1", img: imgrender.RenderedImage{WidthPx: 1, HeightPx: 1}}
	rm, cmd := m.update(msg)
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

	_, cmd := m.update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd != nil {
		t.Errorf("expected nil cmd when no image data, got %v", cmd)
	}
}

func TestDownloadProgressStartsReadingCurrentPage(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.step = stepDownload
	m.total = 2
	m.imageData = make([][]byte, 2)
	m.currentIdx = 0

	rm, cmd := m.update(downloadProgressMsg{index: 0, chapterID: "c1", data: []byte{1}})
	if rm.step != stepRead || !rm.isLoading {
		t.Fatalf("expected stepRead with isLoading, got step=%v loading=%v", rm.step, rm.isLoading)
	}
	if cmd == nil {
		t.Fatal("expected a render cmd")
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

func TestStaleChapterDownloadIgnored(t *testing.T) {
	m := NewReaderModel("m1", "Title", "new-chapter", "2", nil, 0, -1, nil)
	m.step = stepDownload
	m.total = 1
	m.imageData = make([][]byte, 1)
	m.currentIdx = 0

	rm, cmd := m.update(downloadProgressMsg{index: 0, chapterID: "old-chapter", data: []byte{1}})
	if cmd != nil {
		t.Errorf("stale chapter download scheduled work: %v", cmd)
	}
	if len(rm.imageData[0]) != 0 || rm.downloaded != 0 {
		t.Errorf("stale chapter download was applied: data=%d downloaded=%d", len(rm.imageData[0]), rm.downloaded)
	}
}

func TestStaleChapterRenderIgnored(t *testing.T) {
	m := NewReaderModel("m1", "Title", "new-chapter", "2", nil, 0, -1, nil)
	m.step = stepRead
	m.total = 1
	m.imageData = [][]byte{{1}}
	m.currentIdx = 0

	rm, _ := m.update(renderDoneMsg{index: 0, chapterID: "old-chapter", img: imgrender.RenderedImage{WidthPx: 1, HeightPx: 1}})
	if _, ok := rm.imageCache[0]; ok {
		t.Error("stale chapter render was cached into the new chapter")
	}
}

func TestReaderChapterJumpKeys(t *testing.T) {
	cases := []struct {
		name      string
		key       tea.KeyPressMsg
		wantID    string
		wantIndex int
		wantStart int
	}{
		{"j next chapter", tea.KeyPressMsg{Code: 'j', Text: "j"}, "c2", 2, 0},
		{"down next chapter", tea.KeyPressMsg{Code: tea.KeyDown}, "c2", 2, 0},
		{"k previous chapter", tea.KeyPressMsg{Code: 'k', Text: "k"}, "c0", 0, 0},
		{"up previous chapter", tea.KeyPressMsg{Code: tea.KeyUp}, "c0", 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewReaderModel("m1", "Title", "c1", "1", []string{"c0", "c1", "c2"}, 1, -1, nil)
			m.allChapterNumbers = []string{"0", "1", "2"}
			m.step = stepRead
			m.total = 5
			m.currentIdx = 2
			m.imageData = make([][]byte, 5)

			rm, cmd := m.update(tc.key)
			if rm.step != stepLoadingNext {
				t.Errorf("step = %v, want stepLoadingNext", rm.step)
			}

			var view *ViewChapterMsg
			for _, msg := range collectMessages(cmd) {
				if v, ok := msg.(ViewChapterMsg); ok {
					view = &v
				}
			}
			if view == nil {
				t.Fatal("expected a ViewChapterMsg from the chapter jump")
			}
			if view.ChapterID != tc.wantID || view.ChapterIndex != tc.wantIndex || view.StartPageIndex != tc.wantStart {
				t.Errorf("jump = %+v, want id=%s index=%d start=%d", view, tc.wantID, tc.wantIndex, tc.wantStart)
			}
		})
	}
}

func TestReaderChapterJumpWhileRendering(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", []string{"c1", "c2"}, 0, -1, nil)
	m.step = stepRead
	m.isLoading = true
	m.total = 3
	m.currentIdx = 1
	m.imageData = make([][]byte, 3)

	_, cmd := m.update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if cmd == nil {
		t.Fatal("fast chapter jump should not wait for the current page render")
	}
}

func TestReaderChapterJumpStopsAtListEnds(t *testing.T) {
	first := NewReaderModel("m1", "Title", "c0", "0", []string{"c0", "c1"}, 0, -1, nil)
	first.step = stepRead

	if _, cmd := first.update(tea.KeyPressMsg{Code: 'k', Text: "k"}); cmd != nil {
		t.Error("previous chapter at the first chapter should be a no-op")
	}

	last := NewReaderModel("m1", "Title", "c1", "1", []string{"c0", "c1"}, 1, -1, nil)
	last.step = stepRead

	if _, cmd := last.update(tea.KeyPressMsg{Code: 'j', Text: "j"}); cmd != nil {
		t.Error("next chapter at the last chapter should be a no-op")
	}
}

func TestReaderChapterJumpUsesPreloadedNextChapter(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", []string{"c1", "c2"}, 0, -1, nil)
	m.step = stepRead
	m.total = 3
	m.currentIdx = 2
	m.imageData = make([][]byte, 3)
	m.preloadedChapID = "c2"
	m.preloadedURLs = []string{"u1", "u2"}
	m.preloadedImages = [][]byte{{1}, {2}}

	rm, cmd := m.update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if cmd == nil {
		t.Fatal("expected a transition command")
	}
	if rm.chapterID != "c2" || rm.chapterIndex != 1 || rm.currentIdx != 0 {
		t.Errorf("preloaded jump not applied: chapter=%s index=%d page=%d", rm.chapterID, rm.chapterIndex, rm.currentIdx)
	}
	if rm.step != stepRead {
		t.Errorf("step = %v, want stepRead after applying preload", rm.step)
	}
}
