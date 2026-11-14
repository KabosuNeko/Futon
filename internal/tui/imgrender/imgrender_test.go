package imgrender

import (
	"bytes"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestNewPrefersEnvironmentOverride(t *testing.T) {
	t.Setenv("FUTON_RENDERER", "sixel")
	if _, ok := New().(sixelRenderer); !ok {
		t.Fatalf("FUTON_RENDERER=sixel did not select the sixel renderer")
	}

	t.Setenv("FUTON_RENDERER", "kitty")
	if _, ok := New().(kittyRenderer); !ok {
		t.Fatalf("FUTON_RENDERER=kitty did not select the kitty renderer")
	}
}

func TestDecodeAndScaleInBoxPreservesAspectRatio(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 40; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 1, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}

	scaled, err := decodeAndScaleInBox(buf.Bytes(), 10, 10)
	if err != nil {
		t.Fatalf("decodeAndScaleInBox error: %v", err)
	}
	b := scaled.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		t.Fatalf("scaled image is empty: %v", b)
	}
	if got := float64(b.Dx()) / float64(b.Dy()); got < 1.9 || got > 2.1 {
		t.Errorf("aspect ratio = %.2f, want ~2", got)
	}
}

func TestScaleImageInBoxFillsRequestedHeight(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 900, 3001))
	ts := TerminalSize{Cols: 80, Rows: 24, PxW: 640, PxH: 384}

	scaled := scaleImageInBox(img, ts, 10, 10)
	b := scaled.Bounds()
	if b.Dy() != 160 {
		t.Errorf("scaled height = %d, want 160 (10 rows * 16px)", b.Dy())
	}
	if b.Dx() <= 0 || b.Dx() > 80 {
		t.Errorf("scaled width = %d, want within 80px", b.Dx())
	}
}

func TestScaleImageInBoxUsesExactTerminalHeight(t *testing.T) {
	// Pixel height is not divisible by the row count; a floored cell height
	// would leave the remainder as a strip at the bottom.
	img := image.NewRGBA(image.Rect(0, 0, 900, 3001))
	ts := TerminalSize{Cols: 80, Rows: 33, PxW: 800, PxH: 1035}

	scaled := scaleImageInBox(img, ts, 0, 33)
	if got := scaled.Bounds().Dy(); got != 1035 {
		t.Errorf("scaled height = %d, want full terminal height 1035", got)
	}
}

func TestSplitChunks(t *testing.T) {
	chunks := splitChunks("abcdef", 4)
	if len(chunks) != 2 || chunks[0] != "abcd" || chunks[1] != "ef" {
		t.Errorf("splitChunks = %v, want [abcd ef]", chunks)
	}
	if got := splitChunks("", 4); got != nil {
		t.Errorf("splitChunks(empty) = %v, want nil", got)
	}
}

func TestKittyChunkedPayloadMarksLastChunk(t *testing.T) {
	r := kittyRenderer{}
	payload := r.chunkedPayload(10, 20, strings.Repeat("A", kittyChunkSize*2+1))

	if !strings.Contains(payload, "s=10,v=20") {
		t.Errorf("first chunk missing pixel size header")
	}
	if got := strings.Count(payload, "m=1"); got != 2 {
		t.Errorf("continuation chunks = %d, want 2", got)
	}
	if got := strings.Count(payload, "m=0"); got != 1 {
		t.Errorf("final chunks = %d, want 1", got)
	}
	if !strings.HasSuffix(payload, kittyST) {
		t.Errorf("payload does not end with the kitty terminator")
	}
}

func TestNewDoesNotTreatStAsKitty(t *testing.T) {
	t.Setenv("FUTON_RENDERER", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("TERM", "st-256color")
	if _, ok := New().(sixelRenderer); !ok {
		t.Error("TERM=st-256color must use Sixel; plain st has no Kitty graphics")
	}
}

func TestNewPicksKittyForKittyCapableTerminals(t *testing.T) {
	for _, env := range []map[string]string{
		{"TERM": "xterm-kitty"},
		{"TERM": "xterm-ghostty"},
		{"TERM": "xterm-256color", "TERM_PROGRAM": "ghostty"},
		{"TERM": "xterm-256color", "TERM_PROGRAM": "WezTerm"},
	} {
		t.Setenv("FUTON_RENDERER", "")
		t.Setenv("KITTY_WINDOW_ID", "")
		t.Setenv("TERM_PROGRAM", "")
		for k, v := range env {
			t.Setenv(k, v)
		}
		if _, ok := New().(kittyRenderer); !ok {
			t.Errorf("%v: want the Kitty renderer", env)
		}
	}

	t.Setenv("TERM", "foot")
	t.Setenv("TERM_PROGRAM", "")
	if _, ok := New().(sixelRenderer); !ok {
		t.Error("foot must use Sixel")
	}
}

func TestScaleToWidthNeverUpscales(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 900, 5000))
	wide := TerminalSize{Cols: 200, Rows: 50, PxW: 1600, PxH: 800}
	if got := scaleToWidth(img, wide, 0).Bounds(); got.Dx() != 900 || got.Dy() != 5000 {
		t.Errorf("wide terminal: %v, want native 900x5000", got)
	}
	narrow := TerminalSize{Cols: 50, Rows: 40, PxW: 400, PxH: 640}
	if got := scaleToWidth(img, narrow, 0).Bounds(); got.Dx() != 400 || got.Dy() != 2222 {
		t.Errorf("narrow terminal: %v, want 400x2222", got)
	}
}

func TestWindowClampsToBottom(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 100)) // 100px = 10 rows of 10px
	img.Set(0, 95, color.RGBA{R: 255, A: 255})

	w, top, total := window(img, 0, 4, 10)
	if total != 10 || top != 0 || w.Bounds().Dy() != 40 {
		t.Errorf("top window: top=%d total=%d h=%d", top, total, w.Bounds().Dy())
	}

	w, top, _ = window(img, 99, 4, 10)
	if top != 6 || w.Bounds().Dy() != 40 {
		t.Fatalf("past the end: top=%d h=%d, want clamped to 6 and a full window", top, w.Bounds().Dy())
	}
	if r, _, _, _ := w.At(0, 35).RGBA(); r == 0 {
		t.Error("window content is not the bottom of the page")
	}
	if w.Bounds().Min != (image.Point{}) {
		t.Error("window must start at the origin")
	}
}

func TestScaleToWidthSkipsSameSizeScale(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 900, 5000))
	wide := TerminalSize{Cols: 200, Rows: 50, PxW: 1600, PxH: 800}
	if scaleToWidth(img, wide, 0) != image.Image(img) {
		t.Error("a page that already fits is scaled anyway")
	}
}

func TestDecodeRejectsDecompressionBomb(t *testing.T) {
	// A PNG header claiming 60000x60000 RGBA: decoding it would allocate 14 GB.
	ihdr := []byte("IHDR\x00\x00\xea\x60\x00\x00\xea\x60\x08\x06\x00\x00\x00")
	crc := crc32.ChecksumIEEE(ihdr)
	data := append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0d"), ihdr...)
	data = append(data, byte(crc>>24), byte(crc>>16), byte(crc>>8), byte(crc))

	if _, err := ScaleToWidth(data, 80); err == nil || !strings.Contains(err.Error(), "quá lớn") {
		t.Fatalf("ScaleToWidth err = %v, want too large", err)
	}
	if _, err := decodeAndScaleInBox(data, 80, 24); err == nil || !strings.Contains(err.Error(), "quá lớn") {
		t.Fatalf("decodeAndScaleInBox err = %v, want too large", err)
	}
}
