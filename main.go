package main

import (
	"fmt"
	"time"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
)

func main() {
	pixelgl.Run(run)
}

func run() {
	romName := "roms/pokemon-blue.gb"
	cart := NewMBC3(romName)

	scale := 3.0
	width := 160
	height := 144

	cfg := &pixelgl.WindowConfig{
		Bounds: pixel.R(0, 0, float64(width)*scale, float64(height)*scale),
		VSync:  true,
	}
	win, err := pixelgl.NewWindow(*cfg)
	if err != nil {
		panic(err)
	}

	gb := NewGamebert(cart, win)

	d := Display{
		scale: scale,
		win:   win,
	}
	cyclesPerSecond := 4194304
	framesPerSecond := 60
	cyclesPerFrame := uint64(cyclesPerSecond / framesPerSecond)
	frameLength := time.Duration((1.0 / float64(framesPerSecond)) * float64(time.Second))

	lastDraw := time.Now()
	lastCycles := uint64(0)

	for true {
		gb.tick()

		if gb.mb.cycles%cyclesPerFrame < lastCycles%cyclesPerFrame {
			tSinceLastDraw := time.Since(lastDraw)
			tToNextDraw := frameLength - tSinceLastDraw

			if tToNextDraw > 0 {
				time.Sleep(tToNextDraw)
			}

			lastDraw = time.Now()
			d.draw(gb.mb.lcd.renderer.screenBuffer)
		}
		lastCycles = gb.mb.cycles
	}
}
func hex8(x uint8) string {
	return fmt.Sprintf("%02X", x)
}
func hex16(x uint16) string {
	return fmt.Sprintf("%04X", x)
}
