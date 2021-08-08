package main

func chunk16(x uint16) (uint8, uint8) {
	hi := uint8((x >> 8) & 0xFF)
	lo := uint8(x & 0xFF)

	return hi, lo
}

func combine8(hi, lo uint8) uint16 {
	return (uint16(hi) << 8) + uint16(lo)
}

func isBitSet8(x uint8, bitN uint8) bool {
	return (x & (1 << bitN)) != 0
}

func isBitSet16(x uint16, bitN uint8) bool {
	return (x & (1 << bitN)) != 0
}

func isHalfCarry8(x1, x2 uint8) bool {
	onesMask := uint8(0xF)
	// Just checking if the 4th bit is set
	return isBitSet8(uint8(x1&onesMask)+uint8(x2&onesMask), 4)
}

func isHalfCarry16(x1, x2 uint16) bool {
	onesMask := uint16(0xFFF)
	// Just checking if the 4th bit is set
	return isBitSet16(uint16(x1&onesMask)+uint16(x2&onesMask), 12)
}

func isHalfBorrow8(minuend, subtrahend uint8) bool {
	mask := uint8(0xF)
	return (minuend & mask) < (subtrahend & mask)
}

func isHalfBorrow16(minuend, subtrahend uint16) bool {
	mask := uint16(0xFFF)
	return (minuend & mask) < (subtrahend & mask)
}

func rotateThroughL8(x uint8, minus1thBit bool) (uint8, bool) {
	oldBit7 := ((1 << 7) & x) != 0
	rot := x << 1
	if minus1thBit {
		return rot + 1, oldBit7
	} else {
		return rot, oldBit7
	}
}

func rotateThroughR8(x uint8, eighthBit bool) (uint8, bool) {
	oldBit0 := (0x1 & x) != 0
	rot := x >> 1

	if eighthBit {
		return rot + (1 << 7), oldBit0
	} else {
		return rot, oldBit0
	}
}

func rotateR8(x uint8) uint8 {
	oldBit0 := (0x1 & x) != 0
	rot := x >> 1

	if oldBit0 {
		return rot + (1 << 7)
	} else {
		return rot
	}
}

func rotateL8(x uint8) uint8 {
	oldBit7 := ((1 << 7) & x) != 0
	rot := x << 1

	if oldBit7 {
		return rot + 1
	} else {
		return rot
	}
}

func shiftL8(x uint8) (uint8, bool) {
	oldBit7 := ((1 << 7) & x) != 0
	rot := x << 1

	return rot, oldBit7
}

func shiftR8(x uint8) (uint8, bool) {
	oldBit0 := (0x1 & x) != 0
	rot := x >> 1

	return rot, oldBit0
}

type Buffer2D struct {
	data []uint8
	rows uint8
	cols uint8
}

func NewBuffer2D(rows, cols uint8) *Buffer2D {
	return &Buffer2D{
		rows: rows,
		cols: cols,
		data: make([]uint8, uint16(rows)*uint16(cols)),
	}
}

func (b Buffer2D) read(x, y uint8) uint8 {
	return b.data[b.idx(x, y)]
}

func (b *Buffer2D) write(x, y uint8, val uint8) {
	b.data[b.idx(x, y)] = val
}

func (b Buffer2D) idx(x, y uint8) uint16 {
	return uint16(x) + (uint16(b.cols) * uint16(y))
}

func clearBit(x uint8, bitN int) uint8 {
	return x & ^(uint8(1) << bitN)
}
