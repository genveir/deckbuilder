package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"deckbuilder/game"
	"deckbuilder/ui"
)

func main() {
	seedFlag := flag.Int64("seed", 0, "deterministic seed (0 = use current time)")
	flag.Parse()

	seed := *seedFlag
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	fmt.Printf("seed=%d\n", seed)

	ebiten.SetWindowSize(ui.ScreenW, ui.ScreenH)
	ebiten.SetWindowTitle("Nand2Runes — Vertical Slice")
	if err := ebiten.RunGame(game.New(seed)); err != nil {
		log.Fatal(err)
	}
}
