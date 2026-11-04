package imgrender

import (
	"image"
	"os"
)

type RenderedImage struct {
	EscapeSequence string
	WidthPx        int
	HeightPx       int
}

type Renderer interface {
	RenderInBox(imgData []byte, cols, rows int) (RenderedImage, error)
	RenderImage(img image.Image) (RenderedImage, error)
}

// New picks the renderer: FUTON_RENDERER=kitty|sixel overrides detection;
// Kitty for terminals known to implement its graphics protocol (kitty,
// Ghostty, WezTerm), since it is faster; Sixel otherwise.
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
	// Ghostty has no Sixel at all; WezTerm has both and Kitty is faster.
	switch os.Getenv("TERM_PROGRAM") {
	case "ghostty", "WezTerm":
		return kittyRenderer{}
	}
	if term == "xterm-ghostty" {
		return kittyRenderer{}
	}

	return sixelRenderer{}
}
