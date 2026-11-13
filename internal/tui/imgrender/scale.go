package imgrender

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
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

	targetW := maxPxW
	targetH := targetW * imgH / imgW
	if targetH > maxPxH {
		targetH = maxPxH
		targetW = targetH * imgW / imgH
	}
	targetW = max(1, targetW)
	targetH = max(1, targetH)

	// Size both dimensions explicitly: deriving the height can round down a
	// pixel, which drops a whole terminal row and leaves a gap at the bottom.
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Src, nil)
	return dst
}

// cellPixels returns the pixel size of one terminal cell (8×16 if unknown).
func cellPixels(ts TerminalSize) (w, h int) {
	w, h = 8, 16
	if ts.Cols > 0 && ts.PxW > 0 {
		w = max(1, ts.PxW/ts.Cols)
	}
	if ts.Rows > 0 && ts.PxH > 0 {
		h = max(1, ts.PxH/ts.Rows)
	}
	return w, h
}

func currentTerminal() TerminalSize {
	ts, err := GetTerminalSize()
	if err != nil || ts.Cols <= 0 || ts.Rows <= 0 {
		return TerminalSize{Cols: 80, Rows: 24, PxW: 80 * 8, PxH: 24 * 16}
	}
	return ts
}

// ScaleToWidth decodes imgData and scales it to the terminal width (cols
// columns, or the whole terminal when cols <= 0) for webtoon reading. It
// never enlarges past the native width: a 900px strip stays 900px wide on a
// wide terminal instead of turning blurry and huge.
func ScaleToWidth(imgData []byte, cols int) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		return nil, err
	}
	return scaleToWidth(img, currentTerminal(), cols), nil
}

func scaleToWidth(img image.Image, ts TerminalSize, cols int) image.Image {
	if cols <= 0 {
		cols = ts.Cols
	}
	cellW, _ := cellPixels(ts)
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return img
	}
	w := min(b.Dx(), cols*cellW)
	if w == b.Dx() {
		return img // already fits: rescaling to the same size costs 4x the decode
	}
	h := max(1, w*b.Dy()/b.Dx())
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}

// Window cuts the part of a scaled page that covers terminal rows
// [top, top+rows). top is clamped so the window never runs past the bottom.
// It returns the window, the clamped top, and the page height in rows.
func Window(img image.Image, top, rows int) (win image.Image, clampedTop, totalRows int) {
	_, cellH := cellPixels(currentTerminal())
	return window(img, top, rows, cellH)
}

func window(img image.Image, top, rows, cellH int) (image.Image, int, int) {
	b := img.Bounds()
	rows = max(1, rows)
	totalRows := (b.Dy() + cellH - 1) / cellH
	top = max(0, min(top, totalRows-rows))
	y0 := b.Min.Y + top*cellH
	y1 := min(b.Max.Y, y0+rows*cellH)
	// Copy into an origin-based image: encoders differ in how they treat a
	// sub-image whose bounds do not start at (0,0).
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), y1-y0))
	draw.Draw(dst, dst.Bounds(), img, image.Pt(b.Min.X, y0), draw.Src)
	return dst, top, totalRows
}
