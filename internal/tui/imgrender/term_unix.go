//go:build !windows

package imgrender

import (
	"os"

	"golang.org/x/sys/unix"
)

type TerminalSize struct {
	Cols int
	Rows int
	PxW  int
	PxH  int
}

func GetTerminalSize() (TerminalSize, error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return TerminalSize{}, err
	}
	return TerminalSize{
		Cols: int(ws.Col),
		Rows: int(ws.Row),
		PxW:  int(ws.Xpixel),
		PxH:  int(ws.Ypixel),
	}, nil
}
