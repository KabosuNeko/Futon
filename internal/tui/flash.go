package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type clearFlashMsg struct{}

func clearFlashAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return clearFlashMsg{}
	})
}
