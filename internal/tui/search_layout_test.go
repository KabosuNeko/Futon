package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/models"
)

func TestSearchClickAccountsForSystemMessage(t *testing.T) {
	m := testSearchModel()
	m.width, m.height = 100, 30
	m.listMode = resultsSearch
	m.results = []models.Manga{{ID: "1", Title: "A"}, {ID: "2", Title: "B"}, {ID: "3", Title: "C"}}

	before := m.listTopRow()
	m.systemMsg = "Đã là bản mới nhất."
	after := m.listTopRow()
	if after == before {
		t.Fatal("a header line did not move the list; click mapping would be off by one")
	}

	updated, _ := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, Y: after + 2})
	if got := updated.(SearchModel).cursor; got != 2 {
		t.Errorf("click on third row selected %d, want 2", got)
	}
}
