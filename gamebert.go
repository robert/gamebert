package main

import "github.com/faiface/pixel/pixelgl"

type Gamebert struct {
	mb *Motherboard
}

// Could probably do without the Gamebert struct
func NewGamebert(cart Cartridge, win *pixelgl.Window) *Gamebert {
	mb := NewMotherboard(cart, win)

	return &Gamebert{
		mb: mb,
	}
}

func (gb *Gamebert) tick() {
	gb.mb.tick()
}
