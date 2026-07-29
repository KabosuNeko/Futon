package imgrender

import (
	"os"
	"strings"
)

type RenderedImage struct {
	EscapeSequence string
	WidthPx        int
	HeightPx       int
}

type Renderer interface {
	Render(imgData []byte) (RenderedImage, error)
}

func New() Renderer {
	term := os.Getenv("TERM")

	switch os.Getenv("FUTON_RENDERER") {
	case "kitty":
		return kittyRenderer{}
	case "sixel":
		return sixelRenderer{}
	}

	if term == "xterm-kitty" || os.Getenv("KITTY_WINDOW_ID") != "" {
		return kittyRenderer{}
	}

	if strings.HasPrefix(term, "st-") {
		return kittyRenderer{}
	}

	return sixelRenderer{}
}
