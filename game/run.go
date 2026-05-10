package game

import (
	"math/rand"

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

type Run struct {
	Class           runes.Class
	PlayerHP, MaxHP int
	Deck            []runes.Card
	EncounterIdx    int
	TotalEncounters int

	Phase   RunPhase
	Combat  *combat.Combat
	Rewards []runes.Card

	rng *rand.Rand
}

func NewRun(seed int64) *Run {
	return &Run{
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
}

func (r *Run) Update(dt float64) {
	switch r.Phase {
	case RunInCombat:
		r.Combat.Update(dt)
		switch r.Combat.Phase {
		case combat.PhaseWon:
			r.PlayerHP = r.Combat.PlayerHP
			if r.EncounterIdx+1 >= r.TotalEncounters {
				r.Phase = RunWon
				return
			}
			r.Rewards = r.rollRewards()
			r.Phase = RunReward
		case combat.PhaseLost:
			r.PlayerHP = 0
			r.Phase = RunLost
		}
	}
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
		r.Deck = append(r.Deck, r.Rewards[idx])
	}
	r.Rewards = nil
	r.EncounterIdx++
	r.startEncounter()
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
