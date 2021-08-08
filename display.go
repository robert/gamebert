package main

import (
	"image"
	"image/color"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/pixelgl"
)

type Display struct {
	win   *pixelgl.Window
	scale float64
}

func (d *Display) frame(buf *Buffer2D) *image.RGBA {
	width := buf.cols
	height := buf.rows
	m := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))

	for x := uint8(0); x < buf.cols; x++ {
		for y := uint8(0); y < buf.rows; y++ {
			val := (255 - buf.read(x, y)) * 50
			pix := color.RGBA{val, val, val, 0}

			m.Set(int(x), int(y), pix)
		}
	}

	return m
}

func (d *Display) draw(buf *Buffer2D) {
	d.win.Clear(color.Black)

	p := pixel.PictureDataFromImage(d.frame(buf))

	c := d.win.Bounds().Center()
	pixel.NewSprite(p, p.Bounds()).
		Draw(d.win, pixel.IM.Moved(c).Scaled(c, d.scale))

	d.win.Update()
}
