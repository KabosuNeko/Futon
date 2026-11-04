package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/KabosuNeko/Futon/internal/models"
)

func groupedModel() SearchModel {
	m := testSearchModel()
	m.width, m.height = 120, 30
	m.listMode = resultsSearch
	m.currentQuery = "blue lock"
	m.results = []models.Manga{
		{ID: "/qq/bl", Title: "Blue Lock - Tiền Đạo Số 1", Provider: "TruyenQQ"},
		{ID: "/ct/bl", Title: "Blue Lock", Provider: "CuuTruyen"},
		{ID: "/bt/bl", Title: "Blue Lock – Tiền Đạo Số 1", Provider: "BaoTangTruyen"},
		{ID: "/bt/bl2", Title: "Blue Lock", Provider: "BaoTangTruyen"},
	}
	return m
}

func TestSameTitleAcrossSourcesIsOneRow(t *testing.T) {
	m := groupedModel()
	rows := m.resultRows()
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (punctuation and source differences ignored)", len(rows))
	}
	if len(rows[0].variants) != 2 || rows[0].variants[1].Provider != "BaoTangTruyen" {
		t.Errorf("first row variants = %+v", rows[0].variants)
	}
	if m.currentListLen() != 2 {
		t.Errorf("list length = %d, want 2 rows", m.currentListLen())
	}
	view := m.View().Content
	if !strings.Contains(view, "(truyenqq · baotangtruyen)") {
		t.Errorf("row should list every source:\n%s", view)
	}
}

func TestShiftArrowPicksSourceToOpen(t *testing.T) {
	m := groupedModel()
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	m = updated.(SearchModel)
	if f, _ := m.focusedManga(); f.Provider != "BaoTangTruyen" {
		t.Fatalf("focused source = %s, want BaoTangTruyen after shift+right", f.Provider)
	}
	if !strings.Contains(m.View().Content, "(baotangtruyen · truyenqq)") {
		t.Error("the picked source should be listed first")
	}

	_, cmd, _ := m.selectCurrentItem()
	msg := cmd().(ViewMangaMsg)
	if msg.ProviderName != "BaoTangTruyen" || msg.MangaID != "/bt/bl" {
		t.Errorf("enter opened %s %s, want the picked source", msg.ProviderName, msg.MangaID)
	}

	// Wraps around, and the choice survives more results streaming in.
	updated, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift})
	m = updated.(SearchModel)
	m.results = append(m.results, models.Manga{ID: "x", Title: "Other", Provider: "TruyenQQ"})
	if f, _ := m.focusedManga(); f.Provider != "TruyenQQ" {
		t.Errorf("focused source = %s, want TruyenQQ after wrapping", f.Provider)
	}
}

func TestPlainArrowsStillEditTheQuery(t *testing.T) {
	m := groupedModel()
	m.input.SetValue("blue lock")
	updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	m = updated.(SearchModel)
	if m.input.Position() != len("blue lock")-1 {
		t.Errorf("left arrow moved input cursor to %d; it must stay a text-editing key", m.input.Position())
	}
	if f, _ := m.focusedManga(); f.Provider != "TruyenQQ" {
		t.Error("plain left arrow must not change the source")
	}
}
