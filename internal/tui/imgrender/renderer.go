package imgrender

import (
	"os"
)

type RenderedImage struct {
	EscapeSequence string
	WidthPx        int
	HeightPx       int
}

type Renderer interface {
	Render(imgData []byte, terminalWidth int) (RenderedImage, error)
	RenderCapped(imgData []byte, maxWidthPx int) (RenderedImage, error)
}

func New() Renderer {
	if os.Getenv("TERM") == "xterm-kitty" || os.Getenv("KITTY_WINDOW_ID") != "" {
		return kittyRenderer{}
	}
	return sixelRenderer{}
}
