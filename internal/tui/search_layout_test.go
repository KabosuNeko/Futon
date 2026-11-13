package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

func TestSearchScreenFitsNarrowTerminals(t *testing.T) {
	for _, cols := range []int{60, 80} {
		for _, v := range []searchView{viewFeed, viewFavorites, viewHistory, viewSources, viewFilters} {
			m := testSearchModel()
			m.width, m.height = cols, 24
			m.view = v
			m.results = []models.Manga{{ID: "1", Title: "Blue Lock - Tiền Đạo Số 1", Provider: "TruyenQQ"}}
			view := m.View().Content
			lines := strings.Split(view, "\n")
			for _, line := range lines {
				if w := lipgloss.Width(line); w > cols {
					t.Fatalf("tab %d at %d cols: a %d-column line: %q", v, cols, w, line)
				}
			}
			if len(lines) > 24 {
				t.Errorf("tab %d at %dx24: %d rows", v, cols, len(lines))
			}
			if !strings.Contains(view, "Thoát") || !strings.Contains(view, "Bộ lọc]") {
				t.Errorf("tab %d at %d cols lost a hint or a tab", v, cols)
			}
		}
	}
}

func TestSearchScreenFitsOrSaysTooSmall(t *testing.T) {
	var results []models.Manga
	for i := 0; i < 40; i++ {
		results = append(results, models.Manga{ID: fmt.Sprint(i), Title: "Blue Lock - Tiền Đạo Số 1", Provider: "TruyenQQ"})
	}
	for _, cols := range []int{40, 54, 60, 80, 120} {
		for rows := 10; rows <= 30; rows++ {
			for _, v := range []searchView{viewFeed, viewFavorites, viewHistory, viewSources, viewFilters} {
				m := testSearchModel()
				m.width, m.height = cols, rows
				m.view = v
				m.results = results
				lines := strings.Split(m.View().Content, "\n")
				for _, line := range lines {
					if w := lipgloss.Width(line); w > cols {
						t.Fatalf("tab %d at %dx%d: a %d-column line", v, cols, rows, w)
					}
				}
				if len(lines) > rows {
					t.Fatalf("tab %d at %dx%d: %d rows", v, cols, rows, len(lines))
				}
			}
		}
	}

	// Common sizes get the real screen, not the notice.
	for _, size := range [][2]int{{60, 24}, {80, 24}, {80, 16}, {120, 30}} {
		for _, v := range []searchView{viewFeed, viewFavorites, viewHistory, viewSources, viewFilters} {
			m := testSearchModel()
			m.width, m.height = size[0], size[1]
			m.view = v
			m.results = results
			if strings.Contains(m.View().Content, "quá nhỏ") && !(v == viewFilters && size[1] < 20) {
				t.Errorf("tab %d at %dx%d shows the too-small notice", v, size[0], size[1])
			}
		}
	}
}

func TestClickOnTooSmallNoticeDoesNothing(t *testing.T) {
	m := testSearchModel()
	m.width, m.height = 30, 10
	m.listMode = resultsSearch
	m.results = []models.Manga{{ID: "a", Title: "A", Provider: "TruyenQQ"}}
	if !strings.Contains(m.View().Content, "quá nhỏ") {
		t.Fatal("setup: expected the too-small notice")
	}
	if _, cmd := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, Y: 0}); cmd != nil {
		if _, opened := cmd().(ViewMangaMsg); opened {
			t.Error("a click on the notice opened a manga")
		}
	}

	c := NewChapterListModel("m", "T", nil)
	c.setSize(80, 8)
	c.loading = false
	c.chapters = []models.Chapter{{ID: "c1", Number: "1"}}
	if !strings.Contains(c.View().Content, "quá nhỏ") {
		t.Fatal("chapter list at 80x8 should show the notice")
	}
	if _, cmd := c.Update(tea.MouseClickMsg{Button: tea.MouseLeft, Y: 0}); cmd != nil {
		if _, opened := cmd().(ViewChapterMsg); opened {
			t.Error("a click on the chapter-list notice opened a chapter")
		}
	}
}

func TestHeaderNoticeDoesNotPushScreenOffOrTriggerTooSmall(t *testing.T) {
	var results []models.Manga
	for i := 0; i < 40; i++ {
		results = append(results, models.Manga{ID: fmt.Sprint(i), Title: "Blue Lock", Provider: "TruyenQQ"})
	}
	for _, size := range [][2]int{{100, 30}, {80, 24}, {80, 20}, {60, 24}} {
		for _, notice := range []string{"system", "warning"} {
			m := testSearchModel()
			m.width, m.height = size[0], size[1]
			m.listMode = resultsSearch
			m.results = results
			if notice == "system" {
				m.systemMsg = "Đã là bản mới nhất."
			} else {
				m.providerErrors = map[string]string{"MangaDex": "connection refused"}
				m.providerCounts = map[string]int{"MangaDex": 0, "TruyenQQ": 40}
			}
			view := m.View().Content
			if strings.Contains(view, "quá nhỏ") {
				t.Errorf("%dx%d with a %s line shows the too-small notice", size[0], size[1], notice)
			}
			if h := len(strings.Split(view, "\n")); h > size[1] {
				t.Errorf("%dx%d with a %s line: %d rows", size[0], size[1], notice, h)
			}
		}
	}
}
