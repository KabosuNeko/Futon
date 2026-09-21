package tui

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

// lockedBuffer is safe to read while a tea.Program writes to it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func waitForOutput(t *testing.T, out *lockedBuffer, want string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s := out.String(); strings.Contains(s, want) {
			return s
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %q in output:\n%q", want, out.String())
	return ""
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// startProgram runs a headless tea.Program and quits it at test cleanup.
func startProgram(t *testing.T, m tea.Model, width, height int) (*tea.Program, *lockedBuffer) {
	t.Helper()
	out := &lockedBuffer{}
	p := tea.NewProgram(m,
		tea.WithInput(nil),
		tea.WithOutput(out),
		tea.WithWindowSize(width, height),
		tea.WithoutSignals(),
	)
	done := make(chan struct{})
	go func() {
		_, _ = p.Run()
		close(done)
	}()
	t.Cleanup(func() {
		p.Quit()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("program did not exit")
		}
	})
	return p, out
}

// urlProvider returns a fixed page list so the reader can download a test
// image without the network.
type urlProvider struct{ urls []string }

func (urlProvider) Name() string                                     { return "Stub" }
func (urlProvider) Search(string) ([]models.Manga, error)            { return nil, nil }
func (urlProvider) FetchLatest(int) ([]models.Manga, error)          { return nil, nil }
func (urlProvider) Filter(api.FilterOptions) ([]models.Manga, error) { return nil, nil }
func (urlProvider) FetchChapters(string) ([]models.Chapter, error)   { return nil, nil }
func (p urlProvider) FetchPages(string) ([]string, error)            { return p.urls, nil }

// TestReaderRawFrameOrderingEndToEnd drives a real bubbletea v2 program and
// checks the reader frame is painted via tea.Raw, survives the renderer's diff,
// and that leaving the reader emits the image delete + clear.
func TestReaderRawFrameOrderingEndToEnd(t *testing.T) {
	pngData := tinyPNG(t)
	rendered, err := imgrender.New().RenderInBox(pngData, 0, 0)
	if err != nil {
		t.Fatalf("render image: %v", err)
	}
	if rendered.EscapeSequence == "" {
		t.Fatal("expected a non-empty image escape")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngData)
	}))
	defer srv.Close()

	m := NewReaderModel("m1", "Title", "c1", "1", []string{"c1"}, 0, 0, urlProvider{urls: []string{srv.URL + "/p1.png"}})
	p, out := startProgram(t, m, 80, 24)

	waitForOutput(t, out, rendered.EscapeSequence, 5*time.Second)
	s := waitForOutput(t, out, "Trang 1/1", 5*time.Second)
	if idx := strings.Index(s, rendered.EscapeSequence); idx < 0 {
		t.Fatalf("image escape missing from output:\n%q", s)
	}

	// Let several renderer ticks pass: the blank view is settled, so nothing
	// may erase or repaint over the last raw frame (an earlier frame may have
	// been erased by the renderer's transition flush and repainted by the
	// first-frame tick).
	time.Sleep(120 * time.Millisecond)
	s = out.String()
	afterImage := s[strings.LastIndex(s, rendered.EscapeSequence):]
	if strings.Contains(afterImage, "\x1b[2J") || strings.Contains(afterImage, "\x1b[H\x1b[J") {
		t.Fatalf("renderer erased the raw frame after it was drawn:\n%q", afterImage)
	}

	// Leaving the reader (esc) must delete images and clear before the next
	// screen is rendered.
	p.Send(tea.KeyPressMsg{Code: tea.KeyEsc})
	waitForOutput(t, out, kittyClearSeq+"\x1b[H\x1b[2J", 5*time.Second)
}

// coverProbeModel runs a SearchModel without its Init (which would hit the
// network).
type coverProbeModel struct{ SearchModel }

func (m coverProbeModel) Init() tea.Cmd { return nil }

func (m coverProbeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	newM, cmd := m.SearchModel.Update(msg)
	m.SearchModel = newM.(SearchModel)
	return m, cmd
}

// TestSearchCoverDrawAfterPaneEndToEnd checks the cover image is drawn with
// tea.Raw only after the preview pane text has been rendered.
func TestSearchCoverDrawAfterPaneEndToEnd(t *testing.T) {
	coverEscape := "\x1b_Gf=100,a=T;COVERPAYLOAD\x1b\\"
	m := testSearchModel()
	m.width = 100
	m.height = 24
	m.coverCache = map[string]imgrender.RenderedImage{
		"https://example.com/cover.jpg": {EscapeSequence: coverEscape, WidthPx: 100, HeightPx: 150},
	}
	probe := coverProbeModel{SearchModel: m}
	p, out := startProgram(t, probe, 100, 24)

	p.Send(api.MangaSearchResultMsg{
		Manga: []models.Manga{
			{ID: "m1", Title: "One Piece", CoverURL: "https://example.com/cover.jpg", Provider: "OTruyen"},
		},
	})

	paneAt := strings.Index(waitForOutput(t, out, "Chi tiết manga", 5*time.Second), "Chi tiết manga")
	s := waitForOutput(t, out, coverEscape, 5*time.Second)
	if coverAt := strings.Index(s, coverEscape); coverAt < paneAt {
		t.Fatalf("cover drawn before the preview pane rendered (pane at %d, cover at %d)", paneAt, coverAt)
	}

	// The pane is stable after the cover lands, so no renderer erase may follow.
	time.Sleep(120 * time.Millisecond)
	s = out.String()
	afterCover := s[strings.LastIndex(s, coverEscape):]
	if strings.Contains(afterCover, "\x1b[2J") || strings.Contains(afterCover, "\x1b[H\x1b[J") {
		t.Fatalf("renderer erased the cover after it was drawn:\n%q", afterCover)
	}

	// Hiding the cover must emit the delete sequences. The draw sequence also
	// starts with a delete, so wait for the second (bare clear) occurrence.
	p.Send(api.MangaSearchResultMsg{Err: errors.New("boom")})
	deleteSeq := "\x1b_Ga=d,d=A,q=2\x1b\\"
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && strings.Count(out.String(), deleteSeq) < 2 {
		time.Sleep(2 * time.Millisecond)
	}
	s = out.String()
	if strings.Count(s, deleteSeq) < 2 {
		t.Fatalf("expected a cover clear after hiding, got:\n%q", s)
	}
	if strings.LastIndex(s, coverEscape) > strings.LastIndex(s, deleteSeq) {
		t.Fatalf("cover escape redrawn after clear:\n%q", s)
	}
}
