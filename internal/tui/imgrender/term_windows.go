//go:build windows

package imgrender

import "errors"

type TerminalSize struct {
	Cols int
	Rows int
	PxW  int
	PxH  int
}

func GetTerminalSize() (TerminalSize, error) {
	return TerminalSize{}, errors.New("GetTerminalSize not implemented on Windows")
}
