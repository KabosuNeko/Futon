package tui

import (
	"strings"
	"testing"

	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
	tea "github.com/charmbracelet/bubbletea"
)

func TestSearchMouseWheelAndClick(t *testing.T) {
	provider := api.NewOTruyenProvider()
	m := NewSearchModel([]api.MangaProvider{provider})
	m.showingFeed = false
	m.results = []models.Manga{
		{ID: "1", Title: "Manga 1"},
		{ID: "2", Title: "Manga 2"},
		{ID: "3", Title: "Manga 3"},
	}
	m.cursor = 0

	// Mouse wheel down -> cursor 1
	newM, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	sm := newM.(SearchModel)
	if sm.cursor != 1 {
		t.Errorf("expected cursor 1 after wheel down, got %d", sm.cursor)
	}

	// Mouse wheel up -> cursor 0
	newM2, _ := sm.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	sm2 := newM2.(SearchModel)
	if sm2.cursor != 0 {
		t.Errorf("expected cursor 0 after wheel up, got %d", sm2.cursor)
	}

	// Mouse click item index 2 (msg.Y = searchUIOffset + 2)
	newM3, _ := sm2.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: searchUIOffset + 2})
	sm3 := newM3.(SearchModel)
	if sm3.cursor != 2 {
		t.Errorf("expected cursor 2 after click item 2, got %d", sm3.cursor)
	}
}

func TestSearchTabCycleAndFilterModal(t *testing.T) {
	provider := api.NewOTruyenProvider()
	m := NewSearchModel([]api.MangaProvider{provider})
	m.showingFeed = true

	// Press Tab -> switches to Favorites
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	sm := newM.(SearchModel)
	if !sm.showingFavorites {
		t.Errorf("expected showingFavorites after tab 1")
	}

	// Press Tab -> switches to History
	newM, _ = sm.Update(tea.KeyMsg{Type: tea.KeyTab})
	sm = newM.(SearchModel)
	if !sm.showingHistory {
		t.Errorf("expected showingHistory after tab 2")
	}

	// Press Tab -> switches to Sources
	newM, _ = sm.Update(tea.KeyMsg{Type: tea.KeyTab})
	sm = newM.(SearchModel)
	if !sm.showingSources {
		t.Errorf("expected showingSources after tab 3")
	}

	// Press Tab -> switches to Filters
	newM, _ = sm.Update(tea.KeyMsg{Type: tea.KeyTab})
	sm = newM.(SearchModel)
	if !sm.showingFilters {
		t.Errorf("expected showingFilters after tab 4")
	}

	// In Filters: Right arrow changes status
	newM, _ = sm.Update(tea.KeyMsg{Type: tea.KeyRight})
	sm = newM.(SearchModel)
	if sm.filterStatus != 1 {
		t.Errorf("expected filterStatus=1 (Ongoing), got %d", sm.filterStatus)
	}

	// View rendering contains filter modal
	view := sm.View()
	if !strings.Contains(view, "BỘ LỌC TÌM KIẾM") {
		t.Errorf("expected filter modal in view, got:\n%s", view)
	}
}

func TestReaderMouseClicks(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "1", []string{"c1"}, 0, 0, nil)
	m.step = stepRead
	m.total = 3
	m.currentIdx = 0
	m.width = 80

	// Click right side (X=60 > 40) -> Next page (1)
	newM, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 60})
	rm := newM.(ReaderModel)
	if rm.currentIdx != 1 {
		t.Errorf("expected currentIdx 1 after right click, got %d", rm.currentIdx)
	}

	rm.isLoading = false

	// Click left side (X=20 <= 40) -> Prev page (0)
	newM2, _ := rm.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 20})
	rm2 := newM2.(ReaderModel)
	if rm2.currentIdx != 0 {
		t.Errorf("expected currentIdx 0 after left click, got %d", rm2.currentIdx)
	}

	rm2.isLoading = false

	// Wheel down -> Next page (1)
	newM3, _ := rm2.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	rm3 := newM3.(ReaderModel)
	if rm3.currentIdx != 1 {
		t.Errorf("expected currentIdx 1 after wheel down, got %d", rm3.currentIdx)
	}
}

func TestChapterListMouseAndExportKey(t *testing.T) {
	provider := api.NewOTruyenProvider()
	m := NewChapterListModel("m1", "Title", provider)
	m.loading = false
	m.chapters = []models.Chapter{
		{ID: "c1", Number: "1", Title: "Chap 1"},
		{ID: "c2", Number: "2", Title: "Chap 2"},
		{ID: "c3", Number: "3", Title: "Chap 3"},
	}
	m.cursor = 0

	// Wheel down -> cursor 1
	newM, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	cm := newM.(ChapterListModel)
	if cm.cursor != 1 {
		t.Errorf("expected cursor 1 after wheel down, got %d", cm.cursor)
	}

	// Wheel up -> cursor 0
	newM2, _ := cm.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	cm2 := newM2.(ChapterListModel)
	if cm2.cursor != 0 {
		t.Errorf("expected cursor 0 after wheel up, got %d", cm2.cursor)
	}
}
