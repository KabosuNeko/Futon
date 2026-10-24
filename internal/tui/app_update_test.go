package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestUpdateFailureReturnsToSearchWithError(t *testing.T) {
	m := NewAppModel("dev")
	m.state = stateUpdating

	updated, cmd := m.Update(UpdateReadyMsg{Err: errors.New("boom")})
	app := updated.(AppModel)

	if app.state != stateSearch {
		t.Errorf("state = %v, want stateSearch after a failed update", app.state)
	}
	if !strings.Contains(app.search.systemMsg, "boom") {
		t.Errorf("systemMsg = %q, want the update error", app.search.systemMsg)
	}
	if cmd != nil {
		t.Error("a failed update must not quit the program")
	}
	if strings.Contains(app.View().Content, "Cập nhật thành công") {
		t.Error("failed update must not report success")
	}
}

func TestUpdateSuccessQuits(t *testing.T) {
	m := NewAppModel("dev")
	m.state = stateUpdating

	_, cmd := m.Update(UpdateReadyMsg{})
	if cmd == nil {
		t.Fatal("successful update must quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("cmd() = %T, want tea.QuitMsg", cmd())
	}
}
