package imgrender

import (
	"bytes"

	"github.com/mattn/go-sixel"
)

type sixelRenderer struct{}

func (s sixelRenderer) Render(imgData []byte, terminalWidth int) (RenderedImage, error) {
	return s.RenderCapped(imgData, 0)
}

func (s sixelRenderer) RenderCapped(imgData []byte, maxWidthPx int) (RenderedImage, error) {
	img, err := decodeAndScale(imgData, maxWidthPx)
	if err != nil {
		return RenderedImage{}, err
	}

	var buf bytes.Buffer
	if err := sixel.NewEncoder(&buf).Encode(img); err != nil {
		return RenderedImage{}, err
	}

	bounds := img.Bounds()
	return RenderedImage{
		EscapeSequence: buf.String(),
		WidthPx:        bounds.Dx(),
		HeightPx:       bounds.Dy(),
	}, nil
}
