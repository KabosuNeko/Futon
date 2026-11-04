package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/KabosuNeko/Futon/internal/storage"
)

func TestManualUpdateCheckOnlyReports(t *testing.T) {
	m := NewAppModel("v1.0.0")
	m.state = stateSearch

	updated, cmd := m.Update(UpdateCheckedMsg{Available: true, Version: "v9.9.9"})
	app := updated.(AppModel)
	if cmd != nil {
		t.Fatal("an available update must not start installing from the TUI")
	}
	if app.state != stateSearch || !strings.Contains(app.search.systemMsg, "futon update") {
		t.Errorf("state=%v msg=%q, want a hint to run `futon update`", app.state, app.search.systemMsg)
	}
}

func TestCtrlUNoLongerTriggersUpdate(t *testing.T) {
	m := NewAppModel("v1.0.0")
	m.updateAvailable = true
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	app := updated.(AppModel)
	if app.state != stateSearch || app.search.systemMsg != "" {
		t.Errorf("ctrl+u changed state=%v msg=%q; in-app install was removed", app.state, app.search.systemMsg)
	}
}

func TestNewScreensGetKnownTerminalSize(t *testing.T) {
	app := NewAppModel("dev")
	updated, _ := app.Update(tea.WindowSizeMsg{Width: 150, Height: 50})
	app = updated.(AppModel)

	updated, _ = app.Update(ViewMangaMsg{MangaID: "m", Title: "T", ProviderName: "MangaDex"})
	app = updated.(AppModel)
	if app.chapter.width != 150 || app.chapter.height != 50 {
		t.Errorf("chapter list size = %dx%d, want 150x50", app.chapter.width, app.chapter.height)
	}
}

func TestRemovedSourceIsReportedNotRemapped(t *testing.T) {
	app := NewAppModel("dev")
	updated, cmd := app.Update(ViewMangaMsg{MangaID: "one-piece", Title: "One Piece", ProviderName: "OTruyen"})
	app = updated.(AppModel)
	if app.state != stateSearch || cmd != nil {
		t.Fatal("a manga from a removed source must not open with another provider")
	}
	if !strings.Contains(app.search.systemMsg, "OTruyen không còn được hỗ trợ") {
		t.Errorf("systemMsg = %q", app.search.systemMsg)
	}

	// Legacy entries without a provider still open with the first source.
	updated, cmd = app.Update(ViewMangaMsg{MangaID: "x", Title: "X"})
	if cmd == nil {
		t.Error("provider-less legacy entry no longer opens")
	}
	_ = updated
}

func TestMigrateLegacyEntryFoldsTruyenGGIntoTruyenQQ(t *testing.T) {
	cases := []struct{ provider, manga, chapter, wantP, wantM, wantC string }{
		{"TruyenGG", "https://truyenggvn.com/truyen-tranh/futari-bus-22373", "https://truyenggvn.com/truyen-tranh/futari-bus-22373-chap-1.html",
			"TruyenQQ", "/truyen-tranh/futari-bus-22373", "/truyen-tranh/futari-bus-22373-chap-1"},
		{"TruyenQQ", "https://truyenqqko.com/truyen-tranh/x-1", "https://truyenqqko.com/truyen-tranh/x-1-chap-2",
			"TruyenQQ", "/truyen-tranh/x-1", "/truyen-tranh/x-1-chap-2"},
		{"FoxTruyen", "fox-id", "fox-ch", "FoxTruyen", "fox-id", "fox-ch"},
		{"MangaDex", "uuid", "ch-uuid", "MangaDex", "uuid", "ch-uuid"},
	}
	for _, c := range cases {
		p, m, ch := migrateLegacyEntry(c.provider, c.manga, c.chapter)
		if p != c.wantP || m != c.wantM || ch != c.wantC {
			t.Errorf("migrate(%s) = %s %s %s, want %s %s %s", c.provider, p, m, ch, c.wantP, c.wantM, c.wantC)
		}
	}
}

func TestMergedSourceNameOpensSuccessor(t *testing.T) {
	app := NewAppModel("dev")
	if p, ok := app.findProvider("TruyenGG"); !ok || p.Name() != "TruyenQQ" {
		t.Errorf("TruyenGG resolved to %v, %v; want TruyenQQ", p, ok)
	}
}

func TestHistoryEntryOpensStraightIntoReader(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := storage.SaveHistory("m1", "T", "", "c2", "2", 4); err != nil {
		t.Fatal(err)
	}

	m := NewChapterListModel("m1", "T", nil)
	m.resumeChapterID = "c2"
	updated, cmd := m.Update(chaptersLoadedMsg{Chapters: []models.Chapter{{ID: "c1", Number: "1"}, {ID: "c2", Number: "2"}}})
	if cmd == nil {
		t.Fatal("history resume did not open the reader")
	}
	msg, ok := cmd().(ViewChapterMsg)
	if !ok || msg.ChapterID != "c2" || msg.StartPageIndex != -1 {
		t.Fatalf("got %#v, want ViewChapterMsg for c2 resuming the saved page", msg)
	}
	if updated.(ChapterListModel).resumeChapterID != "" {
		t.Error("resume must only happen once (esc from the reader returns to the list)")
	}
}

func TestHistoryResumeFallsBackToListWhenChapterGone(t *testing.T) {
	m := NewChapterListModel("m1", "T", nil)
	m.resumeChapterID = "removed"
	if _, cmd := m.Update(chaptersLoadedMsg{Chapters: []models.Chapter{{ID: "c1"}}}); cmd != nil {
		t.Error("a missing chapter must leave the user on the chapter list")
	}
}
