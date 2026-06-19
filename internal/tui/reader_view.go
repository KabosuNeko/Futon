package tui

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

func (m ReaderModel) View() string {
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
		img, cached := m.getCached(m.currentIdx)
		if m.isLoading || !cached {
			b.WriteString(m.centerText("Đang render ảnh..."))
			return b.String()
		}

		offsetX, offsetY, _, cellsH := m.imageRect(img)
		b.WriteString(m.centeredImage(img))

		footer := fmt.Sprintf("Trang %d/%d | <-/h ->/l : chuyển trang | esc: thoát | ctrl+d: tải ảnh", m.currentIdx+1, m.total)
		if m.hasNextChapter() && m.currentIdx == m.total-1 {
			footer += " | ->: chap tiếp"
		}
		if m.hasPreviousChapter() && m.currentIdx == 0 {
			footer += " | <-: chap trước"
		}
		footerRow := offsetY + cellsH + 1
		footerCol := offsetX + 1
		b.WriteString(fmt.Sprintf("\x1b[%d;%dH", footerRow, footerCol))
		b.WriteString(footer)

		if m.flashMsg != "" {
			flashRow := footerRow + 1
			b.WriteString(fmt.Sprintf("\x1b[%d;%dH", flashRow, footerCol))
			b.WriteString(m.flashMsg)
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

func (m ReaderModel) imageRect(img imgrender.RenderedImage) (offsetX, offsetY, cellsW, cellsH int) {
	ts, err := imgrender.GetTerminalSize()
	if err != nil {
		ts = imgrender.TerminalSize{Cols: m.width, Rows: m.height, PxW: m.width * 8, PxH: m.height * 16}
	}
	if ts.Cols <= 0 {
		ts.Cols = m.width
	}
	if ts.Rows <= 0 {
		ts.Rows = m.height
	}
	if ts.PxW <= 0 {
		ts.PxW = ts.Cols * 8
	}
	if ts.PxH <= 0 {
		ts.PxH = ts.Rows * 16
	}

	cellW := max(1, ts.PxW/ts.Cols)
	cellH := max(1, ts.PxH/ts.Rows)
	cellsW = max(1, img.WidthPx/cellW)
	cellsH = max(1, img.HeightPx/cellH)
	offsetX = max(0, (ts.Cols-cellsW)/2)
	offsetY = max(0, (ts.Rows-cellsH)/2)
	return
}

func (m ReaderModel) centeredImage(img imgrender.RenderedImage) string {
	offsetX, offsetY, _, _ := m.imageRect(img)
	return fmt.Sprintf("\x1b[%d;%dH", offsetY+1, offsetX+1) + img.EscapeSequence
}
