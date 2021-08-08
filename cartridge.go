package main

import (
	"fmt"
	"io/ioutil"
)

type Cartridge interface {
	read(uint16) uint8
	write(uint16, uint8)
}

type MBC0 struct {
	Rom *ROMSegment
}

func NewMBC0(fpath string) *MBC0 {
	data, err := ioutil.ReadFile(fpath)
	if err != nil {
		panic(err)
	}

	if len(data) != 32768 {
		panic(fmt.Sprintf("Not 32kb! %d bytes", len(data)))
	}

	return &MBC0{
		Rom: NewROMSegment(data),
	}
}

func (cart MBC0) read(loc uint16) uint8 {
	return cart.Rom.read(loc)
}

func (cart *MBC0) write(loc uint16, val uint8) {
	// Should never happen, but don't error
}

func NewMBC3(fpath string) *MBC3 {
	data, err := ioutil.ReadFile(fpath)
	if err != nil {
		panic(err)
	}
	return &MBC3{
		rom:             NewROMSegment(data),
		selectedRomBank: 1,
		ram:             NewRAMSegment(0x8000),
		rtc:             make([]byte, 0x10),
		latchedRtc:      make([]byte, 0x10),
	}
}

type MBC3 struct {
	rom             *ROMSegment
	selectedRomBank uint32

	ram             *RAMSegment
	selectedRamBank uint32
	ramEnabled      bool

	rtc        []byte
	latchedRtc []byte
	latched    bool
}

func (r *MBC3) read(loc uint16) byte {
	switch {
	case loc < 0x4000:
		return r.rom.read(uint64(loc))
	case loc < 0x8000:
		return r.rom.read(uint64(loc) - 0x4000 + uint64(r.selectedRomBank)*0x4000)
	default:
		if r.selectedRamBank >= 0x4 {
			if r.latched {
				return r.latchedRtc[r.selectedRamBank]
			}
			return r.rtc[r.selectedRamBank]
		}
		return r.ram.read((0x2000*uint64(r.selectedRamBank) + uint64(loc) - 0xA000))
	}
}

func (r *MBC3) write(loc uint16, value uint8) {
	switch {
	case loc < 0x2000:
		r.ramEnabled = (value & 0xA) != 0
	case loc < 0x4000:
		r.selectedRomBank = uint32(value & 0x7F)
		if r.selectedRomBank == 0x00 {
			r.selectedRomBank++
		}
	case loc < 0x6000:
		r.selectedRamBank = uint32(value)
	case loc < 0x8000:
		if value == 0x1 {
			r.latched = false
		} else if value == 0x0 {
			r.latched = true
			copy(r.rtc, r.latchedRtc)
		}
	case loc < 0xA000:
		panic("This should never happen - mapped to VRAM")
	case loc < 0xC000:
		if r.ramEnabled {
			if r.selectedRamBank >= 0x4 {
				r.rtc[r.selectedRamBank] = value
			} else {
				r.ram.write(uint64(r.selectedRamBank)*0x2000+uint64(loc)-0xA000, value)
			}
		}
	}
}
