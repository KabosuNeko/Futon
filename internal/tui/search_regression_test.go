package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/KabosuNeko/Futon/internal/api"
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

	if m.CurrentProvider().Name() != "OTruyen" {
		t.Fatalf("expected default provider OTruyen, got %s", m.CurrentProvider().Name())
	}

	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	rm := newM.(SearchModel)
	if rm.CurrentProvider().Name() != "MangaDex" {
		t.Errorf("expected provider MangaDex after tab, got %s", rm.CurrentProvider().Name())
	}

	newM, _ = rm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab")})
	rm = newM.(SearchModel)
	if rm.CurrentProvider().Name() != "OTruyen" {
		t.Errorf("expected provider OTruyen after second tab, got %s", rm.CurrentProvider().Name())
	}
}
