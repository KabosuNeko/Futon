package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
)

// resultMode says what the result list in viewFeed holds.
type resultMode int

const (
	resultsLatest resultMode = iota
	resultsFilter
	resultsSearch
)

// maybeLoadMore requests the next page once the cursor reaches the last item
// of a paged list (latest or filter results; search results are not paged).
func (m *SearchModel) maybeLoadMore() tea.Cmd {
	if m.view != viewFeed || m.listMode == resultsSearch || !m.hasMore || m.loadingMore || m.isSearching {
		return nil
	}
	if m.cursor < len(m.resultRows())-1 {
		return nil
	}
	return m.requestResults(m.listMode, m.page+1, true)
}

// requestResults starts a latest/filter/search request, cancelling the one in
// flight. With appendResults the new page is added below the current list.
func (m *SearchModel) requestResults(mode resultMode, page int, appendResults bool) tea.Cmd {
	active, ok := m.activeOrError()
	if !ok {
		return nil
	}
	m.cancelInFlight()
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelRequest = cancel

	m.listMode = mode
	m.page = page
	m.err = nil
	m.providerCounts = nil
	m.providerErrors = nil
	m.searchingProviders = m.activeProviderNames()
	if appendResults {
		m.appendBase = m.results
		m.loadingMore = true
	} else {
		m.appendBase = nil
		m.loadingMore = false
		m.results = nil
		m.cursor = 0
		m.viewportStart = 0
		m.isSearching = true
	}

	var cmd tea.Cmd
	switch mode {
	case resultsLatest:
		cmd = latestAllCmd(ctx, active, page)
	case resultsFilter:
		cmd = filterAllCmd(ctx, active, api.FilterOptions{
			Status: m.filterStatus,
			Sort:   m.filterSort,
			Genre:  m.filterGenre,
			Page:   page,
		})
	default:
		cmd = searchAllCmd(ctx, active, m.currentQuery)
	}
	return m.newSearchCmd(cmd)
}

// cancelInFlight aborts the current result request, if any.
func (m *SearchModel) cancelInFlight() {
	if m.cancelRequest != nil {
		m.cancelRequest()
		m.cancelRequest = nil
	}
}

// mergeResults puts a snapshot below the pages already shown, skipping titles
// that moved between pages while the list was being read.
func (m SearchModel) mergeResults(snapshot []models.Manga) []models.Manga {
	if len(m.appendBase) == 0 {
		return snapshot
	}
	seen := make(map[string]bool, len(m.appendBase))
	for _, r := range m.appendBase {
		seen[r.Provider+"\x00"+r.ID] = true
	}
	merged := append([]models.Manga(nil), m.appendBase...)
	for _, r := range snapshot {
		if !seen[r.Provider+"\x00"+r.ID] {
			merged = append(merged, r)
		}
	}
	return merged
}

// resetSearchResults clears result-bound state when the query changes.
func (m *SearchModel) resetSearchResults() {
	m.results = nil
	m.cursor = 0
	m.viewportStart = 0
	m.isSearching = false
	m.err = nil
	m.providerCounts = nil
	m.providerErrors = nil
	m.searchingProviders = nil
}

type searchTriggerMsg struct {
	query string
}

// searchRequestCmd stamps every snapshot from one result request so the model
// can drop messages that belong to a query the user has already replaced.
func searchRequestCmd(requestID int, cmd tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		msg := cmd()
		if result, ok := msg.(resultsMsg); ok {
			result.RequestID = requestID
			return result
		}
		return msg
	}
}

// newSearchCmd starts a new request generation.
func (m *SearchModel) newSearchCmd(cmd tea.Cmd) tea.Cmd {
	m.pendingSeq++
	return searchRequestCmd(m.pendingSeq, cmd)
}

func debounceSearch(query string, delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return searchTriggerMsg{query: query}
	})
}
