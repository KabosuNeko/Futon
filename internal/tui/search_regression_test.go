package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
)

func TestTypingUpdatesInput(t *testing.T) {
	m := testSearchModel()

	for _, r := range "naruto" {
		newM, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = newM.(SearchModel)
	}

	if m.input.Value() != "naruto" {
		t.Errorf("expected input 'naruto', got %q", m.input.Value())
	}
}

func TestSourceCommandToggles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	providers := []api.MangaProvider{api.NewOTruyenProvider(), api.NewMangaDexProvider()}
	m := NewSearchModel(providers)

	for i := range m.providers {
		if !m.providerToggles[i] {
			t.Fatalf("expected provider %d to be toggled on by default", i)
		}
	}

	for _, r := range "/src" {
		newM, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = newM.(SearchModel)
	}
	newM, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = newM.(SearchModel)
	if !m.showingSources {
		t.Fatalf("expected showingSources after /src")
	}

	newM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m = newM.(SearchModel)
	if m.providerToggles[0] {
		t.Errorf("expected provider 0 to be toggled off after space")
	}

	newM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m = newM.(SearchModel)
	if !m.providerToggles[0] {
		t.Errorf("expected provider 0 to be toggled on after second space")
	}

	newM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	m = newM.(SearchModel)
	if m.sourceCursor != 1 {
		t.Errorf("expected sourceCursor 1, got %d", m.sourceCursor)
	}

	newM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	m = newM.(SearchModel)
	if m.providerToggles[1] {
		t.Errorf("expected provider 1 to be toggled off")
	}

	newM, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = newM.(SearchModel)
	if m.showingSources {
		t.Errorf("expected showingSources false after ESC")
	}

	m.providerToggles = []bool{true, false}
	m.showingSources = false
	active := m.activeProviders()
	if len(active) != 1 {
		t.Fatalf("expected 1 active provider, got %d", len(active))
	}
	if active[0].Name() != "OTruyen" {
		t.Errorf("expected OTruyen, got %s", active[0].Name())
	}
}

func TestSearchViewportScrollsWithCursor(t *testing.T) {
	m := testSearchModel()
	m.width = 80
	m.height = 12
	m.currentQuery = "shonen"
	for i := 0; i < 20; i++ {
		m.results = append(m.results, models.Manga{ID: fmt.Sprintf("m%d", i), Title: fmt.Sprintf("UniqueMangaTitle%d", i)})
	}

	visible := m.listVisibleItems()
	if visible < 1 {
		t.Fatalf("expected visible items > 0, got %d", visible)
	}

	target := visible + 3
	for i := 0; i < target; i++ {
		newM, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		m = newM.(SearchModel)
	}

	if m.cursor != target {
		t.Errorf("expected cursor %d, got %d", target, m.cursor)
	}
	if m.viewportStart <= 0 {
		t.Errorf("expected viewportStart > 0 after scrolling down, got %d", m.viewportStart)
	}

	view := m.View().Content
	if strings.Contains(view, m.results[0].Title) {
		t.Errorf("first result should be scrolled out of view")
	}

	for i := 0; i < target; i++ {
		newM, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
		m = newM.(SearchModel)
	}

	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}
	if m.viewportStart != 0 {
		t.Errorf("expected viewportStart 0 after scrolling back, got %d", m.viewportStart)
	}
}

func TestSearchWindowSizeAdjustsViewport(t *testing.T) {
	m := testSearchModel()
	m.width = 80
	m.height = 24
	m.results = make([]models.Manga, 100)
	m.cursor = 50
	m.viewportStart = 45

	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m = newM.(SearchModel)

	if m.height != 10 {
		t.Errorf("expected height 10, got %d", m.height)
	}
	visible := m.listVisibleItems()
	if m.cursor >= m.viewportStart+visible {
		t.Errorf("cursor %d should be inside viewport [%d, %d) after resize", m.cursor, m.viewportStart, m.viewportStart+visible)
	}
}

func TestSearchViewFitsTerminalHeight(t *testing.T) {
	m := testSearchModel()
	m.width = 80
	m.height = 12
	m.currentQuery = "shonen"
	for i := 0; i < 50; i++ {
		m.results = append(m.results, models.Manga{ID: fmt.Sprintf("m%d", i), Title: fmt.Sprintf("UniqueMangaTitle%d", i)})
	}

	view := m.View().Content
	plain := stripANSI(view)
	lines := strings.Count(plain, "\n")
	if lines > m.height {
		t.Errorf("rendered view has %d lines, exceeding terminal height %d", lines, m.height)
	}
}

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]|\x1b_G[^\x1b]*\x1b\\\\")

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func TestSearchRequestCmdStampsRequestID(t *testing.T) {
	cmd := searchRequestCmd(7, func() tea.Msg { return api.MangaSearchResultMsg{} })
	msg, ok := cmd().(api.MangaSearchResultMsg)
	if !ok {
		t.Fatalf("unexpected message type")
	}
	if msg.RequestID != 7 {
		t.Errorf("RequestID = %d, want 7", msg.RequestID)
	}
}

func TestStaleSearchSnapshotIgnored(t *testing.T) {
	m := testSearchModel()
	m.currentQuery = "new"
	m.isSearching = true
	m.pendingSeq = 2
	m.results = []models.Manga{{ID: "keep", Title: "Keep"}}

	updated, _ := m.Update(api.MangaSearchResultMsg{
		Manga:     []models.Manga{{ID: "old", Title: "Old"}},
		RequestID: 1,
	})
	sm := updated.(SearchModel)
	if len(sm.results) != 1 || sm.results[0].ID != "keep" {
		t.Fatalf("stale snapshot replaced results: %+v", sm.results)
	}
	if !sm.isSearching {
		t.Error("stale final snapshot must not clear isSearching")
	}

	updated, _ = sm.Update(api.MangaSearchResultMsg{RequestID: 1})
	if !updated.(SearchModel).isSearching {
		t.Error("stale final snapshot cleared isSearching")
	}

	updated, _ = sm.Update(api.MangaSearchResultMsg{
		Manga:     []models.Manga{{ID: "new", Title: "New"}},
		RequestID: 2,
	})
	fresh := updated.(SearchModel)
	if len(fresh.results) != 1 || fresh.results[0].ID != "new" {
		t.Fatalf("current snapshot not applied: %+v", fresh.results)
	}
	if fresh.isSearching {
		t.Error("current final snapshot must clear isSearching")
	}
}

func TestInitWithNoEnabledSourceShowsError(t *testing.T) {
	m := testSearchModel()
	m.providerToggles = []bool{false, false}

	var init searchInitMsg
	for _, msg := range drainBatch(t, m.Init()) {
		if got, ok := msg.(searchInitMsg); ok {
			init = got
		}
	}
	if init.err != errNoSource {
		t.Fatalf("searchInitMsg.err = %v, want errNoSource", init.err)
	}

	updated, _ := m.Update(init)
	sm := updated.(SearchModel)
	if sm.err != errNoSource {
		t.Errorf("err = %v, want errNoSource", sm.err)
	}
	if sm.isSearching {
		t.Error("no-source init must not show a loading state")
	}
}
