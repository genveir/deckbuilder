package ui

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

type fontFace struct {
	face   *text.GoXFace
	ascent int // pixels to add to convert top-left y to baseline y
}

var (
	faceSmall fontFace
	faceBody  fontFace
	faceTitle fontFace
)

func init() {
	tt, err := opentype.Parse(goregular.TTF)
	if err != nil {
		panic(err)
	}
	newFace := func(size float64) fontFace {
		f, err := opentype.NewFace(tt, &opentype.FaceOptions{
			Size:    size,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err != nil {
			panic(err)
		}
		m := f.Metrics()
		return fontFace{
			face:   text.NewGoXFace(f),
			ascent: int(m.Ascent>>6) + 1,
		}
	}
	faceSmall = newFace(11)
	faceBody = newFace(13)
	faceTitle = newFace(17)
}

// drawText draws a single line at pixel position (x, y) with top-left origin.
func drawText(screen *ebiten.Image, s string, x, y int, ff fontFace, clr color.Color) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(x), float64(y+ff.ascent))
	op.ColorScale.ScaleWithColor(clr)
	text.Draw(screen, s, ff.face, op)
}

// textWidth returns the pixel width of s in the given face.
func textWidth(s string, ff fontFace) float64 {
	return text.Advance(s, ff.face)
}

// wrapText splits s into lines that each fit within maxW pixels.
func wrapText(s string, maxW float64, ff fontFace) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := words[0]
	for _, w := range words[1:] {
		candidate := current + " " + w
		if text.Advance(candidate, ff.face) <= maxW {
			current = candidate
		} else {
			lines = append(lines, current)
			current = w
		}
	}
	return append(lines, current)
}

// drawWrapped draws text word-wrapped at maxW, advancing y by lineH per line.
// Returns the y after the last line.
func drawWrapped(screen *ebiten.Image, s string, x, y int, maxW float64, lineH int, ff fontFace, clr color.Color) int {
	for _, line := range wrapText(s, maxW, ff) {
		drawText(screen, line, x, y, ff, clr)
		y += lineH
	}
	return y
}

// centerText draws s horizontally centred within [x, x+w].
func centerText(screen *ebiten.Image, s string, x, y, w int, ff fontFace, clr color.Color) {
	adv := text.Advance(s, ff.face)
	ox := x + int(float64(w)/2-adv/2)
	drawText(screen, s, ox, y, ff, clr)
}
