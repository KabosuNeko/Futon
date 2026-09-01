package imgrender

import (
	"bytes"
	"image"

	"github.com/mattn/go-sixel"
)

type sixelRenderer struct{}

func (s sixelRenderer) Render(imgData []byte) (RenderedImage, error) {
	return s.RenderInBox(imgData, 0, 0)
}

func (s sixelRenderer) RenderInBox(imgData []byte, cols, rows int) (RenderedImage, error) {
	img, err := decodeAndScaleInBox(imgData, cols, rows)
	if err != nil {
		return RenderedImage{}, err
	}
	return s.RenderImage(img)
}

func (s sixelRenderer) RenderImage(img image.Image) (RenderedImage, error) {
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
