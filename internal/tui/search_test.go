package tui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/KabosuNeko/Futon/internal/storage"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

func testSearchModel() SearchModel {
	return NewSearchModel([]api.MangaProvider{
		api.NewBaoTangTruyenProvider(),
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
	m.listMode = resultsSearch
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
	m.view = viewFavorites
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
	m.view = viewHistory
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

		if (rm.view == viewFavorites) != tc.wantFavorites {
			t.Errorf("%q: showingFavorites = %v, want %v", tc.input, (rm.view == viewFavorites), tc.wantFavorites)
		}
		if (rm.view == viewHistory) != tc.wantHistory {
			t.Errorf("%q: showingHistory = %v, want %v", tc.input, (rm.view == viewHistory), tc.wantHistory)
		}
		if got := rm.providers[1].(*api.MangaDexProvider).Lang(); got != tc.wantLang {
			t.Errorf("%q: MangaDex language = %v, want %v", tc.input, got, tc.wantLang)
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
		{ID: "m2", Title: "Bleach", CoverURL: "https://example.com/bleach.jpg", Provider: "MangaDex"},
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
		{ID: "m1", Title: "One Piece", CoverURL: "https://example.com/cover.jpg", Provider: "MangaDex"},
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

func (p streamProvider) Name() string                                           { return p.name }
func (p streamProvider) Search(context.Context, string) ([]models.Manga, error) { return p.manga, nil }
func (p streamProvider) FetchLatest(context.Context, int) ([]models.Manga, error) {
	return p.manga, nil
}
func (p streamProvider) Filter(context.Context, api.FilterOptions) ([]models.Manga, error) {
	return p.manga, nil
}
func (p streamProvider) FetchChapters(context.Context, string) ([]models.Chapter, error) {
	return nil, nil
}
func (p streamProvider) FetchPages(context.Context, string) ([]string, error) { return nil, nil }

func TestStreamedResultsApplyWhileSearching(t *testing.T) {
	m := NewSearchModel([]api.MangaProvider{streamProvider{
		name:  "Fake",
		manga: []models.Manga{{ID: "m1", Title: "Naruto"}},
	}})
	m.width, m.height = 100, 24
	m.listMode = resultsSearch
	m.currentQuery = "naruto"
	m.isSearching = true

	cmd := searchAllCmd(context.Background(), m.providers, "naruto")
	msg := cmd().(resultsMsg)
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

	var final resultsMsg
	for _, sub := range drainBatch(t, next) {
		if m, ok := sub.(resultsMsg); ok {
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

func TestSourceTogglesMigrateMergedSourcesToTruyenQQ(t *testing.T) {
	for _, saved := range []string{"TruyenGG", "FoxTruyen"} {
		t.Setenv("HOME", t.TempDir())
		if err := storage.SaveSources([]string{saved, "CuuTruyen"}); err != nil {
			t.Fatalf("SaveSources: %v", err)
		}

		m := NewSearchModel([]api.MangaProvider{
			api.NewTruyenQQProvider(),
			api.NewBaoTangTruyenProvider(),
			api.NewCuuTruyenProvider(),
		})

		if !m.providerToggles[0] {
			t.Errorf("saved %s should enable TruyenQQ, which it was merged into", saved)
		}
		if m.providerToggles[1] {
			t.Error("BaoTangTruyen was not in saved sources and should stay disabled")
		}
		if !m.providerToggles[2] {
			t.Error("CuuTruyen toggle should stay enabled")
		}
	}
}

func TestSearchInputKeepsKittyLikeText(t *testing.T) {
	m := testSearchModel()
	for _, r := range "_Gi=;OK" {
		updated, _ := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = updated.(SearchModel)
	}
	if got := m.input.Value(); got != "_Gi=;OK" {
		t.Errorf("input = %q, typed text was rewritten", got)
	}
}

func TestFeedErrorWrapsToScreenWidth(t *testing.T) {
	m := testSearchModel()
	m.width, m.height = 80, 30
	m.err = errors.New(strings.Repeat("rất dài ", 40))
	for _, line := range strings.Split(m.View().Content, "\n") {
		if w := lipgloss.Width(line); w > m.width {
			t.Fatalf("line is %d cells wide on an %d-cell screen: %q", w, m.width, line)
		}
	}
}

func TestHistoryChapterLabel(t *testing.T) {
	cases := []struct {
		h    storage.ReadHistory
		want string
	}{
		{storage.ReadHistory{ChapterNumber: "12"}, "Ch. 12"},
		{storage.ReadHistory{ChapterNumber: "Chương 12"}, "Chương 12"},
		{storage.ReadHistory{ChapterID: "/truyen-tranh/futari-bus-22373-chap-1"}, "futari-bus-22373-chap-1"},
		{storage.ReadHistory{ChapterID: "/truyen/blue-lock/38"}, "38"},
	}
	for _, c := range cases {
		if got := historyChapterLabel(c.h); got != c.want {
			t.Errorf("historyChapterLabel(%+v) = %q, want %q", c.h, got, c.want)
		}
	}
}

func TestHTMLSourceChapterOpensWithReadableLabel(t *testing.T) {
	m := NewChapterListModel("m1", "T", nil)
	m.loading = false
	m.chapters = []models.Chapter{{ID: "/truyen-tranh/x-chap-1", Title: "Chương 1"}}
	_, cmd := m.selectCurrentChapter()
	msg := cmd().(ViewChapterMsg)
	if msg.ChapterNumber != "Chương 1" || msg.AllChapterNumbers[0] != "Chương 1" {
		t.Errorf("chapter label = %q / %v, want the title", msg.ChapterNumber, msg.AllChapterNumbers)
	}
}

func TestSameTitleOnTwoSourcesIsTwoRows(t *testing.T) {
	m := testSearchModel()
	m.width, m.height = 120, 30
	m.results = []models.Manga{
		{ID: "/bt/bl", Title: "Blue Lock", Provider: "BaoTangTruyen"},
		{ID: "md-bl", Title: "Blue Lock", Provider: "MangaDex"},
	}
	view := m.View().Content
	for _, row := range []string{"Blue Lock (baotangtruyen)", "Blue Lock (mangadex)"} {
		if !strings.Contains(view, row) {
			t.Errorf("missing row %q in:\n%s", row, view)
		}
	}

	m.cursor = 1
	_, cmd, _ := m.handleEnter()
	if msg, ok := cmd().(ViewMangaMsg); !ok || msg.ProviderName != "MangaDex" || msg.MangaID != "md-bl" {
		t.Errorf("enter on row 2 opened %+v, want the MangaDex entry", msg)
	}
}

func TestMissingFavoriteCoverIsLookedUpOnceAfterScrollingStops(t *testing.T) {
	m := testSearchModel()
	m.width, m.height = 120, 30
	m.view = viewFavorites
	m.favorites = []storage.FavoriteManga{
		{MangaID: "m1", Title: "A", Provider: "MangaDex"},
		{MangaID: "m2", Title: "B", Provider: "MangaDex"},
	}

	// Focusing entries only schedules a debounced lookup; scrolling past one
	// must not mark it as looked up.
	m.updateFocusedCover()
	m.cursor = 1
	m.updateFocusedCover()
	if len(m.coverLookups) != 0 {
		t.Fatalf("lookups started while scrolling: %v", m.coverLookups)
	}
	if updated, cmd := m.Update(coverDebounceMsg{mangaID: "m1"}); cmd != nil && len(updated.(SearchModel).coverLookups) > 0 {
		t.Error("an entry scrolled past was looked up")
	}
	updated, _ := m.Update(coverDebounceMsg{mangaID: "m2"})
	m = updated.(SearchModel)
	if !m.coverLookups["MangaDex\x00m2"] || len(m.coverLookups) != 1 {
		t.Fatalf("lookups = %v, want only the entry the cursor stopped on", m.coverLookups)
	}
	if cmd := m.updateFocusedCover(); cmd != nil {
		t.Error("the same entry was looked up twice")
	}

	m, _ = m.applyFoundCover(coverFoundMsg{mangaID: "m2", provider: "MangaDex", url: "https://x/c.jpg"})
	if m.favorites[1].CoverURL != "https://x/c.jpg" || !m.coverLoading {
		t.Errorf("found cover not stored and fetched: %+v loading=%v", m.favorites[1], m.coverLoading)
	}
}

func TestStaleSavedCoverIsLookedUpAgain(t *testing.T) {
	m := testSearchModel()
	m.width, m.height = 120, 30
	m.view = viewHistory
	m.history = []storage.ReadHistory{{MangaID: "m1", MangaTitle: "A", Provider: "MangaDex", CoverURL: "https://x/old.jpg"}}

	updated, cmd := m.Update(coverRenderedMsg{mangaID: "m1", coverURL: "https://x/old.jpg", err: errors.New("HTTP 404")})
	m = updated.(SearchModel)
	if cmd == nil || !m.coverLookups["MangaDex\x00m1"] {
		t.Fatal("a saved cover that fails to load must be looked up again")
	}
	if _, cmd := m.Update(coverRenderedMsg{mangaID: "m1", coverURL: "https://x/old.jpg", err: errors.New("HTTP 404")}); cmd != nil {
		t.Error("looked up again after the lookup already ran this session")
	}
}

func TestLookupOfAnotherEntryKeepsFocusedCoverLoading(t *testing.T) {
	m := testSearchModel()
	m.width, m.height = 120, 30
	m.view = viewFavorites
	m.favorites = []storage.FavoriteManga{
		{MangaID: "a", Title: "A", Provider: "MangaDex"},
		{MangaID: "b", Title: "B", Provider: "MangaDex", CoverURL: "https://x/b.jpg"},
	}
	m.cursor = 1
	if m.updateFocusedCover() == nil || !m.coverLoading {
		t.Fatal("expected B's cover fetch to be scheduled")
	}
	// A's lookup finishes while B's cover is still loading.
	m, cmd := m.applyFoundCover(coverFoundMsg{mangaID: "a", provider: "MangaDex", url: "https://x/a.jpg"})
	if !m.coverLoading {
		t.Error("A's lookup cleared the loading flag of B")
	}
	for _, msg := range collectMessages(cmd) {
		if d, ok := msg.(coverDebounceMsg); ok && d.mangaID == "b" {
			t.Error("B's cover fetch was scheduled a second time")
		}
	}
}
