package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

// coverPaintDelay lets the renderer flush the view that reserves the preview
// area before the cover is drawn. Raw output is flushed before the renderer's
// diff within a tick, so an immediate draw can land before the reserving pane
// (and a resize forces a full renderer erase that would wipe it). Deferring one
// frame puts the draw strictly after the render.
const coverPaintDelay = 30 * time.Millisecond

type coverPaintMsg struct{ key coverPaintKey }

// coverPaintState computes the cover paint key for the current state.
func (m SearchModel) coverPaintState() coverPaintKey {
	_, layout := m.buildContent()
	return layout.cover
}

// clearCoverCmd removes the preview cover from the terminal and invalidates any
// pending paint so the image does not ghost over the next screen. It is emitted
// before the state switches away from search; the clear must land while the
// search view is still the renderer's buffer, otherwise it could erase cells of
// the screen that replaces it.
func (m *SearchModel) clearCoverCmd() tea.Cmd {
	key := m.coverPaintState()
	key.draw = false
	m.coverKey = coverPaintKey{}
	return tea.Raw(m.coverSequence(key))
}

// repaintCoverCmd forces the preview cover to be painted again. The reader
// deletes every terminal image when it exits, so a cover on screen before must
// be redrawn when the search screen comes back.
func (m *SearchModel) repaintCoverCmd() tea.Cmd {
	key := m.coverPaintState()
	m.coverKey = key
	return tea.Tick(coverPaintDelay, func(time.Time) tea.Msg {
		return coverPaintMsg{key: key}
	})
}

// setCover records the rendered preview cover and bumps coverGen so the
// out-of-band cover paint is re-emitted.
func (m *SearchModel) setCover(img *imgrender.RenderedImage) {
	if m.currentCover == img {
		return
	}
	m.currentCover = img
	m.coverGen++
}

func (m *SearchModel) updateFocusedCover() tea.Cmd {
	focused, ok := m.focusedManga()
	if ok && focused.CoverURL == "" && m.canLookupCover(focused) {
		// Debounced like a cover fetch: scrolling past entries must not send
		// a page request for each of them.
		m.setCover(nil)
		m.currentCoverID = focused.ID
		m.coverLoading = true
		return debounceCover(focused.ID, "", 150*time.Millisecond)
	}
	if !ok || focused.CoverURL == "" {
		m.setCover(nil)
		m.currentCoverID = ""
		m.coverLoading = false
		return nil
	}
	if m.currentCoverID == focused.ID && (m.currentCover != nil || m.coverLoading) {
		return nil
	}
	m.currentCoverID = focused.ID
	if img, cached := m.coverCache[focused.CoverURL]; cached {
		m.setCover(&img)
		m.coverLoading = false
		return nil
	}
	m.setCover(nil)
	m.coverLoading = true
	return debounceCover(focused.ID, focused.CoverURL, 150*time.Millisecond)
}

// canLookupCover reports whether focused is a fav/his entry whose cover was
// not looked up yet this session.
func (m SearchModel) canLookupCover(focused models.Manga) bool {
	return (m.view == viewFavorites || m.view == viewHistory) && !m.coverLookups[focused.Provider+"\x00"+focused.ID]
}

// lookupCover asks the source for focused's cover, once per session.
func (m *SearchModel) lookupCover(focused models.Manga) tea.Cmd {
	if m.coverLookups == nil {
		m.coverLookups = make(map[string]bool)
	}
	m.coverLookups[focused.Provider+"\x00"+focused.ID] = true
	m.coverLoading = true
	return lookupCoverCmd(m.providerByName(focused.Provider), focused.ID, focused.Provider)
}

// coverFoundMsg carries the cover looked up for a fav/his entry saved without one.
type coverFoundMsg struct {
	mangaID, provider, url string
	err                    error
}

func lookupCoverCmd(p api.MangaProvider, mangaID, provider string) tea.Cmd {
	return func() tea.Msg {
		url, err := api.LookupCover(context.Background(), p, mangaID)
		return coverFoundMsg{mangaID: mangaID, provider: provider, url: url, err: err}
	}
}

// applyFoundCover stores a looked-up cover in the lists and on disk, then
// shows it if its entry is still focused.
func (m SearchModel) applyFoundCover(msg coverFoundMsg) (SearchModel, tea.Cmd) {
	// The flag belongs to the focused entry, which may be another one by now.
	if msg.mangaID == m.currentCoverID {
		m.coverLoading = false
	}
	if msg.err != nil {
		return m, m.updateFocusedCover()
	}
	for i, f := range m.favorites {
		if f.MangaID == msg.mangaID && f.Provider == msg.provider {
			m.favorites[i].CoverURL = msg.url
		}
	}
	for i, h := range m.history {
		if h.MangaID == msg.mangaID && h.Provider == msg.provider {
			m.history[i].CoverURL = msg.url
		}
	}
	return m, tea.Batch(saveFoundCoverCmd(msg.mangaID, msg.provider, msg.url), m.updateFocusedCover())
}

type coverDebounceMsg struct {
	mangaID  string
	coverURL string
}

type coverRenderedMsg struct {
	mangaID  string
	coverURL string
	rendered imgrender.RenderedImage
	err      error
}

func debounceCover(mangaID, coverURL string, delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return coverDebounceMsg{
			mangaID:  mangaID,
			coverURL: coverURL,
		}
	})
}

func fetchAndRenderCoverCmd(renderer imgrender.Renderer, provider api.MangaProvider, mangaID, coverURL string, cols, rows int) tea.Cmd {
	return func() tea.Msg {
		data, err := api.FetchCover(context.Background(), provider, coverURL)
		if err != nil {
			return coverRenderedMsg{mangaID: mangaID, coverURL: coverURL, err: err}
		}
		rendered, err := renderer.RenderInBox(data, cols, rows)
		if err != nil {
			return coverRenderedMsg{mangaID: mangaID, coverURL: coverURL, err: err}
		}
		return coverRenderedMsg{
			mangaID:  mangaID,
			coverURL: coverURL,
			rendered: rendered,
		}
	}
}

// coverPaintKey captures everything that determines the out-of-band cover
// paint: the pane geometry, whether an image is drawn, and a generation
// counter bumped whenever the cover changes.
type coverPaintKey struct {
	gen       int
	isSplit   bool
	draw      bool
	clearFrom int
	clearTo   int
	eraseCol  int
	eraseW    int
	imgRow    int
	imgCol    int
}

// sameCoverArea reports whether two paint keys cover the same screen cells.
func sameCoverArea(a, b coverPaintKey) bool {
	return a.isSplit == b.isSplit && a.clearFrom == b.clearFrom && a.clearTo == b.clearTo &&
		a.eraseCol == b.eraseCol && a.eraseW == b.eraseW
}

// coverSequence mirrors the sequences the pre-v2 View appended for the preview
// cover, including the save/restore cursor wrapper.
func (m SearchModel) coverSequence(key coverPaintKey) string {
	const deleteAll = "\x1b_Ga=d,d=A,q=2\x1b\\\x1b_Ga=d,d=a,q=2\x1b\\"
	if !key.isSplit {
		return deleteAll
	}

	var clearSeq strings.Builder
	clearSeq.WriteString(deleteAll)
	for r := key.clearFrom; r <= key.clearTo; r++ {
		clearSeq.WriteString(fmt.Sprintf("\x1b[%d;%dH\x1b[%dX", r, key.eraseCol, key.eraseW))
	}

	if !key.draw {
		return fmt.Sprintf("\x1b[s%s\x1b[u", clearSeq.String())
	}
	return fmt.Sprintf("\x1b[s%s\x1b[%d;%dH%s\x1b[u", clearSeq.String(), key.imgRow, key.imgCol, m.currentCover.EscapeSequence)
}

func (m SearchModel) previewBoxSize() (cols, rows int) {
	if m.width < 80 {
		return 0, 0
	}
	previewW := m.previewPaneWidth()
	cols = previewW - 8
	boxH := m.listVisibleItems() + 4
	rows = boxH - 11
	if cols < 12 {
		cols = 12
	}
	if rows < 4 {
		rows = 4
	}
	if rows > 14 {
		rows = 14
	}
	return cols, rows
}
