package main

import (
	"log"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"deckbuilder/game"
	"deckbuilder/ui"
)

func main() {
	ebiten.SetWindowSize(ui.ScreenW, ui.ScreenH)
	ebiten.SetWindowTitle("Nand2Runes — Vertical Slice")
	if err := ebiten.RunGame(game.New(time.Now().UnixNano())); err != nil {
		log.Fatal(err)
	}
}
