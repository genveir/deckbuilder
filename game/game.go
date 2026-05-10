package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"deckbuilder/combat"
	"deckbuilder/ui"
)

type Game struct {
	run *Run
}

func New(seed int64) *Game {
	return &Game{run: NewRun(seed)}
}

func (g *Game) Update() error {
	g.run.Update(1.0 / 60.0)

	switch g.run.Phase {
	case RunSelectClass:
		g.handleClassSelectInput()
	case RunInCombat:
		g.handleCombatInput()
	case RunReward:
		g.handleRewardInput()
	}
	return nil
}

func (g *Game) handleClassSelectInput() {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	mx, my := ebiten.CursorPosition()
	if class, ok := ui.HitClassPick(mx, my); ok {
		g.run.PickClass(class)
	}
}

func (g *Game) handleCombatInput() {
	c := g.run.Combat
	if c.Phase != combat.PhasePlayer {
		return
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if i := ui.HitCard(c, mx, my); i >= 0 {
			c.PlayCard(i)
		} else if ui.HitEndTurn(mx, my) {
			c.EndTurn()
		} else if rx, ry, ok := ui.HitRadar(mx, my); ok {
			c.MoveTowards(rx, ry)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyE) {
		c.EndTurn()
	}
}

func (g *Game) handleRewardInput() {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	mx, my := ebiten.CursorPosition()
	if i := ui.HitReward(g.run.Rewards, mx, my); i >= 0 {
		g.run.PickReward(i)
	} else if ui.HitSkipReward(mx, my) {
		g.run.PickReward(-1)
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	ui.DrawRun(screen, runView(g.run))
}

func (g *Game) Layout(int, int) (int, int) { return ui.ScreenW, ui.ScreenH }

// runView decouples the UI package from the concrete *Run, exposing only what
// rendering needs.
func runView(r *Run) ui.RunView {
	deckSize := len(r.Deck)
	return ui.RunView{
		Phase:           int(r.Phase),
		Class:           r.Class,
		Combat:          r.Combat,
		Rewards:         r.Rewards,
		EncounterIdx:    r.EncounterIdx,
		TotalEncounters: r.TotalEncounters,
		PlayerHP:        r.PlayerHP,
		MaxHP:           r.MaxHP,
		DeckSize:        deckSize,
	}
}
