package tui

import (
	"context"
	"errors"
	"image"
	"reflect"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
	"github.com/mattn/go-runewidth"
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
		{"shift+down next chapter", tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModShift}, "c2", 2, 0},
		{"k previous chapter", tea.KeyPressMsg{Code: 'k', Text: "k"}, "c0", 0, 0},
		{"shift+up previous chapter", tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModShift}, "c0", 0, 0},
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

func newReadingModel(total int) ReaderModel {
	m := NewReaderModel("m1", "Title", "c1", "1", []string{"c1", "c2"}, 0, 0, nil)
	m.step = stepRead
	m.total = total
	m.urls = make([]string, total)
	m.imageData = make([][]byte, total)
	return m
}

func TestReaderPageFailureDoesNotEndChapter(t *testing.T) {
	m := newReadingModel(3)
	m.currentIdx = 1
	m.isLoading = true
	m.downloading[1] = struct{}{}

	rm, _ := m.update(downloadProgressMsg{index: 1, chapterID: "c1", err: errors.New("HTTP 503")})
	if rm.step == stepError {
		t.Fatal("one failed page put the whole chapter into the error state")
	}
	if rm.isLoading {
		t.Error("reader keeps waiting for a page that failed")
	}
	if !strings.Contains(rm.frameContent(), "[r] Thử lại") {
		t.Errorf("failed page does not offer a retry:\n%q", rm.frameContent())
	}

	// Navigation still works past the failed page.
	rm.imageData[2] = []byte{1}
	next, _ := rm.handleNextPage()
	if next.currentIdx != 2 {
		t.Errorf("currentIdx = %d, want 2", next.currentIdx)
	}
}

func TestReaderRetryRedownloadsFailedPage(t *testing.T) {
	m := newReadingModel(2)
	m.failed[0] = errors.New("boom")

	rm, cmd := m.update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if cmd == nil {
		t.Fatal("r did not start a download")
	}
	if _, still := rm.failed[0]; still || !rm.isLoading {
		t.Errorf("retry state: failed=%v isLoading=%v", rm.failed, rm.isLoading)
	}
}

func TestReaderPreloadsWhenResumingPastTrigger(t *testing.T) {
	m := newReadingModel(10)
	m.currentIdx = 9 // resumed on the last page: total-3 was never visited

	if m.maybePreloadNext() == nil || !m.isPreloadingNext {
		t.Fatal("no preload when the chapter opens past the trigger page")
	}
}

func TestReaderPreloadsShortChapter(t *testing.T) {
	m := newReadingModel(2)
	if m.maybePreloadNext() == nil {
		t.Fatal("a chapter shorter than the trigger offset never preloads")
	}
}

func TestReaderDoesNotRetryFailedPreloadOnEveryFlip(t *testing.T) {
	m := newReadingModel(5)
	m.currentIdx = 3
	if m.maybePreloadNext() == nil {
		t.Fatal("expected a preload near the end")
	}
	m, _ = m.handlePreloadComplete(PreloadCompleteMsg{ChapID: "c2"}) // failed: no URLs
	m.currentIdx = 4
	if m.maybePreloadNext() != nil {
		t.Error("a failed preload is retried on every page flip")
	}
}

func TestReaderFlashStaysOnOneRow(t *testing.T) {
	m := newReadingModel(2)
	m.width, m.height = 60, 20
	m.flashMsg = "Đã lưu ảnh: " + strings.Repeat("rất-dài/", 20) + "x.jpg"
	m.failed[0] = errors.New("x") // render the error frame so no image is needed
	frame := m.frameContent()
	i := strings.Index(frame, "Đã lưu ảnh")
	if i < 0 {
		t.Fatal("flash not drawn")
	}
	if w := runewidth.StringWidth(ansiStrip(frame[i:])); w > m.width {
		t.Errorf("flash is %d cells wide on a %d-cell row; it would wrap onto the footer", w, m.width)
	}
}

// ansiStrip removes CSI escape sequences.
func ansiStrip(s string) string {
	return regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`).ReplaceAllString(s, "")
}

func TestReaderFooterFitsNarrowTerminal(t *testing.T) {
	m := newReadingModel(3)
	m.currentIdx = 1
	foot := m.footerText(79)
	if w := runewidth.StringWidth(foot); w > 79 {
		t.Fatalf("footer is %d cells on an 80-column terminal: %q", w, foot)
	}
	for _, keep := range []string{"Trang 2/3", "[esc] Thoát", "[←/h] [→/l] Trang"} {
		if !strings.Contains(foot, keep) {
			t.Errorf("narrow footer dropped %q: %q", keep, foot)
		}
	}
	if strings.Contains(foot, "…") {
		t.Errorf("hints should be dropped whole, not cut mid-word: %q", foot)
	}
}

func TestReaderFooterInWebtoonMode(t *testing.T) {
	m := newReadingModel(3)
	m.fitWidth = true
	foot := m.footerText(200)
	for _, want := range []string{"[↑/↓] Cuộn", "[⇧↑/⇧↓] Chap", "[w] Vừa màn hình"} {
		if !strings.Contains(foot, want) {
			t.Errorf("webtoon footer missing %q: %q", want, foot)
		}
	}
}

// webtoonModel is a reader in webtoon mode on page 0 of 3, whose current
// page is known to be 100 rows tall.
func webtoonModel() ReaderModel {
	m := newReadingModel(3)
	m.fitWidth = true
	m.height = 24 // 22 image rows with the footer
	for i := range m.imageData {
		m.imageData[i] = []byte{1}
	}
	m.scaled = &scaledPage{idx: 0, chapterID: m.chapterID, rows: 100}
	return m
}

func TestWebtoonScrollsWithinPageThenAdvances(t *testing.T) {
	m := webtoonModel()
	view := m.viewRows()

	m, cmd := m.scrollPage(1)
	if m.currentIdx != 0 || m.scrollRow != view/2 || cmd == nil || !m.isLoading {
		t.Fatalf("first scroll: page %d row %d loading %v", m.currentIdx, m.scrollRow, m.isLoading)
	}

	// The window comes back clamped to the bottom of the page.
	m, _ = m.update(renderDoneMsg{index: 0, chapterID: m.chapterID, windowed: true, requested: m.scrollRow, scroll: 100 - view, totalRows: 100})
	if m.isLoading || m.scrollRow != 100-view {
		t.Fatalf("after render: row %d loading %v", m.scrollRow, m.isLoading)
	}

	// At the bottom, the next scroll moves to the next page, at its top.
	m, _ = m.scrollPage(1)
	if m.currentIdx != 1 || m.scrollRow != 0 {
		t.Errorf("past the bottom: page %d row %d, want page 1 row 0", m.currentIdx, m.scrollRow)
	}
}

func TestWebtoonScrollUpEntersPreviousPageAtBottom(t *testing.T) {
	m := webtoonModel()
	m.currentIdx = 1
	m.scaled = nil

	m, cmd := m.scrollPage(-1)
	if m.currentIdx != 0 || m.scrollRow != bottomOfPage || cmd == nil {
		t.Fatalf("scroll up at top: page %d row %d, want page 0 at the bottom", m.currentIdx, m.scrollRow)
	}
}

func TestWebtoonStaleWindowIsIgnored(t *testing.T) {
	m := webtoonModel()
	m.scrollRow = 22
	m.isLoading = true
	m, _ = m.update(renderDoneMsg{index: 0, chapterID: m.chapterID, windowed: true, requested: 11, scroll: 11, totalRows: 100})
	if !m.isLoading || m.scrollRow != 22 {
		t.Errorf("an older scroll's window replaced the newer request: row %d loading %v", m.scrollRow, m.isLoading)
	}
}

func TestWebtoonKeysKeepChapterJumpsOnJK(t *testing.T) {
	m := webtoonModel()
	if m2, _ := m.update(tea.KeyPressMsg{Code: tea.KeyDown}); m2.chapterIndex != 0 || m2.scrollRow == 0 {
		t.Error("down must scroll in webtoon mode")
	}
	if m2, _ := m.update(tea.KeyPressMsg{Code: 'j', Text: "j"}); m2.step != stepLoadingNext {
		t.Error("j must still jump to the next chapter")
	}
}

func TestToggleFitWidthRerendersAndSaves(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newReadingModel(2)
	m.imageData[0] = []byte{1}
	m.setCached(0, imgrender.RenderedImage{EscapeSequence: "old"})

	m, cmd := m.update(tea.KeyPressMsg{Code: 'w', Text: "w"})
	if !m.fitWidth || !m.isLoading || cmd == nil {
		t.Fatalf("w: fitWidth=%v loading=%v", m.fitWidth, m.isLoading)
	}
	if _, cached := m.imageCache[0]; cached {
		t.Error("pages rendered for the other mode must be dropped")
	}
}

func TestReaderModeCarriesAcrossChapters(t *testing.T) {
	app := NewAppModel("dev")
	app.reader = NewReaderModel("m", "T", "c1", "1", []string{"c1", "c2"}, 0, 0, nil)
	app.reader.fitWidth = true
	app.reader.hideFooter = true

	updated, _ := app.Update(ViewChapterMsg{MangaID: "m", ChapterID: "c2", AllChapterIDs: []string{"c1", "c2"}, ChapterIndex: 1})
	next := updated.(AppModel).reader
	if !next.fitWidth {
		t.Error("the next chapter lost webtoon mode")
	}
	if !next.hideFooter {
		t.Error("the next chapter showed the footer again after ctrl+h hid it")
	}
}

func TestCtrlHIsRememberedAcrossLaunches(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := newReadingModel(1)
	m.imageData[0] = []byte{1}

	_, cmd := m.update(tea.KeyPressMsg{Code: 'h', Mod: tea.ModCtrl})
	collectMessages(cmd) // runs the save command

	if !NewAppModel("dev").readerHideFooter {
		t.Error("a new launch does not start with the footer hidden")
	}
}

func TestWebtoonKeepsPrefetchedPageScaled(t *testing.T) {
	m := webtoonModel()
	next := image.NewRGBA(image.Rect(0, 0, 1, 1))
	m, _ = m.update(renderDoneMsg{index: 1, chapterID: m.chapterID, windowed: true, scaled: next, totalRows: 50})

	m.currentIdx = 1
	if s := m.scaledFor(1); s == nil || s.img != next || s.rows != 50 {
		t.Fatal("the prefetched page is decoded and scaled again when it becomes current")
	}
}

func TestReaderRawFrameHasNoEscapesFromTitles(t *testing.T) {
	m := newReadingModel(1)
	m.failed[0] = errors.New("x") // error frame: no image needed
	m.flashMsg = "Đã lưu ảnh: ~/Downloads/Futon/Evil\x1b]52;c;cm0gLXJmIH4=\a_Pg1.jpg"
	if frame := m.frameContent(); strings.Contains(frame, "\x1b]52") || strings.ContainsRune(frame, '\a') {
		t.Errorf("an OSC 52 sequence from a title reached the terminal: %q", frame)
	}
}

func TestChapterTitlesAreCleanedOnLoad(t *testing.T) {
	p := escProvider{}
	msg := fetchChaptersCmd(p, "m")().(chaptersLoadedMsg)
	if strings.ContainsRune(msg.Chapters[0].Title, 0x1b) {
		t.Errorf("chapter title kept an escape: %q", msg.Chapters[0].Title)
	}
}

type escProvider struct{ pagedProvider }

func (escProvider) FetchChapters(context.Context, string) ([]models.Chapter, error) {
	return []models.Chapter{{ID: "c", Title: "Chap\x1b]0;pwned\a 1"}}, nil
}

func TestRenderStartedBeforeModeToggleIsDropped(t *testing.T) {
	m := newReadingModel(3)
	for i := range m.imageData {
		m.imageData[i] = []byte{1}
	}
	oldGen := m.renderGen
	m, _ = m.toggleFitWidth() // a fit-screen render of page 1 is still in flight

	m, _ = m.update(renderDoneMsg{index: 1, chapterID: m.chapterID, gen: oldGen, img: imgrender.RenderedImage{WidthPx: 1, HeightPx: 1}})
	if _, cached := m.imageCache[1]; cached {
		t.Error("a render from before the toggle was cached in webtoon mode")
	}
}

func TestResizeClearsRenderedPagesInFitScreenMode(t *testing.T) {
	m := newReadingModel(3)
	m.setCached(1, imgrender.RenderedImage{WidthPx: 1, HeightPx: 1})
	m, _ = m.update(tea.WindowSizeMsg{Width: 60, Height: 20})
	if _, cached := m.imageCache[1]; cached {
		t.Error("page 1 kept its render for the old terminal size")
	}
}

func TestReaderShowsSettingsSaveError(t *testing.T) {
	m := newReadingModel(1)
	m, _ = m.update(storageSavedMsg{err: errors.New("parse userdata JSON")})
	if !strings.Contains(m.flashMsg, "parse userdata JSON") {
		t.Errorf("flash = %q, want the save error", m.flashMsg)
	}
}

func TestPlainArrowsDoNotSwitchChapters(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyDown}, {Code: tea.KeyUp}} {
		m := NewReaderModel("m1", "Title", "c1", "1", []string{"c0", "c1", "c2"}, 1, -1, nil)
		m.step = stepRead
		m.total = 5
		m.imageData = make([][]byte, 5)
		if rm, _ := m.update(key); rm.step != stepRead || rm.chapterID != "c1" {
			t.Errorf("%s switched chapters; only shift+arrows and j/k may", key.String())
		}
	}
}

func TestReaderPaintsNothingAfterEsc(t *testing.T) {
	m := newReadingModel(3)
	m.imageData[0] = []byte{1}
	m.isLoading = true // the page on screen is still rendering

	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = updated.(ReaderModel)

	img := imgrender.RenderedImage{EscapeSequence: "\x1b_Gimg\x1b\\", WidthPx: 10, HeightPx: 10}
	for _, msg := range []tea.Msg{
		renderDoneMsg{index: 0, chapterID: m.chapterID, gen: m.renderGen, img: img}, // finishes during the flush
		tea.WindowSizeMsg{Width: 100, Height: 30},                                   // routed here on the search screen
	} {
		updated, cmd := m.Update(msg)
		m = updated.(ReaderModel)
		if raw := collectRawPayloads(cmd); raw != "" {
			t.Errorf("%T after esc painted %q over the next screen", msg, raw)
		}
	}
}
