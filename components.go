package main

type Register8Bit struct {
	val  uint8
	name string
	mask uint8
}

func (r *Register8Bit) write(val uint8) {
	if r.mask > 0 {
		r.val = val & r.mask
	} else {
		r.val = val
	}
}
func (r Register8Bit) read() uint8 {
	return r.val
}
func (r *Register8Bit) inc(val uint8) {
	r.write(r.val + val)
}
func (r *Register8Bit) dec(val uint8) {
	r.write(r.val - val)
}
func (r *Register8Bit) setBit(offset int, bitVal bool) {
	var newVal uint8
	if bitVal {
		newVal = r.val | uint8(1<<offset)
	} else {
		newVal = r.val & ^uint8(1<<offset)
	}
	r.write(newVal)
}

type Register16Bit struct {
	val  uint16
	name string
}

func (r *Register16Bit) write(val uint16) {
	r.val = val
}
func (r Register16Bit) read() uint16 {
	return r.val
}
func (r *Register16Bit) inc(val uint16) {
	r.write(r.val + val)
}
func (r *Register16Bit) dec(val uint16) {
	r.write(r.val - val)
}

type UnifiedRegister16Bit struct {
	hi *Register8Bit
	lo *Register8Bit
}

func (r *UnifiedRegister16Bit) write(val uint16) {
	hi, lo := chunk16(val)

	r.hi.write(hi)
	r.lo.write(lo)
}
func (r UnifiedRegister16Bit) read() uint16 {
	return combine8(r.hi.read(), r.lo.read())
}
func (r *UnifiedRegister16Bit) inc(val uint16) {
	// TODO: this can be sped up to avoid calling .read
	r.write(r.read() + val)
}
func (r *UnifiedRegister16Bit) dec(val uint16) {
	// TODO: this can be sped up to avoid calling .read
	r.write(r.read() - val)
}

type Flag struct {
	reg    *Register8Bit
	offset int
	name   string
}

func (f *Flag) write(val bool) {
	f.reg.setBit(f.offset, val)
}
func (f Flag) read() bool {
	return (f.reg.read() & uint8(1<<f.offset)) != 0
}
func (f Flag) readUint8() uint8 {
	if f.read() {
		return 1
	} else {
		return 0
	}
}

type R8Bit interface {
	read() uint8
}
type RW8Bit interface {
	read() uint8
	write(uint8)
}

type R16Bit interface {
	read() uint16
}
type RW16Bit interface {
	read() uint16
	write(uint16)
}

type RAMByte struct {
	mb     *Motherboard
	offset uint16
}

func (rb *RAMByte) write(val uint8) {
	rb.mb.writeByte(rb.offset, val)
}
func (rb *RAMByte) inc(val uint8) {
	rb.write(rb.read() + val)
}
func (rb *RAMByte) read() uint8 {
	return rb.mb.readByte(rb.offset)
}

type RAMWord struct {
	mb     *Motherboard
	offset uint16
}

func (rb *RAMWord) write(val uint16) {
	rb.mb.writeWord(rb.offset, val)
}
func (rb RAMWord) read() uint16 {
	return rb.mb.readWord(rb.offset)
}

type RAMSegment struct {
	data []uint8
}

func (ram *RAMSegment) write(loc interface{}, val uint8) {
	switch v := loc.(type) {
	case uint8:
		ram.data[v] = val
	case uint16:
		ram.data[v] = val
	case uint32:
		ram.data[v] = val
	case uint64:
		ram.data[v] = val
	default:
		panic(v)
	}
}
func (ram RAMSegment) read(loc interface{}) uint8 {
	switch v := loc.(type) {
	case uint8:
		return ram.data[v]
	case uint16:
		return ram.data[v]
	case uint32:
		return ram.data[v]
	case uint64:
		return ram.data[v]
	default:
		panic(v)
	}
}

func NewRAMSegment(size uint64) *RAMSegment {
	return &RAMSegment{
		data: make([]uint8, size),
	}
}

type ROMSegment struct {
	data []uint8
}

func (rom ROMSegment) read(loc interface{}) uint8 {
	switch v := loc.(type) {
	case uint8:
		return rom.data[v]
	case uint16:
		return rom.data[v]
	case uint32:
		return rom.data[v]
	case uint64:
		return rom.data[v]
	default:
		panic(v)
	}
}
func NewROMSegment(data []uint8) *ROMSegment {
	return &ROMSegment{
		data: data,
	}
}
