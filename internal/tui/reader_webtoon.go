package tui

import (
	"fmt"
	"image"
	"math"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/KabosuNeko/Futon/internal/storage"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

// Webtoon (fit-width) mode: a page is scaled to the terminal width and
// shown through a window of terminal height that scrolls. Each page is
// decoded and scaled once (scaledPage); a scroll only crops and encodes the
// visible part.

// bottomOfPage asks a render to clamp the window to the page bottom, used
// when reading upwards into the previous page.
const bottomOfPage = math.MaxInt32

// scaledPage is the current page scaled to the terminal width.
type scaledPage struct {
	idx       int
	chapterID string
	img       image.Image
	rows      int // page height in terminal rows
}

// windowFrame is the last rendered window of the current page. It stays on
// screen while the next window renders, so scrolling does not flicker.
type windowFrame struct {
	idx       int
	chapterID string
	scroll    int
	img       imgrender.RenderedImage
	ok        bool
}

// viewRows is how many terminal rows the image may use.
func (m ReaderModel) viewRows() int {
	rows := m.height
	if ts, err := imgrender.GetTerminalSize(); err == nil && ts.Rows > 0 {
		rows = ts.Rows
	}
	if !m.hideFooter {
		rows -= 2 // flash + footer
	}
	return max(1, rows)
}

// renderCmd renders page idx for the current mode: fitted to the screen, or
// (webtoon mode) the window at the current scroll for the page on screen and
// the top window for pages rendered ahead.
func (m ReaderModel) renderCmd(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.imageData) || len(m.imageData[idx]) == 0 {
		return nil
	}
	if !m.fitWidth {
		return renderPage(m.renderer, m.imageData[idx], idx, m.chapterID, m.renderGen, m.imageRows())
	}
	scroll := 0
	var scaled image.Image
	if idx == m.currentIdx {
		scroll = m.scrollRow
	}
	if s := m.scaledFor(idx); s != nil {
		scaled = s.img
	}
	return renderWindow(m.renderer, m.imageData[idx], scaled, idx, m.chapterID, m.renderGen, scroll, m.viewRows())
}

func renderWindow(r imgrender.Renderer, data []byte, scaled image.Image, index int, chapterID string, gen, scroll, rows int) tea.Cmd {
	return func() tea.Msg {
		fail := func(err error) tea.Msg {
			return renderDoneMsg{index: index, chapterID: chapterID, gen: gen, err: fmt.Errorf("render trang %d: %w", index+1, err)}
		}
		if scaled == nil {
			var err error
			if scaled, err = imgrender.ScaleToWidth(data, 0); err != nil {
				return fail(err)
			}
		}
		win, top, total := imgrender.Window(scaled, scroll, rows)
		img, err := r.RenderImage(win)
		if err != nil {
			return fail(err)
		}
		return renderDoneMsg{
			index: index, chapterID: chapterID, gen: gen, img: img,
			windowed: true, scaled: scaled, requested: scroll, scroll: top, totalRows: total,
		}
	}
}

// scaledFor returns page idx already scaled to the width, if kept.
func (m ReaderModel) scaledFor(idx int) *scaledPage {
	for _, s := range []*scaledPage{m.scaled, m.nextScaled} {
		if s != nil && s.idx == idx && s.chapterID == m.chapterID {
			return s
		}
	}
	return nil
}

// handleWindowRendered stores a webtoon-mode render.
func (m ReaderModel) handleWindowRendered(msg renderDoneMsg) (ReaderModel, tea.Cmd) {
	if msg.scroll == 0 {
		m.setCached(msg.index, msg.img) // page tops double as the prefetch cache
	}
	page := &scaledPage{idx: msg.index, chapterID: msg.chapterID, img: msg.scaled, rows: msg.totalRows}
	if msg.index == m.currentIdx+1 {
		m.nextScaled = page
	}
	if msg.index == m.currentIdx {
		m.scaled = page
		if msg.requested != m.scrollRow {
			return m, nil // superseded by a newer scroll, which is on its way
		}
		m.scrollRow = msg.scroll
		m.window = windowFrame{idx: msg.index, chapterID: msg.chapterID, scroll: msg.scroll, img: msg.img, ok: true}
		m.isLoading = false
	}
	cmds := m.scheduleDownloads()
	if next := m.nextRenderIndex(); next >= 0 {
		cmds = append(cmds, m.renderCmd(next))
	}
	return m, tea.Batch(cmds...)
}

// currentFrame is the image to draw for the page on screen.
func (m ReaderModel) currentFrame() (imgrender.RenderedImage, bool) {
	if m.fitWidth {
		if m.window.ok && m.window.idx == m.currentIdx && m.window.chapterID == m.chapterID {
			return m.window.img, true
		}
		if m.scrollRow != 0 {
			return imgrender.RenderedImage{}, false
		}
	}
	return m.getCached(m.currentIdx)
}

// scrollPage scrolls half a screen in webtoon mode, moving to the next page
// past the bottom and to the bottom of the previous page above the top.
func (m ReaderModel) scrollPage(dir int) (ReaderModel, tea.Cmd) {
	if m.step != stepRead || m.isLoading {
		return m, nil
	}
	if _, failed := m.failed[m.currentIdx]; failed {
		if dir > 0 {
			return m.handleNextPage()
		}
		return m.handlePrevPage()
	}
	view := m.viewRows()
	step := max(1, view/2)
	page := m.scaledFor(m.currentIdx)

	if dir > 0 {
		if page != nil && m.scrollRow+view >= page.rows {
			return m.handleNextPage()
		}
		m.scrollRow += step
	} else {
		if m.scrollRow == 0 {
			prevIdx, chapter := m.currentIdx, m.chapterID
			nm, cmd := m.handlePrevPage()
			if nm.chapterID == chapter && nm.currentIdx == prevIdx-1 {
				nm.scrollRow = bottomOfPage
				nm.isLoading = true
				return nm, tea.Batch(cmd, nm.renderCmd(nm.currentIdx))
			}
			return nm, cmd
		}
		m.scrollRow = max(0, m.scrollRow-step)
	}
	m.isLoading = true
	return m, m.renderCmd(m.currentIdx)
}

// toggleFitWidth switches between fit-to-screen and webtoon mode and
// remembers the choice.
func (m ReaderModel) toggleFitWidth() (ReaderModel, tea.Cmd) {
	m.fitWidth = !m.fitWidth
	on := m.fitWidth
	m.scrollRow = 0
	m.resetImageCache()
	m.flashMsg = "Chế độ: vừa màn hình"
	if m.fitWidth {
		m.flashMsg = "Chế độ webtoon: vừa chiều ngang, [↑/↓] cuộn"
	}
	cmds := []tea.Cmd{saveUserDataCmd(func(ud *storage.UserData) { ud.ReaderFitWidth = on }), clearFlashAfter(2 * time.Second)}
	if m.step == stepRead && m.validCurrentImage() {
		m.isLoading = true
		cmds = append(cmds, m.renderCmd(m.currentIdx))
	}
	return m, tea.Batch(cmds...)
}

// scrollPercent is how far down the current page the window is (webtoon mode).
func (m ReaderModel) scrollPercent() (int, bool) {
	page := m.scaledFor(m.currentIdx)
	if !m.fitWidth || page == nil {
		return 0, false
	}
	span := page.rows - m.viewRows()
	if span <= 0 {
		return 100, true
	}
	return min(100, m.scrollRow*100/span), true
}
