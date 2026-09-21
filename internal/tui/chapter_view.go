package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/KabosuNeko/Futon/internal/storage"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
	"github.com/mattn/go-runewidth"
)

func (m ChapterListModel) View() tea.View {
	w, h := m.width, m.height
	if ts, err := imgrender.GetTerminalSize(); err == nil && ts.Cols > 0 && ts.Rows > 0 {
		w, h = ts.Cols, ts.Rows
	}
	if w == 0 || h == 0 {
		return tea.NewView("Loading...")
	}

	boxW := min(76, max(36, w-8))
	visible := m.visibleItems()
	boxH := visible + 4

	titleText := m.mangaTitle
	if titleText == "" {
		titleText = "Danh sách Chapter"
	}
	cleanedTitle := titleText
	if m.provider != nil {
		cleanedTitle = cleanTitle(titleText, m.provider.Name())
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("2"))

	var headerParts []string
	headerParts = append(headerParts, titleStyle.Render("󰈙 "+cleanedTitle))
	if m.provider != nil && m.provider.Name() != "" {
		headerParts = append(headerParts, providerBadge(m.provider.Name()))
	}
	headerLine := strings.Join(headerParts, "  ")

	var subHeader string
	if m.inputBuffer != "" {
		jumpStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("3")).
			Padding(0, 1).
			Foreground(lipgloss.Color("3")).
			Bold(true)
		subHeader = jumpStyle.Render(fmt.Sprintf("󰅂 Nhảy đến Chapter: %s█", m.inputBuffer))
	}

	headerElements := []string{headerLine}
	if subHeader != "" {
		headerElements = append(headerElements, subHeader)
	}
	headerNode := lipgloss.JoinVertical(lipgloss.Center, headerElements...)

	cardHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6"))

	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))

	historyBadgeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("3")).
		Italic(true)

	var contentLines []string
	contentLines = append(contentLines, cardHeaderStyle.Render(fmt.Sprintf("󰋑 Danh sách chapter (%d)", len(m.chapters))))
	contentLines = append(contentLines, dividerStyle.Render(strings.Repeat("─", max(1, boxW-4))))

	if m.err != nil {
		errStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)
		contentLines = append(contentLines, errStyle.Render(fmt.Sprintf("󰅚 Lỗi: %v", m.err)))
	} else if m.loading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			Italic(true)
		contentLines = append(contentLines, loadingStyle.Render(" Đang tải danh sách chapter..."))
	} else if len(m.chapters) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)
		contentLines = append(contentLines, emptyStyle.Render("󰅚 Không có chapter nào."))
	} else {
		history, hasHistory := storage.GetHistory(m.mangaID)

		viewportEnd := m.viewportStart + visible
		if viewportEnd > len(m.chapters) {
			viewportEnd = len(m.chapters)
		}

		maxLineW := boxW - 4
		for i := m.viewportStart; i < viewportEnd; i++ {
			ch := m.chapters[i]
			isCursor := i == m.cursor
			isHistory := hasHistory && ch.ID == history.ChapterID

			prefix := "  "
			if isCursor {
				prefix = "> "
			}

			title := ch.Title
			if title == "" {
				title = "Không tiêu đề"
			}

			truncW := maxLineW
			badge := ""
			if isHistory {
				badge = " [Đang đọc]"
				truncW = max(10, maxLineW-runewidth.StringWidth(badge))
			}
			line := runewidth.Truncate(fmt.Sprintf("%sCh. %s - %s", prefix, ch.Number, title), truncW, "...")
			if isCursor {
				line = selectedStyle.Render(line)
			} else {
				line = normalStyle.Render(line)
			}
			if badge != "" {
				line += historyBadgeStyle.Render(badge)
			}
			contentLines = append(contentLines, line)
		}
	}

	innerW := max(1, boxW-4)
	var paddedLines []string
	for _, line := range contentLines {
		lw := runewidth.StringWidth(line)
		pad := innerW - lw
		if pad > 0 {
			line = line + strings.Repeat(" ", pad)
		}
		paddedLines = append(paddedLines, line)
	}
	for len(paddedLines) < boxH-2 {
		paddedLines = append(paddedLines, strings.Repeat(" ", innerW))
	}

	cardContent := lipgloss.JoinVertical(lipgloss.Left, paddedLines...)

	cardBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(boxW).
		Height(boxH).
		Render(cardContent)

	var footer string
	if m.inputBuffer != "" {
		footer = "[enter] Nhảy đến chapter  [esc] Hủy  [^c] Thoát"
	} else {
		footer = "[enter] Mở đọc  [↑/↓] Chọn  [ctrl+e] Xuất chap  [ctrl+a] Xuất cả bộ  [ctrl+f] Yêu thích  [gõ số] Nhảy chap  [esc] Quay lại"
	}

	footerNode := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Render(footer)

	var elements []string
	elements = append(elements, headerNode)
	elements = append(elements, cardBox)
	elements = append(elements, footerNode)

	content := lipgloss.JoinVertical(lipgloss.Center, elements...)

	contentH := lipgloss.Height(content)
	topPad := max(0, (h-contentH)/2)

	placed := lipgloss.PlaceHorizontal(w, lipgloss.Center, content)
	if topPad > 0 {
		placed = strings.Repeat("\n", topPad) + placed
	}

	if m.flashMsg != "" {
		flashStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			Bold(true)
		placed = placed + "\n" + flashStyle.Render(m.flashMsg)
	}

	return tea.NewView(placed)
}

func (m *ChapterListModel) jumpToChapter(number string) bool {
	q := strings.TrimSpace(number)
	if q == "" {
		return false
	}
	for i, ch := range m.chapters {
		if ch.Number == q {
			m.cursor = i
			m.adjustViewport()
			return true
		}
	}
	for i, ch := range m.chapters {
		if strings.HasPrefix(ch.Number, q) {
			m.cursor = i
			m.adjustViewport()
			return true
		}
	}
	for i, ch := range m.chapters {
		if strings.Contains(ch.Number, q) {
			m.cursor = i
			m.adjustViewport()
			return true
		}
	}
	lowerQ := strings.ToLower(q)
	for i, ch := range m.chapters {
		if ch.Number == "" && strings.Contains(strings.ToLower(ch.Title), lowerQ) {
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
	visible := h - 10
	if m.inputBuffer != "" {
		visible -= 3
	}
	if visible < 4 {
		visible = 4
	}
	if visible > 18 {
		visible = 18
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
