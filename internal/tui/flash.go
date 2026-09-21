package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

type clearFlashMsg struct{}

func clearFlashAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return clearFlashMsg{}
	})
}
