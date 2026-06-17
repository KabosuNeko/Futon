package imgrender

import (
	"bytes"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"github.com/nfnt/resize"
)

func decodeAndScale(imgData []byte, maxWidthPx int) (image.Image, error) {
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

	targetW := uint(ts.PxW)
	if maxWidthPx > 0 && int(targetW) > maxWidthPx {
		targetW = uint(maxWidthPx)
	}
	scaledH := int(targetW) * imgH / imgW
	if scaledH > ts.PxH {
		targetW = uint(ts.PxH * imgW / imgH)
	}

	return resize.Resize(targetW, 0, img, resize.Lanczos3), nil
}
