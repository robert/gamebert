package main

import (
	"fmt"
)

var opcodes = LoadOpcodes()

type CPU struct {
	a *Register8Bit
	b *Register8Bit
	c *Register8Bit
	d *Register8Bit
	e *Register8Bit
	f *Register8Bit
	h *Register8Bit
	l *Register8Bit

	bc *UnifiedRegister16Bit
	de *UnifiedRegister16Bit
	hl *UnifiedRegister16Bit
	af *UnifiedRegister16Bit

	zFlag *Flag
	nFlag *Flag
	hFlag *Flag
	cFlag *Flag

	pc *Register16Bit
	sp *Register16Bit

	interruptsTriggered *Register8Bit
	interruptsEnabled   *Register8Bit

	intTriggeredVBlank *Flag
	intTriggeredStat   *Flag
	intTriggeredTimer  *Flag
	intTriggeredSerial *Flag
	intTriggeredJoypad *Flag

	intEnabledVBlank *Flag
	intEnabledStat   *Flag
	intEnabledTimer  *Flag
	intEnabledSerial *Flag
	intEnabledJoypad *Flag

	masterInterruptsEnabled bool
	halted                  bool

	mb *Motherboard
}

func NewCPU(motherboard *Motherboard) *CPU {
	a := &Register8Bit{name: "a"}
	b := &Register8Bit{name: "b"}
	c := &Register8Bit{name: "c"}
	d := &Register8Bit{name: "d"}
	e := &Register8Bit{name: "e"}
	f := &Register8Bit{name: "f", mask: 0b11110000}
	h := &Register8Bit{name: "h"}
	l := &Register8Bit{name: "l"}

	bc := &UnifiedRegister16Bit{hi: b, lo: c}
	de := &UnifiedRegister16Bit{hi: d, lo: e}
	hl := &UnifiedRegister16Bit{hi: h, lo: l}
	af := &UnifiedRegister16Bit{hi: a, lo: f}

	zFlag := &Flag{reg: f, offset: 7, name: "z"}
	nFlag := &Flag{reg: f, offset: 6, name: "n"}
	hFlag := &Flag{reg: f, offset: 5, name: "h"}
	cFlag := &Flag{reg: f, offset: 4, name: "c"}

	pc := &Register16Bit{name: "pc"}

	interruptsTriggered := &Register8Bit{}
	interruptsEnabled := &Register8Bit{}

	return &CPU{
		a: a,
		b: b,
		c: c,
		d: d,
		e: e,
		f: f,
		h: h,
		l: l,

		bc: bc,
		de: de,
		hl: hl,
		af: af,

		zFlag: zFlag,
		nFlag: nFlag,
		hFlag: hFlag,
		cFlag: cFlag,

		pc: pc,
		sp: &Register16Bit{name: "sp"},

		interruptsTriggered: interruptsTriggered,
		interruptsEnabled:   interruptsEnabled,

		intTriggeredVBlank: &Flag{reg: interruptsTriggered, offset: 0, name: "itvb"},
		intTriggeredStat:   &Flag{reg: interruptsTriggered, offset: 1, name: "itst"},
		intTriggeredTimer:  &Flag{reg: interruptsTriggered, offset: 2, name: "itti"},
		intTriggeredSerial: &Flag{reg: interruptsTriggered, offset: 3, name: "itse"},
		intTriggeredJoypad: &Flag{reg: interruptsTriggered, offset: 4, name: "itjo"},

		intEnabledVBlank: &Flag{reg: interruptsEnabled, offset: 0, name: "ievb"},
		intEnabledStat:   &Flag{reg: interruptsEnabled, offset: 1, name: "iest"},
		intEnabledTimer:  &Flag{reg: interruptsEnabled, offset: 2, name: "ieti"},
		intEnabledSerial: &Flag{reg: interruptsEnabled, offset: 3, name: "iese"},
		intEnabledJoypad: &Flag{reg: interruptsEnabled, offset: 4, name: "iejo"},

		masterInterruptsEnabled: true,
		halted:                  false,

		mb: motherboard,
	}
}

func (cpu *CPU) initToPostBootROM() {
	cpu.a.write(0x01)
	cpu.f.write(0xB0)
	cpu.b.write(0x00)
	cpu.c.write(0x13)
	cpu.d.write(0x00)
	cpu.e.write(0xD8)
	cpu.h.write(0x01)
	cpu.l.write(0x4D)
	cpu.sp.write(0xFFFE)
	cpu.pc.write(0x0100)
}

func (cpu *CPU) tick() uint8 {
	// Use a mask because only the first 5 bits of the interrupt flags
	// are used.
	mask := uint8(0b11111)

	// Are any interrupts both triggered and enabled?
	if (cpu.interruptsTriggered.read()&mask)&(cpu.interruptsEnabled.read()&mask) != 0 {
		cpu.halted = false

		cpu.maybeHandleInterrupt(cpu.intTriggeredVBlank, cpu.intEnabledVBlank, 0x0040)
		cpu.maybeHandleInterrupt(cpu.intTriggeredStat, cpu.intEnabledStat, 0x0048)
		cpu.maybeHandleInterrupt(cpu.intTriggeredTimer, cpu.intEnabledTimer, 0x0050)
		cpu.maybeHandleInterrupt(cpu.intTriggeredSerial, cpu.intEnabledSerial, 0x0058)
		cpu.maybeHandleInterrupt(cpu.intTriggeredJoypad, cpu.intEnabledJoypad, 0x0060)
	}

	if !cpu.halted {
		return cpu.fetchAndExecute()
	} else {
		return 4
	}
}

func (cpu *CPU) maybeHandleInterrupt(triggeredFlag *Flag, enabledFlag *Flag, jumpToAddr uint16) bool {
	if triggeredFlag.read() && enabledFlag.read() {
		// TODO: handle halted
		if cpu.masterInterruptsEnabled {
			triggeredFlag.write(false)

			// TODO: refactor into generic `call` type method
			cpu.mb.writeWord(cpu.sp.read()-2, cpu.pc.read())
			cpu.sp.dec(2)

			cpu.pc.write(jumpToAddr)

			cpu.masterInterruptsEnabled = false
		}

		return true
	} else {
		return false
	}
}

func (cpu *CPU) fetchAndExecute() uint8 {
	op := cpu.nextOp()

	cpu.pc.inc(op.bytesConsumed())
	cycles := cpu.executeOp(op)

	return cycles
}

func (cpu *CPU) nextOp() *operation {
	pc := cpu.pc.read()
	opcodeAddr := cpu.mb.readByte(pc)
	ann := opcodes.GetUnprefixed(opcodeAddr)

	var cbAnn *Opcode
	if opcodeAddr == 0xCB {
		cbOpcodeAddr := cpu.mb.readByte(pc + 1)
		cbAnn = opcodes.GetCbPrefixed(cbOpcodeAddr)
	}

	return &operation{
		opcode:   ann,
		cbOpcode: cbAnn,
		pc:       pc,
		mb:       cpu.mb,
	}
}

func (cpu *CPU) inc8(rw RW8Bit) {
	oldVal := rw.read()
	newVal := oldVal + 1

	rw.write(newVal)

	hFlag := isHalfCarry8(oldVal, 1)

	cpu.zFlag.write(newVal == 0)
	cpu.nFlag.write(false)
	cpu.hFlag.write(hFlag)
}

func (cpu *CPU) dec8(rw RW8Bit) {
	oldVal := rw.read()
	newVal := oldVal - 1

	rw.write(newVal)

	cpu.zFlag.write(newVal == 0)
	cpu.nFlag.write(true)
	cpu.hFlag.write(isHalfBorrow8(oldVal, 1))
}

func (cpu *CPU) inc16(rw RW16Bit) {
	rw.write(rw.read() + 1)
}

func (cpu *CPU) dec16(rw RW16Bit) {
	rw.write(rw.read() - 1)
}

func (cpu *CPU) ld8(dst RW8Bit, src R8Bit) {
	dst.write(src.read())
}

func (cpu *CPU) ld16(dst RW16Bit, src R16Bit) {
	dst.write(src.read())
}

func (cpu *CPU) jr(src R8Bit) {
	srcVal := src.read()

	signedD8 := int8(srcVal)

	if signedD8 > 0 {
		cpu.pc.inc(uint16(signedD8))
	} else {
		cpu.pc.dec(uint16(-signedD8))
	}
}

func (cpu *CPU) jrCond(cond bool, src R8Bit) {
	if cond {
		cpu.jr(src)
	}
}

func (cpu *CPU) jp(src R16Bit) {
	cpu.pc.write(src.read())
}

func (cpu *CPU) jpCond(cond bool, src R16Bit) {
	if cond {
		cpu.jp(src)
	}
}

func (cpu *CPU) rl(src RW8Bit) {
	rotated, oldBit7 := rotateThroughL8(src.read(), cpu.cFlag.read())
	src.write(rotated)

	cpu.zFlag.write(rotated == 0)
	cpu.nFlag.write(false)
	cpu.hFlag.write(false)
	cpu.cFlag.write(oldBit7)
}

func (cpu *CPU) rla() {
	rotated, oldBit7 := rotateThroughL8(cpu.a.read(), cpu.cFlag.read())
	cpu.a.write(rotated)

	cpu.zFlag.write(false)
	cpu.nFlag.write(false)
	cpu.hFlag.write(false)
	cpu.cFlag.write(oldBit7)
}

func (cpu *CPU) rlc(src RW8Bit) {
	rotated := rotateL8(src.read())
	src.write(rotated)

	cpu.zFlag.write(rotated == 0)
	cpu.nFlag.write(false)
	cpu.hFlag.write(false)
	cpu.cFlag.write(isBitSet8(rotated, 0))
}

func (cpu *CPU) rlca() {
	rotated := rotateL8(cpu.a.read())
	cpu.a.write(rotated)

	cpu.zFlag.write(false)
	cpu.nFlag.write(false)
	cpu.hFlag.write(false)
	cpu.cFlag.write(isBitSet8(rotated, 0))
}

func (cpu *CPU) rrc(src RW8Bit) {
	rotated := rotateR8(src.read())
	src.write(rotated)

	cpu.zFlag.write(rotated == 0)
	cpu.nFlag.write(false)
	cpu.hFlag.write(false)
	cpu.cFlag.write(isBitSet8(rotated, 7))
}

func (cpu *CPU) rrca() {
	rotated := rotateR8(cpu.a.read())
	cpu.a.write(rotated)

	cpu.zFlag.write(false)
	cpu.nFlag.write(false)
	cpu.hFlag.write(false)
	cpu.cFlag.write(isBitSet8(rotated, 7))
}

func (cpu *CPU) rr(src RW8Bit) {
	newVal, oldBit0 := rotateThroughR8(src.read(), cpu.cFlag.read())
	src.write(newVal)

	cpu.zFlag.write(newVal == 0)
	cpu.nFlag.write(false)
	cpu.hFlag.write(false)
	cpu.cFlag.write(oldBit0)
}

func (cpu *CPU) sla(src RW8Bit) {
	newVal, oldBit7 := shiftL8(src.read())
	src.write(newVal)

	cpu.zFlag.write(newVal == 0)
	cpu.nFlag.write(false)
	cpu.hFlag.write(false)
	cpu.cFlag.write(oldBit7)
}

func (cpu *CPU) sra(src RW8Bit) {
	oldVal := src.read()
	newVal, oldBit0 := shiftR8(oldVal)
	if isBitSet8(oldVal, 7) {
		newVal += (1 << 7)
	}
	src.write(newVal)

	cpu.zFlag.write(newVal == 0)
	cpu.nFlag.write(false)
	cpu.hFlag.write(false)
	cpu.cFlag.write(oldBit0)
}

func (cpu *CPU) srl(src RW8Bit) {
	newVal, oldBit0 := shiftR8(src.read())
	src.write(newVal)

	cpu.zFlag.write(newVal == 0)
	cpu.nFlag.write(false)
	cpu.hFlag.write(false)
	cpu.cFlag.write(oldBit0)
}

func (cpu *CPU) swap(src RW8Bit) {
	oldVal := src.read()
	newVal := oldVal>>4 + (oldVal&0xF)<<4

	src.write(newVal)

	cpu.zFlag.write(newVal == 0)
	cpu.nFlag.write(false)
	cpu.hFlag.write(false)
	cpu.cFlag.write(false)
}

func (cpu *CPU) cp(src R8Bit) {
	a := cpu.a.read()
	srcVal := src.read()

	cpu.zFlag.write(a == srcVal)
	cpu.nFlag.write(true)
	cpu.hFlag.write(isHalfBorrow8(a, srcVal))
	cpu.cFlag.write(a < srcVal)
}

func (cpu *CPU) add8(dst RW8Bit, src R8Bit) {
	oldDstVal := dst.read()
	srcVal := src.read()

	newDstVal := oldDstVal + srcVal

	dst.write(newDstVal)

	cpu.zFlag.write(newDstVal == 0)
	cpu.nFlag.write(false)
	cpu.hFlag.write(isHalfCarry8(oldDstVal, srcVal))
	cpu.cFlag.write(newDstVal < oldDstVal)
}

func (cpu *CPU) adc(dst RW8Bit, src R8Bit) {
	oldDstVal := dst.read()
	srcVal := src.read()

	cFlagVal := uint8(0)
	if cpu.cFlag.read() {
		cFlagVal = 1
	}

	newDstVal := oldDstVal + srcVal + cFlagVal

	dst.write(newDstVal)

	hFlag := isBitSet8(oldDstVal&0xF+srcVal&0xF+cFlagVal&0xF, 4)

	cpu.zFlag.write(newDstVal == 0)
	cpu.nFlag.write(false)
	cpu.hFlag.write(hFlag)
	cpu.cFlag.write(newDstVal <= oldDstVal && (srcVal != 0 || cFlagVal != 0))
}

func (cpu *CPU) add16(dst RW16Bit, src R16Bit) {
	oldDstVal := dst.read()
	srcVal := src.read()

	newDstVal := oldDstVal + srcVal

	dst.write(newDstVal)

	hFlag := isHalfCarry16(oldDstVal, srcVal)
	cFlag := newDstVal < oldDstVal

	cpu.nFlag.write(false)
	cpu.hFlag.write(hFlag)
	cpu.cFlag.write(cFlag)
}

func (cpu *CPU) or(src R8Bit) {
	newA := cpu.a.read() | src.read()

	cpu.a.write(newA)

	cpu.zFlag.write(newA == 0)
	cpu.nFlag.write(false)
	cpu.hFlag.write(false)
	cpu.cFlag.write(false)
}

func (cpu *CPU) xor(src R8Bit) {
	newA := cpu.a.read() ^ src.read()

	cpu.a.write(newA)

	cpu.zFlag.write(newA == 0)
	cpu.nFlag.write(false)
	cpu.hFlag.write(false)
	cpu.cFlag.write(false)
}

func (cpu *CPU) and(src R8Bit) {
	newVal := cpu.a.read() & src.read()
	cpu.a.write(newVal)

	cpu.zFlag.write(newVal == 0)
	cpu.nFlag.write(false)
	cpu.hFlag.write(true)
	cpu.cFlag.write(false)
}

func (cpu *CPU) push(src R16Bit) {
	cpu.mb.writeWord(cpu.sp.read()-2, src.read())
	cpu.sp.dec(2)
}

func (cpu *CPU) pop(dst RW16Bit) {
	sp := cpu.mb.readWord(cpu.sp.read())
	cpu.sp.inc(2)

	dst.write(sp)
}

func (cpu *CPU) sub(src R8Bit) {
	oldAVal := cpu.a.read()
	srcVal := src.read()

	newAVal := oldAVal - srcVal

	cpu.a.write(newAVal)

	cpu.zFlag.write(newAVal == 0)
	cpu.nFlag.write(true)
	cpu.hFlag.write(isHalfBorrow8(oldAVal, srcVal))
	cpu.cFlag.write(srcVal > oldAVal)
}

func (cpu *CPU) sbc(src R8Bit) {
	oldAVal := cpu.a.read()

	cFlagVal := cpu.cFlag.readUint8()
	srcVal := src.read()

	newAVal := oldAVal - srcVal - cFlagVal

	cpu.a.write(newAVal)

	hFlag := oldAVal&0xF < srcVal&0xF+cFlagVal

	cpu.zFlag.write(newAVal == 0)
	cpu.nFlag.write(true)
	cpu.hFlag.write(hFlag)
	cpu.cFlag.write(newAVal >= oldAVal && (srcVal != 0 || cFlagVal != 0))
}

func (cpu *CPU) rst(src R16Bit) {
	cpu.mb.writeWord(cpu.sp.read()-2, cpu.pc.read())
	cpu.sp.dec(2)

	cpu.pc.write(src.read())
}

func (cpu *CPU) ret(cond bool) {
	if cond {
		valAtSp := cpu.mb.readWord(cpu.sp.read())
		cpu.pc.write(valAtSp)

		cpu.sp.inc(2)
	}
}

func (cpu *CPU) call(src R16Bit) {
	cpu.mb.writeWord(cpu.sp.read()-2, cpu.pc.read())
	cpu.sp.dec(2)

	cpu.pc.write(src.read())
}

func (cpu *CPU) bit(src R8Bit, bitN uint8) {
	cpu.zFlag.write(!isBitSet8(src.read(), bitN))
	cpu.nFlag.write(false)
	cpu.hFlag.write(true)
}

func (cpu *CPU) res(src RW8Bit, bitN uint8) {
	oldVal := src.read()
	newVal := oldVal & (uint8(0xFF) - (1 << bitN))

	src.write(newVal)
}

func (cpu *CPU) set(src RW8Bit, bitN uint8) {
	oldVal := src.read()
	newVal := oldVal | 1<<bitN

	src.write(newVal)
}

func (cpu *CPU) executeOp(op *operation) uint8 {
	var cyclesOverride uint8

	opcode := op.opcode

	assertSig := func(believedSig string) {
		// if believedSig != opcode.Sig() {
		// 	panic(fmt.Sprintf("Believed:\t%s Actual:\t%s", believedSig, opcode.Sig()))
		// }
	}

	switch opcode.Addr {
	case 0x00:
		assertSig("NOP")

	case 0x01:
		assertSig("LD BC d16")
		cpu.ld16(cpu.bc, op.d16Val())

	case 0x02:
		assertSig("LD (BC) A")
		cpu.ld8(cpu.byteAt(cpu.bc.read()), cpu.a)

	case 0x03:
		assertSig("INC BC")
		cpu.inc16(cpu.bc)

	case 0x04:
		assertSig("INC B")
		cpu.inc8(cpu.b)

	case 0x05:
		assertSig("DEC B")
		cpu.dec8(cpu.b)

	case 0x06:
		assertSig("LD B d8")
		cpu.ld8(cpu.b, op.d8Val())

	case 0x07:
		assertSig("RLCA")
		cpu.rlca()

	case 0x08:
		assertSig("LD (a16) SP")
		cpu.ld16(op.wordAtd16(), cpu.sp)

	case 0x09:
		assertSig("ADD HL BC")
		cpu.add16(cpu.hl, cpu.bc)

	case 0x0A:
		assertSig("LD A (BC)")
		cpu.ld8(cpu.a, cpu.byteAt(cpu.bc.read()))

	case 0x0B:
		assertSig("DEC BC")
		cpu.dec16(cpu.bc)

	case 0x0C:
		assertSig("INC C")
		cpu.inc8(cpu.c)

	case 0x0D:
		assertSig("DEC C")
		cpu.dec8(cpu.c)

	case 0x0E:
		assertSig("LD C d8")
		cpu.ld8(cpu.c, op.d8Val())

	case 0x0F:
		assertSig("RRCA")
		cpu.rrca()

	case 0x10:
		assertSig("STOP 0")

	case 0x11:
		assertSig("LD DE d16")
		cpu.ld16(cpu.de, op.d16Val())

	case 0x12:
		assertSig("LD (DE) A")
		cpu.ld8(cpu.byteAt(cpu.de.read()), cpu.a)

	case 0x13:
		assertSig("INC DE")
		cpu.inc16(cpu.de)

	case 0x14:
		assertSig("INC D")
		cpu.inc8(cpu.d)

	case 0x15:
		assertSig("DEC D")
		cpu.dec8(cpu.d)

	case 0x16:
		assertSig("LD D d8")
		cpu.ld8(cpu.d, op.d8Val())

	case 0x17:
		assertSig("RLA")
		cpu.rla()

	case 0x18:
		assertSig("JR r8")
		cpu.jr(op.d8Val())

	case 0x19:
		assertSig("ADD HL DE")
		cpu.add16(cpu.hl, cpu.de)

	case 0x1A:
		assertSig("LD A (DE)")
		cpu.ld8(cpu.a, cpu.byteAt(cpu.de.read()))

	case 0x1B:
		assertSig("DEC DE")
		cpu.dec16(cpu.de)

	case 0x1C:
		assertSig("INC E")
		cpu.inc8(cpu.e)

	case 0x1D:
		assertSig("DEC E")
		cpu.dec8(cpu.e)

	case 0x1E:
		assertSig("LD E d8")
		cpu.ld8(cpu.e, op.d8Val())

	case 0x1F:
		assertSig("RRA")
		rotated, oldBit0 := rotateThroughR8(cpu.a.read(), cpu.cFlag.read())

		cpu.a.write(rotated)

		cpu.zFlag.write(false)
		cpu.nFlag.write(false)
		cpu.hFlag.write(false)
		cpu.cFlag.write(oldBit0)

	case 0x20:
		assertSig("JR NZ r8")
		cond := !cpu.zFlag.read()
		cpu.jrCond(cond, op.d8Val())

		if cond {
			cyclesOverride = 12
		} else {
			cyclesOverride = 8
		}

	case 0x21:
		assertSig("LD HL d16")
		cpu.hl.write(op.d16Val().read())

	case 0x22:
		assertSig("LD (HL+) A")
		cpu.ld8(cpu.byteAt(cpu.hl.read()), cpu.a)
		cpu.hl.inc(1)

	case 0x23:
		assertSig("INC HL")
		cpu.inc16(cpu.hl)

	case 0x24:
		assertSig("INC H")
		cpu.inc8(cpu.h)

	case 0x25:
		assertSig("DEC H")
		cpu.dec8(cpu.h)

	case 0x26:
		assertSig("LD H d8")
		cpu.ld8(cpu.h, op.d8Val())

	case 0x27:
		assertSig("DAA")

		// https://ehaskins.com/2018-01-30%20Z80%20DAA/ is helpful

		t := cpu.a.read()
		corr := uint8(0)
		if cpu.hFlag.read() {
			corr |= 0x06
		}
		if cpu.cFlag.read() {
			corr |= 0x60
		}

		if cpu.nFlag.read() {
			t -= corr
		} else {
			if t&0x0F > 0x09 {
				corr |= 0x06
			}
			if t > 0x99 {
				corr |= 0x60
			}
			t += corr
		}

		cpu.a.write(t)

		cpu.zFlag.write(t == 0)
		cpu.hFlag.write(false)
		cpu.cFlag.write(corr&0x60 != 0)

	case 0x28:
		assertSig("JR Z r8")

		cond := cpu.zFlag.read()
		cpu.jrCond(cond, op.d8Val())

		if cond {
			cyclesOverride = 12
		} else {
			cyclesOverride = 8
		}

	case 0x29:
		assertSig("ADD HL HL")
		cpu.add16(cpu.hl, cpu.hl)

	case 0x2A:
		assertSig("LD A (HL+)")
		cpu.ld8(cpu.a, cpu.byteAt(cpu.hl.read()))
		cpu.hl.inc(1)

	case 0x2B:
		assertSig("DEC HL")
		cpu.hl.dec(1)

	case 0x2C:
		assertSig("INC L")
		cpu.inc8(cpu.l)

	case 0x2D:
		assertSig("DEC L")
		cpu.dec8(cpu.l)

	case 0x2E:
		assertSig("LD L d8")
		cpu.ld8(cpu.l, op.d8Val())

	case 0x2F:
		assertSig("CPL")

		cpu.a.write(cpu.a.read() ^ 0xFF)
		cpu.nFlag.write(true)
		cpu.hFlag.write(true)

	case 0x30:
		assertSig("JR NC r8")

		cond := !cpu.cFlag.read()
		cpu.jrCond(cond, op.d8Val())

		if cond {
			cyclesOverride = 12
		} else {
			cyclesOverride = 8
		}

	case 0x31:
		assertSig("LD SP d16")
		cpu.ld16(cpu.sp, op.d16Val())

	case 0x32:
		assertSig("LD (HL-) A")
		cpu.ld8(cpu.byteAt(cpu.hl.read()), cpu.a)
		cpu.hl.dec(1)

	case 0x33:
		assertSig("INC SP")
		cpu.inc16(cpu.sp)

	case 0x34:
		assertSig("INC (HL)")
		cpu.inc8(cpu.byteAt(cpu.hl.read()))

	case 0x35:
		assertSig("DEC (HL)")
		cpu.dec8(cpu.byteAt(cpu.hl.read()))

	case 0x36:
		assertSig("LD (HL) d8")
		cpu.ld8(cpu.byteAt(cpu.hl.read()), op.d8Val())

	case 0x37:
		assertSig("SCF")
		cpu.nFlag.write(false)
		cpu.hFlag.write(false)
		cpu.cFlag.write(true)

	case 0x38:
		assertSig("JR C r8")

		cond := cpu.cFlag.read()
		cpu.jrCond(cond, op.d8Val())

		if cond {
			cyclesOverride = 12
		} else {
			cyclesOverride = 8
		}

	case 0x39:
		assertSig("ADD HL SP")
		cpu.add16(cpu.hl, cpu.sp)

	case 0x3A:
		assertSig("LD A (HL-)")
		cpu.ld8(cpu.a, cpu.byteAt(cpu.hl.read()))
		cpu.hl.dec(1)

	case 0x3B:
		assertSig("DEC SP")
		cpu.dec16(cpu.sp)

	case 0x3C:
		assertSig("INC A")
		cpu.inc8(cpu.a)

	case 0x3D:
		assertSig("DEC A")
		cpu.dec8(cpu.a)

	case 0x3E:
		assertSig("LD A d8")
		cpu.ld8(cpu.a, op.d8Val())

	case 0x3F:
		assertSig("CCF")
		cpu.nFlag.write(false)
		cpu.hFlag.write(false)
		cpu.cFlag.write(!cpu.cFlag.read())

	case 0x40:
		assertSig("LD B B")
		cpu.ld8(cpu.b, cpu.b)

	case 0x41:
		assertSig("LD B C")
		cpu.ld8(cpu.b, cpu.c)

	case 0x42:
		assertSig("LD B D")
		cpu.ld8(cpu.b, cpu.d)

	case 0x43:
		assertSig("LD B E")
		cpu.ld8(cpu.b, cpu.e)

	case 0x44:
		assertSig("LD B H")
		cpu.ld8(cpu.b, cpu.h)

	case 0x45:
		assertSig("LD B L")
		cpu.ld8(cpu.b, cpu.l)

	case 0x46:
		assertSig("LD B (HL)")
		cpu.ld8(cpu.b, cpu.byteAt(cpu.hl.read()))

	case 0x47:
		assertSig("LD B A")
		cpu.ld8(cpu.b, cpu.a)

	case 0x48:
		assertSig("LD C B")
		cpu.ld8(cpu.c, cpu.b)

	case 0x49:
		assertSig("LD C C")
		cpu.ld8(cpu.c, cpu.c)

	case 0x4A:
		assertSig("LD C D")
		cpu.ld8(cpu.c, cpu.d)

	case 0x4B:
		assertSig("LD C E")
		cpu.ld8(cpu.c, cpu.e)

	case 0x4C:
		assertSig("LD C H")
		cpu.ld8(cpu.c, cpu.h)

	case 0x4D:
		assertSig("LD C L")
		cpu.ld8(cpu.c, cpu.l)

	case 0x4E:
		assertSig("LD C (HL)")
		cpu.ld8(cpu.c, cpu.byteAt(cpu.hl.read()))

	case 0x4F:
		assertSig("LD C A")
		cpu.ld8(cpu.c, cpu.a)

	case 0x50:
		assertSig("LD D B")
		cpu.ld8(cpu.d, cpu.b)

	case 0x51:
		assertSig("LD D C")
		cpu.ld8(cpu.d, cpu.c)

	case 0x52:
		assertSig("LD D D")
		cpu.ld8(cpu.d, cpu.d)

	case 0x53:
		assertSig("LD D E")
		cpu.ld8(cpu.d, cpu.e)

	case 0x54:
		assertSig("LD D H")
		cpu.ld8(cpu.d, cpu.h)

	case 0x55:
		assertSig("LD D L")
		cpu.ld8(cpu.d, cpu.l)

	case 0x56:
		assertSig("LD D (HL)")
		cpu.ld8(cpu.d, cpu.byteAt(cpu.hl.read()))

	case 0x57:
		assertSig("LD D A")
		cpu.ld8(cpu.d, cpu.a)

	case 0x58:
		assertSig("LD E B")
		cpu.ld8(cpu.e, cpu.b)

	case 0x59:
		assertSig("LD E C")
		cpu.ld8(cpu.e, cpu.c)

	case 0x5A:
		assertSig("LD E D")
		cpu.ld8(cpu.e, cpu.d)

	case 0x5B:
		assertSig("LD E E")
		cpu.ld8(cpu.e, cpu.e)

	case 0x5C:
		assertSig("LD E H")
		cpu.ld8(cpu.e, cpu.h)

	case 0x5D:
		assertSig("LD E L")
		cpu.ld8(cpu.e, cpu.l)

	case 0x5E:
		assertSig("LD E (HL)")
		cpu.ld8(cpu.e, cpu.byteAt(cpu.hl.read()))

	case 0x5F:
		assertSig("LD E A")
		cpu.ld8(cpu.e, cpu.a)

	case 0x60:
		assertSig("LD H B")
		cpu.ld8(cpu.h, cpu.b)

	case 0x61:
		assertSig("LD H C")
		cpu.ld8(cpu.h, cpu.c)

	case 0x62:
		assertSig("LD H D")
		cpu.ld8(cpu.h, cpu.d)

	case 0x63:
		assertSig("LD H E")
		cpu.ld8(cpu.h, cpu.e)

	case 0x64:
		assertSig("LD H H")
		cpu.ld8(cpu.h, cpu.h)

	case 0x65:
		assertSig("LD H L")
		cpu.ld8(cpu.h, cpu.l)

	case 0x66:
		assertSig("LD H (HL)")
		cpu.ld8(cpu.h, cpu.byteAt(cpu.hl.read()))

	case 0x67:
		assertSig("LD H A")
		cpu.ld8(cpu.h, cpu.a)

	case 0x68:
		assertSig("LD L B")
		cpu.ld8(cpu.l, cpu.b)

	case 0x69:
		assertSig("LD L C")
		cpu.ld8(cpu.l, cpu.c)

	case 0x6A:
		assertSig("LD L D")
		cpu.ld8(cpu.l, cpu.d)

	case 0x6B:
		assertSig("LD L E")
		cpu.ld8(cpu.l, cpu.e)

	case 0x6C:
		assertSig("LD L H")
		cpu.ld8(cpu.l, cpu.h)

	case 0x6D:
		assertSig("LD L L")
		cpu.ld8(cpu.l, cpu.l)

	case 0x6E:
		assertSig("LD L (HL)")
		cpu.ld8(cpu.l, cpu.byteAt(cpu.hl.read()))

	case 0x6F:
		assertSig("LD L A")
		cpu.ld8(cpu.l, cpu.a)

	case 0x70:
		assertSig("LD (HL) B")
		cpu.ld8(cpu.byteAt(cpu.hl.read()), cpu.b)

	case 0x71:
		assertSig("LD (HL) C")
		cpu.ld8(cpu.byteAt(cpu.hl.read()), cpu.c)

	case 0x72:
		assertSig("LD (HL) D")
		cpu.ld8(cpu.byteAt(cpu.hl.read()), cpu.d)

	case 0x73:
		assertSig("LD (HL) E")
		cpu.ld8(cpu.byteAt(cpu.hl.read()), cpu.e)

	case 0x74:
		assertSig("LD (HL) H")
		cpu.ld8(cpu.byteAt(cpu.hl.read()), cpu.h)

	case 0x75:
		assertSig("LD (HL) L")
		cpu.ld8(cpu.byteAt(cpu.hl.read()), cpu.l)

	case 0x76:
		assertSig("HALT")
		cpu.halted = true

	case 0x77:
		assertSig("LD (HL) A")
		cpu.ld8(cpu.byteAt(cpu.hl.read()), cpu.a)

	case 0x78:
		assertSig("LD A B")
		cpu.ld8(cpu.a, cpu.b)

	case 0x79:
		assertSig("LD A C")
		cpu.ld8(cpu.a, cpu.c)

	case 0x7A:
		assertSig("LD A D")
		cpu.ld8(cpu.a, cpu.d)

	case 0x7B:
		assertSig("LD A E")
		cpu.ld8(cpu.a, cpu.e)

	case 0x7C:
		assertSig("LD A H")
		cpu.ld8(cpu.a, cpu.h)

	case 0x7D:
		assertSig("LD A L")
		cpu.ld8(cpu.a, cpu.l)

	case 0x7E:
		assertSig("LD A (HL)")
		cpu.ld8(cpu.a, cpu.byteAt(cpu.hl.read()))

	case 0x7F:
		assertSig("LD A A")
		cpu.ld8(cpu.a, cpu.a)

	case 0x80:
		assertSig("ADD A B")
		cpu.add8(cpu.a, cpu.b)

	case 0x81:
		assertSig("ADD A C")
		cpu.add8(cpu.a, cpu.c)

	case 0x82:
		assertSig("ADD A D")
		cpu.add8(cpu.a, cpu.d)

	case 0x83:
		assertSig("ADD A E")
		cpu.add8(cpu.a, cpu.e)

	case 0x84:
		assertSig("ADD A H")
		cpu.add8(cpu.a, cpu.h)

	case 0x85:
		assertSig("ADD A L")
		cpu.add8(cpu.a, cpu.l)

	case 0x86:
		assertSig("ADD A (HL)")
		cpu.add8(cpu.a, cpu.byteAt(cpu.hl.read()))

	case 0x87:
		assertSig("ADD A A")
		cpu.add8(cpu.a, cpu.a)

	case 0x88:
		assertSig("ADC A B")
		cpu.adc(cpu.a, cpu.b)

	case 0x89:
		assertSig("ADC A C")
		cpu.adc(cpu.a, cpu.c)

	case 0x8A:
		assertSig("ADC A D")
		cpu.adc(cpu.a, cpu.d)

	case 0x8B:
		assertSig("ADC A E")
		cpu.adc(cpu.a, cpu.e)

	case 0x8C:
		assertSig("ADC A H")
		cpu.adc(cpu.a, cpu.h)

	case 0x8D:
		assertSig("ADC A L")
		cpu.adc(cpu.a, cpu.l)

	case 0x8E:
		assertSig("ADC A (HL)")
		cpu.adc(cpu.a, cpu.byteAt(cpu.hl.read()))

	case 0x8F:
		assertSig("ADC A A")
		cpu.adc(cpu.a, cpu.a)

	case 0x90:
		assertSig("SUB B")
		cpu.sub(cpu.b)

	case 0x91:
		assertSig("SUB C")
		cpu.sub(cpu.c)

	case 0x92:
		assertSig("SUB D")
		cpu.sub(cpu.d)

	case 0x93:
		assertSig("SUB E")
		cpu.sub(cpu.e)

	case 0x94:
		assertSig("SUB H")
		cpu.sub(cpu.h)

	case 0x95:
		assertSig("SUB L")
		cpu.sub(cpu.l)

	case 0x96:
		assertSig("SUB (HL)")
		cpu.sub(cpu.byteAt(cpu.hl.read()))

	case 0x97:
		assertSig("SUB A")
		cpu.sub(cpu.a)

	case 0x98:
		assertSig("SBC A B")
		cpu.sbc(cpu.b)

	case 0x99:
		assertSig("SBC A C")
		cpu.sbc(cpu.c)

	case 0x9A:
		assertSig("SBC A D")
		cpu.sbc(cpu.d)

	case 0x9B:
		assertSig("SBC A E")
		cpu.sbc(cpu.e)

	case 0x9C:
		assertSig("SBC A H")
		cpu.sbc(cpu.h)

	case 0x9D:
		assertSig("SBC A L")
		cpu.sbc(cpu.l)

	case 0x9E:
		assertSig("SBC A (HL)")
		cpu.sbc(cpu.byteAt(cpu.hl.read()))

	case 0x9F:
		assertSig("SBC A A")
		cpu.sbc(cpu.a)

	case 0xA0:
		assertSig("AND B")
		cpu.and(cpu.b)

	case 0xA1:
		assertSig("AND C")
		cpu.and(cpu.c)

	case 0xA2:
		assertSig("AND D")
		cpu.and(cpu.d)

	case 0xA3:
		assertSig("AND E")
		cpu.and(cpu.e)

	case 0xA4:
		assertSig("AND H")
		cpu.and(cpu.h)

	case 0xA5:
		assertSig("AND L")
		cpu.and(cpu.l)

	case 0xA6:
		assertSig("AND (HL)")
		cpu.and(cpu.byteAt(cpu.hl.read()))

	case 0xA7:
		assertSig("AND A")
		cpu.and(cpu.a)

	case 0xA8:
		assertSig("XOR B")
		cpu.xor(cpu.b)

	case 0xA9:
		assertSig("XOR C")
		cpu.xor(cpu.c)

	case 0xAA:
		assertSig("XOR D")
		cpu.xor(cpu.d)

	case 0xAB:
		assertSig("XOR E")
		cpu.xor(cpu.e)

	case 0xAC:
		assertSig("XOR H")
		cpu.xor(cpu.h)

	case 0xAD:
		assertSig("XOR L")
		cpu.xor(cpu.l)

	case 0xAE:
		assertSig("XOR (HL)")
		cpu.xor(cpu.byteAt(cpu.hl.read()))

	case 0xAF:
		assertSig("XOR A")
		cpu.xor(cpu.a)

	case 0xB0:
		assertSig("OR B")
		cpu.or(cpu.b)

	case 0xB1:
		assertSig("OR C")
		cpu.or(cpu.c)

	case 0xB2:
		assertSig("OR D")
		cpu.or(cpu.d)

	case 0xB3:
		assertSig("OR E")
		cpu.or(cpu.e)

	case 0xB4:
		assertSig("OR H")
		cpu.or(cpu.h)

	case 0xB5:
		assertSig("OR L")
		cpu.or(cpu.l)

	case 0xB6:
		assertSig("OR (HL)")
		cpu.or(cpu.byteAt(cpu.hl.read()))

	case 0xB7:
		assertSig("OR A")
		cpu.or(cpu.a)

	case 0xB8:
		assertSig("CP B")
		cpu.cp(cpu.b)

	case 0xB9:
		assertSig("CP C")
		cpu.cp(cpu.c)

	case 0xBA:
		assertSig("CP D")
		cpu.cp(cpu.d)

	case 0xBB:
		assertSig("CP E")
		cpu.cp(cpu.e)

	case 0xBC:
		assertSig("CP H")
		cpu.cp(cpu.h)

	case 0xBD:
		assertSig("CP L")
		cpu.cp(cpu.l)

	case 0xBE:
		assertSig("CP (HL)")
		cpu.cp(cpu.byteAt(cpu.hl.read()))

	case 0xBF:
		assertSig("CP A")
		cpu.cp(cpu.a)

	case 0xC0:
		assertSig("RET NZ")
		cond := !cpu.zFlag.read()
		cpu.ret(cond)

		if cond {
			cyclesOverride = 20
		} else {
			cyclesOverride = 8
		}

	case 0xC1:
		assertSig("POP BC")
		cpu.pop(cpu.bc)

	case 0xC2:
		assertSig("JP NZ a16")
		cond := !cpu.zFlag.read()
		cpu.jpCond(cond, op.d16Val())

		if cond {
			cyclesOverride = 16
		} else {
			cyclesOverride = 12
		}

	case 0xC3:
		assertSig("JP a16")
		cpu.jp(op.d16Val())

	case 0xC4:
		assertSig("CALL NZ a16")
		cond := !cpu.zFlag.read()
		if cond {
			cpu.call(op.d16Val())
		}

		if cond {
			cyclesOverride = 24
		} else {
			cyclesOverride = 12
		}

	case 0xC5:
		assertSig("PUSH BC")
		cpu.push(cpu.bc)

	case 0xC6:
		assertSig("ADD A d8")
		cpu.add8(cpu.a, op.d8Val())

	case 0xC7:
		assertSig("RST 00H")
		cpu.rst(AsValue16(0x00))

	case 0xC8:
		assertSig("RET Z")
		cond := cpu.zFlag.read()
		cpu.ret(cond)

		if cond {
			cyclesOverride = 20
		} else {
			cyclesOverride = 8
		}

	case 0xC9:
		assertSig("RET")
		cpu.ret(true)

	case 0xCA:
		assertSig("JP Z a16")
		cond := cpu.zFlag.read()
		cpu.jpCond(cond, op.d16Val())

		if cond {
			cyclesOverride = 16
		} else {
			cyclesOverride = 12
		}

	case 0xCB:
		cyclesOverride = cpu.executeCBOp(op)

	case 0xCC:
		assertSig("CALL Z a16")
		cond := cpu.zFlag.read()
		if cond {
			cpu.call(op.d16Val())
		}

		if cond {
			cyclesOverride = 16
		} else {
			cyclesOverride = 12
		}

	case 0xCD:
		assertSig("CALL a16")
		cpu.call(op.d16Val())

	case 0xCE:
		assertSig("ADC A d8")
		cpu.adc(cpu.a, op.d8Val())

	case 0xCF:
		assertSig("RST 08H")
		cpu.rst(AsValue16(0x08))

	case 0xD0:
		assertSig("RET NC")
		cond := !cpu.cFlag.read()
		cpu.ret(cond)

		if cond {
			cyclesOverride = 20
		} else {
			cyclesOverride = 8
		}

	case 0xD1:
		assertSig("POP DE")
		cpu.pop(cpu.de)

	case 0xD2:
		assertSig("JP NC a16")
		cond := !cpu.cFlag.read()
		cpu.jpCond(cond, op.d16Val())

		if cond {
			cyclesOverride = 16
		} else {
			cyclesOverride = 12
		}

	case 0xD4:
		assertSig("CALL NC a16")
		cond := !cpu.cFlag.read()
		if cond {
			cpu.call(op.d16Val())
		}

		if cond {
			cyclesOverride = 24
		} else {
			cyclesOverride = 12
		}

	case 0xD5:
		assertSig("PUSH DE")
		cpu.push(cpu.de)

	case 0xD6:
		assertSig("SUB d8")
		cpu.sub(op.d8Val())

	case 0xD7:
		assertSig("RST 10H")
		cpu.rst(AsValue16(0x10))

	case 0xD8:
		assertSig("RET C")
		cond := cpu.cFlag.read()
		cpu.ret(cond)

		if cond {
			cyclesOverride = 20
		} else {
			cyclesOverride = 8
		}

	case 0xD9:
		assertSig("RETI")
		cpu.masterInterruptsEnabled = true

		cpu.ret(true)

	case 0xDA:
		assertSig("JP C a16")
		cond := cpu.cFlag.read()
		cpu.jpCond(cond, op.d16Val())

		if cond {
			cyclesOverride = 16
		} else {
			cyclesOverride = 12
		}

	case 0xDC:
		assertSig("CALL C a16")
		cond := cpu.cFlag.read()
		if cond {
			cpu.call(op.d16Val())
		}

		if cond {
			cyclesOverride = 24
		} else {
			cyclesOverride = 12
		}

	case 0xDE:
		assertSig("SBC A d8")
		cpu.sbc(op.d8Val())

	case 0xDF:
		assertSig("RST 18H")
		cpu.rst(AsValue16(0x18))

	case 0xE0:
		assertSig("LDH (a8) A")
		cpu.ld8(op.byteAtd8PlusFF00(), cpu.a)

	case 0xE1:
		assertSig("POP HL")
		sp := cpu.mb.readWord(cpu.sp.read())
		cpu.sp.inc(2)

		cpu.hl.write(sp)

	case 0xE2:
		assertSig("LD (C) A")
		cpu.ld8(cpu.byteAt(0xFF00+uint16(cpu.c.read())), cpu.a)

	case 0xE5:
		assertSig("PUSH HL")
		cpu.push(cpu.hl)

	case 0xE6:
		assertSig("AND d8")
		cpu.and(op.d8Val())

	case 0xE7:
		assertSig("RST 20H")
		cpu.rst(AsValue16(0x20))

	case 0xE8:
		assertSig("ADD SP r8")
		unsignedD8 := op.d8Val().read()
		signedD8 := int8(unsignedD8)

		sp := cpu.sp.read()

		var newSp uint16
		if signedD8 > 0 {
			newSp = sp + uint16(signedD8)
		} else {
			newSp = sp - uint16(-signedD8)
		}
		cpu.sp.write(newSp)

		hFlag := isHalfCarry8(uint8(sp&0xFF), unsignedD8)
		cFlag := uint8(newSp&0xFF) < uint8(sp&0xFF)

		cpu.zFlag.write(false)
		cpu.nFlag.write(false)
		cpu.hFlag.write(hFlag)
		cpu.cFlag.write(cFlag)

	case 0xE9:
		assertSig("JP HL")
		cpu.jp(cpu.hl)

	case 0xEA:
		assertSig("LD (a16) A")
		cpu.ld8(op.byteAtd16(), cpu.a)

	case 0xEE:
		assertSig("XOR d8")
		cpu.xor(op.d8Val())

	case 0xEF:
		assertSig("RST 28H")
		cpu.rst(AsValue16(0x28))

	case 0xF0:
		assertSig("LDH A (a8)")
		cpu.ld8(cpu.a, op.byteAtd8PlusFF00())

	case 0xF1:
		assertSig("POP AF")
		cpu.pop(cpu.af)

	case 0xF2:
		assertSig("LD A (C)")
		cpu.ld8(cpu.a, cpu.byteAt(0xFF00+uint16(cpu.c.read())))

	case 0xF3:
		assertSig("DI")
		cpu.masterInterruptsEnabled = false

	case 0xF5:
		assertSig("PUSH AF")
		cpu.push(cpu.af)

	case 0xF6:
		assertSig("OR d8")
		cpu.or(op.d8Val())

	case 0xF7:
		assertSig("RST 30H")
		cpu.rst(AsValue16(0x30))

	case 0xF8:
		assertSig("LD HL SP+r8")

		unsignedD8 := op.d8Val().read()
		signedD8 := int8(unsignedD8)
		sp := cpu.sp.read()
		newHl := sp
		if signedD8 > 0 {
			newHl += uint16(signedD8)
		} else {
			newHl -= uint16(-signedD8)
		}

		cpu.hl.write(newHl)

		hFlag := isHalfCarry8(uint8(sp&0xFF), unsignedD8)
		cFlag := uint8(newHl&0xFF) < uint8(sp&0xFF)

		cpu.zFlag.write(false)
		cpu.nFlag.write(false)
		cpu.hFlag.write(hFlag)
		cpu.cFlag.write(cFlag)

	case 0xF9:
		assertSig("LD SP HL")
		cpu.ld16(cpu.sp, cpu.hl)

	case 0xFA:
		assertSig("LD A (a16)")
		cpu.ld8(cpu.a, op.byteAtd16())

	case 0xFB:
		assertSig("EI")
		cpu.masterInterruptsEnabled = true

	case 0xFE:
		assertSig("CP d8")
		cpu.cp(op.d8Val())

	case 0xFF:
		assertSig("RST 38H")
		cpu.rst(AsValue16(0x38))

	default:
		panic(fmt.Sprintf("Opcode not implemented: %s", opcode.Addr))
	}

	if len(opcode.Cycles) == 1 {
		return uint8(opcode.Cycles[0])
	} else if cyclesOverride > 0 {
		return cyclesOverride
	} else {
		panic(fmt.Sprintf("%v", opcode))
	}
}

func (cpu *CPU) executeCBOp(op *operation) uint8 {
	var cyclesOverride uint8

	opcode := op.cbOpcode
	assertSig := func(believedSig string) {
		// if believedSig != opcode.Sig() {
		// 	panic(fmt.Sprintf("Believed:\t%s Actual:\t%s", believedSig, opcode.Sig()))
		// }
	}

	switch op.cbOpcode.Addr {
	case 0x00:
		assertSig("RLC B")
		cpu.rlc(cpu.b)

	case 0x01:
		assertSig("RLC C")
		cpu.rlc(cpu.c)

	case 0x02:
		assertSig("RLC D")
		cpu.rlc(cpu.d)

	case 0x03:
		assertSig("RLC E")
		cpu.rlc(cpu.e)

	case 0x04:
		assertSig("RLC H")
		cpu.rlc(cpu.h)

	case 0x05:
		assertSig("RLC L")
		cpu.rlc(cpu.l)

	case 0x06:
		assertSig("RLC (HL)")
		cpu.rlc(cpu.byteAt(cpu.hl.read()))

	case 0x07:
		assertSig("RLC A")
		cpu.rlc(cpu.a)

	case 0x08:
		assertSig("RRC B")
		cpu.rrc(cpu.b)

	case 0x09:
		assertSig("RRC C")
		cpu.rrc(cpu.c)

	case 0x0A:
		assertSig("RRC D")
		cpu.rrc(cpu.d)

	case 0x0B:
		assertSig("RRC E")
		cpu.rrc(cpu.e)

	case 0x0C:
		assertSig("RRC H")
		cpu.rrc(cpu.h)

	case 0x0D:
		assertSig("RRC L")
		cpu.rrc(cpu.l)

	case 0x0E:
		assertSig("RRC (HL)")
		cpu.rrc(cpu.byteAt(cpu.hl.read()))

	case 0x0F:
		assertSig("RRC A")
		cpu.rrc(cpu.a)

	case 0x10:
		assertSig("RL B")
		cpu.rl(cpu.b)

	case 0x11:
		assertSig("RL C")
		cpu.rl(cpu.c)

	case 0x12:
		assertSig("RL D")
		cpu.rl(cpu.d)

	case 0x13:
		assertSig("RL E")
		cpu.rl(cpu.e)

	case 0x14:
		assertSig("RL H")
		cpu.rl(cpu.h)

	case 0x15:
		assertSig("RL L")
		cpu.rl(cpu.l)

	case 0x16:
		assertSig("RL (HL)")
		cpu.rl(cpu.byteAt(cpu.hl.read()))

	case 0x17:
		assertSig("RL A")
		cpu.rl(cpu.a)

	case 0x18:
		assertSig("RR B")
		cpu.rr(cpu.b)

	case 0x19:
		assertSig("RR C")
		cpu.rr(cpu.c)

	case 0x1A:
		assertSig("RR D")
		cpu.rr(cpu.d)

	case 0x1B:
		assertSig("RR E")
		cpu.rr(cpu.e)

	case 0x1C:
		assertSig("RR H")
		cpu.rr(cpu.h)

	case 0x1D:
		assertSig("RR L")
		cpu.rr(cpu.l)

	case 0x1E:
		assertSig("RR (HL)")
		cpu.rr(cpu.byteAt(cpu.hl.read()))

	case 0x1F:
		assertSig("RR A")
		cpu.rr(cpu.a)

	case 0x20:
		assertSig("SLA B")
		cpu.sla(cpu.b)

	case 0x21:
		assertSig("SLA C")
		cpu.sla(cpu.c)

	case 0x22:
		assertSig("SLA D")
		cpu.sla(cpu.d)

	case 0x23:
		assertSig("SLA E")
		cpu.sla(cpu.e)

	case 0x24:
		assertSig("SLA H")
		cpu.sla(cpu.h)

	case 0x25:
		assertSig("SLA L")
		cpu.sla(cpu.l)

	case 0x26:
		assertSig("SLA (HL)")
		cpu.sla(cpu.byteAt(cpu.hl.read()))

	case 0x27:
		assertSig("SLA A")
		cpu.sla(cpu.a)

	case 0x28:
		assertSig("SRA B")
		cpu.sra(cpu.b)

	case 0x29:
		assertSig("SRA C")
		cpu.sra(cpu.c)

	case 0x2A:
		assertSig("SRA D")
		cpu.sra(cpu.d)

	case 0x2B:
		assertSig("SRA E")
		cpu.sra(cpu.e)

	case 0x2C:
		assertSig("SRA H")
		cpu.sra(cpu.h)

	case 0x2D:
		assertSig("SRA L")
		cpu.sra(cpu.l)

	case 0x2E:
		assertSig("SRA (HL)")
		cpu.sra(cpu.byteAt(cpu.hl.read()))

	case 0x2F:
		assertSig("SRA A")
		cpu.sra(cpu.a)

	case 0x30:
		assertSig("SWAP B")
		cpu.swap(cpu.b)

	case 0x31:
		assertSig("SWAP C")
		cpu.swap(cpu.c)

	case 0x32:
		assertSig("SWAP D")
		cpu.swap(cpu.d)

	case 0x33:
		assertSig("SWAP E")
		cpu.swap(cpu.e)

	case 0x34:
		assertSig("SWAP H")
		cpu.swap(cpu.h)

	case 0x35:
		assertSig("SWAP L")
		cpu.swap(cpu.l)

	case 0x36:
		assertSig("SWAP (HL)")
		cpu.swap(cpu.byteAt(cpu.hl.read()))

	case 0x37:
		assertSig("SWAP A")
		cpu.swap(cpu.a)

	case 0x38:
		assertSig("SRL B")
		cpu.srl(cpu.b)

	case 0x39:
		assertSig("SRL C")
		cpu.srl(cpu.c)

	case 0x3A:
		assertSig("SRL D")
		cpu.srl(cpu.d)

	case 0x3B:
		assertSig("SRL E")
		cpu.srl(cpu.e)

	case 0x3C:
		assertSig("SRL H")
		cpu.srl(cpu.h)

	case 0x3D:
		assertSig("SRL L")
		cpu.srl(cpu.l)

	case 0x3E:
		assertSig("SRL (HL)")
		cpu.srl(cpu.byteAt(cpu.hl.read()))

	case 0x3F:
		assertSig("SRL A")
		cpu.srl(cpu.a)

	case 0x40:
		assertSig("BIT 0 B")
		cpu.bit(cpu.b, 0)

	case 0x41:
		assertSig("BIT 0 C")
		cpu.bit(cpu.c, 0)

	case 0x42:
		assertSig("BIT 0 D")
		cpu.bit(cpu.d, 0)

	case 0x43:
		assertSig("BIT 0 E")
		cpu.bit(cpu.e, 0)

	case 0x44:
		assertSig("BIT 0 H")
		cpu.bit(cpu.h, 0)

	case 0x45:
		assertSig("BIT 0 L")
		cpu.bit(cpu.l, 0)

	case 0x46:
		assertSig("BIT 0 (HL)")
		cpu.bit(cpu.byteAt(cpu.hl.read()), 0)

	case 0x47:
		assertSig("BIT 0 A")
		cpu.bit(cpu.a, 0)

	case 0x48:
		assertSig("BIT 1 B")
		cpu.bit(cpu.b, 1)

	case 0x49:
		assertSig("BIT 1 C")
		cpu.bit(cpu.c, 1)

	case 0x4A:
		assertSig("BIT 1 D")
		cpu.bit(cpu.d, 1)

	case 0x4B:
		assertSig("BIT 1 E")
		cpu.bit(cpu.e, 1)

	case 0x4C:
		assertSig("BIT 1 H")
		cpu.bit(cpu.h, 1)

	case 0x4D:
		assertSig("BIT 1 L")
		cpu.bit(cpu.l, 1)

	case 0x4E:
		assertSig("BIT 1 (HL)")
		cpu.bit(cpu.byteAt(cpu.hl.read()), 1)

	case 0x4F:
		assertSig("BIT 1 A")
		cpu.bit(cpu.a, 1)

	case 0x50:
		assertSig("BIT 2 B")
		cpu.bit(cpu.b, 2)

	case 0x51:
		assertSig("BIT 2 C")
		cpu.bit(cpu.c, 2)

	case 0x52:
		assertSig("BIT 2 D")
		cpu.bit(cpu.d, 2)

	case 0x53:
		assertSig("BIT 2 E")
		cpu.bit(cpu.e, 2)

	case 0x54:
		assertSig("BIT 2 H")
		cpu.bit(cpu.h, 2)

	case 0x55:
		assertSig("BIT 2 L")
		cpu.bit(cpu.l, 2)

	case 0x56:
		assertSig("BIT 2 (HL)")
		cpu.bit(cpu.byteAt(cpu.hl.read()), 2)

	case 0x57:
		assertSig("BIT 2 A")
		cpu.bit(cpu.a, 2)

	case 0x58:
		assertSig("BIT 3 B")
		cpu.bit(cpu.b, 3)

	case 0x59:
		assertSig("BIT 3 C")
		cpu.bit(cpu.c, 3)

	case 0x5A:
		assertSig("BIT 3 D")
		cpu.bit(cpu.d, 3)

	case 0x5B:
		assertSig("BIT 3 E")
		cpu.bit(cpu.e, 3)

	case 0x5C:
		assertSig("BIT 3 H")
		cpu.bit(cpu.h, 3)

	case 0x5D:
		assertSig("BIT 3 L")
		cpu.bit(cpu.l, 3)

	case 0x5E:
		assertSig("BIT 3 (HL)")
		cpu.bit(cpu.byteAt(cpu.hl.read()), 3)

	case 0x5F:
		assertSig("BIT 3 A")
		cpu.bit(cpu.a, 3)

	case 0x60:
		assertSig("BIT 4 B")
		cpu.bit(cpu.b, 4)

	case 0x61:
		assertSig("BIT 4 C")
		cpu.bit(cpu.c, 4)

	case 0x62:
		assertSig("BIT 4 D")
		cpu.bit(cpu.d, 4)

	case 0x63:
		assertSig("BIT 4 E")
		cpu.bit(cpu.e, 4)

	case 0x64:
		assertSig("BIT 4 H")
		cpu.bit(cpu.h, 4)

	case 0x65:
		assertSig("BIT 4 L")
		cpu.bit(cpu.l, 4)

	case 0x66:
		assertSig("BIT 4 (HL)")
		cpu.bit(cpu.byteAt(cpu.hl.read()), 4)

	case 0x67:
		assertSig("BIT 4 A")
		cpu.bit(cpu.a, 4)

	case 0x68:
		assertSig("BIT 5 B")
		cpu.bit(cpu.b, 5)

	case 0x69:
		assertSig("BIT 5 C")
		cpu.bit(cpu.c, 5)

	case 0x6A:
		assertSig("BIT 5 D")
		cpu.bit(cpu.d, 5)

	case 0x6B:
		assertSig("BIT 5 E")
		cpu.bit(cpu.e, 5)

	case 0x6C:
		assertSig("BIT 5 H")
		cpu.bit(cpu.h, 5)

	case 0x6D:
		assertSig("BIT 5 L")
		cpu.bit(cpu.l, 5)

	case 0x6E:
		assertSig("BIT 5 (HL)")
		cpu.bit(cpu.byteAt(cpu.hl.read()), 5)

	case 0x6F:
		assertSig("BIT 5 A")
		cpu.bit(cpu.a, 5)

	case 0x70:
		assertSig("BIT 6 B")
		cpu.bit(cpu.b, 6)

	case 0x71:
		assertSig("BIT 6 C")
		cpu.bit(cpu.c, 6)

	case 0x72:
		assertSig("BIT 6 D")
		cpu.bit(cpu.d, 6)

	case 0x73:
		assertSig("BIT 6 E")
		cpu.bit(cpu.e, 6)

	case 0x74:
		assertSig("BIT 6 H")
		cpu.bit(cpu.h, 6)

	case 0x75:
		assertSig("BIT 6 L")
		cpu.bit(cpu.l, 6)

	case 0x76:
		assertSig("BIT 6 (HL)")
		cpu.bit(cpu.byteAt(cpu.hl.read()), 6)

	case 0x77:
		assertSig("BIT 6 A")
		cpu.bit(cpu.a, 6)

	case 0x78:
		assertSig("BIT 7 B")
		cpu.bit(cpu.b, 7)

	case 0x79:
		assertSig("BIT 7 C")
		cpu.bit(cpu.c, 7)

	case 0x7A:
		assertSig("BIT 7 D")
		cpu.bit(cpu.d, 7)

	case 0x7B:
		assertSig("BIT 7 E")
		cpu.bit(cpu.e, 7)

	case 0x7C:
		assertSig("BIT 7 H")
		cpu.bit(cpu.h, 7)

	case 0x7D:
		assertSig("BIT 7 L")
		cpu.bit(cpu.l, 7)

	case 0x7E:
		assertSig("BIT 7 (HL)")
		cpu.bit(cpu.byteAt(cpu.hl.read()), 7)

	case 0x7F:
		assertSig("BIT 7 A")
		cpu.bit(cpu.a, 7)

	case 0x80:
		assertSig("RES 0 B")
		cpu.res(cpu.b, 0)

	case 0x81:
		assertSig("RES 0 C")
		cpu.res(cpu.c, 0)

	case 0x82:
		assertSig("RES 0 D")
		cpu.res(cpu.d, 0)

	case 0x83:
		assertSig("RES 0 E")
		cpu.res(cpu.e, 0)

	case 0x84:
		assertSig("RES 0 H")
		cpu.res(cpu.h, 0)

	case 0x85:
		assertSig("RES 0 L")
		cpu.res(cpu.l, 0)

	case 0x86:
		assertSig("RES 0 (HL)")
		cpu.res(cpu.byteAt(cpu.hl.read()), 0)

	case 0x87:
		assertSig("RES 0 A")
		cpu.res(cpu.a, 0)

	case 0x88:
		assertSig("RES 1 B")
		cpu.res(cpu.b, 1)

	case 0x89:
		assertSig("RES 1 C")
		cpu.res(cpu.c, 1)

	case 0x8A:
		assertSig("RES 1 D")
		cpu.res(cpu.d, 1)

	case 0x8B:
		assertSig("RES 1 E")
		cpu.res(cpu.e, 1)

	case 0x8C:
		assertSig("RES 1 H")
		cpu.res(cpu.h, 1)

	case 0x8D:
		assertSig("RES 1 L")
		cpu.res(cpu.l, 1)

	case 0x8E:
		assertSig("RES 1 (HL)")
		cpu.res(cpu.byteAt(cpu.hl.read()), 1)

	case 0x8F:
		assertSig("RES 1 A")
		cpu.res(cpu.a, 1)

	case 0x90:
		assertSig("RES 2 B")
		cpu.res(cpu.b, 2)

	case 0x91:
		assertSig("RES 2 C")
		cpu.res(cpu.c, 2)

	case 0x92:
		assertSig("RES 2 D")
		cpu.res(cpu.d, 2)

	case 0x93:
		assertSig("RES 2 E")
		cpu.res(cpu.e, 2)

	case 0x94:
		assertSig("RES 2 H")
		cpu.res(cpu.h, 2)

	case 0x95:
		assertSig("RES 2 L")
		cpu.res(cpu.l, 2)

	case 0x96:
		assertSig("RES 2 (HL)")
		cpu.res(cpu.byteAt(cpu.hl.read()), 2)

	case 0x97:
		assertSig("RES 2 A")
		cpu.res(cpu.a, 2)

	case 0x98:
		assertSig("RES 3 B")
		cpu.res(cpu.b, 3)

	case 0x99:
		assertSig("RES 3 C")
		cpu.res(cpu.c, 3)

	case 0x9A:
		assertSig("RES 3 D")
		cpu.res(cpu.d, 3)

	case 0x9B:
		assertSig("RES 3 E")
		cpu.res(cpu.e, 3)

	case 0x9C:
		assertSig("RES 3 H")
		cpu.res(cpu.h, 3)

	case 0x9D:
		assertSig("RES 3 L")
		cpu.res(cpu.l, 3)

	case 0x9E:
		assertSig("RES 3 (HL)")
		cpu.res(cpu.byteAt(cpu.hl.read()), 3)

	case 0x9F:
		assertSig("RES 3 A")
		cpu.res(cpu.a, 3)

	case 0xA0:
		assertSig("RES 4 B")
		cpu.res(cpu.b, 4)

	case 0xA1:
		assertSig("RES 4 C")
		cpu.res(cpu.c, 4)

	case 0xA2:
		assertSig("RES 4 D")
		cpu.res(cpu.d, 4)

	case 0xA3:
		assertSig("RES 4 E")
		cpu.res(cpu.e, 4)

	case 0xA4:
		assertSig("RES 4 H")
		cpu.res(cpu.h, 4)

	case 0xA5:
		assertSig("RES 4 L")
		cpu.res(cpu.l, 4)

	case 0xA6:
		assertSig("RES 4 (HL)")
		cpu.res(cpu.byteAt(cpu.hl.read()), 4)

	case 0xA7:
		assertSig("RES 4 A")
		cpu.res(cpu.a, 4)

	case 0xA8:
		assertSig("RES 5 B")
		cpu.res(cpu.b, 5)

	case 0xA9:
		assertSig("RES 5 C")
		cpu.res(cpu.c, 5)

	case 0xAA:
		assertSig("RES 5 D")
		cpu.res(cpu.d, 5)

	case 0xAB:
		assertSig("RES 5 E")
		cpu.res(cpu.e, 5)

	case 0xAC:
		assertSig("RES 5 H")
		cpu.res(cpu.h, 5)

	case 0xAD:
		assertSig("RES 5 L")
		cpu.res(cpu.l, 5)

	case 0xAE:
		assertSig("RES 5 (HL)")
		cpu.res(cpu.byteAt(cpu.hl.read()), 5)

	case 0xAF:
		assertSig("RES 5 A")
		cpu.res(cpu.a, 5)

	case 0xB0:
		assertSig("RES 6 B")
		cpu.res(cpu.b, 6)

	case 0xB1:
		assertSig("RES 6 C")
		cpu.res(cpu.c, 6)

	case 0xB2:
		assertSig("RES 6 D")
		cpu.res(cpu.d, 6)

	case 0xB3:
		assertSig("RES 6 E")
		cpu.res(cpu.e, 6)

	case 0xB4:
		assertSig("RES 6 H")
		cpu.res(cpu.h, 6)

	case 0xB5:
		assertSig("RES 6 L")
		cpu.res(cpu.l, 6)

	case 0xB6:
		assertSig("RES 6 (HL)")
		cpu.res(cpu.byteAt(cpu.hl.read()), 6)

	case 0xB7:
		assertSig("RES 6 A")
		cpu.res(cpu.a, 6)

	case 0xB8:
		assertSig("RES 7 B")
		cpu.res(cpu.b, 7)

	case 0xB9:
		assertSig("RES 7 C")
		cpu.res(cpu.c, 7)

	case 0xBA:
		assertSig("RES 7 D")
		cpu.res(cpu.d, 7)

	case 0xBB:
		assertSig("RES 7 E")
		cpu.res(cpu.e, 7)

	case 0xBC:
		assertSig("RES 7 H")
		cpu.res(cpu.h, 7)

	case 0xBD:
		assertSig("RES 7 L")
		cpu.res(cpu.l, 7)

	case 0xBE:
		assertSig("RES 7 (HL)")
		cpu.res(cpu.byteAt(cpu.hl.read()), 7)

	case 0xBF:
		assertSig("RES 7 A")
		cpu.res(cpu.a, 7)

	case 0xC0:
		assertSig("SET 0 B")
		cpu.set(cpu.b, 0)

	case 0xC1:
		assertSig("SET 0 C")
		cpu.set(cpu.c, 0)

	case 0xC2:
		assertSig("SET 0 D")
		cpu.set(cpu.d, 0)

	case 0xC3:
		assertSig("SET 0 E")
		cpu.set(cpu.e, 0)

	case 0xC4:
		assertSig("SET 0 H")
		cpu.set(cpu.h, 0)

	case 0xC5:
		assertSig("SET 0 L")
		cpu.set(cpu.l, 0)

	case 0xC6:
		assertSig("SET 0 (HL)")
		cpu.set(cpu.byteAt(cpu.hl.read()), 0)

	case 0xC7:
		assertSig("SET 0 A")
		cpu.set(cpu.a, 0)

	case 0xC8:
		assertSig("SET 1 B")
		cpu.set(cpu.b, 1)

	case 0xC9:
		assertSig("SET 1 C")
		cpu.set(cpu.c, 1)

	case 0xCA:
		assertSig("SET 1 D")
		cpu.set(cpu.d, 1)

	case 0xCB:
		assertSig("SET 1 E")
		cpu.set(cpu.e, 1)

	case 0xCC:
		assertSig("SET 1 H")
		cpu.set(cpu.h, 1)

	case 0xCD:
		assertSig("SET 1 L")
		cpu.set(cpu.l, 1)

	case 0xCE:
		assertSig("SET 1 (HL)")
		cpu.set(cpu.byteAt(cpu.hl.read()), 1)

	case 0xCF:
		assertSig("SET 1 A")
		cpu.set(cpu.a, 1)

	case 0xD0:
		assertSig("SET 2 B")
		cpu.set(cpu.b, 2)

	case 0xD1:
		assertSig("SET 2 C")
		cpu.set(cpu.c, 2)

	case 0xD2:
		assertSig("SET 2 D")
		cpu.set(cpu.d, 2)

	case 0xD3:
		assertSig("SET 2 E")
		cpu.set(cpu.e, 2)

	case 0xD4:
		assertSig("SET 2 H")
		cpu.set(cpu.h, 2)

	case 0xD5:
		assertSig("SET 2 L")
		cpu.set(cpu.l, 2)

	case 0xD6:
		assertSig("SET 2 (HL)")
		cpu.set(cpu.byteAt(cpu.hl.read()), 2)

	case 0xD7:
		assertSig("SET 2 A")
		cpu.set(cpu.a, 2)

	case 0xD8:
		assertSig("SET 3 B")
		cpu.set(cpu.b, 3)

	case 0xD9:
		assertSig("SET 3 C")
		cpu.set(cpu.c, 3)

	case 0xDA:
		assertSig("SET 3 D")
		cpu.set(cpu.d, 3)

	case 0xDB:
		assertSig("SET 3 E")
		cpu.set(cpu.e, 3)

	case 0xDC:
		assertSig("SET 3 H")
		cpu.set(cpu.h, 3)

	case 0xDD:
		assertSig("SET 3 L")
		cpu.set(cpu.l, 3)

	case 0xDE:
		assertSig("SET 3 (HL)")
		cpu.set(cpu.byteAt(cpu.hl.read()), 3)

	case 0xDF:
		assertSig("SET 3 A")
		cpu.set(cpu.a, 3)

	case 0xE0:
		assertSig("SET 4 B")
		cpu.set(cpu.b, 4)

	case 0xE1:
		assertSig("SET 4 C")
		cpu.set(cpu.c, 4)

	case 0xE2:
		assertSig("SET 4 D")
		cpu.set(cpu.d, 4)

	case 0xE3:
		assertSig("SET 4 E")
		cpu.set(cpu.e, 4)

	case 0xE4:
		assertSig("SET 4 H")
		cpu.set(cpu.h, 4)

	case 0xE5:
		assertSig("SET 4 L")
		cpu.set(cpu.l, 4)

	case 0xE6:
		assertSig("SET 4 (HL)")
		cpu.set(cpu.byteAt(cpu.hl.read()), 4)

	case 0xE7:
		assertSig("SET 4 A")
		cpu.set(cpu.a, 4)

	case 0xE8:
		assertSig("SET 5 B")
		cpu.set(cpu.b, 5)

	case 0xE9:
		assertSig("SET 5 C")
		cpu.set(cpu.c, 5)

	case 0xEA:
		assertSig("SET 5 D")
		cpu.set(cpu.d, 5)

	case 0xEB:
		assertSig("SET 5 E")
		cpu.set(cpu.e, 5)

	case 0xEC:
		assertSig("SET 5 H")
		cpu.set(cpu.h, 5)

	case 0xED:
		assertSig("SET 5 L")
		cpu.set(cpu.l, 5)

	case 0xEE:
		assertSig("SET 5 (HL)")
		cpu.set(cpu.byteAt(cpu.hl.read()), 5)

	case 0xEF:
		assertSig("SET 5 A")
		cpu.set(cpu.a, 5)

	case 0xF0:
		assertSig("SET 6 B")
		cpu.set(cpu.b, 6)

	case 0xF1:
		assertSig("SET 6 C")
		cpu.set(cpu.c, 6)

	case 0xF2:
		assertSig("SET 6 D")
		cpu.set(cpu.d, 6)

	case 0xF3:
		assertSig("SET 6 E")
		cpu.set(cpu.e, 6)

	case 0xF4:
		assertSig("SET 6 H")
		cpu.set(cpu.h, 6)

	case 0xF5:
		assertSig("SET 6 L")
		cpu.set(cpu.l, 6)

	case 0xF6:
		assertSig("SET 6 (HL)")
		cpu.set(cpu.byteAt(cpu.hl.read()), 6)

	case 0xF7:
		assertSig("SET 6 A")
		cpu.set(cpu.a, 6)

	case 0xF8:
		assertSig("SET 7 B")
		cpu.set(cpu.b, 7)

	case 0xF9:
		assertSig("SET 7 C")
		cpu.set(cpu.c, 7)

	case 0xFA:
		assertSig("SET 7 D")
		cpu.set(cpu.d, 7)

	case 0xFB:
		assertSig("SET 7 E")
		cpu.set(cpu.e, 7)

	case 0xFC:
		assertSig("SET 7 H")
		cpu.set(cpu.h, 7)

	case 0xFD:
		assertSig("SET 7 L")
		cpu.set(cpu.l, 7)

	case 0xFE:
		assertSig("SET 7 (HL)")
		cpu.set(cpu.byteAt(cpu.hl.read()), 7)

	case 0xFF:
		assertSig("SET 7 A")
		cpu.set(cpu.a, 7)

	default:
		panic("This should never happen")
	}

	if len(opcode.Cycles) == 1 {
		return uint8(opcode.Cycles[0])
	} else if cyclesOverride > 0 {
		return cyclesOverride
	} else {
		panic(fmt.Sprintf("%v", opcode))
	}
}

func (cpu *CPU) byteAt(addr uint16) *RAMByte {
	return &RAMByte{mb: cpu.mb, offset: addr}
}

func (cpu *CPU) wordAt(addr uint16) *RAMWord {
	return &RAMWord{mb: cpu.mb, offset: addr}
}

type Value8 struct {
	val uint8
}

func (rv Value8) read() uint8 {
	return rv.val
}
func AsValue8(x uint8) *Value8 {
	return &Value8{
		val: x,
	}
}

type Value16 struct {
	val uint16
}

func (rv Value16) read() uint16 {
	return rv.val
}
func AsValue16(x uint16) *Value16 {
	return &Value16{
		val: x,
	}
}

type operation struct {
	opcode   *Opcode
	cbOpcode *Opcode
	pc       uint16
	mb       *Motherboard
}

func (o *operation) d8Val() *RAMByte {
	offset := o.pc + uint16(o.opcode.Length-1)
	return &RAMByte{mb: o.mb, offset: offset}
}
func (o *operation) d16Val() *RAMWord {
	offset := o.pc + uint16(o.opcode.Length-2)
	return &RAMWord{mb: o.mb, offset: offset}
}
func (o *operation) byteAtd8PlusFF00() *RAMByte {
	d8 := o.d8Val().read()
	return &RAMByte{mb: o.mb, offset: 0xFF00 + uint16(d8)}
}
func (o *operation) byteAtd16() *RAMByte {
	d16 := o.d16Val().read()
	return &RAMByte{mb: o.mb, offset: d16}
}
func (o *operation) wordAtd16() *RAMWord {
	d16 := o.d16Val().read()
	return &RAMWord{mb: o.mb, offset: d16}
}
func (o operation) bytesConsumed() uint16 {
	if o.cbOpcode != nil {
		return uint16(o.cbOpcode.Length)
	} else {
		return uint16(o.opcode.Length)
	}
}
