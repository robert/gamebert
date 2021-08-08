package main

import (
	"fmt"
	"io/ioutil"

	"github.com/faiface/pixel/pixelgl"
)

type JoypadIO struct {
	joyp *Register8Bit
	win  *pixelgl.Window
}

func (j *JoypadIO) write(val uint8) {
	j.joyp.setBit(4, isBitSet8(val, 4))
	j.joyp.setBit(5, isBitSet8(val, 5))
}

func (j *JoypadIO) read() uint8 {
	joyp := j.joyp.read()

	joypadInput := uint8(0b1111)
	if !isBitSet8(j.joyp.read(), 4) {
		if j.win.Pressed(pixelgl.KeyRight) {
			joypadInput = clearBit(joypadInput, 0)
		}
		if j.win.Pressed(pixelgl.KeyLeft) {
			joypadInput = clearBit(joypadInput, 1)
		}
		if j.win.Pressed(pixelgl.KeyUp) {
			joypadInput = clearBit(joypadInput, 2)
		}
		if j.win.Pressed(pixelgl.KeyDown) {
			joypadInput = clearBit(joypadInput, 3)
		}
	} else {
		// A
		if j.win.Pressed(pixelgl.KeyA) {
			joypadInput = clearBit(joypadInput, 0)
		}
		// B
		if j.win.Pressed(pixelgl.KeyS) {
			joypadInput = clearBit(joypadInput, 1)
		}
		// Select
		if j.win.Pressed(pixelgl.KeyD) {
			joypadInput = clearBit(joypadInput, 2)
		}
		// Start
		if j.win.Pressed(pixelgl.KeyF) {
			joypadInput = clearBit(joypadInput, 3)
		}
	}
	return joyp | joypadInput
}

func NewJoypadIO(win *pixelgl.Window) *JoypadIO {
	return &JoypadIO{
		joyp: &Register8Bit{},
		win:  win,
	}
}

type Motherboard struct {
	cpu *CPU
	lcd *LCD

	timer *Timer

	cart Cartridge

	internalRAM0      *RAMSegment
	internalRAM1      *RAMSegment
	nonIOInternalRAM0 *RAMSegment
	nonIOInternalRAM1 *RAMSegment
	ioPorts           *RAMSegment

	joypadIO *JoypadIO

	bootROM        *ROMSegment
	bootROMEnabled bool

	debug bool

	cycles uint64
}

func NewMotherboard(cart Cartridge, win *pixelgl.Window) *Motherboard {
	bootROMData, err := ioutil.ReadFile("dmg_boot.bin")
	if err != nil {
		panic(err)
	}
	bootROM := NewROMSegment(bootROMData)

	timer := NewTimer()

	mb := &Motherboard{
		cart:              cart,
		timer:             timer,
		internalRAM0:      NewRAMSegment(8 * 1024),
		internalRAM1:      NewRAMSegment(0x7F),
		nonIOInternalRAM0: NewRAMSegment(0x60),
		nonIOInternalRAM1: NewRAMSegment(0x34),
		ioPorts:           NewRAMSegment(0x4C),
		joypadIO:          NewJoypadIO(win),
		bootROM:           bootROM,
		bootROMEnabled:    true,
	}
	cpu := NewCPU(mb)
	mb.cpu = cpu

	lcd := NewLCD(mb)
	mb.lcd = lcd

	return mb
}

func (mb *Motherboard) tick() {
	cycles := mb.cpu.tick()
	vBlankInterruptRequested, statInterruptRequested := mb.lcd.tick(cycles)

	if vBlankInterruptRequested {
		mb.cpu.intTriggeredVBlank.write(true)
	}
	if statInterruptRequested {
		mb.cpu.intTriggeredStat.write(true)
	}

	timerInterruptRequested := mb.timer.tick(cycles)

	if timerInterruptRequested {
		mb.cpu.intTriggeredTimer.write(true)
	}

	mb.cycles += uint64(cycles)
}

func (mb Motherboard) readWord(loc uint16) uint16 {
	return combine8(mb.readByte(loc+1), mb.readByte(loc))
}

func (mb Motherboard) readByte(loc uint16) uint8 {
	notImplemented := func() {
		panic(fmt.Sprintf("Not implemented: reading memory from: %04x", loc))
	}

	if loc < 0x4000 {
		if loc <= 0xFF && mb.bootROMEnabled {
			return mb.bootROM.read(loc)
		} else {
			return mb.cart.read(loc)
		}
	} else if loc < 0x8000 {
		return mb.cart.read(loc)
	} else if loc < 0xA000 {
		return mb.lcd.vRAM.read(loc - 0x8000)
	} else if loc < 0xC000 {
		return mb.cart.read(loc)
	} else if loc < 0xE000 {
		return mb.internalRAM0.read(loc - 0xC000)
	} else if loc < 0xFE00 {
		return mb.readByte(loc - 0x2000)
	} else if loc < 0xFEA0 {
		return mb.lcd.oam.read(loc - 0xA0)
	} else if loc < 0xFF00 {
		return mb.nonIOInternalRAM0.read(loc - 0xFEA0)
	} else if loc < 0xFF4C {
		if loc == 0xFF00 {
			return mb.joypadIO.read()
		} else if loc == 0xFF01 {
			// Serial buffer - not implemented
			return 0x0
		} else if loc == 0xFF04 {
			return mb.timer.div.read()
		} else if loc == 0xFF05 {
			return mb.timer.tima.read()
		} else if loc == 0xFF06 {
			return mb.timer.tma.read()
		} else if loc == 0xFF07 {
			return mb.timer.tac.read()
		} else if loc == 0xFF0F {
			return mb.cpu.interruptsTriggered.read()
		} else if loc < 0xFF40 {
			// Sound
			return 0x0
		} else if loc < 0xFF4C {
			return mb.lcd.readByte(loc)
		} else {
			// IO
			notImplemented()
		}
	} else if loc < 0xFF80 {
		return mb.nonIOInternalRAM1.read(loc - 0xFF4C)
	} else if loc < 0xFFFF {
		return mb.internalRAM1.read(loc - 0xFF80)
	} else if loc == 0xFFFF {
		return mb.cpu.interruptsEnabled.read()
	} else {
		notImplemented()
	}

	panic("This should never happen")
}

func (mb *Motherboard) writeWord(loc, val uint16) {
	hi, lo := chunk16(val)

	mb.writeByte(loc, lo)
	mb.writeByte(loc+1, hi)
}

func (mb *Motherboard) writeByte(loc uint16, val uint8) {
	notImplemented := func() {
		panic(fmt.Sprintf("Not implemented: writine memory to: %04x", loc))
	}

	if loc < 0x4000 {
		mb.cart.write(loc, val)
	} else if loc < 0x8000 {
		mb.cart.write(loc, val)
	} else if loc < 0xA000 {
		mb.lcd.vRAM.write(loc-0x8000, val)
	} else if loc < 0xC000 {
		mb.cart.write(loc, val)
	} else if loc < 0xE000 {
		mb.internalRAM0.write(loc-0xC000, val)
	} else if loc < 0xFE00 {
		mb.writeByte(loc-0x2000, val)
	} else if loc < 0xFEA0 {
		mb.lcd.oam.write(loc-0xFE00, val)
	} else if loc < 0xFF00 {
		mb.nonIOInternalRAM0.write(loc-0xFEA0, val)
	} else if loc < 0xFF4C {
		if loc == 0xFF00 {
			mb.joypadIO.write(val)
		} else if loc == 0xFF01 {
			mb.ioPorts.write(loc-0xFF00, val)
		} else if loc == 0xFF04 {
			// https://gbdev.io/pandocs/Timer_and_Divider_Registers.html
			mb.timer.div.write(0)
		} else if loc == 0xFF05 {
			mb.timer.tima.write(val)
		} else if loc == 0xFF06 {
			mb.timer.tma.write(val)
		} else if loc == 0xFF07 {
			// We use & 0b111 because only the first 3 bits are used.
			// https://gbdev.io/pandocs/Timer_and_Divider_Registers.html
			mb.timer.tac.write(val & 0b111)
		} else if loc == 0xFF0F {
			mb.cpu.interruptsTriggered.write(val)
		} else if loc < 0xFF40 {
			// Sound
			// TODO
		} else if loc < 0xFF4C {
			mb.lcd.writeByte(loc, val)
		} else {
			notImplemented()
		}
	} else if loc < 0xFF80 {
		if mb.bootROMEnabled && loc == 0xFF50 && (val == 0x1 || val == 0x11) {
			mb.bootROMEnabled = false
		} else {
			mb.nonIOInternalRAM1.write(loc-0xFF4C, val)
		}
	} else if loc < 0xFFFF {
		mb.internalRAM1.write(loc-0xFF80, val)
	} else if loc == 0xFFFF {
		mb.cpu.interruptsEnabled.write(val)
	} else {
		notImplemented()
	}
}
