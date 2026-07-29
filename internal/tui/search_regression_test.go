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

func TestSourceCommandToggles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	providers := []api.MangaProvider{api.NewOTruyenProvider(), api.NewMangaDexProvider()}
	m := NewSearchModel(providers)

	// All should be checked by default
	for i := range m.providers {
		if !m.providerToggles[i] {
			t.Fatalf("expected provider %d to be toggled on by default", i)
		}
	}

	// Type /src and press enter
	for _, r := range "/src" {
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = newM.(SearchModel)
	}
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(SearchModel)
	if !m.showingSources {
		t.Fatalf("expected showingSources after /src")
	}

	// Space toggles off current (cursor = 0)
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = newM.(SearchModel)
	if m.providerToggles[0] {
		t.Errorf("expected provider 0 to be toggled off after space")
	}

	// Space toggles back on
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = newM.(SearchModel)
	if !m.providerToggles[0] {
		t.Errorf("expected provider 0 to be toggled on after second space")
	}

	// Down arrow moves cursor
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newM.(SearchModel)
	if m.sourceCursor != 1 {
		t.Errorf("expected sourceCursor 1, got %d", m.sourceCursor)
	}

	// Space toggles provider at cursor 1
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = newM.(SearchModel)
	if m.providerToggles[1] {
		t.Errorf("expected provider 1 to be toggled off")
	}

	// ESC closes sources
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newM.(SearchModel)
	if m.showingSources {
		t.Errorf("expected showingSources false after ESC")
	}

	// activeProviders returns only checked ones
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
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
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
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
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

	view := m.View()
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
