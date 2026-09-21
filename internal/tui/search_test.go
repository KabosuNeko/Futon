package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/KabosuNeko/Futon/internal/storage"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
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

	view := m.View().Content
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

	view := m.View().Content
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

	view := m.View().Content
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

	view := m.View().Content
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
		newM, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		rm := newM.(SearchModel)

		if rm.showingFavorites != tc.wantFavorites {
			t.Errorf("%q: showingFavorites = %v, want %v", tc.input, rm.showingFavorites, tc.wantFavorites)
		}
		if rm.showingHistory != tc.wantHistory {
			t.Errorf("%q: showingHistory = %v, want %v", tc.input, rm.showingHistory, tc.wantHistory)
		}
		if rm.chapterLanguage != tc.wantLang {
			t.Errorf("%q: chapterLanguage = %v, want %v", tc.input, rm.chapterLanguage, tc.wantLang)
		}
		if tc.wantSystemMsg != "" && rm.systemMsg != tc.wantSystemMsg {
			t.Errorf("%q: systemMsg = %q, want %q", tc.input, rm.systemMsg, tc.wantSystemMsg)
		}
	}
}

func TestSearchViewSplitLayoutAndPreview(t *testing.T) {
	m := testSearchModel()
	m.width = 100
	m.height = 24
	m.results = []models.Manga{
		{
			ID:       "m1",
			Title:    "One Piece",
			CoverURL: "https://example.com/onepiece.jpg",
			Provider: "MangaDex",
			Author:   "Eiichiro Oda",
			Year:     "1997",
			Status:   "ongoing",
			Genres:   []string{"Action", "Adventure", "Comedy"},
		},
		{ID: "m2", Title: "Bleach", CoverURL: "https://example.com/bleach.jpg", Provider: "OTruyen"},
	}
	m.currentQuery = "shonen"
	m.cursor = 0

	view := m.View().Content
	if !strings.Contains(view, "Chi tiết manga") {
		t.Errorf("expected preview pane header in split view")
	}
	if !strings.Contains(view, "One Piece") {
		t.Errorf("expected focused manga title in preview pane")
	}
	if !strings.Contains(view, "MangaDex") {
		t.Errorf("expected provider badge in preview pane")
	}
	if !strings.Contains(view, "Eiichiro Oda") {
		t.Errorf("expected author in preview pane")
	}
	if !strings.Contains(view, "1997") {
		t.Errorf("expected year in preview pane")
	}
	if !strings.Contains(view, "Đang ra") {
		t.Errorf("expected status in preview pane")
	}
	if !strings.Contains(view, "[Action]") {
		t.Errorf("expected genre badge in preview pane")
	}
}

func TestCoverDebounceAndRenderMsg(t *testing.T) {
	m := testSearchModel()
	m.results = []models.Manga{
		{ID: "m1", Title: "One Piece", CoverURL: "https://example.com/cover.jpg", Provider: "OTruyen"},
	}
	m.cursor = 0

	rendered := imgrender.RenderedImage{
		EscapeSequence: "\x1b_Gtest\x1b\\",
		WidthPx:        100,
		HeightPx:       150,
	}
	msg := coverRenderedMsg{
		mangaID:  "m1",
		coverURL: "https://example.com/cover.jpg",
		rendered: rendered,
	}

	newM, _ := m.Update(msg)
	rm := newM.(SearchModel)

	if rm.currentCover == nil {
		t.Fatalf("expected currentCover to be set")
	}
	if rm.currentCover.EscapeSequence != "\x1b_Gtest\x1b\\" {
		t.Errorf("unexpected EscapeSequence: %s", rm.currentCover.EscapeSequence)
	}
	if _, cached := rm.coverCache["https://example.com/cover.jpg"]; !cached {
		t.Errorf("expected cover to be in coverCache")
	}
}

func TestFilteredIndicesMatchesActiveList(t *testing.T) {
	m := testSearchModel()
	m.filterQuery = "nar"

	m.favorites = []storage.FavoriteManga{{Title: "Naruto"}, {Title: "Bleach"}}
	if got := m.filteredFavIndices(); !reflect.DeepEqual(got, []int{0}) {
		t.Errorf("filteredFavIndices = %v, want [0]", got)
	}

	m.history = []storage.ReadHistory{{MangaID: "naruto-id"}, {MangaTitle: "Bleach"}}
	if got := m.filteredHistoryIndices(); !reflect.DeepEqual(got, []int{0}) {
		t.Errorf("filteredHistoryIndices = %v, want [0] via MangaID fallback", got)
	}

	m.filterQuery = "manga"
	if got := m.filteredProviderIndices(); !reflect.DeepEqual(got, []int{1}) {
		t.Errorf("filteredProviderIndices = %v, want [1]", got)
	}

	m.filterQuery = ""
	if got := m.filteredFavIndices(); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Errorf("filteredFavIndices with empty query = %v, want [0 1]", got)
	}
}

func TestSearchWithNoActiveSourceUsesErrNoSource(t *testing.T) {
	m := testSearchModel()
	m.providerToggles = []bool{false, false}
	m.input.SetValue("naruto")

	newM, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	rm := newM.(SearchModel)
	if rm.err != errNoSource {
		t.Fatalf("expected errNoSource, got %v", rm.err)
	}
	if rm.err.Error() != "Chọn ít nhất một nguồn trong /src" {
		t.Errorf("unexpected error message: %q", rm.err.Error())
	}
}

type streamProvider struct {
	name  string
	manga []models.Manga
}

func (p streamProvider) Name() string                            { return p.name }
func (p streamProvider) Search(string) ([]models.Manga, error)   { return p.manga, nil }
func (p streamProvider) FetchLatest(int) ([]models.Manga, error) { return p.manga, nil }
func (p streamProvider) Filter(api.FilterOptions) ([]models.Manga, error) {
	return p.manga, nil
}
func (p streamProvider) FetchChapters(string) ([]models.Chapter, error) { return nil, nil }
func (p streamProvider) FetchPages(string) ([]string, error)            { return nil, nil }

func TestStreamedResultsApplyWhileSearching(t *testing.T) {
	m := NewSearchModel([]api.MangaProvider{streamProvider{
		name:  "Fake",
		manga: []models.Manga{{ID: "m1", Title: "Naruto"}},
	}})
	m.width, m.height = 100, 24
	m.showingFeed = false
	m.currentQuery = "naruto"
	m.isSearching = true

	cmd := api.GlobalSearchCmd(m.providers, "naruto")
	msg := cmd().(api.MangaSearchResultMsg)
	if msg.Stream == nil {
		t.Fatal("single-provider search should still stream a snapshot")
	}

	updated, next := m.Update(msg)
	sm := updated.(SearchModel)
	if len(sm.results) != 1 || !sm.isSearching {
		t.Fatalf("partial snapshot must apply results and keep searching, got results=%d isSearching=%v", len(sm.results), sm.isSearching)
	}
	if next == nil {
		t.Fatal("expected a command to await the next snapshot")
	}

	var final api.MangaSearchResultMsg
	for _, sub := range drainBatch(t, next) {
		if m, ok := sub.(api.MangaSearchResultMsg); ok {
			final = m
		}
	}
	if final.Stream != nil {
		t.Fatal("expected the drained message to be the final snapshot")
	}

	updated2, _ := sm.Update(final)
	sm2 := updated2.(SearchModel)
	if sm2.isSearching {
		t.Error("final snapshot must clear isSearching")
	}
	if len(sm2.results) != 1 {
		t.Errorf("results after final = %d, want 1", len(sm2.results))
	}
}

// drainBatch runs nested batch commands and returns every message they emit.
func drainBatch(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, drainBatch(t, c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}
