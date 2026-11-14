package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/storage"
)

func TestSourceToggleSavesInCommand(t *testing.T) {
	m := testSearchModel()
	m.view = viewSources
	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if updated.(SearchModel).providerToggles[0] {
		t.Fatal("space did not toggle the source")
	}
	if cmd == nil {
		t.Fatal("toggle must return a command that persists the change (no disk I/O in Update)")
	}
}

func TestSavedSourcesAllRemovedEnablesEverything(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := storage.SaveSources([]string{"OTruyen"}); err != nil {
		t.Fatal(err)
	}
	toggles := loadSourceToggles([]api.MangaProvider{api.NewMangaDexProvider(), api.NewBaoTangTruyenProvider()})
	for i, on := range toggles {
		if !on {
			t.Errorf("source %d disabled; a saved list of only removed sources must enable all", i)
		}
	}
}

func TestSourcesTabUsableWhileFeedHasError(t *testing.T) {
	m := testSearchModel()
	m.width, m.height = 120, 30
	m.err = errors.New("MangaDex: không kết nối được api.mangadex.org: connection refused")

	m, _ = m.switchView(viewSources)
	m.err = errors.New("feed failed again") // a late feed snapshot
	view := m.View().Content
	if strings.Contains(view, "Lỗi:") {
		t.Errorf("feed error hides the sources tab:\n%s", view)
	}
	if !strings.Contains(view, "MangaDex") || !strings.Contains(view, "BaoTangTruyen") {
		t.Errorf("sources list not shown:\n%s", view)
	}
}

func TestLegacySavedSourcesLongerThanProvidersAreKept(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// An old list: every source but MangaDex, including ones since removed.
	if err := storage.SaveSources([]string{"OTruyen", "TruyenGG", "FoxTruyen", "TruyenQQ", "BaoTangTruyen"}); err != nil {
		t.Fatal(err)
	}
	providers := []api.MangaProvider{api.NewMangaDexProvider(), api.NewTruyenQQProvider(), api.NewBaoTangTruyenProvider()}
	if got := loadSourceToggles(providers); got[0] || !got[1] || !got[2] {
		t.Errorf("toggles = %v, want MangaDex off as saved", got)
	}
}
