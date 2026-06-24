package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
)

func TestTypingUpdatesInput(t *testing.T) {
	m := testSearchModel()

	for _, r := range "naruto" {
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newM.(SearchModel)
	}

	if m.input.Value() != "naruto" {
		t.Errorf("expected input 'naruto', got %q", m.input.Value())
	}
}

func TestTabCyclesProviders(t *testing.T) {
	providers := []api.MangaProvider{api.NewOTruyenProvider(), api.NewMangaDexProvider()}
	m := NewSearchModel(providers)

	// Default is "All" mode, CurrentProvider returns nil
	if m.CurrentProvider() != nil {
		t.Fatalf("expected nil (All mode), got %v", m.CurrentProvider())
	}

	// Tab 1: All -> OTruyen
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	rm := newM.(SearchModel)
	if rm.CurrentProvider().Name() != "OTruyen" {
		t.Errorf("expected provider OTruyen after tab, got %s", rm.CurrentProvider().Name())
	}

	// Tab 2: OTruyen -> MangaDex
	newM, _ = rm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	rm = newM.(SearchModel)
	if rm.CurrentProvider().Name() != "MangaDex" {
		t.Errorf("expected provider MangaDex after second tab, got %s", rm.CurrentProvider().Name())
	}

	// Tab 3: MangaDex -> All
	newM, _ = rm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	rm = newM.(SearchModel)
	if rm.CurrentProvider() != nil {
		t.Errorf("expected All mode after third tab, got %v", rm.CurrentProvider().Name())
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
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("down")})
		m = newM.(SearchModel)
	}

	if m.cursor != target {
		t.Errorf("expected cursor %d, got %d", target, m.cursor)
	}
	if m.viewportStart <= 0 {
		t.Errorf("expected viewportStart > 0 after scrolling down, got %d", m.viewportStart)
	}

	view := m.View()
	if strings.Contains(view, m.results[0].Title) {
		t.Errorf("first result should be scrolled out of view")
	}

	for i := 0; i < target; i++ {
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("up")})
		m = newM.(SearchModel)
	}

	if m.cursor != 0 {
		t.Errorf("expected cursor 0, got %d", m.cursor)
	}
	if m.viewportStart != 0 {
		t.Errorf("expected viewportStart 0 after scrolling back, got %d", m.viewportStart)
	}
}

func TestSearchJKNavigation(t *testing.T) {
	m := testSearchModel()
	m.width = 80
	m.height = 24
	m.results = []models.Manga{
		{ID: "m1", Title: "Alpha"},
		{ID: "m2", Title: "Beta"},
	}

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newM.(SearchModel)
	if m.cursor != 1 {
		t.Errorf("expected cursor 1 after j, got %d", m.cursor)
	}

	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = newM.(SearchModel)
	if m.cursor != 0 {
		t.Errorf("expected cursor 0 after k, got %d", m.cursor)
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

	view := m.View()
	plain := stripANSI(view)
	lines := strings.Count(plain, "\n")
	if lines > m.height {
		t.Errorf("rendered view has %d lines, exceeding terminal height %d", lines, m.height)
	}
}

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}
