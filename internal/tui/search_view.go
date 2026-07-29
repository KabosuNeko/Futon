package tui

import (
	"fmt"
	"strings"

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
	} else if m.showingSources {
		content = m.renderSources(content)
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

	var footer string
	switch {
	case m.showingSources:
		footer = "space: chọn/bỏ chọn  |  up/down: di chuyển  |  esc: quay lại  |  ctrl+c: thoát"
	case m.showingFavorites:
		footer = "enter: mở truyện  |  ctrl+d: xóa yêu thích  |  esc: quay lại  |  ctrl+c: thoát"
	case m.showingHistory:
		footer = "enter: mở truyện  |  ctrl+d: xóa lịch sử  |  esc: quay lại  |  ctrl+c: thoát"
	default:
		footer = "ctrl+c: thoát  |  /fav: truyện yêu thích  |  /his: lịch sử đọc  |  /src: chọn nguồn  |  /lang: chỉnh ngôn ngữ"
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

func (m SearchModel) renderSources(content string) string {
	indices := m.filteredProviderIndices()
	items := make([]string, len(indices))
	for i, idx := range indices {
		check := "[ ]"
		if m.providerToggles[idx] {
			check = "[X]"
		}
		items[i] = fmt.Sprintf("%s %s", check, m.providers[idx].Name())
	}
	var title string
	if m.filterQuery != "" {
		title = fmt.Sprintf("Nguồn (lọc: %s):", m.filterQuery)
	} else {
		title = "Chọn nguồn:"
	}
	return lipgloss.JoinVertical(lipgloss.Center, content,
		m.renderList(title, "Không tìm thấy nguồn nào.", items, m.sourceCursor))
}

func (m SearchModel) renderSourceName() string {
	active := m.activeProviders()
	if len(active) == 0 {
		return "Chưa chọn nguồn"
	}
	names := make([]string, len(active))
	for i, p := range active {
		names[i] = p.Name()
	}
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names, ", ")
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
	indices := m.filteredFavIndices()
	items := make([]string, len(indices))
	for i, idx := range indices {
		items[i] = fmt.Sprintf("• %s", m.favorites[idx].Title)
	}
	var emptyMsg string
	if m.filterQuery != "" {
		emptyMsg = "Không tìm thấy truyện yêu thích."
	} else {
		emptyMsg = "Chưa có truyện yêu thích nào."
	}
	return lipgloss.JoinVertical(lipgloss.Center, content,
		m.renderList("Truyện Yêu Thích:", emptyMsg, items, m.cursor))
}

func (m SearchModel) renderHistory(content string) string {
	indices := m.filteredHistoryIndices()
	items := make([]string, len(indices))
	for i, idx := range indices {
		h := m.history[idx]
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
	var emptyMsg string
	if m.filterQuery != "" {
		emptyMsg = "Không tìm thấy lịch sử đọc."
	} else {
		emptyMsg = "Chưa có lịch sử đọc nào."
	}
	return lipgloss.JoinVertical(lipgloss.Center, content,
		m.renderList("Lịch Sử Đọc:", emptyMsg, items, m.cursor))
}
