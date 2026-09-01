package imgrender

import (
	"bytes"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"github.com/nfnt/resize"
)

func decodeAndScale(imgData []byte) (image.Image, error) {
	return decodeAndScaleInBox(imgData, 0, 0)
}

func decodeAndScaleInBox(imgData []byte, targetCols, targetRows int) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	imgW := bounds.Dx()
	imgH := bounds.Dy()

	if imgW <= 0 || imgH <= 0 {
		return img, nil
	}

	ts, err := GetTerminalSize()
	if err != nil {
		ts = TerminalSize{Cols: 80, Rows: 24, PxW: 80 * 8, PxH: 24 * 16}
	}
	if ts.Cols <= 0 {
		ts.Cols = 80
	}
	if ts.Rows <= 0 {
		ts.Rows = 24
	}
	if ts.PxW <= 0 {
		ts.PxW = ts.Cols * 8
	}
	if ts.PxH <= 0 {
		ts.PxH = ts.Rows * 16
	}

	cellW := max(1, ts.PxW/ts.Cols)
	cellH := max(1, ts.PxH/ts.Rows)

	if targetCols <= 0 {
		targetCols = ts.Cols
	}
	if targetRows <= 0 {
		targetRows = ts.Rows - 1
	}

	maxPxW := targetCols * cellW
	maxPxH := targetRows * cellH

	targetW := uint(maxPxW)
	scaledH := int(targetW) * imgH / imgW
	if scaledH > maxPxH {
		targetW = uint(maxPxH * imgW / imgH)
	}

	if targetW == 0 {
		targetW = 1
	}

	return resize.Resize(targetW, 0, img, resize.Lanczos3), nil
}
