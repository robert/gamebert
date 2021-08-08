package main

type Timer struct {
	div  *Register8Bit // 0xFF04
	tima *Register8Bit // 0xFF05
	tma  *Register8Bit // 0xFF06
	tac  *Register8Bit // 0xFF07

	counter uint16

	divCounter  uint8
	timaCounter uint16
}

func NewTimer() *Timer {
	return &Timer{
		div:  &Register8Bit{name: "div"},
		tima: &Register8Bit{name: "tima"},
		tma:  &Register8Bit{name: "tma"},
		tac:  &Register8Bit{name: "tac"},

		counter: 0,
	}
}

func (t *Timer) tick(cycles uint8) bool {
	t.divCounter += cycles
	// If divCounter has overflowed
	if t.divCounter < cycles {
		t.div.inc(cycles)
	}

	if isBitSet8(t.tac.read(), 2) {
		t.timaCounter += uint16(cycles)

		freq := t.tacFreq()

		if t.timaCounter >= freq {
			t.timaCounter -= freq

			if t.tima.read() == 0xFF {
				t.tima.write(t.tma.read())
				return true
			} else {
				t.tima.inc(1)
				return false
			}
		}
	}

	return false
}

var tacFreqs = []uint16{1024, 16, 64, 256}

func (t Timer) tacFreq() uint16 {
	return tacFreqs[t.tac.read()&0b11]
}
