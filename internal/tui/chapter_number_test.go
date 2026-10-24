package tui

import (
	"strings"
	"testing"

	"github.com/KabosuNeko/Futon/internal/models"
)

func TestChapterNavCmdSetsChapterNumber(t *testing.T) {
	ids := []string{"c1", "c2", "c3"}
	nums := []string{"101", "102", "103"}

	cmd := chapterNavCmd("c3", "m1", "Title", ids, nums, 2, 0)
	if cmd == nil {
		t.Fatal("expected a cmd")
	}
	msg, ok := cmd().(ViewChapterMsg)
	if !ok {
		t.Fatalf("expected ViewChapterMsg, got %T", cmd())
	}
	if msg.ChapterNumber != "103" {
		t.Errorf("expected ChapterNumber 103, got %q", msg.ChapterNumber)
	}
	if msg.ChapterID != "c3" || msg.ChapterIndex != 2 {
		t.Errorf("unexpected msg fields: %+v", msg)
	}
	if len(msg.AllChapterNumbers) != 3 || msg.AllChapterNumbers[0] != "101" {
		t.Errorf("AllChapterNumbers not propagated: %v", msg.AllChapterNumbers)
	}
}

func TestChapterNavCmdMissingNumber(t *testing.T) {
	cases := []struct {
		name    string
		numbers []string
		index   int
	}{
		{"out of range index", []string{"101"}, 9},
		{"nil numbers", nil, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := chapterNavCmd("c2", "m1", "Title", []string{"c1", "c2"}, tc.numbers, tc.index, 0)
			msg := cmd().(ViewChapterMsg)
			if msg.ChapterNumber != "" {
				t.Errorf("expected empty ChapterNumber for %s, got %q", tc.name, msg.ChapterNumber)
			}
		})
	}
}

func TestApplyPreloadedChapterUpdatesChapterNumber(t *testing.T) {
	cases := []struct {
		name    string
		numbers []string
		want    string
	}{
		{"with numbers", []string{"101", "102", "103"}, "102"},
		{"nil numbers keep old", nil, "101"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := NewReaderModel("m1", "Title", "c1", "101", []string{"c1", "c2", "c3"}, 0, -1, nil)
			m.allChapterNumbers = tc.numbers
			m.preloadedChapID = "c2"
			m.preloadedURLs = []string{"u1", "u2"}
			m.preloadedImages = [][]byte{{1}, {2}}

			m.applyPreloadedChapter("c2")

			if m.chapterNumber != tc.want {
				t.Errorf("expected chapterNumber %q after preload apply, got %q", tc.want, m.chapterNumber)
			}
			if m.chapterIndex != 1 {
				t.Errorf("expected chapterIndex 1, got %d", m.chapterIndex)
			}
			if m.chapterID != "c2" {
				t.Errorf("expected chapterID c2, got %q", m.chapterID)
			}
		})
	}
}

func TestAppModelForwardsAllChapterNumbers(t *testing.T) {
	m := NewAppModel("t")
	nums := []string{"101", "102"}
	newModel, _ := m.Update(ViewChapterMsg{
		MangaID:           "m1",
		MangaTitle:        "Title",
		ChapterID:         "c1",
		ChapterNumber:     "101",
		AllChapterIDs:     []string{"c1", "c2"},
		AllChapterNumbers: nums,
		ChapterIndex:      0,
		StartPageIndex:    -1,
	})
	app := newModel.(AppModel)

	if app.reader.allChapterNumbers == nil {
		t.Fatal("expected allChapterNumbers set on reader")
	}
	if len(app.reader.allChapterNumbers) != 2 || app.reader.allChapterNumbers[1] != "102" {
		t.Errorf("unexpected allChapterNumbers: %v", app.reader.allChapterNumbers)
	}
}

func TestChapterListViewCardAndFooter(t *testing.T) {
	m := NewChapterListModel("m1", "One Piece", nil)
	m.width = 80
	m.height = 24
	m.chapters = []models.Chapter{
		{ID: "c1", Number: "1", Title: "Romance Dawn"},
		{ID: "c2", Number: "2", Title: "They Call Him Straw Hat"},
	}
	m.loading = false

	view := m.View().Content
	if !strings.Contains(view, "Danh sách chapter") {
		t.Errorf("expected card header in chapter view")
	}
	if !strings.Contains(view, "Ch. 1 - Romance Dawn") {
		t.Errorf("expected chapter 1 in list")
	}
	if !strings.Contains(view, "[enter] Mở đọc") || !strings.Contains(view, "[esc] Quay lại") {
		t.Errorf("expected matched footer keys in chapter view, got:\n%s", view)
	}
}

func TestJumpToChapterMatchesTitleWhenNumberDoesNot(t *testing.T) {
	m := NewChapterListModel("m1", "Title", nil)
	m.chapters = []models.Chapter{
		{ID: "c1", Number: "10", Title: "Main story"},
		{ID: "c2", Number: "20", Title: "Bonus 202"},
	}

	if !m.jumpToChapter("202") {
		t.Fatal("expected a title match when no chapter number contains the query")
	}
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (title match)", m.cursor)
	}
}

func TestChapterLabelFallsBackToTitle(t *testing.T) {
	cases := []struct {
		chapter models.Chapter
		want    string
	}{
		{models.Chapter{Number: "12", Title: "Ignored"}, "12"},
		{models.Chapter{Title: "Chương 3"}, "Chương 3"},
		{models.Chapter{}, "unknown"},
	}
	for _, tc := range cases {
		if got := chapterLabel(tc.chapter); got != tc.want {
			t.Errorf("chapterLabel(%+v) = %q, want %q", tc.chapter, got, tc.want)
		}
	}
}

func TestChapterListShowsTitleWhenNumberMissing(t *testing.T) {
	m := NewChapterListModel("m1", "Title", nil)
	m.width = 80
	m.height = 24
	m.chapters = []models.Chapter{
		{ID: "c1", Title: "Chương đặc biệt"},
	}
	m.loading = false

	view := m.View().Content
	if !strings.Contains(view, "Chương đặc biệt") {
		t.Fatalf("expected chapter title in view, got:\n%s", view)
	}
	if strings.Contains(view, "Ch.  -") {
		t.Errorf("chapter without a number must not render an empty Ch. label:\n%s", view)
	}
}
