package game

import (
	"fmt"
	"math/rand"
	"strings"

	"deckbuilder/combat"
	"deckbuilder/enemies"
	"deckbuilder/runes"
)

type RunPhase int

const (
	RunSelectClass RunPhase = iota
	RunInCombat
	RunReward
	RunWon
	RunLost
)

const RewardChoices = 3

type EncounterReport struct {
	HPBefore int
	HPAfter  int
	Turns    int
}

type Run struct {
	Seed            int64
	Class           runes.Class
	PlayerHP, MaxHP int
	Deck            []runes.Card
	EncounterIdx    int
	TotalEncounters int

	Phase   RunPhase
	Combat  *combat.Combat
	Rewards []runes.Card

	// Telemetry for the end-of-run report.
	EncounterReports []EncounterReport
	CardsPicked      []string

	reportPrinted bool
	rng           *rand.Rand
}

func NewRun(seed int64) *Run {
	return &Run{
		Seed:            seed,
		MaxHP:           combat.DefaultMaxHP,
		PlayerHP:        combat.DefaultMaxHP,
		TotalEncounters: numEncounters(),
		Phase:           RunSelectClass,
		rng:             rand.New(rand.NewSource(seed)),
	}
}

// PickClass starts the run with the given class and its starter deck.
func (r *Run) PickClass(class runes.Class) {
	if r.Phase != RunSelectClass {
		return
	}
	r.Class = class
	r.Deck = runes.StarterDeck(class)
	r.startEncounter()
}

func (r *Run) startEncounter() {
	foes := makeEncounter(r.EncounterIdx)
	r.Combat = combat.New(r.rng.Int63(), r.PlayerHP, r.MaxHP, r.Deck, foes)
	r.Phase = RunInCombat
	r.EncounterReports = append(r.EncounterReports, EncounterReport{HPBefore: r.PlayerHP})
}

func (r *Run) Update(dt float64) {
	switch r.Phase {
	case RunInCombat:
		r.Combat.Update(dt)
		switch r.Combat.Phase {
		case combat.PhaseWon:
			r.PlayerHP = r.Combat.PlayerHP
			r.recordEncounterEnd()
			if r.EncounterIdx+1 >= r.TotalEncounters {
				r.Phase = RunWon
				r.printReport()
				return
			}
			r.Rewards = r.rollRewards()
			r.Phase = RunReward
		case combat.PhaseLost:
			r.PlayerHP = 0
			r.recordEncounterEnd()
			r.Phase = RunLost
			r.printReport()
		}
	}
}

func (r *Run) recordEncounterEnd() {
	if len(r.EncounterReports) == 0 {
		return
	}
	last := &r.EncounterReports[len(r.EncounterReports)-1]
	last.HPAfter = r.PlayerHP
	last.Turns = r.Combat.Turn
}

func (r *Run) rollRewards() []runes.Card {
	pool := runes.RewardPool(r.Class)
	r.rng.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	n := RewardChoices
	if n > len(pool) {
		n = len(pool)
	}
	out := make([]runes.Card, n)
	copy(out, pool[:n])
	return out
}

// PickReward adds the chosen card to the deck and starts the next encounter.
// idx is an index into Rewards; -1 means "skip — take nothing".
func (r *Run) PickReward(idx int) {
	if r.Phase != RunReward {
		return
	}
	if idx >= 0 && idx < len(r.Rewards) {
		card := r.Rewards[idx]
		r.Deck = append(r.Deck, card)
		r.CardsPicked = append(r.CardsPicked, card.Name)
	} else {
		r.CardsPicked = append(r.CardsPicked, "(skipped)")
	}
	r.Rewards = nil
	r.EncounterIdx++
	r.startEncounter()
}

func (r *Run) printReport() {
	if r.reportPrinted {
		return
	}
	r.reportPrinted = true

	outcome := "Run complete"
	if r.Phase == RunLost {
		outcome = "Run failed"
	}
	totalTurns := 0
	for _, er := range r.EncounterReports {
		totalTurns += er.Turns
	}

	var b strings.Builder
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "=== Run report ===")
	fmt.Fprintf(&b, "Seed:     %d\n", r.Seed)
	fmt.Fprintf(&b, "Class:    %s\n", r.Class)
	fmt.Fprintf(&b, "Outcome:  %s\n", outcome)
	fmt.Fprintf(&b, "Final HP: %d/%d\n", r.PlayerHP, r.MaxHP)
	fmt.Fprintf(&b, "Turns:    %d total across %d fights\n", totalTurns, len(r.EncounterReports))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Fights:")
	for i, er := range r.EncounterReports {
		fmt.Fprintf(&b, "  %d: HP %d -> %d  (%d turns)\n", i+1, er.HPBefore, er.HPAfter, er.Turns)
	}
	if len(r.CardsPicked) > 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Picks:")
		for i, name := range r.CardsPicked {
			fmt.Fprintf(&b, "  After fight %d: %s\n", i+1, name)
		}
	}
	fmt.Fprintln(&b, "==================")
	fmt.Print(b.String())
}

// --- Encounter table ---

func numEncounters() int { return 3 }

func makeEncounter(i int) []*enemies.Enemy {
	switch i {
	case 0:
		return []*enemies.Enemy{
			enemies.NewGoblin(140, -60),
			enemies.NewWraith(-110, 100),
		}
	case 1:
		return []*enemies.Enemy{
			enemies.NewGoblin(150, 80),
			enemies.NewGoblin(-130, 60),
			enemies.NewWraith(20, -150),
		}
	default:
		return []*enemies.Enemy{
			enemies.NewTroll(0, -180),
		}
	}
}
