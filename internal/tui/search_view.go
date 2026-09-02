package tui

import (
	"fmt"
	"strings"

	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

func (m SearchModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(boxColor(m.input.Value())).
		Padding(0, 1)

	header := boxStyle.Render(m.input.View())

	tabFeed := "[󰑓 Mới cập nhật]"
	tabFav := "[ Yêu thích]"
	tabHis := "[ Lịch sử]"
	tabSrc := "[󰖟 Nguồn]"
	tabFlt := "[󰈲 Bộ lọc]"

	activeTabStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	inactiveTabStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	var tabs []string
	if m.showingFilters {
		tabs = []string{inactiveTabStyle.Render(tabFeed), inactiveTabStyle.Render(tabFav), inactiveTabStyle.Render(tabHis), inactiveTabStyle.Render(tabSrc), activeTabStyle.Render(tabFlt)}
	} else if m.showingSources {
		tabs = []string{inactiveTabStyle.Render(tabFeed), inactiveTabStyle.Render(tabFav), inactiveTabStyle.Render(tabHis), activeTabStyle.Render(tabSrc), inactiveTabStyle.Render(tabFlt)}
	} else if m.showingFavorites {
		tabs = []string{inactiveTabStyle.Render(tabFeed), activeTabStyle.Render(tabFav), inactiveTabStyle.Render(tabHis), inactiveTabStyle.Render(tabSrc), inactiveTabStyle.Render(tabFlt)}
	} else if m.showingHistory {
		tabs = []string{inactiveTabStyle.Render(tabFeed), inactiveTabStyle.Render(tabFav), activeTabStyle.Render(tabHis), inactiveTabStyle.Render(tabSrc), inactiveTabStyle.Render(tabFlt)}
	} else {
		tabs = []string{activeTabStyle.Render(tabFeed), inactiveTabStyle.Render(tabFav), inactiveTabStyle.Render(tabHis), inactiveTabStyle.Render(tabSrc), inactiveTabStyle.Render(tabFlt)}
	}
	tabBar := strings.Join(tabs, " ")

	var headerParts []string
	headerParts = append(headerParts, header, tabBar)

	if m.systemMsg != "" {
		sysStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			MarginTop(1)
		headerParts = append(headerParts, sysStyle.Render(m.systemMsg))
	} else if len(m.providerErrors) > 0 && !m.isSearching && !m.showingSources && !m.showingFavorites && !m.showingHistory && m.err == nil && len(m.results) > 0 {
		warnStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			MarginTop(1)
		var failed []string
		for name := range m.providerErrors {
			failed = append(failed, name)
		}
		total := len(m.providerCounts)
		if total == 0 {
			total = len(failed) + 1
		}
		ok := total - len(failed)
		wmsg := fmt.Sprintf("󰀦 %s lỗi (%d/%d nguồn)", strings.Join(failed, ", "), ok, total)
		headerParts = append(headerParts, warnStyle.Render(wmsg))
	}

	headerNode := lipgloss.JoinVertical(lipgloss.Center, headerParts...)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("3")).
		MarginTop(1)
	subtleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		MarginTop(1)

	var listContent string

	if m.showingFilters {
		listContent = m.renderFilterModal()
	} else if m.err != nil {
		errStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			MarginTop(1)
		listContent = errStyle.Render(fmt.Sprintf("󰅚 Lỗi: %v", m.err))
	} else if m.isSearching {
		msg := " Đang tìm kiếm..."
		if m.showingFeed {
			msg = "󰑓 Đang nạp danh sách truyện mới..."
		} else if len(m.searchingProviders) > 0 {
			msg = fmt.Sprintf(" Đang tìm trên %s...", strings.Join(m.searchingProviders, ", "))
		}
		listContent = statusStyle.Render(msg)
	} else if m.loadingFavorites {
		listContent = statusStyle.Render(" Đang tải danh sách yêu thích...")
	} else if m.loadingHistory {
		listContent = statusStyle.Render(" Đang tải lịch sử đọc...")
	} else if m.showingSources {
		listContent = m.renderSources("")
	} else if m.showingFavorites {
		listContent = m.renderFavorites("")
	} else if m.showingHistory {
		listContent = m.renderHistory("")
	} else if len(m.currentQuery) >= 3 && len(m.results) == 0 {
		listContent = subtleStyle.Render("Không tìm thấy kết quả.")
	} else if len(m.results) > 0 {
		listContent = m.renderSearchResults("")
	}

	hasItems := (len(m.results) > 0 && !m.showingFavorites && !m.showingHistory && !m.showingSources && !m.showingFilters) ||
		(m.showingFavorites && len(m.filteredFavIndices()) > 0) ||
		(m.showingHistory && len(m.filteredHistoryIndices()) > 0)

	var listPaneW, previewPaneW int
	isSplit := m.width >= 80 && m.height >= 16 && hasItems && !m.showingSources

	var body string
	if isSplit {
		preview := m.renderPreviewPane()
		previewPaneW = m.previewPaneWidth()
		listPaneW = m.listPaneWidth()
		body = lipgloss.JoinHorizontal(lipgloss.Top, listContent, "   ", preview)
	} else {
		body = listContent
	}

	footer := m.renderFooter()

	var content string
	if body != "" {
		content = lipgloss.JoinVertical(lipgloss.Center, headerNode, "", body, "", footer)
	} else {
		content = lipgloss.JoinVertical(lipgloss.Center, headerNode, "", footer)
	}

	contentH := lipgloss.Height(content)
	contentW := lipgloss.Width(content)
	topPad := max(0, (m.height-contentH)/2)
	topRow := topPad + 1
	leftCol := max(1, (m.width-contentW)/2+1)

	placed := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, content)
	if topPad > 0 {
		placed = strings.Repeat("\n", topPad) + placed
	}

	if m.flashMsg != "" {
		flashStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")).
			Bold(true)
		placed = placed + "\n" + flashStyle.Render(m.flashMsg)
	}

	if isSplit {
		headerH := lipgloss.Height(headerNode)
		bodyRow := topRow + headerH + 1

		actualListW := listPaneW + 3
		previewCol := leftCol + actualListW

		imgRow := bodyRow + 9
		boxH := m.listVisibleItems() + 4
		previewInnerW := previewPaneW - 4

		var clearSeq strings.Builder
		clearSeq.WriteString("\x1b_Ga=d,d=A,q=2\x1b\\\x1b_Ga=d,d=a,q=2\x1b\\")
		for r := bodyRow + 8; r <= bodyRow+boxH-2; r++ {
			clearSeq.WriteString(fmt.Sprintf("\x1b[%d;%dH\x1b[%dX", r, previewCol+3, max(1, previewInnerW-2)))
		}

		if m.currentCover != nil && m.currentCover.EscapeSequence != "" {
			ts, _ := imgrender.GetTerminalSize()
			cellW := 8
			if ts.Cols > 0 && ts.PxW > 0 {
				cellW = max(1, ts.PxW/ts.Cols)
			}
			imgCellW := max(1, m.currentCover.WidthPx/cellW)
			indent := max(1, (previewInnerW-imgCellW)/2)
			imgCol := previewCol + 2 + indent

			imageCmd := fmt.Sprintf("\x1b[s%s\x1b[%d;%dH%s\x1b[u", clearSeq.String(), imgRow, imgCol, m.currentCover.EscapeSequence)
			placed = placed + imageCmd
		} else {
			placed = placed + fmt.Sprintf("\x1b[s%s\x1b[u", clearSeq.String())
		}
	} else {
		placed = placed + "\x1b_Ga=d,d=A,q=2\x1b\\\x1b_Ga=d,d=a,q=2\x1b\\"
	}

	return placed
}

func (m SearchModel) renderPreviewPane() string {
	focused, ok := m.focusedManga()
	if !ok {
		return ""
	}

	w := m.previewPaneWidth()

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6"))

	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("7"))

	metaLabelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	metaValueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7"))

	tagStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6"))

	var contentLines []string
	contentLines = append(contentLines, headerStyle.Render("󰋼 Chi tiết manga"))
	contentLines = append(contentLines, dividerStyle.Render(strings.Repeat("─", max(1, w-4))))

	cleanedTitle := cleanTitle(focused.Title, focused.Provider)
	if cleanedTitle == "" {
		cleanedTitle = focused.ID
	}
	contentLines = append(contentLines, titleStyle.Render(runewidth.Truncate(cleanedTitle, w-4, "...")))

	if focused.Provider != "" {
		sourceLabel := metaLabelStyle.Render("Nguồn: ")
		contentLines = append(contentLines, sourceLabel+providerBadge(focused.Provider))
	} else {
		contentLines = append(contentLines, "")
	}

	if focused.Author != "" {
		authorText := runewidth.Truncate(focused.Author, w-14, "...")
		contentLines = append(contentLines, metaLabelStyle.Render("Tác giả: ")+metaValueStyle.Render(authorText))
	} else {
		contentLines = append(contentLines, "")
	}

	var metaParts []string
	if focused.Year != "" {
		metaParts = append(metaParts, metaLabelStyle.Render("Năm: ")+metaValueStyle.Render(focused.Year))
	}
	if focused.Status != "" {
		st := focused.Status
		switch strings.ToLower(st) {
		case "ongoing", "dang_phat_hanh", "đang tiến hành", "đang phát hành":
			st = "Đang ra"
		case "completed", "hoan_thanh", "hoàn thành":
			st = "Hoàn thành"
		case "hiatus":
			st = "Tạm ngưng"
		}
		metaParts = append(metaParts, metaLabelStyle.Render("Trạng thái: ")+metaValueStyle.Render(st))
	}
	if len(metaParts) > 0 {
		contentLines = append(contentLines, strings.Join(metaParts, "  ·  "))
	} else {
		contentLines = append(contentLines, "")
	}

	if len(focused.Genres) > 0 {
		var genreBadges []string
		currLen := 0
		maxLen := w - 6
		for _, g := range focused.Genres {
			badge := "[" + g + "]"
			badgeW := runewidth.StringWidth(badge) + 1
			if currLen+badgeW > maxLen && len(genreBadges) > 0 {
				break
			}
			genreBadges = append(genreBadges, tagStyle.Render(badge))
			currLen += badgeW
		}
		if len(genreBadges) > 0 {
			contentLines = append(contentLines, strings.Join(genreBadges, " "))
		} else {
			contentLines = append(contentLines, "")
		}
	} else {
		contentLines = append(contentLines, "")
	}

	contentLines = append(contentLines, "")
	contentLines = append(contentLines, "")

	visible := m.listVisibleItems()
	boxH := visible + 4
	if boxH < 8 {
		boxH = 8
	}

	maxImgRows := boxH - 11
	if maxImgRows < 2 {
		maxImgRows = 2
	}

	imgRows := 0
	if m.coverLoading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			Italic(true)
		contentLines = append(contentLines, loadingStyle.Render("⏳ Đang tải ảnh bìa..."))
	} else if m.currentCover != nil {
		ts, _ := imgrender.GetTerminalSize()
		cellH := 16
		if ts.Rows > 0 && ts.PxH > 0 {
			cellH = max(1, ts.PxH/ts.Rows)
		}
		imgRows = max(1, m.currentCover.HeightPx/cellH)
		if imgRows > maxImgRows {
			imgRows = maxImgRows
		}
		rowPlaceholder := strings.Repeat(" ", max(1, w-4))
		for i := 0; i < imgRows; i++ {
			contentLines = append(contentLines, rowPlaceholder)
		}
	} else if focused.CoverURL == "" {
		contentLines = append(contentLines, metaLabelStyle.Render("󰋩 (Không có ảnh bìa)"))
	} else {
		contentLines = append(contentLines, metaLabelStyle.Render("󰋩 (Chưa tải ảnh bìa)"))
	}

	innerW := max(1, w-4)
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

	previewContent := lipgloss.JoinVertical(lipgloss.Left, paddedLines...)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(w).
		Height(boxH)

	return boxStyle.Render(previewContent)
}

func providerBadge(provider string) string {
	if provider == "" {
		return ""
	}
	var color string
	p := strings.ToLower(provider)
	switch {
	case strings.Contains(p, "mangadex"):
		color = "3"
	case strings.Contains(p, "otruyen"):
		color = "2"
	case strings.Contains(p, "truyenqq"):
		color = "6"
	case strings.Contains(p, "foxtruyen"):
		color = "5"
	case strings.Contains(p, "baotang"):
		color = "4"
	default:
		color = "7"
	}
	tagStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(color))
	return tagStyle.Render("[" + provider + "]")
}

func (m SearchModel) renderList(title, emptyMsg string, items []string, cursor int) string {
	w := m.listPaneWidth()
	isSplit := m.width >= 80 && m.height >= 16 && !m.showingSources

	visible := m.listVisibleItems()
	viewportEnd := m.viewportStart + visible
	if viewportEnd > len(items) {
		viewportEnd = len(items)
	}

	if !isSplit {
		if len(items) == 0 && emptyMsg != "" {
			emptyStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("8")).
				MarginTop(1)
			return emptyStyle.Render(emptyMsg)
		}
		titleStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")).
			MarginTop(1).
			Bold(true).
			Render(title)
		normalStyle := lipgloss.NewStyle().MarginTop(0)
		selectedStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true).
			MarginTop(0)
		lines := []string{titleStyle}
		for i := m.viewportStart; i < viewportEnd; i++ {
			prefix := "  "
			style := normalStyle
			if i == cursor {
				prefix = "> "
				style = selectedStyle
			}
			itemText := runewidth.Truncate(items[i], w-4, "...")
			lines = append(lines, style.Render(prefix+itemText))
		}
		return lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	boxH := visible + 4
	if boxH < 8 {
		boxH = 8
	}
	maxBoxH := m.height - searchUIOffset
	if boxH > maxBoxH {
		boxH = maxBoxH
	}

	if len(items) == 0 && emptyMsg != "" {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			MarginTop(1)
		cardStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(0, 1).
			Width(w).
			Height(boxH)
		return cardStyle.Render(emptyStyle.Render(emptyMsg))
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("2")).
		Bold(true)

	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	normalStyle := lipgloss.NewStyle()

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")).
		Bold(true)

	maxItemW := w - 6
	if maxItemW < 20 {
		maxItemW = 20
	}

	var lines []string
	lines = append(lines, titleStyle.Render(title))
	lines = append(lines, dividerStyle.Render(strings.Repeat("─", max(1, w-4))))

	for i := m.viewportStart; i < viewportEnd; i++ {
		prefix := "  "
		style := normalStyle
		if i == cursor {
			prefix = "> "
			style = selectedStyle
		}
		itemText := runewidth.Truncate(items[i], maxItemW, "...")
		padLen := maxItemW - runewidth.StringWidth(itemText)
		if padLen > 0 && i == cursor {
			itemText = itemText + strings.Repeat(" ", padLen)
		}
		lines = append(lines, style.Render(prefix+itemText))
	}

	renderedCount := viewportEnd - m.viewportStart
	for i := renderedCount; i < visible; i++ {
		lines = append(lines, "")
	}

	listCard := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1).
		Width(w).
		Height(boxH)

	return listCard.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m SearchModel) renderSources(content string) string {
	indices := m.filteredProviderIndices()
	items := make([]string, len(indices))
	for i, idx := range indices {
		check := "[ ]"
		if m.providerToggles[idx] {
			check = "[󰄲]"
		}
		items[i] = fmt.Sprintf("%s %s", check, m.providers[idx].Name())
	}
	var title string
	if m.filterQuery != "" {
		title = fmt.Sprintf("󰖟 Nguồn (lọc: %s):", m.filterQuery)
	} else {
		title = "󰖟 Chọn nguồn:"
	}
	res := m.renderList(title, "Không tìm thấy nguồn nào.", items, m.sourceCursor)
	if content != "" {
		return lipgloss.JoinVertical(lipgloss.Center, content, res)
	}
	return res
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

func cleanTitle(title, provider string) string {
	if provider != "" {
		suffix := fmt.Sprintf(" (%s)", strings.ToLower(provider))
		if strings.HasSuffix(strings.ToLower(title), suffix) {
			return title[:len(title)-len(suffix)]
		}
	}
	return title
}

func (m SearchModel) renderFilterModal() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("4")).
		Bold(true)

	activeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("2")).
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))

	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("6")).
		Bold(true)

	var b strings.Builder
	b.WriteString(titleStyle.Render("󰈲 BỘ LỌC TÌM KIẾM"))
	b.WriteString("\n\n")

	if m.filterCursor == 0 {
		b.WriteString(cursorStyle.Render("► ") + activeStyle.Render(fmt.Sprintf("Trạng thái:  < %s >", filterStatusOptions[m.filterStatus])))
	} else {
		b.WriteString(fmt.Sprintf("   Trạng thái:  < %s >", filterStatusOptions[m.filterStatus]))
	}
	b.WriteString("\n")

	if m.filterCursor == 1 {
		b.WriteString(cursorStyle.Render("► ") + activeStyle.Render(fmt.Sprintf("Sắp xếp:     < %s >", filterSortOptions[m.filterSort])))
	} else {
		b.WriteString(fmt.Sprintf("   Sắp xếp:     < %s >", filterSortOptions[m.filterSort]))
	}
	b.WriteString("\n")

	if m.filterCursor == 2 {
		b.WriteString(cursorStyle.Render("► ") + activeStyle.Render(fmt.Sprintf("Thể loại:    < %s >", filterGenreOptions[m.filterGenre])))
	} else {
		b.WriteString(fmt.Sprintf("   Thể loại:    < %s >", filterGenreOptions[m.filterGenre]))
	}
	b.WriteString("\n\n")

	btnApply := "[ Áp dụng (Enter) ]"
	btnReset := "[ Đặt lại (Reset) ]"
	if m.filterCursor == 3 {
		btnApply = activeStyle.Render("[ Áp dụng (Enter) ]")
	}
	if m.filterCursor == 4 {
		btnReset = activeStyle.Render("[ Đặt lại (Reset) ]")
	}
	b.WriteString(fmt.Sprintf("  %s    %s\n\n", btnApply, btnReset))

	b.WriteString(dimStyle.Render("[←/→] Đổi giá trị   [↑/↓] Chọn dòng   [esc] Đóng"))

	return boxStyle.Render(b.String())
}

func (m SearchModel) renderSearchResults(content string) string {
	items := make([]string, len(m.results))
	for i, manga := range m.results {
		t := cleanTitle(manga.Title, manga.Provider)
		if manga.Provider != "" {
			items[i] = fmt.Sprintf("%s (%s)", t, strings.ToLower(manga.Provider))
		} else {
			items[i] = t
		}
	}
	title := fmt.Sprintf("Kết quả cho \"%s\" (%d/%d):", m.currentQuery, m.cursor+1, len(m.results))
	if m.showingFeed && len(m.currentQuery) == 0 {
		title = fmt.Sprintf("󰑓 Truyện Mới Cập Nhật (%d/%d):", m.cursor+1, len(m.results))
	}
	res := m.renderList(title, "", items, m.cursor)
	if content != "" {
		return lipgloss.JoinVertical(lipgloss.Center, content, res)
	}
	return res
}

func (m SearchModel) renderFavorites(content string) string {
	indices := m.filteredFavIndices()
	items := make([]string, len(indices))
	for i, idx := range indices {
		fav := m.favorites[idx]
		t := cleanTitle(fav.Title, fav.Provider)
		if fav.Provider != "" {
			items[i] = fmt.Sprintf("%s (%s)", t, strings.ToLower(fav.Provider))
		} else {
			items[i] = t
		}
	}
	var emptyMsg string
	if m.filterQuery != "" {
		emptyMsg = "Không tìm thấy truyện yêu thích."
	} else {
		emptyMsg = "Chưa có truyện yêu thích nào."
	}
	title := fmt.Sprintf(" Truyện Yêu Thích (%d):", len(indices))
	if len(indices) > 0 {
		title = fmt.Sprintf(" Truyện Yêu Thích (%d/%d):", m.cursor+1, len(indices))
	}
	res := m.renderList(title, emptyMsg, items, m.cursor)
	if content != "" {
		return lipgloss.JoinVertical(lipgloss.Center, content, res)
	}
	return res
}

func (m SearchModel) renderHistory(content string) string {
	indices := m.filteredHistoryIndices()
	items := make([]string, len(indices))
	for i, idx := range indices {
		h := m.history[idx]
		title := cleanTitle(h.MangaTitle, h.Provider)
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
		items[i] = fmt.Sprintf("%s - Ch. %s (Trang %d)", title, chLabel, h.PageIndex+1)
	}
	var emptyMsg string
	if m.filterQuery != "" {
		emptyMsg = "Không tìm thấy lịch sử đọc."
	} else {
		emptyMsg = "Chưa có lịch sử đọc nào."
	}
	title := fmt.Sprintf(" Lịch Sử Đọc (%d):", len(indices))
	if len(indices) > 0 {
		title = fmt.Sprintf(" Lịch Sử Đọc (%d/%d):", m.cursor+1, len(indices))
	}
	res := m.renderList(title, emptyMsg, items, m.cursor)
	if content != "" {
		return lipgloss.JoinVertical(lipgloss.Center, content, res)
	}
	return res
}

func (m SearchModel) renderFooter() string {
	pill := func(key, desc string) string {
		kStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("6"))
		dStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			PaddingRight(1)
		return "[" + kStyle.Render(key) + "] " + dStyle.Render(desc)
	}

	switch {
	case m.showingFilters:
		return lipgloss.JoinHorizontal(lipgloss.Center,
			pill("←/→", "Đổi giá trị"),
			pill("↑/↓", "Chọn dòng"),
			pill("enter", "Áp dụng"),
			pill("esc", "Đóng"),
			pill("^c", "Thoát"),
		)
	case m.showingSources:
		return lipgloss.JoinHorizontal(lipgloss.Center,
			pill("space", "Bật/Tắt"),
			pill("↑/↓", "Di chuyển"),
			pill("esc", "Quay lại"),
			pill("^c", "Thoát"),
		)
	case m.showingFavorites:
		return lipgloss.JoinHorizontal(lipgloss.Center,
			pill("enter", "Mở đọc"),
			pill("↑/↓", "Chọn"),
			pill("tab", "Chuyển tab"),
			pill("^d", "Xóa"),
			pill("esc", "Quay lại"),
			pill("^c", "Thoát"),
		)
	case m.showingHistory:
		return lipgloss.JoinHorizontal(lipgloss.Center,
			pill("enter", "Mở đọc"),
			pill("↑/↓", "Chọn"),
			pill("tab", "Chuyển tab"),
			pill("^d", "Xóa"),
			pill("esc", "Quay lại"),
			pill("^c", "Thoát"),
		)
	default:
		return lipgloss.JoinHorizontal(lipgloss.Center,
			pill("enter", "Mở đọc"),
			pill("↑/↓", "Chọn"),
			pill("tab", "Chuyển tab"),
			pill("/f", "Bộ lọc"),
			pill("/src", "Nguồn"),
			pill("^c", "Thoát"),
		)
	}
}
