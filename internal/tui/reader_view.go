package tui

import (
	"fmt"
	"strings"

	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
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

		b.WriteString(m.centeredImage(img))

		var footerParts []string
		footerParts = append(footerParts, fmt.Sprintf("[󰈙 Trang %d/%d]", m.currentIdx+1, m.total))
		footerParts = append(footerParts, "[󰁕/h] [󰁖/l] Trang")
		if m.hasPreviousChapter() && m.currentIdx == 0 {
			footerParts = append(footerParts, "[󰁕] Chap trước")
		}
		if m.hasNextChapter() && m.currentIdx == m.total-1 {
			footerParts = append(footerParts, "[󰁖] Chap tiếp")
		}
		footerParts = append(footerParts, "[ctrl+e] Xuất CBZ")
		footerParts = append(footerParts, "[esc] Thoát")
		footerParts = append(footerParts, "[ctrl+d] Lưu ảnh")

		rawFooter := strings.Join(footerParts, "  ")

		w, h := m.width, m.height
		if ts, err := imgrender.GetTerminalSize(); err == nil && ts.Cols > 0 && ts.Rows > 0 {
			w, h = ts.Cols, ts.Rows
		}
		footerRow := max(1, h)
		footerCol := max(1, (w-runewidth.StringWidth(rawFooter))/2)

		styledFooter := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Render(rawFooter)

		b.WriteString(fmt.Sprintf("\x1b[%d;%dH", footerRow, footerCol))
		b.WriteString(styledFooter)

		if m.flashMsg != "" {
			flashCol := max(1, (w-runewidth.StringWidth(m.flashMsg))/2)
			flashRow := max(1, footerRow-1)
			styledFlash := lipgloss.NewStyle().
				Foreground(lipgloss.Color("3")).
				Bold(true).
				Render(m.flashMsg)
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

// Convert image pixel size to terminal cell count — Kitty/Sixel geometry math.
func (m ReaderModel) imageRect(img imgrender.RenderedImage) (offsetX, offsetY, cellsW, cellsH int) {
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
	cellsW = max(1, img.WidthPx/cellW)
	cellsH = max(1, img.HeightPx/cellH)
	offsetX = max(0, (ts.Cols-cellsW)/2)
	offsetY = max(0, (ts.Rows-2-cellsH)/2)
	return
}

func (m ReaderModel) centeredImage(img imgrender.RenderedImage) string {
	offsetX, offsetY, _, _ := m.imageRect(img)
	return fmt.Sprintf("\x1b[%d;%dH", offsetY+1, offsetX+1) + img.EscapeSequence
}
