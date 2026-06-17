package imgrender

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"strconv"
	"strings"
)

const kittyChunkSize = 2048

var (
	kittyEsc   = string([]byte{0x1b})
	kittyST    = kittyEsc + "\\"
	kittyStart = kittyEsc + "_G"
)

type kittyRenderer struct{}

func (r kittyRenderer) Render(imgData []byte, terminalWidth int) (RenderedImage, error) {
	return r.RenderCapped(imgData, 0)
}

func (r kittyRenderer) RenderCapped(imgData []byte, maxWidthPx int) (RenderedImage, error) {
	img, err := decodeAndScale(imgData, maxWidthPx)
	if err != nil {
		return RenderedImage{}, err
	}

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return RenderedImage{}, err
	}

	b64 := base64.StdEncoding.EncodeToString(pngBuf.Bytes())
	b64 = strings.ReplaceAll(b64, "\n", "")
	b64 = strings.ReplaceAll(b64, "\r", "")

	bounds := img.Bounds()
	return RenderedImage{
		EscapeSequence: r.chunkedPayload(bounds.Dx(), bounds.Dy(), b64),
		WidthPx:        bounds.Dx(),
		HeightPx:       bounds.Dy(),
	}, nil
}

func (r kittyRenderer) chunkedPayload(w, h int, b64 string) string {
	if len(b64) <= kittyChunkSize {
		return r.chunk(w, h, b64, "0")
	}
	var sb strings.Builder
	chunks := splitChunks(b64, kittyChunkSize)
	for i, chunk := range chunks {
		m := "1"
		if i == len(chunks)-1 {
			m = "0"
		}
		if i == 0 {
			sb.WriteString(r.chunk(w, h, chunk, m))
		} else {
			sb.WriteString(r.continuationChunk(chunk, m))
		}
	}
	return sb.String()
}

func (r kittyRenderer) chunk(w, h int, data, m string) string {
	return kittyStart + "a=T,f=100,s=" + strconv.Itoa(w) + ",v=" + strconv.Itoa(h) + ",m=" + m + ";" + data + kittyST
}

func (r kittyRenderer) continuationChunk(data, m string) string {
	return kittyStart + "m=" + m + ";" + data + kittyST
}

func splitChunks(s string, chunkSize int) []string {
	if len(s) == 0 {
		return nil
	}
	var chunks []string
	for i := 0; i < len(s); i += chunkSize {
		end := i + chunkSize
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	return chunks
}
