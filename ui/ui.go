package ui

import (
	"fmt"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"deckbuilder/combat"
	"deckbuilder/runes"
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

	// Reward screen layout
	RewardCardW    = 220
	RewardCardH    = 300
	RewardGap      = 40
	RewardTopY     = 220
	SkipBtnW, SkipBtnH = 220, 50
)

// RunView is the rendering surface — what game/Run exposes to the UI without
// the UI depending on the game package.
type RunView struct {
	Phase           int
	Combat          *combat.Combat
	Rewards         []runes.Card
	EncounterIdx    int
	TotalEncounters int
	PlayerHP, MaxHP int
	DeckSize        int
}

// Run phase constants kept in sync with game.RunPhase.
const (
	phaseInCombat = 0
	phaseReward   = 1
	phaseRunWon   = 2
	phaseRunLost  = 3
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
	moveOkCol    = color.RGBA{120, 220, 140, 220}
	moveOverCol  = color.RGBA{230, 160, 90, 220}
	moveRingCol  = color.RGBA{120, 220, 140, 90}
	tooltipBg    = color.RGBA{20, 20, 32, 240}
	tooltipEdge  = color.RGBA{120, 130, 170, 255}

	fireCol     = color.RGBA{255, 150, 70, 255}
	frostCol    = color.RGBA{120, 200, 255, 255}
	physicalCol = color.RGBA{230, 230, 230, 255}
)

func damageTypeColor(t runes.DamageType) color.RGBA {
	switch t {
	case runes.Fire:
		return fireCol
	case runes.Frost:
		return frostCol
	default:
		return physicalCol
	}
}

func DrawRun(screen *ebiten.Image, v RunView) {
	screen.Fill(bgColor)
	switch v.Phase {
	case phaseInCombat:
		drawCombatScreen(screen, v)
	case phaseReward:
		drawCombatScreen(screen, v) // backdrop
		drawRewardOverlay(screen, v)
	case phaseRunWon:
		drawCombatScreen(screen, v)
		drawEndOverlay(screen, v, true)
	case phaseRunLost:
		drawCombatScreen(screen, v)
		drawEndOverlay(screen, v, false)
	}
}

func drawCombatScreen(screen *ebiten.Image, v RunView) {
	c := v.Combat
	drawRadar(screen, c)
	drawPopups(screen, c)
	drawHand(screen, c)
	drawHUD(screen, v)
	drawEndTurn(screen, c)
	drawCardTooltip(screen, c)
	drawPhaseBanner(screen, c)
}

// drawColoredText renders text via DebugPrint into an offscreen image and
// composites it with a color scale. Allocates per call; fine for small,
// short-lived bits of text like damage popups.
func drawColoredText(screen *ebiten.Image, s string, x, y int, c color.RGBA, alpha float32) {
	w := len(s)*7 + 4
	if w < 8 {
		w = 8
	}
	img := ebiten.NewImage(w, 16)
	ebitenutil.DebugPrint(img, s)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.Scale(
		float32(c.R)/255*alpha,
		float32(c.G)/255*alpha,
		float32(c.B)/255*alpha,
		alpha,
	)
	screen.DrawImage(img, op)
}

func drawPopups(screen *ebiten.Image, c *combat.Combat) {
	for _, p := range c.Popups {
		t := p.Age / combat.PopupLife
		if t > 1 {
			continue
		}
		alpha := float32(1.0 - t*t) // ease-out fade
		dy := -float64(40) * t
		x := int(RadarCX + p.X)
		y := int(RadarCY + p.Y + dy - 10)
		drawColoredText(screen, fmt.Sprintf("%d", p.Amount), x-6, y, damageTypeColor(p.Type), alpha)
	}
}

func drawRadar(screen *ebiten.Image, c *combat.Combat) {
	vector.DrawFilledCircle(screen, RadarCX, RadarCY, RadarRadius, radarColor, true)
	for _, r := range []float32{RadarRadius * 0.33, RadarRadius * 0.66, RadarRadius} {
		vector.StrokeCircle(screen, RadarCX, RadarCY, r, 1, radarRing, true)
	}
	drawMovePreview(screen, c)
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

func drawMovePreview(screen *ebiten.Image, c *combat.Combat) {
	if c.Phase != combat.PhasePlayer {
		return
	}
	if c.MovementBudget <= 0 {
		return
	}
	// Budget ring around the player.
	vector.StrokeCircle(screen, RadarCX, RadarCY, float32(c.MovementBudget), 1, moveRingCol, true)

	mx, my := ebiten.CursorPosition()
	rx, ry, ok := HitRadar(mx, my)
	if !ok {
		return
	}
	dist := math.Hypot(rx, ry)
	if dist < 1 {
		return
	}
	inBudget := dist <= c.MovementBudget
	step := dist
	if !inBudget {
		step = c.MovementBudget
	}
	gx := float32(RadarCX + rx/dist*step)
	gy := float32(RadarCY + ry/dist*step)

	col := moveOkCol
	if !inBudget {
		col = moveOverCol
	}
	vector.StrokeLine(screen, RadarCX, RadarCY, gx, gy, 2, col, true)
	vector.StrokeCircle(screen, gx, gy, 8, 2, col, true)

	label := fmt.Sprintf("%.0f / %.0f", dist, c.MovementBudget)
	if !inBudget {
		label += "  (clamped)"
	}
	ebitenutil.DebugPrintAt(screen, label, int(gx)+12, int(gy)-6)
}

func drawHand(screen *ebiten.Image, c *combat.Combat) {
	mx, my := ebiten.CursorPosition()
	for i, card := range c.Hand {
		x, y := cardRect(i)
		playable, _ := cardPlayable(c, card)
		bg := cardBg
		if !playable {
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

// cardPlayable evaluates both energy and CanPlay constraints. Returns
// playable + a short reason if not.
func cardPlayable(c *combat.Combat, card runes.Card) (bool, string) {
	if card.Cost > c.Energy {
		return false, "not enough energy"
	}
	if card.CanPlay != nil {
		if ok, why := card.CanPlay(c); !ok {
			return false, why
		}
	}
	return true, ""
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

func drawCardTooltip(screen *ebiten.Image, c *combat.Combat) {
	mx, my := ebiten.CursorPosition()
	i := HitCard(c, mx, my)
	if i < 0 {
		return
	}
	card := c.Hand[i]
	cx, cy := cardRect(i)

	const (
		tipW = 240
		tipH = 60
		pad  = 8
	)
	tx := cx + (CardW-tipW)/2
	if tx < 4 {
		tx = 4
	}
	if tx+tipW > ScreenW-4 {
		tx = ScreenW - 4 - tipW
	}
	ty := cy - tipH - 8

	vector.DrawFilledRect(screen, float32(tx), float32(ty), tipW, tipH, tooltipBg, true)
	vector.StrokeRect(screen, float32(tx), float32(ty), tipW, tipH, 1, tooltipEdge, true)
	header := fmt.Sprintf("%s %s   (cost %d)", card.Glyph, card.Name, card.Cost)
	ebitenutil.DebugPrintAt(screen, header, tx+pad, ty+pad)
	ebitenutil.DebugPrintAt(screen, card.Description, tx+pad, ty+pad+20)
	if ok, why := cardPlayable(c, card); !ok {
		ebitenutil.DebugPrintAt(screen, "("+why+")", tx+pad, ty+pad+36)
	}
}

func drawHUD(screen *ebiten.Image, v RunView) {
	c := v.Combat
	lines := []string{
		fmt.Sprintf("Encounter %d / %d", v.EncounterIdx+1, v.TotalEncounters),
		fmt.Sprintf("HP: %d/%d", c.PlayerHP, c.PlayerMaxHP),
		fmt.Sprintf("Armor: %d", c.PlayerArmor),
		fmt.Sprintf("Energy: %d/%d", c.Energy, c.MaxEnergy),
		fmt.Sprintf("Move: %.0f", c.MovementBudget),
		fmt.Sprintf("Deck: %d  Discard: %d  Total: %d", len(c.Draw), len(c.Discard), v.DeckSize),
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
	if c.Phase == combat.PhaseEnemy {
		ebitenutil.DebugPrintAt(screen, "(enemy turn)", 560, 700)
	}
	_ = green
	_ = white
	_ = yellow
}

// --- Reward screen ---

func rewardCardRect(i, n int) (int, int) {
	totalW := n*RewardCardW + (n-1)*RewardGap
	startX := (ScreenW - totalW) / 2
	x := startX + i*(RewardCardW+RewardGap)
	return x, RewardTopY
}

func skipBtnRect() (int, int) {
	x := (ScreenW - SkipBtnW) / 2
	y := RewardTopY + RewardCardH + 30
	return x, y
}

func drawRewardOverlay(screen *ebiten.Image, v RunView) {
	// Dim backdrop.
	vector.DrawFilledRect(screen, 0, 0, ScreenW, ScreenH, color.RGBA{0, 0, 0, 180}, true)
	ebitenutil.DebugPrintAt(screen, "Choose a rune to add to your deck.", (ScreenW/2)-130, RewardTopY-40)

	mx, my := ebiten.CursorPosition()
	for i, card := range v.Rewards {
		x, y := rewardCardRect(i, len(v.Rewards))
		bg := cardBg
		hovered := mx >= x && mx < x+RewardCardW && my >= y && my < y+RewardCardH
		if hovered {
			bg = cardBgHi
		}
		vector.DrawFilledRect(screen, float32(x), float32(y), RewardCardW, RewardCardH, bg, true)
		vector.StrokeRect(screen, float32(x), float32(y), RewardCardW, RewardCardH, 1, tooltipEdge, true)

		ebitenutil.DebugPrintAt(screen, card.Glyph, x+12, y+12)
		ebitenutil.DebugPrintAt(screen, card.Name, x+12, y+34)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Cost: %d", card.Cost), x+12, y+58)
		// Wrap description across lines.
		drawWrappedText(screen, card.Description, x+12, y+90, RewardCardW-24, 16)
	}

	sx, sy := skipBtnRect()
	bg := cardBg
	if mx >= sx && mx < sx+SkipBtnW && my >= sy && my < sy+SkipBtnH {
		bg = cardBgHi
	}
	vector.DrawFilledRect(screen, float32(sx), float32(sy), SkipBtnW, SkipBtnH, bg, true)
	ebitenutil.DebugPrintAt(screen, "SKIP — take nothing", sx+30, sy+18)
}

// drawWrappedText is a quick word-wrap using the debug font's ~7px-per-char.
func drawWrappedText(screen *ebiten.Image, s string, x, y, maxW, lineH int) {
	const charW = 7
	maxChars := maxW / charW
	if maxChars < 1 {
		maxChars = 1
	}
	words := splitWords(s)
	line := ""
	cy := y
	for _, w := range words {
		candidate := w
		if line != "" {
			candidate = line + " " + w
		}
		if len(candidate) > maxChars && line != "" {
			ebitenutil.DebugPrintAt(screen, line, x, cy)
			cy += lineH
			line = w
		} else {
			line = candidate
		}
	}
	if line != "" {
		ebitenutil.DebugPrintAt(screen, line, x, cy)
	}
}

func splitWords(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == ' ' || r == '\n' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
		} else {
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// HitReward returns the index of the reward card at (mx,my), or -1.
func HitReward(rewards []runes.Card, mx, my int) int {
	for i := range rewards {
		x, y := rewardCardRect(i, len(rewards))
		if mx >= x && mx < x+RewardCardW && my >= y && my < y+RewardCardH {
			return i
		}
	}
	return -1
}

func HitSkipReward(mx, my int) bool {
	x, y := skipBtnRect()
	return mx >= x && mx < x+SkipBtnW && my >= y && my < y+SkipBtnH
}

// --- End-of-run overlay ---

func drawEndOverlay(screen *ebiten.Image, v RunView, won bool) {
	vector.DrawFilledRect(screen, 0, 0, ScreenW, ScreenH, color.RGBA{0, 0, 0, 200}, true)
	msg := "RUN COMPLETE"
	if !won {
		msg = "RUN FAILED"
	}
	ebitenutil.DebugPrintAt(screen, msg, ScreenW/2-40, ScreenH/2-20)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Final HP: %d/%d   Deck size: %d", v.PlayerHP, v.MaxHP, v.DeckSize), ScreenW/2-110, ScreenH/2+10)
	ebitenutil.DebugPrintAt(screen, "Close the window to exit.", ScreenW/2-80, ScreenH/2+40)
}
