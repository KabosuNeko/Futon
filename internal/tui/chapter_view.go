package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/KabosuNeko/Futon/internal/storage"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

func (m ChapterListModel) View() string {
	w, h := m.width, m.height
	if ts, err := imgrender.GetTerminalSize(); err == nil && ts.Cols > 0 && ts.Rows > 0 {
		w, h = ts.Cols, ts.Rows
	}
	if w == 0 || h == 0 {
		return "Loading..."
	}

	headerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(0, 2).
		Bold(true)

	headerText := m.mangaTitle
	if headerText == "" {
		headerText = "Danh sách Chapter"
	}
	header := headerStyle.Render(headerText)

	var body string
	if m.err != nil {
		errStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			MarginTop(1)
		body = lipgloss.JoinVertical(lipgloss.Center, body,
			errStyle.Render(fmt.Sprintf("Lỗi: %v", m.err)))
	} else if m.loading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")).
			MarginTop(1)
		body = lipgloss.JoinVertical(lipgloss.Center, body,
			loadingStyle.Render("Đang tải danh sách chapter..."))
	} else if len(m.chapters) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			MarginTop(1)
		body = lipgloss.JoinVertical(lipgloss.Center, body,
			emptyStyle.Render("Không có chapter nào."))
	} else {
		normalStyle := lipgloss.NewStyle().MarginTop(0)
		selectedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("51")).
			MarginTop(0)

		visible := m.visibleItems()
		viewportEnd := m.viewportStart + visible
		if viewportEnd > len(m.chapters) {
			viewportEnd = len(m.chapters)
		}

		var lines []string
		for i := m.viewportStart; i < viewportEnd; i++ {
			ch := m.chapters[i]
			prefix := "  "
			style := normalStyle
			if i == m.cursor {
				prefix = "> "
				style = selectedStyle
			}

			title := ch.Title
			if title == "" {
				title = "Không tiêu đề"
			}
			lines = append(lines,
				style.Render(fmt.Sprintf("%sCh. %s - %s", prefix, ch.Number, title)),
			)
		}
		body = lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	var hint string
	if m.inputBuffer != "" {
		hint = fmt.Sprintf("Chuyển đến Chapter: %s█", m.inputBuffer)
	} else {
		hint = "↑/↓: chọn  |  esc: quay lại  |  q: thoát  |  ctrl+f: yêu thích  |  gõ số + Enter: nhảy chapter"
	}

	content := lipgloss.JoinVertical(lipgloss.Center, header, body, hintStyle.Render(hint))
	placed := lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, content)

	if m.flashMsg == "" {
		return placed
	}
	flashStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true)
	flash := flashStyle.Render(m.flashMsg)
	return placed + "\n" + flash
}

func (m *ChapterListModel) jumpToChapter(number string) bool {
	for i, ch := range m.chapters {
		if ch.Number == number {
			m.cursor = i
			m.adjustViewport()
			return true
		}
	}
	return false
}

func (m *ChapterListModel) restoreHistoryPosition() {
	if m.mangaID == "" || len(m.chapters) == 0 {
		return
	}
	history, ok := storage.GetHistory(m.mangaID)
	if !ok {
		return
	}
	for i, ch := range m.chapters {
		if ch.ID == history.ChapterID {
			m.cursor = i
			m.adjustViewport()
			return
		}
	}
}

func (m ChapterListModel) visibleItems() int {
	h := m.height
	if ts, err := imgrender.GetTerminalSize(); err == nil && ts.Rows > 0 {
		h = ts.Rows
	}
	visible := h - 4
	if visible < 1 {
		visible = 1
	}
	return visible
}

func (m *ChapterListModel) adjustViewport() {
	visible := m.visibleItems()
	if m.cursor < m.viewportStart {
		m.viewportStart = m.cursor
	}
	if m.cursor >= m.viewportStart+visible {
		m.viewportStart = m.cursor - visible + 1
	}
	if m.viewportStart < 0 {
		m.viewportStart = 0
	}
	if m.viewportStart >= len(m.chapters) && len(m.chapters) > 0 {
		m.viewportStart = len(m.chapters) - 1
	}
}
