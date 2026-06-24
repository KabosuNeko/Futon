package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m SearchModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(boxColor(m.input.Value())).
		Padding(1, 2)

	content := boxStyle.Render(m.input.View())

	if m.systemMsg != "" {
		sysStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			MarginTop(1)
		content = lipgloss.JoinVertical(lipgloss.Center, content,
			sysStyle.Render(m.systemMsg))
	}

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		MarginTop(1)
	subtleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		MarginTop(1)

	if m.err != nil {
		errStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			MarginTop(1)
		errMsg := errStyle.Render(fmt.Sprintf("Lỗi: %v", m.err))
		content = lipgloss.JoinVertical(lipgloss.Center, content, errMsg)
	} else if m.isSearching {
		content = lipgloss.JoinVertical(lipgloss.Center, content,
			statusStyle.Render("Đang tìm kiếm..."))
	} else if m.loadingFavorites {
		content = lipgloss.JoinVertical(lipgloss.Center, content,
			statusStyle.Render("Đang tải danh sách yêu thích..."))
	} else if m.loadingHistory {
		content = lipgloss.JoinVertical(lipgloss.Center, content,
			statusStyle.Render("Đang tải lịch sử đọc..."))
	} else if m.showingFavorites {
		content = m.renderFavorites(content)
	} else if m.showingHistory {
		content = m.renderHistory(content)
	} else if len(m.currentQuery) >= 3 && len(m.results) == 0 {
		content = lipgloss.JoinVertical(lipgloss.Center, content,
			subtleStyle.Render("Không tìm thấy kết quả."))
	}

	if !m.showingFavorites && len(m.results) > 0 {
		content = m.renderSearchResults(content)
	}

	sourceName := "All"
	if m.providerIdx >= 0 {
		if p := m.CurrentProvider(); p != nil {
			sourceName = p.Name()
		}
	}

	var footer string
	switch {
	case m.showingFavorites:
		footer = fmt.Sprintf("enter: mở truyện  |  ctrl+d: xóa yêu thích  |  esc: quay lại  |  ctrl+c: thoát  |  Nguồn: %s", sourceName)
	case m.showingHistory:
		footer = fmt.Sprintf("enter: mở truyện  |  ctrl+d: xóa lịch sử  |  esc: quay lại  |  ctrl+c: thoát  |  Nguồn: %s", sourceName)
	default:
		footer = fmt.Sprintf("ctrl+c: thoát  |  /fav: truyện yêu thích  |  /his: lịch sử đọc  |  /lang: chỉnh ngôn ngữ  |  tab: đổi nguồn  |  Nguồn: %s", sourceName)
	}
	content = lipgloss.JoinVertical(lipgloss.Center, content, subtleStyle.Render(footer))

	placed := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)

	if m.flashMsg == "" {
		return placed
	}
	flashStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true)
	flash := flashStyle.Render(m.flashMsg)
	return placed + "\n" + flash
}

func (m SearchModel) renderList(title, emptyMsg string, items []string, cursor int) string {
	if len(items) == 0 && emptyMsg != "" {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			MarginTop(1)
		return emptyStyle.Render(emptyMsg)
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")).
		MarginTop(1).
		Render(title)

	normalStyle := lipgloss.NewStyle().MarginTop(0)
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("51")).
		MarginTop(0)

	visible := m.listVisibleItems()
	viewportEnd := m.viewportStart + visible
	if viewportEnd > len(items) {
		viewportEnd = len(items)
	}

	lines := []string{titleStyle}
	for i := m.viewportStart; i < viewportEnd; i++ {
		prefix := "  "
		style := normalStyle
		if i == cursor {
			prefix = "> "
			style = selectedStyle
		}
		lines = append(lines, style.Render(prefix+items[i]))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m SearchModel) renderSearchResults(content string) string {
	items := make([]string, len(m.results))
	for i, manga := range m.results {
		items[i] = fmt.Sprintf("• %s", manga.Title)
	}
	title := fmt.Sprintf("Kết quả cho \"%s\":", m.currentQuery)
	return lipgloss.JoinVertical(lipgloss.Center, content,
		m.renderList(title, "", items, m.cursor))
}

func (m SearchModel) renderFavorites(content string) string {
	items := make([]string, len(m.favorites))
	for i, fav := range m.favorites {
		items[i] = fmt.Sprintf("• %s", fav.Title)
	}
	return lipgloss.JoinVertical(lipgloss.Center, content,
		m.renderList("Truyện Yêu Thích:", "Chưa có truyện yêu thích nào.", items, m.cursor))
}

func (m SearchModel) renderHistory(content string) string {
	items := make([]string, len(m.history))
	for i, h := range m.history {
		title := h.MangaTitle
		if title == "" {
			title = h.MangaID
		}
		chLabel := h.ChapterNumber
		if chLabel == "" {
			chLabel = h.ChapterID
			if len(chLabel) > 8 {
				chLabel = chLabel[:8]
			}
		}
		items[i] = fmt.Sprintf("• %s - Ch. %s (Trang %d)", title, chLabel, h.PageIndex+1)
	}
	return lipgloss.JoinVertical(lipgloss.Center, content,
		m.renderList("Lịch Sử Đọc:", "Chưa có lịch sử đọc nào.", items, m.cursor))
}
