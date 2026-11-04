package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
	"github.com/mattn/go-runewidth"
)

// View reports a blank, non-empty screen to the renderer. The reader frame
// (image, footer, flash) is painted out-of-band with tea.Raw because the v2
// cell renderer strips cursor positioning and graphics sequences embedded in a
// View. The blank screen must stay non-empty: an empty View makes the renderer
// shrink its cell buffer to zero height and emit a full-screen erase on every
// frame, which would wipe the raw frame (verified against bubbletea v2.0.9).
func (m ReaderModel) View() tea.View {
	return tea.NewView(m.blankScreen())
}

// blankScreen returns a space-filled screen sized to the terminal.
func (m ReaderModel) blankScreen() string {
	w, h := m.width, m.height
	if ts, err := imgrender.GetTerminalSize(); err == nil && ts.Cols > 0 && ts.Rows > 0 {
		w, h = ts.Cols, ts.Rows
	}
	return strings.Repeat(" ", max(1, w*h))
}

// frameContent builds the entire reader frame. The layout and wording match the
// pre-v2 View byte for byte; only the transport changed to tea.Raw.
func (m ReaderModel) frameContent() string {
	var b strings.Builder

	b.WriteString("\x1b[H\x1b[2J")

	switch m.step {
	case stepFetchURLs:
		b.WriteString(m.centerText("Đang lấy danh sách ảnh..."))
		return b.String()

	case stepDownload:
		pct := 0
		if m.total > 0 {
			pct = m.downloaded * 100 / m.total
		}
		b.WriteString(m.centerText(fmt.Sprintf("Đang tải trang %d/%d - %d%%", m.downloaded, m.total, pct)))
		return b.String()

	case stepRead:
		img, cached := m.currentFrame()
		if pageErr, failed := m.failed[m.currentIdx]; failed && !m.isLoading {
			b.WriteString(m.centerText(fmt.Sprintf("Lỗi trang %d: %v — [r] Thử lại", m.currentIdx+1, pageErr)))
		} else if !cached || (m.isLoading && !m.fitWidth) {
			// Webtoon mode keeps the previous window up while scrolling.
			b.WriteString(m.centerText("Đang render ảnh..."))
			return b.String()
		} else {
			b.WriteString(m.centeredImage(img))
		}

		w, h := m.width, m.height
		if ts, err := imgrender.GetTerminalSize(); err == nil && ts.Cols > 0 && ts.Rows > 0 {
			w, h = ts.Cols, ts.Rows
		}

		if !m.hideFooter {
			rawFooter := m.footerText(w - 1)
			footerRow := max(1, h)
			footerCol := max(1, (w-runewidth.StringWidth(rawFooter))/2)

			styledFooter := lipgloss.NewStyle().
				Foreground(lipgloss.Color("8")).
				Render(rawFooter)

			b.WriteString(fmt.Sprintf("\x1b[%d;%dH", footerRow, footerCol))
			b.WriteString(styledFooter)
		}

		if m.flashMsg != "" {
			// One row only: a wrapped flash would spill onto the footer row.
			flash := runewidth.Truncate(m.flashMsg, max(1, w-2), "…")
			flashCol := max(1, (w-runewidth.StringWidth(flash))/2)
			flashRow := max(1, h-1)
			if m.hideFooter {
				flashRow = max(1, h)
			}
			styledFlash := lipgloss.NewStyle().
				Foreground(lipgloss.Color("3")).
				Bold(true).
				Render(flash)
			b.WriteString(fmt.Sprintf("\x1b[%d;%dH", flashRow, flashCol))
			b.WriteString(styledFlash)
		}
		return b.String()

	case stepLoadingNext:
		b.WriteString(m.centerText("Đang chuyển chapter..."))
		return b.String()

	case stepError:
		b.WriteString(m.centerText(fmt.Sprintf("Lỗi: %v", m.err)))
		return b.String()
	}

	return b.String()
}

// Fall back to model dimensions if terminal query fails (e.g. non-TTY).
func (m ReaderModel) centerText(text string) string {
	w, h := m.width, m.height
	if ts, err := imgrender.GetTerminalSize(); err == nil && ts.Cols > 0 && ts.Rows > 0 {
		w, h = ts.Cols, ts.Rows
	}
	textWidth := runewidth.StringWidth(text)
	col := max(1, (w-textWidth)/2)
	row := max(1, h/2)
	return fmt.Sprintf("\x1b[%d;%dH%s", row, col, text)
}

// imageRows returns 0 for the default size that reserves footer/flash rows, or
// the full terminal height when the footer is hidden.
func (m ReaderModel) imageRows() int {
	if !m.hideFooter {
		return 0
	}
	if ts, err := imgrender.GetTerminalSize(); err == nil && ts.Rows > 0 {
		return ts.Rows
	}
	return max(1, m.height)
}

// Convert image pixel size to terminal cell count — Kitty/Sixel geometry math.
func (m ReaderModel) imageRect(img imgrender.RenderedImage) (offsetX, offsetY int) {
	ts, err := imgrender.GetTerminalSize()
	if err != nil || ts.Cols <= 0 || ts.Rows <= 0 {
		ts = imgrender.TerminalSize{Cols: m.width, Rows: m.height, PxW: m.width * 8, PxH: m.height * 16}
	}
	if ts.PxW <= 0 {
		ts.PxW = ts.Cols * 8
	}
	if ts.PxH <= 0 {
		ts.PxH = ts.Rows * 16
	}

	cellW := max(1, ts.PxW/ts.Cols)
	cellH := max(1, ts.PxH/ts.Rows)
	cellsW := max(1, (img.WidthPx+cellW-1)/cellW)
	cellsH := max(1, (img.HeightPx+cellH-1)/cellH)
	offsetX = max(0, (ts.Cols-cellsW)/2)
	reservedRows := 2
	if m.hideFooter {
		reservedRows = 0
	}
	offsetY = max(0, (ts.Rows-reservedRows-cellsH)/2)
	return
}

func (m ReaderModel) centeredImage(img imgrender.RenderedImage) string {
	offsetX, offsetY := m.imageRect(img)
	return fmt.Sprintf("\x1b[%d;%dH", offsetY+1, offsetX+1) + img.EscapeSequence
}

// footerHint is one reader footer entry; lower prio values are kept longer.
type footerHint struct {
	text string
	prio int
}

// footerText lists the reader's key hints in display order, dropping the
// least important ones until the line fits width: the footer must never wrap,
// since a second line would scroll the whole reader frame.
func (m ReaderModel) footerText(width int) string {
	page := fmt.Sprintf("[󰈙 Trang %d/%d]", m.currentIdx+1, m.total)
	if pct, ok := m.scrollPercent(); ok {
		page = fmt.Sprintf("[󰈙 Trang %d/%d · %d%%]", m.currentIdx+1, m.total, pct)
	}
	hints := []footerHint{{page, 0}, {"[←/h] [→/l] Trang", 2}}
	if m.fitWidth {
		hints = append(hints, footerHint{"[↑/↓] Cuộn", 2}, footerHint{"[k/j] Chap", 3}, footerHint{"[w] Vừa màn hình", 4})
	} else {
		hints = append(hints, footerHint{"[↑/k] [↓/j] Chap", 3}, footerHint{"[w] Webtoon", 4})
	}
	if m.hasPreviousChapter() && m.currentIdx == 0 {
		hints = append(hints, footerHint{"[←] Chap trước", 8})
	}
	if m.hasNextChapter() && m.currentIdx == m.total-1 {
		hints = append(hints, footerHint{"[→] Chap tiếp", 8})
	}
	hints = append(hints,
		footerHint{"[ctrl+e] Xuất CBZ", 6},
		footerHint{"[ctrl+d] Lưu ảnh", 5},
		footerHint{"[ctrl+h] Ẩn gợi ý", 7},
		footerHint{"[esc] Thoát", 1},
	)

	join := func(hs []footerHint) string {
		parts := make([]string, len(hs))
		for i, h := range hs {
			parts[i] = h.text
		}
		return strings.Join(parts, "  ")
	}
	for runewidth.StringWidth(join(hints)) > width && len(hints) > 1 {
		drop := 0
		for i, h := range hints {
			if h.prio > hints[drop].prio {
				drop = i
			}
		}
		hints = append(hints[:drop], hints[drop+1:]...)
	}
	return runewidth.Truncate(join(hints), max(1, width), "…")
}
