package tui

import "charm.land/lipgloss/v2"

// listTopRow is the 0-based terminal row of the first visible list item.
func (m SearchModel) listTopRow() int {
	_, layout := m.buildContent()
	return layout.listTop
}

func (m SearchModel) listVisibleItems() int {
	// 8 rows go to the input box, one tab row, one hint row and spacing; a
	// notice line or wrapped tabs and hints take more, and the list must
	// give those rows back or the screen overflows.
	h := m.height - m.extraChromeRows()
	boxH := h - 8
	if boxH > 22 {
		boxH = 22
	}
	if boxH < 6 {
		boxH = 6
	}
	visible := boxH - 4
	if visible < 1 {
		visible = 1
	}
	return visible
}

func (m SearchModel) currentListLen() int {
	switch m.view {
	case viewFavorites:
		return len(m.filteredFavIndices())
	case viewHistory:
		return len(m.filteredHistoryIndices())
	case viewSources:
		return len(m.filteredProviderIndices())
	default:
		return len(m.resultRows())
	}
}

func (m *SearchModel) adjustViewport() {
	visible := m.listVisibleItems()
	total := m.currentListLen()

	if m.cursor < m.viewportStart {
		m.viewportStart = m.cursor
	}
	if m.cursor >= m.viewportStart+visible {
		m.viewportStart = m.cursor - visible + 1
	}
	if m.viewportStart < 0 {
		m.viewportStart = 0
	}
	if m.viewportStart >= total && total > 0 {
		m.viewportStart = total - 1
	}
}

// moveCursor shifts the list cursor by delta, clamped to the active list.
// It reports whether the cursor moved.
func (m *SearchModel) moveCursor(delta int) bool {
	next := m.cursor + delta
	if next < 0 || next >= m.currentListLen() {
		return false
	}
	m.cursor = next
	m.adjustViewport()
	return true
}

func (m SearchModel) previewPaneWidth() int {
	if m.width < 80 {
		return 0
	}
	w := 42
	if m.width >= 120 {
		w = 50
	}
	if m.width >= 150 {
		w = 56
	}
	if w > m.width-50 {
		w = m.width - 50
	}
	return w
}

func (m SearchModel) listPaneWidth() int {
	if m.width < 80 {
		return m.width - 4
	}
	w := 62
	if m.width >= 120 {
		w = 72
	}
	if m.width >= 150 {
		w = 80
	}
	maxW := m.width - m.previewPaneWidth() - 8
	if w > maxW {
		w = maxW
	}
	if w < 36 {
		w = 36
	}
	return w
}

// searchLayout is the screen geometry computed alongside the rendered view.
type searchLayout struct {
	cover coverPaintKey
	// listTop is the 0-based terminal row of the first visible list item,
	// used to map mouse clicks to list entries.
	listTop int
}

// previewImageRowOffset is the cover image's row inside the preview card:
// border, header, divider, title, source, author, meta, genres, two spacers.
const previewImageRowOffset = 9

// listIsCard reports whether lists render as a bordered card (wide screens)
// rather than a bare title + rows.
func (m SearchModel) listIsCard() bool {
	// The preview pane needs about 20 rows; below that it pushed the footer
	// off screen and could paint the cover past the bottom row.
	return m.width >= 80 && m.height-m.extraChromeRows() >= 20 && m.view != viewSources
}

// listItemOffset is the number of rows from the top of the list content to
// its first item: card border + title + divider, or top margin + title.
func (m SearchModel) listItemOffset() int {
	if m.listIsCard() {
		return 3
	}
	return 2
}

// extraChromeRows is how many rows the header and footer use beyond their
// one-line minimum.
func (m SearchModel) extraChromeRows() int {
	extra := lipgloss.Height(m.renderTabBar()) - 1 + lipgloss.Height(m.renderFooter()) - 1
	if notice := m.headerNotice(); notice != "" {
		extra += lipgloss.Height(notice)
	}
	return extra
}
