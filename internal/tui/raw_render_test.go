package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

// collectRawPayloads executes cmd and concatenates every tea.RawMsg payload it
// produces, walking tea.BatchMsg children.
func collectRawPayloads(cmd tea.Cmd) string {
	if cmd == nil {
		return ""
	}
	var b strings.Builder
	var walk func(tea.Cmd)
	walk = func(c tea.Cmd) {
		if c == nil {
			return
		}
		switch msg := c().(type) {
		case tea.RawMsg:
			b.WriteString(fmt.Sprint(msg.Msg))
		case tea.BatchMsg:
			for _, sub := range msg {
				walk(sub)
			}
		}
	}
	walk(cmd)
	return b.String()
}

func TestReaderViewIsBlankNonEmpty(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.step = stepRead
	m.width = 80
	m.height = 24

	view := m.View().Content
	if view == "" {
		t.Fatal("blank view must stay non-empty: an empty view makes the v2 renderer erase the screen every frame")
	}
	if strings.TrimSpace(view) != "" {
		t.Errorf("expected the renderer view to contain only spaces, got %q", view)
	}
	if strings.Contains(view, "\x1b") {
		t.Errorf("expected no escape sequences in the renderer view, got %q", view)
	}
}

func TestReaderFrameEmittedOnlyWhenChanged(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.step = stepFetchURLs
	m.width = 80
	m.height = 24

	first, cmd := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if frame := collectRawPayloads(cmd); !strings.Contains(frame, "Đang lấy danh sách ảnh...") {
		t.Fatalf("expected first raw frame, got %q", frame)
	}

	// Same frame content: must not re-emit (a redundant raw would clear and
	// redraw the pending image).
	_, cmd2 := first.(ReaderModel).Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd2 != nil {
		t.Errorf("expected no cmd for an unchanged frame, got %v", cmd2)
	}
}

func TestReaderFirstFrameMsgForcesRepaint(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.step = stepFetchURLs
	m.width = 80
	m.height = 24

	_, cmd := m.Update(readerFirstFrameMsg{})
	payload := collectRawPayloads(cmd)
	if !strings.Contains(payload, "Đang lấy danh sách ảnh...") {
		t.Fatalf("expected forced first-frame repaint, got %q", payload)
	}
	// The wrapper must not append a duplicate frame.
	if n := strings.Count(payload, "\x1b[H\x1b[2J"); n != 1 {
		t.Errorf("expected exactly one frame, got %d:\n%q", n, payload)
	}
}

func TestReaderPageReadyFrameContainsImage(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.step = stepRead
	m.total = 3
	m.currentIdx = 1
	m.width = 80
	m.height = 24
	m.imageData = [][]byte{{1}, {1}, {1}}

	rendered := imgrender.RenderedImage{
		EscapeSequence: "\x1b_Gf=100,a=T;PAYLOAD\x1b\\",
		WidthPx:        800,
		HeightPx:       1200,
	}
	m.setCached(m.currentIdx, rendered)

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	frame := collectRawPayloads(cmd)
	if !strings.Contains(frame, rendered.EscapeSequence) {
		t.Errorf("expected the page-ready frame to contain the image escape, got %q", frame)
	}
	if !strings.Contains(frame, "Trang 2/3") {
		t.Errorf("expected page footer in the page-ready frame, got %q", frame)
	}

	ox, oy := m.imageRect(rendered)
	wantMove := fmt.Sprintf("\x1b[%d;%dH", oy+1, ox+1)
	if !strings.Contains(frame, wantMove) {
		t.Errorf("expected image cursor position %q in frame %q", wantMove, frame)
	}
}

func TestReaderResizeReframes(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", nil, 0, -1, nil)
	m.step = stepRead
	m.total = 1
	m.imageData = [][]byte{{1}}
	m.width = 80
	m.height = 24

	rendered := imgrender.RenderedImage{EscapeSequence: "\x1b_Gimg\x1b\\", WidthPx: 10, HeightPx: 10}

	newM, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if frame := collectRawPayloads(cmd); !strings.Contains(frame, "Đang render ảnh...") {
		t.Errorf("expected loading frame after resize, got %q", frame)
	}

	sm := newM.(ReaderModel)
	_, cmd2 := sm.Update(renderDoneMsg{index: 0, img: rendered})
	frame := collectRawPayloads(cmd2)
	if !strings.Contains(frame, rendered.EscapeSequence) {
		t.Errorf("expected re-rendered image frame after resize, got %q", frame)
	}
	if !strings.Contains(frame, "Trang 1/1") {
		t.Errorf("expected footer after resize, got %q", frame)
	}
}

func TestClearGraphicsCmdEmitsRaw(t *testing.T) {
	payload := collectRawPayloads(clearGraphicsCmd())
	if payload != kittyClearSeq {
		t.Errorf("clearGraphicsCmd payload = %q, want %q", payload, kittyClearSeq)
	}
	if strings.Contains(payload, "\x1b[H\x1b[2J") {
		t.Errorf("clearGraphicsCmd must not clear the screen, got %q", payload)
	}
}

func TestClearScreenCmdEmitsRaw(t *testing.T) {
	payload := collectRawPayloads(clearScreenCmd())
	if !strings.Contains(payload, kittyClearSeq) {
		t.Errorf("clearScreenCmd payload missing image delete, got %q", payload)
	}
	if !strings.Contains(payload, "\x1b[H\x1b[2J") {
		t.Errorf("clearScreenCmd payload missing clear+home, got %q", payload)
	}
	if strings.Index(payload, kittyClearSeq) > strings.Index(payload, "\x1b[H\x1b[2J") {
		t.Errorf("image delete must precede the clear, got %q", payload)
	}
}

func TestReaderEscLeavesWithCmd(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", []string{"c1"}, 0, -1, nil)
	m.step = stepRead
	m.width = 80
	m.height = 24

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected esc to return the save/clear/back command sequence")
	}
}

func splitSearchModelWithCover() (SearchModel, imgrender.RenderedImage) {
	m := testSearchModel()
	m.width = 100
	m.height = 24
	m.results = []models.Manga{
		{ID: "m1", Title: "One Piece", CoverURL: "https://example.com/cover.jpg", Provider: "OTruyen"},
	}
	m.cursor = 0
	rendered := imgrender.RenderedImage{
		EscapeSequence: "\x1b_Gf=100,a=T;COVER\x1b\\",
		WidthPx:        100,
		HeightPx:       150,
	}
	return m, rendered
}

func TestSearchViewHasNoCoverEscapes(t *testing.T) {
	m, rendered := splitSearchModelWithCover()
	m.currentCover = &rendered

	view := m.View().Content
	for _, seq := range []string{"\x1b_G", "\x1b[s", "\x1b[u", rendered.EscapeSequence} {
		if strings.Contains(view, seq) {
			t.Errorf("cover sequence %q must not be part of the rendered view", seq)
		}
	}
}

// nextCoverPaint waits out the deferred cover paint scheduled by Update and
// returns the raw payload it produces.
func nextCoverPaint(t *testing.T, m SearchModel, cmd tea.Cmd) string {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a deferred cover paint command")
	}
	msg := cmd()
	if _, ok := msg.(coverPaintMsg); !ok {
		t.Fatalf("expected coverPaintMsg, got %T", msg)
	}
	_, rawCmd := m.Update(msg)
	return collectRawPayloads(rawCmd)
}

func TestSearchCoverDrawEmitsRawSequence(t *testing.T) {
	m, rendered := splitSearchModelWithCover()

	newM, cmd := m.Update(coverRenderedMsg{
		mangaID:  "m1",
		coverURL: "https://example.com/cover.jpg",
		rendered: rendered,
	})
	sm := newM.(SearchModel)
	if sm.currentCover == nil {
		t.Fatal("expected currentCover to be set")
	}

	payload := nextCoverPaint(t, sm, cmd)
	if payload == "" {
		t.Fatal("expected a raw cover paint")
	}
	for _, want := range []string{
		"\x1b_Ga=d,d=A,q=2\x1b\\",
		"\x1b_Ga=d,d=a,q=2\x1b\\",
		"\x1b[s",
		"\x1b[u",
		rendered.EscapeSequence,
	} {
		if !strings.Contains(payload, want) {
			t.Errorf("cover paint missing %q, got %q", want, payload)
		}
	}

	key := sm.coverPaintState()
	wantMove := fmt.Sprintf("\x1b[%d;%dH", key.imgRow, key.imgCol)
	if !strings.Contains(payload, wantMove) {
		t.Errorf("cover paint missing image position %q, got %q", wantMove, payload)
	}
}

func TestSearchCoverClearOnHide(t *testing.T) {
	m, rendered := splitSearchModelWithCover()

	newM, paintCmd := m.Update(coverRenderedMsg{
		mangaID:  "m1",
		coverURL: "https://example.com/cover.jpg",
		rendered: rendered,
	})
	sm := newM.(SearchModel)
	if sm.currentCover == nil {
		t.Fatal("expected currentCover to be set")
	}
	_ = nextCoverPaint(t, sm, paintCmd)

	// A failed search clears the cover.
	newM2, cmd := sm.Update(api.MangaSearchResultMsg{Err: errors.New("boom")})
	sm2 := newM2.(SearchModel)
	if sm2.currentCover != nil {
		t.Fatal("expected currentCover to be cleared on search error")
	}

	payload := nextCoverPaint(t, sm2, cmd)
	if !strings.Contains(payload, "\x1b_Ga=d,d=A,q=2\x1b\\") || !strings.Contains(payload, "\x1b_Ga=d,d=a,q=2\x1b\\") {
		t.Errorf("expected image delete sequences after hiding the cover, got %q", payload)
	}
	if strings.Contains(payload, rendered.EscapeSequence) {
		t.Errorf("cleared cover must not be redrawn, got %q", payload)
	}
}

func TestSearchCoverClearsWhenLayoutLeavesSplit(t *testing.T) {
	m, rendered := splitSearchModelWithCover()

	newM, paintCmd := m.Update(coverRenderedMsg{
		mangaID:  "m1",
		coverURL: "https://example.com/cover.jpg",
		rendered: rendered,
	})
	sm := newM.(SearchModel)
	_ = nextCoverPaint(t, sm, paintCmd)

	newM2, cmd := sm.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	sm2 := newM2.(SearchModel)
	into := nextCoverPaint(t, sm2, cmd)
	want := "\x1b_Ga=d,d=A,q=2\x1b\\\x1b_Ga=d,d=a,q=2\x1b\\"
	if into != want {
		t.Errorf("leaving the split layout payload = %q, want %q", into, want)
	}
	if key := sm2.coverPaintState(); key.isSplit {
		t.Error("expected narrow layout to disable the split preview")
	}
}

func TestSearchCoverPaintNotRepeatedOnUnrelatedMsg(t *testing.T) {
	m, rendered := splitSearchModelWithCover()

	newM, paintCmd := m.Update(coverRenderedMsg{
		mangaID:  "m1",
		coverURL: "https://example.com/cover.jpg",
		rendered: rendered,
	})
	sm := newM.(SearchModel)
	if payload := nextCoverPaint(t, sm, paintCmd); payload == "" {
		t.Fatal("expected the initial cover paint")
	}

	_, cmd := sm.Update(clearFlashMsg{})
	if payload := collectRawPayloads(cmd); payload != "" {
		t.Errorf("unrelated message must not repaint the cover, got %q", payload)
	}
}

func TestSearchRepaintCoverAfterReaderExit(t *testing.T) {
	m, rendered := splitSearchModelWithCover()
	m.currentCover = &rendered
	m.coverKey = m.coverPaintState()

	cmd := m.repaintCoverCmd()
	if cmd == nil {
		t.Fatal("expected a repaint command")
	}
	msg := cmd()
	if _, ok := msg.(coverPaintMsg); !ok {
		t.Fatalf("expected coverPaintMsg, got %T", msg)
	}
	_, rawCmd := m.Update(msg)
	payload := collectRawPayloads(rawCmd)
	if !strings.Contains(payload, rendered.EscapeSequence) {
		t.Errorf("repaint must redraw the cover, got %q", payload)
	}
	if !strings.Contains(payload, "\x1b_Ga=d,d=A,q=2\x1b\\") {
		t.Errorf("repaint must clear the old image first, got %q", payload)
	}
}

func TestBackToSearchRepaintsCover(t *testing.T) {
	m, rendered := splitSearchModelWithCover()
	m.currentCover = &rendered
	m.coverKey = m.coverPaintState()

	app := NewAppModel("dev")
	app.state = stateChapters
	app.search = m

	updated, cmd := app.Update(BackToSearchMsg{})
	if cmd == nil {
		t.Fatal("expected the search state to schedule a cover repaint")
	}
	app = updated.(AppModel)
	_, rawCmd := app.Update(cmd())
	if payload := collectRawPayloads(rawCmd); !strings.Contains(payload, rendered.EscapeSequence) {
		t.Errorf("expected the cover repaint after returning to search, got %q", payload)
	}
}

func TestClearCoverCmdRemovesImageWithoutRedraw(t *testing.T) {
	m, rendered := splitSearchModelWithCover()
	m.currentCover = &rendered
	m.coverKey = m.coverPaintState()

	payload := collectRawPayloads(m.clearCoverCmd())
	if payload == "" {
		t.Fatal("expected a raw cover clear")
	}
	for _, want := range []string{"\x1b_Ga=d,d=A,q=2\x1b\\", "\x1b_Ga=d,d=a,q=2\x1b\\", "\x1b[s", "\x1b[u"} {
		if !strings.Contains(payload, want) {
			t.Errorf("cover clear missing %q, got %q", want, payload)
		}
	}
	if strings.Contains(payload, rendered.EscapeSequence) {
		t.Errorf("cover clear must not redraw the image, got %q", payload)
	}
	if m.coverKey != (coverPaintKey{}) {
		t.Errorf("cover clear must invalidate pending paints, got %+v", m.coverKey)
	}
}

func TestLeaveSearchClearsCoverBeforeChapters(t *testing.T) {
	app := NewAppModel("dev")
	app.search.width = 100
	app.search.height = 24
	app.search.results = []models.Manga{
		{ID: "m1", Title: "One Piece", CoverURL: "https://example.com/cover.jpg", Provider: "OTruyen"},
	}
	rendered := imgrender.RenderedImage{
		EscapeSequence: "\x1b_Gf=100,a=T;COVER\x1b\\",
		WidthPx:        100,
		HeightPx:       150,
	}
	app.search.currentCover = &rendered
	app.search.coverKey = app.search.coverPaintState()

	updated, cmd := app.Update(ViewMangaMsg{MangaID: "m1", Title: "One Piece", ProviderName: "OTruyen"})
	if cmd == nil {
		t.Fatal("expected a clear command before switching to chapters")
	}
	if updated.(AppModel).state == stateChapters {
		t.Error("state must not switch before the cover clear has been emitted")
	}

	updated2, _ := updated.(AppModel).Update(chaptersReadyMsg{})
	if updated2.(AppModel).state != stateChapters {
		t.Error("chaptersReadyMsg must switch to the chapter list")
	}
}
