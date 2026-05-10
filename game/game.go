package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"deckbuilder/combat"
	"deckbuilder/ui"
)

type Game struct {
	combat *combat.Combat
}

func New(seed int64) *Game {
	return &Game{combat: combat.New(seed)}
}

func (g *Game) Update() error {
	g.combat.Update(1.0 / 60.0)

	if g.combat.Phase == combat.PhasePlayer {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			mx, my := ebiten.CursorPosition()
			if i := ui.HitCard(g.combat, mx, my); i >= 0 {
				g.combat.PlayCard(i)
			} else if ui.HitEndTurn(mx, my) {
				g.combat.EndTurn()
			} else if rx, ry, ok := ui.HitRadar(mx, my); ok {
				g.combat.MoveTowards(rx, ry)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyE) {
			g.combat.EndTurn()
		}
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	ui.Draw(screen, g.combat)
}

func (g *Game) Layout(int, int) (int, int) { return ui.ScreenW, ui.ScreenH }
