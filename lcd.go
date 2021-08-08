package main

import (
	"fmt"
	"sort"
)

const (
	dotsPerLine  = 456
	maxLy        = 153
	viewportRows = 144
	viewportCols = 160
)

type LCD struct {
	mb       *Motherboard
	renderer *Renderer

	lcdc *Register8Bit // 0xFF40
	stat *Register8Bit // 0xFF41

	scy  *Register8Bit // 0xFF42
	scx  *Register8Bit // 0xFF43
	ly   *Register8Bit // 0xFF44
	lyc  *Register8Bit // 0xFF45
	dma  *Register8Bit // 0xFF46
	bgp  *Register8Bit // 0xFF47
	obp0 *Register8Bit // 0xFF48
	obp1 *Register8Bit // 0xFF49
	wy   *Register8Bit // 0xFF4A
	wx   *Register8Bit // 0xFF4B

	flagLycInterrupt    *Flag // 6
	flagOAMInterrupt    *Flag // 5
	flagVBlankInterrupt *Flag // 4
	flagHBlankInterrupt *Flag // 3
	flagLyc             *Flag // 2
	flagMode1           *Flag // 1
	flagMode0           *Flag // 0

	flagLcdEnabled          *Flag // 7
	flagWindowmapSelect     *Flag // 6
	flagWindowEnabled       *Flag // 5
	flagTiledataSelect      *Flag // 4
	flagBackgroundMapSelect *Flag // 3
	flagSpriteHeight        *Flag // 2
	flagSpriteEnabled       *Flag // 1
	flagBackgroundEnabled   *Flag // 0

	vRAM *RAMSegment
	oam  *RAMSegment

	clock int
}

func NewLCD(mb *Motherboard) *LCD {
	lcdc := &Register8Bit{name: "lcdc"}
	stat := &Register8Bit{name: "stat"}

	lcd := &LCD{
		mb: mb,

		lcdc: lcdc,
		stat: stat,
		scy:  &Register8Bit{name: "scy"},
		scx:  &Register8Bit{name: "scx"},
		ly:   &Register8Bit{name: "ly"},
		lyc:  &Register8Bit{name: "lyc"},
		dma:  &Register8Bit{name: "dma"},
		bgp:  &Register8Bit{name: "bgp"},
		obp0: &Register8Bit{name: "obp0"},
		obp1: &Register8Bit{name: "obp1"},
		wy:   &Register8Bit{name: "wy"},
		wx:   &Register8Bit{name: "wx"},

		flagLycInterrupt:    &Flag{reg: stat, offset: 6, name: "lyci"},
		flagOAMInterrupt:    &Flag{reg: stat, offset: 5, name: "oami"},
		flagVBlankInterrupt: &Flag{reg: stat, offset: 4, name: "vbli"},
		flagHBlankInterrupt: &Flag{reg: stat, offset: 3, name: "hbli"},
		flagLyc:             &Flag{reg: stat, offset: 2, name: "lycf"},
		flagMode1:           &Flag{reg: stat, offset: 1, name: "mod1"},
		flagMode0:           &Flag{reg: stat, offset: 0, name: "mod0"},

		flagLcdEnabled:          &Flag{reg: lcdc, offset: 7, name: "lcde"},
		flagWindowmapSelect:     &Flag{reg: lcdc, offset: 6, name: "wmap"},
		flagWindowEnabled:       &Flag{reg: lcdc, offset: 5, name: "wien"},
		flagTiledataSelect:      &Flag{reg: lcdc, offset: 4, name: "tida"},
		flagBackgroundMapSelect: &Flag{reg: lcdc, offset: 3, name: "bmap"},
		flagSpriteHeight:        &Flag{reg: lcdc, offset: 2, name: "spht"},
		flagSpriteEnabled:       &Flag{reg: lcdc, offset: 1, name: "spen"},
		flagBackgroundEnabled:   &Flag{reg: lcdc, offset: 0, name: "bgen"},

		vRAM:  NewRAMSegment(0x2000),
		oam:   NewRAMSegment(0xA0),
		clock: 0,
	}
	lcd.flagLcdEnabled.write(true)

	renderer := NewRenderer(lcd)
	lcd.renderer = renderer

	return lcd
}

func (lcd *LCD) writeByte(loc uint16, val uint8) {
	lcd.getReg(loc).write(val)

	// DMA transfer
	if loc == 0xFF46 {
		// i.e. 0xXX => 0xXX00
		srcStart := uint16(val) * 0x100
		dstStart := uint16(0xFE00)

		segmentSize := uint16(0xA0)
		for i := uint16(0); i < segmentSize; i++ {
			transferVal := lcd.mb.readByte(srcStart + i)
			lcd.mb.writeByte(dstStart+i, transferVal)
		}
	}
}

func (lcd LCD) readByte(loc uint16) uint8 {
	// XXX just for test logs
	// if loc == 0xFF44 {
	// 	return 0x90
	// }
	return lcd.getReg(loc).read()
}

func (lcd *LCD) getReg(loc uint16) *Register8Bit {
	regMap := map[uint16]*Register8Bit{
		0xFF40: lcd.lcdc,
		0xFF41: lcd.stat,
		0xFF42: lcd.scy,
		0xFF43: lcd.scx,
		0xFF44: lcd.ly,
		0xFF45: lcd.lyc,
		0xFF46: lcd.dma,
		0xFF47: lcd.bgp,
		0xFF48: lcd.obp0,
		0xFF49: lcd.obp1,
		0xFF4A: lcd.wy,
		0xFF4B: lcd.wx,
	}
	reg, ok := regMap[loc]
	if !ok {
		panic(fmt.Sprintf("Unknown register at %04x", loc))
	}
	return reg
}

const (
	mode2Limit = 456 - 80
	mode3Limit = mode2Limit - 172
)

func (lcd *LCD) tick(cycles uint8) (bool, bool) {
	if !lcd.flagLcdEnabled.read() {
		lcd.clock = 0
		lcd.ly.write(0)
		lcd.writeStatMode(0)

		return false, false
	}

	requestVBlankInterrupt, requestStatInterruptIfModeChanged := false, false

	lcd.clock += int(cycles)

	oldMode := lcd.readStatMode()

	var nextMode uint8
	switch {
	// Mode 1 - vblank
	case lcd.ly.read() >= viewportRows:
		nextMode = uint8(1)
		requestStatInterruptIfModeChanged = lcd.flagVBlankInterrupt.read()

	// Mode 3 - drawing pixels
	case lcd.clock <= mode3Limit:
		nextMode = uint8(3)

		if nextMode != oldMode {
			lcd.renderer.scanline()
		}

	// Mode 2 - OAM scan
	case lcd.clock <= mode2Limit:
		nextMode = uint8(2)
		requestStatInterruptIfModeChanged = lcd.flagOAMInterrupt.read()

	// Mode 0 - hblank
	case lcd.clock <= 456+32:
		nextMode = uint8(0)
		requestStatInterruptIfModeChanged = lcd.flagHBlankInterrupt.read()

	default:
		panic("This should never happen")
	}

	lcd.writeStatMode(nextMode)

	requestStatInterrupt := requestStatInterruptIfModeChanged && nextMode != oldMode

	if lcd.ly.read() == lcd.lyc.read() {
		lcd.flagLyc.write(true)
		if lcd.flagLycInterrupt.read() {
			requestStatInterrupt = true
		}
	} else {
		lcd.flagLyc.write(false)
	}

	// Update clock and line number if reached end of line
	if lcd.clock >= dotsPerLine {
		lcd.clock = 0
		lcd.ly.inc(1)

		if lcd.ly.read() > maxLy {
			lcd.ly.write(0)
		}

		if lcd.ly.read() == viewportRows {
			requestVBlankInterrupt = true
		}
	}

	return requestVBlankInterrupt, requestStatInterrupt
}

func (lcd *LCD) readStatMode() uint8 {
	mode := uint8(0)
	if lcd.flagMode0.read() {
		mode += 1
	}
	if lcd.flagMode1.read() {
		mode += 2
	}

	return mode
}

func (lcd *LCD) writeStatMode(mode uint8) {
	lcd.flagMode0.write(isBitSet8(mode, 0))
	lcd.flagMode1.write(isBitSet8(mode, 1))
}

type Renderer struct {
	lcd          *LCD
	screenBuffer *Buffer2D
}

func NewRenderer(lcd *LCD) *Renderer {
	return &Renderer{
		lcd:          lcd,
		screenBuffer: NewBuffer2D(viewportRows, viewportCols),
	}
}

func (rd *Renderer) scanline() {
	// This is the scanline we are drawing
	ly := rd.lcd.ly.read()

	// Top-left co-ord to display of the larger map, for the background
	scrollX, scrollY := rd.lcd.scx.read(), rd.lcd.scy.read()
	// Ditto, but for the window
	winX, winY := (rd.lcd.wx.read() - 7), rd.lcd.wy.read()

	// Where do we read our tiles (?) for the window? We have a choice of 2 tilemaps
	var wTileOffset uint16
	if rd.lcd.flagWindowmapSelect.read() {
		wTileOffset = 0x1C00
	} else {
		wTileOffset = 0x1800
	}

	// Ditto for the background
	var bgTilemapOffset uint16
	if rd.lcd.flagBackgroundMapSelect.read() {
		bgTilemapOffset = 0x1C00
	} else {
		bgTilemapOffset = 0x1800
	}

	// Display the window if the flag is enabled, and if the window top row is
	// less than the current row being scanned.
	drawWindow := rd.lcd.flagWindowEnabled.read() && winY <= ly

	var yMap uint8
	if drawWindow {
		yMap = ly - winY
	} else {
		yMap = scrollY + ly
	}

	// Which addressing mode should be used to get tiles?
	useSigned := !rd.lcd.flagTiledataSelect.read()

	// Iterate across all x-cols for the current row
	for xViewport := uint8(0); xViewport < viewportCols; xViewport++ {
		// The x co-ordinate in the frame of the whole map - the viewport X co-ord + the BG X offset
		xMap := xViewport + scrollX

		useWindow := drawWindow && winX <= xViewport && winY <= ly

		var tilemapOffset uint16
		if useWindow {
			// This is where we will use the window tile offset
			tilemapOffset = wTileOffset
			// Translate to window map space if necessary
			if xViewport >= winX {
				xMap = xViewport - winX
			}
		} else if rd.lcd.flagBackgroundEnabled.read() {
			tilemapOffset = bgTilemapOffset
		}

		if useWindow || rd.lcd.flagBackgroundEnabled.read() {
			// Find the addr in memory that contains the tile we want to display
			// in this location.
			tileRow := uint16(yMap/8) * 32
			tileCol := uint16(xMap / 8)

			tileIndexAddress := tilemapOffset + tileRow + tileCol
			tileIndex := rd.lcd.vRAM.read(tileIndexAddress)

			var tileMemLoc uint16
			if useSigned {
				// https://gbdev.io/pandocs/Tile_Data.html
				if tileIndex < 128 {
					tileMemLoc = 0x1000 + uint16(tileIndex)*16
				} else {
					tileMemLoc = 0x0800 + uint16(tileIndex-128)*16
				}
			} else {
				tileMemLoc = uint16(tileIndex) * 16
			}

			line := (yMap % 8) * 2
			tileMemLoc += uint16(line)

			tileByte0 := rd.lcd.vRAM.read(tileMemLoc)
			tileByte1 := rd.lcd.vRAM.read(tileMemLoc + 1)

			colorBit := uint8(int8((xMap%8)-7) * -1)
			colorNum := 0
			if isBitSet8(tileByte0, colorBit) {
				colorNum += 1
			}
			if isBitSet8(tileByte1, colorBit) {
				colorNum += 2
			}

			palette := rd.lcd.bgp.read()
			iCols := []uint8{
				(palette) & 0b11,
				(palette >> 2) & 0b11,
				(palette >> 4) & 0b11,
				(palette >> 6) & 0b11,
			}

			rd.screenBuffer.write(xViewport, ly, iCols[colorNum])
		}
	}

	if rd.lcd.flagSpriteEnabled.read() {
		spriteHeight := uint8(8)
		if rd.lcd.flagSpriteHeight.read() {
			spriteHeight = uint8(16)
		}

		// Iterate through OAM and find which sprites are on the current y-line (ly)
		spriteMemLocs := make([]uint16, 0)
		spriteXes := make([]uint8, 0)
		for spriteN := uint16(0); spriteN < uint16(40); spriteN++ {
			spriteMemLoc := spriteN * 4
			y := rd.lcd.oam.read(spriteMemLoc) - 16
			x := rd.lcd.oam.read(spriteMemLoc+1) - 8

			if y <= ly && ly < y+spriteHeight {
				spriteMemLocs = append(spriteMemLocs, spriteMemLoc)
				spriteXes = append(spriteXes, x)
			}

			if len(spriteMemLocs) >= 10 {
				break
			}
		}

		sort.Slice(spriteMemLocs, func(i, j int) bool {
			if spriteXes[i] == spriteXes[j] {
				return spriteMemLocs[i] < spriteMemLocs[j]
			} else {
				return spriteXes[i] < spriteXes[j]
			}
		})

		spritePixels := make([]*spritePixel, viewportCols)

		// Iterate through sprites and I guess display the pixels for that sprite on this row?
		for _, spriteMemLoc := range spriteMemLocs {
			// Co-ords relative to viewport, not map
			spriteY := rd.lcd.oam.read(spriteMemLoc) - 16
			spriteX := rd.lcd.oam.read(spriteMemLoc+1) - 8

			tileIndex := rd.lcd.oam.read(spriteMemLoc + 2)
			// See pandocs - LSB ignored in 8x16 mode
			if rd.lcd.flagSpriteHeight.read() {
				tileIndex &= 0b11111110
			}

			// https://gbdev.io/pandocs/OAM.html#byte-3--attributesflags
			attributes := rd.lcd.oam.read(spriteMemLoc + 3)

			usePalette0 := isBitSet8(attributes, 4)
			xFlip := isBitSet8(attributes, 5)
			yFlip := isBitSet8(attributes, 6)
			// Background and window on top of sprite (unless transparent)
			bgWinPriority := isBitSet8(attributes, 7)
			_ = bgWinPriority

			tileY := ly - spriteY
			if yFlip {
				tileY = spriteHeight - tileY - 1
			}

			// Load the data containing the sprite data for this line
			tileMemLoc := uint16(tileIndex)*16 + uint16(tileY)*2
			tileByte0 := rd.lcd.vRAM.read(tileMemLoc)
			tileByte1 := rd.lcd.vRAM.read(tileMemLoc + 1)

			for tileX := uint8(0); uint8(tileX) < 8; tileX++ {
				pixelX := spriteX + 7 - tileX
				// Is this pixel of the sprite off the side of the screen?
				if pixelX < 0 || pixelX > viewportCols {
					continue
				}
				pix := spritePixels[pixelX]
				if pix != nil && pix.colorNum != 0 {
					continue
				}

				colorBit := tileX
				if xFlip {
					colorBit = uint8(int8((tileX)-7) * -1)
				}

				colorNum := uint8(0)
				if isBitSet8(tileByte0, colorBit) {
					colorNum += 1
				}
				if isBitSet8(tileByte1, colorBit) {
					colorNum += 2
				}

				// Continue if colorNum is 0, because for sprites this means transparent
				if colorNum == 0 {
					continue
				}

				sp := spritePixel{
					palette0:      usePalette0,
					bgWinPriority: bgWinPriority,
					colorNum:      colorNum,
				}
				spritePixels[pixelX] = &sp
			}
		}

		for x := uint8(0); x < uint8(len(spritePixels)); x++ {
			pix := spritePixels[x]
			if pix != nil {
				palette := rd.lcd.obp0.read()
				if !pix.palette0 {
					palette = rd.lcd.obp1.read()
				}

				iCols := []uint8{
					(palette) & 0b11,
					(palette >> 2) & 0b11,
					(palette >> 4) & 0b11,
					(palette >> 6) & 0b11,
				}
				if pix.bgWinPriority {
					// Pixel is already written, but is transparent
					if rd.screenBuffer.read(x, ly) == 0 {
						rd.screenBuffer.write(x, ly, iCols[pix.colorNum])
					}
				} else {
					rd.screenBuffer.write(x, ly, iCols[pix.colorNum])
				}
			}
		}
	}
}

type spritePixel struct {
	palette0      bool
	bgWinPriority bool
	colorNum      uint8
}
