package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
)

// pagedProvider returns one fixed page of manga per FetchLatest call.
type pagedProvider struct {
	name  string
	pages map[int][]models.Manga
}

func (p pagedProvider) Name() string { return p.name }

func (p pagedProvider) Search(context.Context, string) ([]models.Manga, error) {
	return nil, nil
}

func (p pagedProvider) FetchLatest(_ context.Context, page int) ([]models.Manga, error) {
	return p.pages[page], nil
}

func (p pagedProvider) Filter(_ context.Context, opts api.FilterOptions) ([]models.Manga, error) {
	return p.pages[opts.Page], nil
}

func (p pagedProvider) FetchChapters(context.Context, string) ([]models.Chapter, error) {
	return nil, nil
}

func (p pagedProvider) FetchPages(context.Context, string) ([]string, error) { return nil, nil }

// runResults drives a result request to its final snapshot.
func runResults(t *testing.T, m SearchModel, cmd tea.Cmd) SearchModel {
	t.Helper()
	for cmd != nil {
		var next tea.Cmd
		for _, msg := range drainBatch(t, cmd) {
			if res, ok := msg.(resultsMsg); ok {
				updated, c := m.Update(res)
				m = updated.(SearchModel)
				next = c
			}
		}
		cmd = next
	}
	return m
}

func TestFeedLoadsNextPageAtEndOfList(t *testing.T) {
	p := pagedProvider{name: "P", pages: map[int][]models.Manga{
		1: {{ID: "a"}, {ID: "b"}},
		2: {{ID: "b"}, {ID: "c"}}, // "b" moved onto page 2 meanwhile
	}}
	m := NewSearchModel([]api.MangaProvider{p})
	m.providerToggles = []bool{true}

	m = runResults(t, m, m.requestResults(resultsLatest, 1, false))
	if len(m.results) != 2 || !m.hasMore {
		t.Fatalf("page 1: results=%d hasMore=%v", len(m.results), m.hasMore)
	}

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = updated.(SearchModel)
	if !m.loadingMore {
		t.Fatal("reaching the last item did not request the next page")
	}
	m = runResults(t, m, cmd)

	var ids []string
	for _, r := range m.results {
		ids = append(ids, r.ID)
	}
	if strings.Join(ids, ",") != "a,b,c" {
		t.Errorf("results = %v, want a,b,c (appended, deduplicated)", ids)
	}
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (appending must not jump to the top)", m.cursor)
	}
	if m.page != 2 {
		t.Errorf("page = %d, want 2", m.page)
	}

	// Page 3 is empty: the list is exhausted.
	m.cursor = len(m.results) - 1
	m = runResults(t, m, m.maybeLoadMore())
	if m.hasMore {
		t.Error("an empty page must end paging")
	}
	if m.maybeLoadMore() != nil {
		t.Error("no more pages should be requested after the end")
	}
}

func TestSearchResultsAreNotPaged(t *testing.T) {
	m := testSearchModel()
	m.listMode = resultsSearch
	m.hasMore = true
	m.results = []models.Manga{{ID: "a"}}
	if m.maybeLoadMore() != nil {
		t.Error("search results must not request more pages")
	}
}

func TestNewRequestCancelsInFlightOne(t *testing.T) {
	m := testSearchModel()
	_ = m.requestResults(resultsLatest, 1, false)
	first := m.cancelRequest
	ctxCanceled := false
	m.cancelRequest = func() { ctxCanceled = true; first() }

	_ = m.requestResults(resultsSearch, 1, false)
	if !ctxCanceled {
		t.Error("starting a new request did not cancel the previous one")
	}
}
