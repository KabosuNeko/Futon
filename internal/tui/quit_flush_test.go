package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/KabosuNeko/Futon/internal/storage"
	tea "github.com/charmbracelet/bubbletea"
)

// execCmd simulates the bubbletea runtime: nested commands returned by
// tea.Sequence/tea.Batch are executed in order so their side effects run.
func execCmd(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if msg == nil {
		return
	}
	v := reflect.ValueOf(msg)
	if v.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			if c, ok := v.Index(i).Interface().(tea.Cmd); ok {
				execCmd(c)
			}
		}
	}
}

func TestCtrlCInReaderFlushesHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const mangaID = "quit-flush-test-1"

	m := NewAppModel("dev")
	m.state = stateReader
	m.reader = NewReaderModel(mangaID, "Title", "c1", "1", []string{"c1", "c2"}, 0, -1, nil)
	m.reader.currentIdx = 3

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected a cmd for ctrl+c in reader")
	}
	if _, ok := model.(AppModel); !ok {
		t.Fatalf("expected AppModel back, got %T", model)
	}

	execCmd(cmd)

	h, ok := storage.GetHistory(mangaID)
	if !ok {
		t.Fatal("expected history saved before quit")
	}
	if h.ChapterID != "c1" || h.PageIndex != 3 {
		t.Errorf("expected history c1/page 3, got %s/%d", h.ChapterID, h.PageIndex)
	}
}

func TestCtrlCInSearchQuits(t *testing.T) {
	m := NewAppModel("dev")
	m.state = stateSearch

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit cmd in search state")
	}
	if got := cmd(); got == nil {
		t.Fatal("expected a msg from quit")
	}
}

func TestReaderKeysCtrlCFlushesHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const mangaID = "quit-flush-test-2"

	m := NewReaderModel(mangaID, "Title", "c1", "1", []string{"c1", "c2"}, 0, -1, nil)
	m.currentIdx = 2

	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected a cmd for ctrl+c")
	}
	if _, ok := model.(ReaderModel); !ok {
		t.Fatalf("expected ReaderModel back, got %T", model)
	}

	execCmd(cmd)

	h, ok := storage.GetHistory(mangaID)
	if !ok {
		t.Fatal("expected history saved before quit")
	}
	if h.ChapterID != "c1" || h.PageIndex != 2 {
		t.Errorf("expected history c1/page 2, got %s/%d", h.ChapterID, h.PageIndex)
	}
}

func TestReaderKeysCtrlCNoMangaQuitsDirectly(t *testing.T) {
	m := NewReaderModel("", "Title", "", "1", nil, 0, -1, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit cmd")
	}
}

func TestQueryBannerSaysCtrlU(t *testing.T) {
	m := NewAppModel("dev")
	m.updateAvailable = true
	m.updateVersion = "v9.9.9"
	m.state = stateSearch

	view := m.View()
	if strings.Contains(view, "Nhấn 'U'") {
		t.Error("banner still says 'U'")
	}
	if !strings.Contains(view, "Nhấn Ctrl+u") {
		t.Errorf("banner missing Ctrl+u hint, got:\n%s", view)
	}
}
