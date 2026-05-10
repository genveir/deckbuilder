package ui

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"deckbuilder/combat"
)

const (
	ScreenW = 1280
	ScreenH = 800

	RadarCX, RadarCY = 480, 360
	RadarRadius      = 320

	HandY      = 640
	CardW      = 110
	CardH      = 150
	CardGap    = 12
	CardStartX = 80

	EndTurnX, EndTurnY  = 1100, 660
	EndTurnW, EndTurnH = 140, 60
)

var (
	bgColor      = color.RGBA{18, 18, 28, 255}
	radarColor   = color.RGBA{40, 50, 80, 255}
	radarRing    = color.RGBA{70, 90, 130, 255}
	playerColor  = color.RGBA{120, 220, 255, 255}
	enemyColor   = color.RGBA{230, 90, 90, 255}
	enemyDeadCol = color.RGBA{60, 60, 60, 255}
	cardBg       = color.RGBA{50, 40, 70, 255}
	cardBgHi     = color.RGBA{90, 70, 130, 255}
	cardBgDim    = color.RGBA{30, 25, 40, 255}
	white        = color.RGBA{240, 240, 240, 255}
	yellow       = color.RGBA{240, 220, 100, 255}
	green        = color.RGBA{120, 220, 120, 255}
)

func Draw(screen *ebiten.Image, c *combat.Combat) {
	screen.Fill(bgColor)
	drawRadar(screen, c)
	drawHand(screen, c)
	drawHUD(screen, c)
	drawEndTurn(screen, c)
	drawPhaseBanner(screen, c)
}

func drawRadar(screen *ebiten.Image, c *combat.Combat) {
	vector.DrawFilledCircle(screen, RadarCX, RadarCY, RadarRadius, radarColor, true)
	for _, r := range []float32{RadarRadius * 0.33, RadarRadius * 0.66, RadarRadius} {
		vector.StrokeCircle(screen, RadarCX, RadarCY, r, 1, radarRing, true)
	}
	// player dot
	vector.DrawFilledCircle(screen, RadarCX, RadarCY, 8, playerColor, true)

	for _, e := range c.Enemies {
		ex := float32(RadarCX + e.X)
		ey := float32(RadarCY + e.Y)
		col := enemyColor
		if e.HP <= 0 {
			col = enemyDeadCol
		}
		vector.DrawFilledCircle(screen, ex, ey, 10, col, true)
		label := fmt.Sprintf("%s %d/%d", e.Name, e.HP, e.MaxHP)
		ebitenutil.DebugPrintAt(screen, label, int(ex)-30, int(ey)+14)
		if e.HP > 0 && e.Intent != "" {
			ebitenutil.DebugPrintAt(screen, "intent: "+e.Intent, int(ex)-30, int(ey)+28)
		}
	}
}

func drawHand(screen *ebiten.Image, c *combat.Combat) {
	mx, my := ebiten.CursorPosition()
	for i, card := range c.Hand {
		x, y := cardRect(i)
		bg := cardBg
		if card.Cost > c.Energy {
			bg = cardBgDim
		} else if mx >= x && mx < x+CardW && my >= y && my < y+CardH {
			bg = cardBgHi
		}
		vector.DrawFilledRect(screen, float32(x), float32(y), CardW, CardH, bg, true)
		ebitenutil.DebugPrintAt(screen, card.Glyph, x+8, y+6)
		ebitenutil.DebugPrintAt(screen, card.Name, x+8, y+24)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Cost: %d", card.Cost), x+8, y+CardH-22)
	}
}

func cardRect(i int) (int, int) {
	return CardStartX + i*(CardW+CardGap), HandY
}

// HitCard returns the index of the card at (mx,my), or -1.
func HitCard(c *combat.Combat, mx, my int) int {
	for i := range c.Hand {
		x, y := cardRect(i)
		if mx >= x && mx < x+CardW && my >= y && my < y+CardH {
			return i
		}
	}
	return -1
}

// HitRadar returns radar-relative coordinates if (mx,my) is inside the radar.
func HitRadar(mx, my int) (rx, ry float64, ok bool) {
	dx := float64(mx - RadarCX)
	dy := float64(my - RadarCY)
	if math.Hypot(dx, dy) > RadarRadius {
		return 0, 0, false
	}
	return dx, dy, true
}

func HitEndTurn(mx, my int) bool {
	return mx >= EndTurnX && mx < EndTurnX+EndTurnW &&
		my >= EndTurnY && my < EndTurnY+EndTurnH
}

func drawHUD(screen *ebiten.Image, c *combat.Combat) {
	lines := []string{
		fmt.Sprintf("HP: %d/%d", c.PlayerHP, c.PlayerMaxHP),
		fmt.Sprintf("Armor: %d", c.PlayerArmor),
		fmt.Sprintf("Energy: %d/%d", c.Energy, c.MaxEnergy),
		fmt.Sprintf("Move: %.0f", c.MovementBudget),
		fmt.Sprintf("Deck: %d  Discard: %d", len(c.Draw), len(c.Discard)),
	}
	for i, l := range lines {
		ebitenutil.DebugPrintAt(screen, l, 880, 40+i*18)
	}
	ebitenutil.DebugPrintAt(screen, "Click card: play   |   Click radar: move   |   E or button: end turn", 40, 8)
}

func drawEndTurn(screen *ebiten.Image, c *combat.Combat) {
	col := cardBg
	if c.Phase != 0 { // not player turn
		col = cardBgDim
	}
	vector.DrawFilledRect(screen, EndTurnX, EndTurnY, EndTurnW, EndTurnH, col, true)
	ebitenutil.DebugPrintAt(screen, "END TURN", EndTurnX+30, EndTurnY+24)
}

func drawPhaseBanner(screen *ebiten.Image, c *combat.Combat) {
	switch c.Phase {
	case 2:
		ebitenutil.DebugPrintAt(screen, "VICTORY — close the window", 540, 380)
	case 3:
		ebitenutil.DebugPrintAt(screen, "DEFEAT — close the window", 540, 380)
	case 1:
		ebitenutil.DebugPrintAt(screen, "(enemy turn)", 560, 700)
	}
	_ = green
	_ = white
	_ = yellow
}
