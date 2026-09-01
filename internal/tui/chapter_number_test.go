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

func TestChapterNavCmdOutOfRangeNumberEmpty(t *testing.T) {
	cmd := chapterNavCmd("c9", "m1", "Title", []string{"c1"}, []string{"101"}, 9, 0)
	msg := cmd().(ViewChapterMsg)
	if msg.ChapterNumber != "" {
		t.Errorf("expected empty ChapterNumber for out-of-range index, got %q", msg.ChapterNumber)
	}
}

func TestChapterNavCmdNilNumbersNoPanic(t *testing.T) {
	cmd := chapterNavCmd("c2", "m1", "Title", []string{"c1", "c2"}, nil, 1, 0)
	msg := cmd().(ViewChapterMsg)
	if msg.ChapterNumber != "" {
		t.Errorf("expected empty ChapterNumber with nil numbers, got %q", msg.ChapterNumber)
	}
}

func TestApplyPreloadedChapterUpdatesChapterNumber(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "101", []string{"c1", "c2", "c3"}, 0, -1, nil)
	m.allChapterNumbers = []string{"101", "102", "103"}
	m.preloadedChapID = "c2"
	m.preloadedURLs = []string{"u1", "u2"}
	m.preloadedImages = [][]byte{{1}, {2}}

	m.applyPreloadedChapter("c2")

	if m.chapterNumber != "102" {
		t.Errorf("expected chapterNumber 102 after preload apply, got %q", m.chapterNumber)
	}
	if m.chapterIndex != 1 {
		t.Errorf("expected chapterIndex 1, got %d", m.chapterIndex)
	}
	if m.chapterID != "c2" {
		t.Errorf("expected chapterID c2, got %q", m.chapterID)
	}
}

func TestApplyPreloadedChapterNilNumbersKeepsOld(t *testing.T) {
	m := NewReaderModel("m1", "Title", "c1", "101", []string{"c1", "c2"}, 0, -1, nil)
	m.preloadedChapID = "c2"
	m.preloadedURLs = []string{"u1"}
	m.preloadedImages = [][]byte{{1}}

	m.applyPreloadedChapter("c2")

	if m.chapterNumber != "101" {
		t.Errorf("expected chapterNumber unchanged with nil numbers, got %q", m.chapterNumber)
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

	view := m.View()
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
