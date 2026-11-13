package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/KabosuNeko/Futon/internal/models"
)

// pageProvider serves one page per chapter from base; chapter "bad" fails.
type pageProvider struct {
	pagedProvider
	base string
}

func (p pageProvider) FetchPages(_ context.Context, chapterID string) ([]string, error) {
	return []string{p.base + "/" + chapterID + ".jpg"}, nil
}

// drainBatchExport follows a batch export to its final message.
func drainBatchExport(t *testing.T, cmd tea.Cmd) []batchExportMsg {
	t.Helper()
	var msgs []batchExportMsg
	for cmd != nil {
		msg, ok := cmd().(batchExportMsg)
		if !ok {
			t.Fatalf("unexpected message %T", msg)
		}
		msgs = append(msgs, msg)
		cmd = msg.next
	}
	return msgs
}

func TestExportAllChaptersWritesEachChapterAndReports(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	jpg := []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "bad") {
			http.NotFound(w, r)
			return
		}
		w.Write(jpg)
	}))
	defer srv.Close()

	chapters := []models.Chapter{{ID: "c1", Number: "1"}, {ID: "bad", Number: "2"}, {ID: "c3", Number: "3"}}
	provider := pageProvider{pagedProvider{name: "P"}, srv.URL}
	msgs := drainBatchExport(t, exportAllChaptersCmd(context.Background(), "m1", provider, "Bộ Truyện", chapters))

	if len(msgs) != 4 {
		t.Fatalf("messages = %d, want one per chapter + final", len(msgs))
	}
	if msgs[0].done != 1 || len(msgs[1].failed) != 1 || msgs[2].done != 2 {
		t.Errorf("progress = %+v", msgs[:3])
	}
	if len(msgs[0].failed) != 0 {
		t.Error("an earlier progress message was changed by a later failure")
	}
	final := msgs[3]
	if !final.final || final.done != 2 || strings.Join(final.failed, ",") != "2" || final.canceled {
		t.Fatalf("final = %+v", final)
	}
	for _, name := range []string{"Bộ Truyện_Chap_1.cbz", "Bộ Truyện_Chap_3.cbz"} {
		if _, err := os.Stat(filepath.Join(final.destDir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
	if filepath.Base(final.destDir) != "Bộ Truyện" {
		t.Errorf("destDir = %s, want a folder per manga", final.destDir)
	}
}

func TestExportAllChaptersStopsWhenCancelled(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	chapters := []models.Chapter{{ID: "c1", Number: "1"}, {ID: "c2", Number: "2"}}
	msgs := drainBatchExport(t, exportAllChaptersCmd(ctx, "m1", pageProvider{pagedProvider{name: "P"}, "http://127.0.0.1:1"}, "T", chapters))
	final := msgs[len(msgs)-1]
	if !final.canceled || final.done != 0 {
		t.Errorf("final = %+v, want cancelled with nothing exported", final)
	}
}

func TestBatchExportProgressRoutesToChapterWhileReading(t *testing.T) {
	app := NewAppModel("dev")
	app.chapter = NewChapterListModel("m1", "Title", nil)
	app.chapter.cancelExport = func() {}
	app.state = stateReader

	updated, _ := app.Update(batchExportMsg{mangaID: "m1", done: 3, total: 3, final: true, destDir: "/x"})
	app = updated.(AppModel)
	if app.chapter.cancelExport != nil {
		t.Error("final export message did not reach the chapter list")
	}
	if !strings.Contains(app.chapter.flashMsg, "3/3") {
		t.Errorf("flash = %q", app.chapter.flashMsg)
	}
}

func TestChapterEscCancelsRunningExport(t *testing.T) {
	m := NewChapterListModel("m1", "Title", nil)
	canceled := false
	m.cancelExport = func() { canceled = true }

	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	cm := updated.(ChapterListModel)
	if !canceled || cm.cancelExport != nil {
		t.Fatal("esc did not cancel the export")
	}
	if cmd != nil {
		if _, back := cmd().(BackToSearchMsg); back {
			t.Error("esc left the screen instead of only cancelling the export")
		}
	}
}

func TestBatchExportOfAnotherMangaKeepsDraining(t *testing.T) {
	m := NewChapterListModel("other", "Other", nil)
	next := func() tea.Msg { return nil }
	_, cmd := m.Update(batchExportMsg{mangaID: "m1", next: next})
	if cmd == nil {
		t.Error("progress of a manga no longer shown must still be drained")
	}
}
