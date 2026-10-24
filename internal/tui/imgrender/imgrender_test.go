package imgrender

import (
	"bytes"
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
