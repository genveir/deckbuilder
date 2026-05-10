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

	StageY      = 470
	StageCardW  = 100
	StageCardH  = 100
	StageGap    = 10
	StageStartX = 80

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
	Class           runes.Class
	Combat          *combat.Combat
	Rewards         []runes.Card
	EncounterIdx    int
	TotalEncounters int
	PlayerHP, MaxHP int
	DeckSize        int
}

// Run phase constants kept in sync with game.RunPhase.
const (
	phaseSelectClass = 0
	phaseInCombat    = 1
	phaseReward      = 2
	phaseRunWon      = 3
	phaseRunLost     = 4
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
	minionCol   = color.RGBA{120, 220, 160, 255}
	wallCol     = color.RGBA{170, 160, 140, 255}
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
	case phaseSelectClass:
		drawClassSelect(screen)
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
	drawPlacementPreview(screen, c)
	drawStage(screen, c)
	drawHand(screen, c)
	drawHUD(screen, v)
	drawEndTurn(screen, c)
	drawCardTooltip(screen, c)
	drawPhaseBanner(screen, c)
}

func drawStage(screen *ebiten.Image, c *combat.Combat) {
	header := "Spell stage (right-click to unstage):"
	if c.SpellCast {
		header = "Spell cast — press E to end turn"
	} else if len(c.Stage) == 0 {
		header = "Spell stage:  (click a rune to add it)"
	}
	ebitenutil.DebugPrintAt(screen, header, StageStartX, StageY-20)

	for i, sc := range c.Stage {
		x := StageStartX + i*(StageCardW+StageGap)
		y := StageY
		vector.DrawFilledRect(screen, float32(x), float32(y), StageCardW, StageCardH, cardBg, true)
		vector.StrokeRect(screen, float32(x), float32(y), StageCardW, StageCardH, 1, tooltipEdge, true)
		ebitenutil.DebugPrintAt(screen, sc.Card.Glyph, x+8, y+6)
		ebitenutil.DebugPrintAt(screen, sc.Card.Name, x+8, y+24)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Cost: %d", sc.Card.Cost), x+8, y+StageCardH-20)
	}
	if !c.SpellCast && len(c.Stage) > 0 {
		total := 0
		for _, sc := range c.Stage {
			total += sc.Card.Cost
		}
		summary := fmt.Sprintf("%d rune(s), %d energy — press E to cast", len(c.Stage), total)
		ebitenutil.DebugPrintAt(screen, summary, StageStartX, StageY+StageCardH+8)
	}
}

func drawPlacementPreview(screen *ebiten.Image, c *combat.Combat) {
	if c.PendingCardIdx < 0 || c.PendingCardIdx >= len(c.Hand) {
		return
	}
	card := c.Hand[c.PendingCardIdx]
	mx, my := ebiten.CursorPosition()
	if rx, ry, ok := HitRadar(mx, my); ok {
		gx := float32(RadarCX + rx)
		gy := float32(RadarCY + ry)
		switch card.PlacementShape {
		case runes.PlacementWall:
			// perpendicular preview through cursor (length 100, matches card)
			d := math.Hypot(rx, ry)
			if d == 0 {
				rx, ry, d = 1, 0, 1
			}
			pxv := float32(-ry / d)
			pyv := float32(rx / d)
			const half = float32(50)
			vector.StrokeLine(screen, gx-pxv*half, gy-pyv*half, gx+pxv*half, gy+pyv*half, 4, wallCol, true)
			vector.StrokeLine(screen, RadarCX, RadarCY, gx, gy, 1, minionCol, true)
		default:
			vector.StrokeCircle(screen, gx, gy, 8, 2, minionCol, true)
			vector.StrokeLine(screen, RadarCX, RadarCY, gx, gy, 1, minionCol, true)
		}
	}
	banner := fmt.Sprintf("Placing: %s — left-click radar to confirm, right-click to cancel", card.Name)
	ebitenutil.DebugPrintAt(screen, banner, 40, 30)
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
		x := int(float64(RadarCX) + (p.X - c.Player.X))
		y := int(float64(RadarCY) + (p.Y - c.Player.Y) + dy - 10)
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

	for _, w := range c.Walls {
		if w.HP <= 0 {
			continue
		}
		x1 := float32(RadarCX + (w.X1 - c.Player.X))
		y1 := float32(RadarCY + (w.Y1 - c.Player.Y))
		x2 := float32(RadarCX + (w.X2 - c.Player.X))
		y2 := float32(RadarCY + (w.Y2 - c.Player.Y))
		vector.StrokeLine(screen, x1, y1, x2, y2, 5, wallCol, true)
		mx := int((x1 + x2) / 2)
		my := int((y1 + y2) / 2)
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("wall %d/%d", w.HP, w.MaxHP), mx-24, my-18)
	}

	for _, e := range c.Enemies {
		ex := float32(RadarCX + (e.X - c.Player.X))
		ey := float32(RadarCY + (e.Y - c.Player.Y))
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

	for _, m := range c.Minions {
		if m.HP <= 0 {
			continue
		}
		mx := float32(RadarCX + (m.X - c.Player.X))
		my := float32(RadarCY + (m.Y - c.Player.Y))
		vector.DrawFilledCircle(screen, mx, my, 7, minionCol, true)
		label := fmt.Sprintf("M %d/%d  (%d/t)", m.HP, m.MaxHP, m.AttackPower)
		ebitenutil.DebugPrintAt(screen, label, int(mx)-32, int(my)+10)
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
		if i == c.PendingCardIdx {
			bg = cardBgHi
		} else if !playable {
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

// cardPlayable evaluates whether a hand card can be staged right now,
// accounting for spell-already-cast, energy, and CanPlay constraints.
func cardPlayable(c *combat.Combat, card runes.Card) (bool, string) {
	if c.SpellCast {
		return false, "spell already cast this turn"
	}
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
		fmt.Sprintf("Class: %s", v.Class),
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
	label := "END TURN"
	if len(c.Stage) > 0 {
		label = fmt.Sprintf("CAST (%d)", len(c.Stage))
	}
	vector.DrawFilledRect(screen, EndTurnX, EndTurnY, EndTurnW, EndTurnH, col, true)
	ebitenutil.DebugPrintAt(screen, label, EndTurnX+18, EndTurnY+24)
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

// --- Class select screen ---

const (
	classCardW = 320
	classCardH = 400
	classGap   = 40
	classTopY  = 180
)

type classOption struct {
	class       runes.Class
	title       string
	description []string
}

func classOptions() []classOption {
	return []classOption{
		{
			class: runes.ClassElementalist,
			title: "Elementalist",
			description: []string{
				"Exploits the type system.",
				"Match damage type to enemy weakness for 1.5x damage.",
				"",
				"Defense: Earth Armor (cannot move once cast).",
				"No mobility runes — stand and burn.",
				"",
				"Starter: 4 Fire, 3 Earth Armor, 2 Frost.",
			},
		},
		{
			class: runes.ClassMesmer,
			title: "Mesmer",
			description: []string{
				"Reads enemy intent. Punishes both",
				"aggression and passivity.",
				"",
				"Defense: positioning and delay.",
				"Aphyr stalls; Move keeps distance.",
				"",
				"Starter: 2 Aphyr, 3 Isa-aggressive,",
				"3 Isa-passive, 2 Move.",
			},
		},
		{
			class: runes.ClassNecromancer,
			title: "Necromancer",
			description: []string{
				"Summons minions as persistent",
				"processes that occupy the radar.",
				"",
				"Defense: minions intercept aggression.",
				"Drain heals; sacrifice burns minion HP",
				"for damage.",
				"",
				"Starter: 4 Thurisaz, 3 Maðr, 3 Ár.",
			},
		},
	}
}

func classCardRect(i int) (int, int) {
	opts := classOptions()
	n := len(opts)
	totalW := n*classCardW + (n-1)*classGap
	startX := (ScreenW - totalW) / 2
	x := startX + i*(classCardW+classGap)
	return x, classTopY
}

func drawClassSelect(screen *ebiten.Image) {
	ebitenutil.DebugPrintAt(screen, "NAND2RUNES — choose your class", ScreenW/2-110, 100)

	mx, my := ebiten.CursorPosition()
	for i, opt := range classOptions() {
		x, y := classCardRect(i)
		bg := cardBg
		if mx >= x && mx < x+classCardW && my >= y && my < y+classCardH {
			bg = cardBgHi
		}
		vector.DrawFilledRect(screen, float32(x), float32(y), classCardW, classCardH, bg, true)
		vector.StrokeRect(screen, float32(x), float32(y), classCardW, classCardH, 1, tooltipEdge, true)
		ebitenutil.DebugPrintAt(screen, opt.title, x+18, y+18)
		for j, line := range opt.description {
			ebitenutil.DebugPrintAt(screen, line, x+18, y+60+j*20)
		}
	}
}

// HitClassPick returns the class at (mx, my) if any.
func HitClassPick(mx, my int) (runes.Class, bool) {
	for i, opt := range classOptions() {
		x, y := classCardRect(i)
		if mx >= x && mx < x+classCardW && my >= y && my < y+classCardH {
			return opt.class, true
		}
	}
	return 0, false
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
