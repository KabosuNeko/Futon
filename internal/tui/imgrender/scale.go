package imgrender

import (
	"bytes"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"github.com/nfnt/resize"
)

func decodeAndScaleInBox(imgData []byte, targetCols, targetRows int) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return img, nil
	}

	ts, err := GetTerminalSize()
	if err != nil {
		ts = TerminalSize{Cols: 80, Rows: 24, PxW: 80 * 8, PxH: 24 * 16}
	}
	return scaleImageInBox(img, ts, targetCols, targetRows), nil
}

func scaleImageInBox(img image.Image, ts TerminalSize, targetCols, targetRows int) image.Image {
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

	if targetCols <= 0 {
		targetCols = ts.Cols
	}
	if targetRows <= 0 {
		// A terminal with one row would otherwise make maxPxH negative and
		// underflow the uint width below.
		targetRows = max(1, ts.Rows-2)
	}

	bounds := img.Bounds()
	imgW := bounds.Dx()
	imgH := bounds.Dy()
	if imgW <= 0 || imgH <= 0 {
		return img
	}

	// Use the exact pixel span of the requested cell area. Multiplying a
	// floored cell size would drop the remainder pixels and leave a thin strip
	// at the bottom when the image should fill the terminal.
	maxPxW := targetCols * ts.PxW / ts.Cols
	maxPxH := targetRows * ts.PxH / ts.Rows

	targetW := uint(maxPxW)
	targetH := int(targetW) * imgH / imgW
	if targetH > maxPxH {
		targetH = maxPxH
		targetW = uint(targetH * imgW / imgH)
	}
	if targetW == 0 {
		targetW = 1
	}
	if targetH < 1 {
		targetH = 1
	}

	// Pass both dimensions: letting Resize derive the height can round down a
	// pixel, which drops a whole terminal row and leaves a gap at the bottom.
	return resize.Resize(targetW, uint(targetH), img, resize.Lanczos3)
}
