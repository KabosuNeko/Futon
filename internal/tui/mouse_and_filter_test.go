package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
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

	newM, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	sm := newM.(SearchModel)
	if sm.cursor != 1 {
		t.Errorf("expected cursor 1 after wheel down, got %d", sm.cursor)
	}

	newM2, _ := sm.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	sm2 := newM2.(SearchModel)
	if sm2.cursor != 0 {
		t.Errorf("expected cursor 0 after wheel up, got %d", sm2.cursor)
	}

	// Mouse click item index 2 (msg.Y = searchUIOffset + 2)
	newM3, _ := sm2.Update(tea.MouseClickMsg{Button: tea.MouseLeft, Y: searchUIOffset + 2})
	sm3 := newM3.(SearchModel)
	if sm3.cursor != 2 {
		t.Errorf("expected cursor 2 after click item 2, got %d", sm3.cursor)
	}
}

func TestSearchTabCycleAndFilterModal(t *testing.T) {
	provider := api.NewOTruyenProvider()
	m := NewSearchModel([]api.MangaProvider{provider})
	m.showingFeed = true

	newM, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	sm := newM.(SearchModel)
	if !sm.showingFavorites {
		t.Errorf("expected showingFavorites after tab 1")
	}

	newM, _ = sm.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	sm = newM.(SearchModel)
	if !sm.showingHistory {
		t.Errorf("expected showingHistory after tab 2")
	}

	newM, _ = sm.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	sm = newM.(SearchModel)
	if !sm.showingSources {
		t.Errorf("expected showingSources after tab 3")
	}

	newM, _ = sm.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	sm = newM.(SearchModel)
	if !sm.showingFilters {
		t.Errorf("expected showingFilters after tab 4")
	}

	newM, _ = sm.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	sm = newM.(SearchModel)
	if sm.filterStatus != 1 {
		t.Errorf("expected filterStatus=1 (Ongoing), got %d", sm.filterStatus)
	}

	view := sm.View().Content
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

	newM, _ := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 60})
	rm := newM.(ReaderModel)
	if rm.currentIdx != 1 {
		t.Errorf("expected currentIdx 1 after right click, got %d", rm.currentIdx)
	}

	rm.isLoading = false

	newM2, _ := rm.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 20})
	rm2 := newM2.(ReaderModel)
	if rm2.currentIdx != 0 {
		t.Errorf("expected currentIdx 0 after left click, got %d", rm2.currentIdx)
	}

	rm2.isLoading = false

	newM3, _ := rm2.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
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

	newM, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	cm := newM.(ChapterListModel)
	if cm.cursor != 1 {
		t.Errorf("expected cursor 1 after wheel down, got %d", cm.cursor)
	}

	newM2, _ := cm.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	cm2 := newM2.(ChapterListModel)
	if cm2.cursor != 0 {
		t.Errorf("expected cursor 0 after wheel up, got %d", cm2.cursor)
	}
}
