package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/KabosuNeko/Futon/internal/storage"
)

func testSearchModel() SearchModel {
	return NewSearchModel([]api.MangaProvider{
		api.NewOTruyenProvider(),
		api.NewMangaDexProvider(),
	})
}

func TestSearchViewShowsResults(t *testing.T) {
	m := testSearchModel()
	m.width = 80
	m.height = 24
	m.results = []models.Manga{
		{ID: "m1", Title: "Naruto"},
		{ID: "m2", Title: "One Piece"},
	}
	m.currentQuery = "shonen"
	m.cursor = 1

	view := m.View()
	if !strings.Contains(view, "Kết quả cho") {
		t.Errorf("expected result title in view")
	}
	if !strings.Contains(view, "Naruto") || !strings.Contains(view, "One Piece") {
		t.Errorf("expected manga titles in view")
	}
	if !strings.Contains(view, "> ") || !strings.Contains(view, "One Piece") {
		t.Errorf("expected selected cursor marker and title")
	}
}

func TestSearchViewShowsFavorites(t *testing.T) {
	m := testSearchModel()
	m.width = 80
	m.height = 24
	m.showingFavorites = true
	m.favorites = []storage.FavoriteManga{
		{MangaID: "m1", Title: "Bleach"},
	}

	view := m.View()
	if !strings.Contains(view, "Truyện Yêu Thích") {
		t.Errorf("expected favorites title in view")
	}
	if !strings.Contains(view, "Bleach") {
		t.Errorf("expected favorite title in view")
	}
}

func TestSearchViewShowsHistory(t *testing.T) {
	m := testSearchModel()
	m.width = 80
	m.height = 24
	m.showingHistory = true
	m.history = []storage.ReadHistory{
		{MangaID: "m1", MangaTitle: "Doraemon", ChapterNumber: "1", PageIndex: 3},
	}

	view := m.View()
	if !strings.Contains(view, "Lịch Sử Đọc") {
		t.Errorf("expected history title in view")
	}
	if !strings.Contains(view, "Doraemon") {
		t.Errorf("expected history title in view")
	}
}

func TestSearchViewEmptyResults(t *testing.T) {
	m := testSearchModel()
	m.width = 80
	m.height = 24
	m.currentQuery = "xyz"
	m.results = []models.Manga{}

	view := m.View()
	if !strings.Contains(view, "Không tìm thấy kết quả") {
		t.Errorf("expected no-result message in view")
	}
}

func TestSearchSlashCommands(t *testing.T) {
	cases := []struct {
		input         string
		wantFavorites bool
		wantHistory   bool
		wantLang      string
		wantSystemMsg string
	}{
		{"/fav", true, false, "vi", ""},
		{"/his", false, true, "vi", ""},
		{"/lang en", false, false, "en", "Đã cài đặt ngôn ngữ chapter mặc định: en"},
		{"/lang xx", false, false, "vi", "Dùng: /lang vi hoặc /lang en"},
	}

	for _, tc := range cases {
		m := testSearchModel()
		m.input.SetValue(tc.input)
		newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter")})
		rm := newM.(SearchModel)

		if rm.showingFavorites != tc.wantFavorites {
			t.Errorf("%q: showingFavorites = %v, want %v", tc.input, rm.showingFavorites, tc.wantFavorites)
		}
		if rm.showingHistory != tc.wantHistory {
			t.Errorf("%q: showingHistory = %v, want %v", tc.input, rm.showingHistory, tc.wantHistory)
		}
		if rm.chapterLang != tc.wantLang {
			t.Errorf("%q: chapterLang = %v, want %v", tc.input, rm.chapterLang, tc.wantLang)
		}
		if tc.wantSystemMsg != "" && rm.systemMsg != tc.wantSystemMsg {
			t.Errorf("%q: systemMsg = %q, want %q", tc.input, rm.systemMsg, tc.wantSystemMsg)
		}
	}
}
