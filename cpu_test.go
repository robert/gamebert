package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

type Test struct {
	Name   string      `yaml:"name"`
	Input  *TestInput  `yaml:"input"`
	Output *TestOutput `yaml:"output"`
}

type TestInput struct {
	BootROMEnabled *bool `yaml:"bootromEnabled"`

	Cpu *cpuState `yaml:"cpu"`
	Lcd *lcdState `yaml:"lcd"`
	Ppu *ppuState `yaml:"ppu"`

	Timer *timerState `yaml:"timer"`

	InternalRAM0 []*setByte `yaml:"internalRAM0"`
	InternalRAM1 []*setByte `yaml:"internalRAM1"`

	NonIOInternalRAM0 []*setByte `yaml:"nonIOInternalRAM0"`
	NonIOInternalRAM1 []*setByte `yaml:"nonIOInternalRAM1"`
}

type TestOutput struct {
	BootROMEnabled *bool `yaml:"bootromEnabled"`

	Cpu *cpuState `yaml:"cpu"`
	Lcd *lcdState `yaml:"lcd"`
	Ppu *ppuState `yaml:"ppu"`

	Timer *timerState `yaml:"timer"`

	InternalRAM0 []*setByte `yaml:"internalRAM0"`
	InternalRAM1 []*setByte `yaml:"internalRAM1"`

	NonIOInternalRAM0 []*setByte `yaml:"nonIOInternalRAM0"`
	NonIOInternalRAM1 []*setByte `yaml:"nonIOInternalRAM1"`
}

type cpuState struct {
	Registers *cpuRegisters `yaml:"registers"`
	Flags     *cpuFlags     `yaml:"flags"`

	MasterInterruptsEnabled *bool `yaml:"masterInterruptsEnabled"`
}

type setByte struct {
	Offset *uint16 `yaml:"offset"`
	Val    *uint8  `yaml:"val"`
}

type cpuRegisters struct {
	A *uint8 `yaml:"a"`
	B *uint8 `yaml:"b"`
	C *uint8 `yaml:"c"`
	D *uint8 `yaml:"d"`
	E *uint8 `yaml:"e"`
	F *uint8 `yaml:"f"`
	H *uint8 `yaml:"h"`
	L *uint8 `yaml:"l"`

	Pc *uint16 `yaml:"pc"`
	Sp *uint16 `yaml:"sp"`
}

type cpuFlags struct {
	Interrupts *cpuInterruptFlags `yaml:"interrupts"`
	Common     *cpuCommonFlags    `yaml:"common"`
}

type cpuCommonFlags struct {
	Z *bool `yaml:"z"`
	N *bool `yaml:"n"`
	H *bool `yaml:"h"`
	C *bool `yaml:"c"`
}

type cpuInterruptFlags struct {
	Triggered *cpuInterruptTriggeredFlags `yaml:"triggered"`
	Enabled   *cpuInterruptEnabledFlags   `yaml:"enabled"`
}

type cpuInterruptTriggeredFlags struct {
	Vblank *bool `yaml:"vblank"`
	Stat   *bool `yaml:"stat"`
	Timer  *bool `yaml:"timer"`
	Serial *bool `yaml:"serial"`
	Joypad *bool `yaml:"joypad"`
}
type cpuInterruptEnabledFlags struct {
	Vblank *bool `yaml:"vblank"`
	Stat   *bool `yaml:"stat"`
	Timer  *bool `yaml:"timer"`
	Serial *bool `yaml:"serial"`
	Joypad *bool `yaml:"joypad"`
}

type ppuState struct {
	Vram []*setByte `yaml:"vram"`
	Oam  []*setByte `yaml:"oam"`
}

type timerState struct {
	Registers *timerRegisters `yaml:"registers"`
	Counter   *uint16         `yaml:"counter"`
}

type timerRegisters struct {
	Div  *uint8 `yaml:"div"`
	Tima *uint8 `yaml:"tima"`
	Tma  *uint8 `yaml:"tma"`
	Tac  *uint8 `yaml:"tac"`
}

type lcdState struct {
	Registers *lcdRegisters
	Flags     *lcdFlags

	Clock *int
}

type lcdRegisters struct {
	// Trying disabling accessing these registers directly, since we only
	// care about them for their flags in tests. Deal with flags directly.
	//Lcdc *uint8
	//Stat *uint8
	Scy  *uint8
	Scx  *uint8
	Ly   *uint8
	Lyc  *uint8
	Dma  *uint8
	Bgp  *uint8
	Obp0 *uint8
	Obp1 *uint8
	Wx   *uint8
	Wy   *uint8
}

type lcdFlags struct {
	Stat *lcdStatFlags
	Lcdc *lcdLcdcFlags
}

type lcdStatFlags struct {
	Lyci *bool
	Oami *bool
	Vbli *bool
	Hbli *bool
	Lycf *bool
	Mod1 *bool
	Mod0 *bool
}

type lcdLcdcFlags struct {
	Lcde *bool
	Wmap *bool
	Wien *bool
	Tida *bool
	Bmap *bool
	Spht *bool
	Spen *bool
	Bgen *bool
}

func setupEnv(inp *TestInput) *Motherboard {
	cart := NewCartridge("dr-mario.gb")
	mb := NewMotherboard(cart)
	cpu := mb.cpu

	if inp.Cpu != nil {
		if inp.Cpu.MasterInterruptsEnabled != nil {
			cpu.masterInterruptsEnabled = *inp.Cpu.MasterInterruptsEnabled
		}
		if inp.BootROMEnabled != nil {
			mb.bootROMEnabled = *inp.BootROMEnabled
		}
		if inp.Cpu.Registers != nil {
			regs := inp.Cpu.Registers
			if regs.A != nil {
				cpu.a.write(*regs.A)
			}
			if regs.B != nil {
				cpu.b.write(*regs.B)
			}
			if regs.C != nil {
				cpu.c.write(*regs.C)
			}
			if regs.D != nil {
				cpu.d.write(*regs.D)
			}
			if regs.E != nil {
				cpu.e.write(*regs.E)
			}
			if regs.F != nil {
				cpu.f.write(*regs.F)
			}
			if regs.H != nil {
				cpu.h.write(*regs.H)
			}
			if regs.L != nil {
				cpu.l.write(*regs.L)
			}

			if regs.Pc != nil {
				cpu.pc.write(*regs.Pc)
			}
			if regs.Sp != nil {
				cpu.sp.write(*regs.Sp)
			}
		}

		if inp.Cpu.Flags != nil {
			flags := inp.Cpu.Flags

			if flags.Common != nil {
				if flags.Common.Z != nil {
					cpu.zFlag.write(*flags.Common.Z)
				}
				if flags.Common.N != nil {
					cpu.nFlag.write(*flags.Common.N)
				}
				if flags.Common.H != nil {
					cpu.hFlag.write(*flags.Common.H)
				}
				if flags.Common.C != nil {
					cpu.cFlag.write(*flags.Common.C)
				}
			}

			if flags.Interrupts != nil {
				if flags.Interrupts.Triggered != nil {
					itFlags := flags.Interrupts.Triggered

					if itFlags.Vblank != nil {
						mb.cpu.intTriggeredVBlank.write(*itFlags.Vblank)
					}
					if itFlags.Stat != nil {
						mb.cpu.intTriggeredStat.write(*itFlags.Stat)
					}
					if itFlags.Timer != nil {
						mb.cpu.intTriggeredTimer.write(*itFlags.Timer)
					}
					if itFlags.Serial != nil {
						mb.cpu.intTriggeredSerial.write(*itFlags.Serial)
					}
					if itFlags.Joypad != nil {
						mb.cpu.intTriggeredJoypad.write(*itFlags.Joypad)
					}
				}

				if flags.Interrupts.Enabled != nil {
					ieFlags := flags.Interrupts.Enabled

					if ieFlags.Vblank != nil {
						mb.cpu.intEnabledVBlank.write(*ieFlags.Vblank)
					}
					if ieFlags.Stat != nil {
						mb.cpu.intEnabledStat.write(*ieFlags.Stat)
					}
					if ieFlags.Timer != nil {
						mb.cpu.intEnabledTimer.write(*ieFlags.Timer)
					}
					if ieFlags.Serial != nil {
						mb.cpu.intEnabledSerial.write(*ieFlags.Serial)
					}
					if ieFlags.Joypad != nil {
						mb.cpu.intEnabledJoypad.write(*ieFlags.Joypad)
					}
				}
			}
		}
	}

	if inp.Timer != nil {
		timerRegs := inp.Timer.Registers
		if timerRegs != nil {
			if timerRegs.Div != nil {
				mb.timer.div.write(*timerRegs.Div)
			}
			if timerRegs.Tima != nil {
				mb.timer.tima.write(*timerRegs.Tima)
			}
			if timerRegs.Tma != nil {
				mb.timer.tma.write(*timerRegs.Tma)
			}
			if timerRegs.Tac != nil {
				mb.timer.tac.write(*timerRegs.Tac)
			}
		}

		if inp.Timer.Counter != nil {
			mb.timer.counter = *inp.Timer.Counter
		}
	}

	if inp.InternalRAM0 != nil {
		for _, sb := range inp.InternalRAM0 {
			mb.internalRAM0.write(*sb.Offset, *sb.Val)
		}
	}
	if inp.InternalRAM1 != nil {
		for _, sb := range inp.InternalRAM1 {
			mb.internalRAM1.write(*sb.Offset, *sb.Val)
		}
	}

	if inp.NonIOInternalRAM0 != nil {
		for _, sb := range inp.NonIOInternalRAM0 {
			mb.nonIOInternalRAM0.write(*sb.Offset, *sb.Val)
		}
	}

	if inp.NonIOInternalRAM1 != nil {
		for _, sb := range inp.NonIOInternalRAM1 {
			mb.nonIOInternalRAM1.write(*sb.Offset, *sb.Val)
		}
	}

	if inp.Ppu != nil {
		for _, sb := range inp.Ppu.Vram {
			mb.ppu.vRAM.write(*sb.Offset, *sb.Val)
		}
		for _, sb := range inp.Ppu.Oam {
			mb.ppu.oam.write(*sb.Offset, *sb.Val)
		}
	}

	if inp.Lcd != nil {
		lcd := mb.lcd
		if inp.Lcd.Clock != nil {
			lcd.clock = *inp.Lcd.Clock
		}

		if inp.Lcd.Registers != nil {
			regs := inp.Lcd.Registers
			if regs.Scy != nil {
				lcd.scy.write(*regs.Scy)
			}
			if regs.Scx != nil {
				lcd.scx.write(*regs.Scx)
			}
			if regs.Ly != nil {
				lcd.ly.write(*regs.Ly)
			}
			if regs.Lyc != nil {
				lcd.lyc.write(*regs.Lyc)
			}
			if regs.Dma != nil {
				lcd.dma.write(*regs.Dma)
			}
			if regs.Bgp != nil {
				lcd.bgp.write(*regs.Bgp)
			}
			if regs.Obp0 != nil {
				lcd.obp0.write(*regs.Obp0)
			}
			if regs.Obp1 != nil {
				lcd.obp1.write(*regs.Obp1)
			}
			if regs.Wx != nil {
				lcd.wx.write(*regs.Wx)
			}
			if regs.Wy != nil {
				lcd.wy.write(*regs.Wy)
			}
		}
		if inp.Lcd.Flags != nil {
			flags := inp.Lcd.Flags
			if flags.Stat != nil {
				stat := flags.Stat
				if stat.Lyci != nil {
					lcd.flagLycInterrupt.write(*stat.Lyci)
				}
				if stat.Oami != nil {
					lcd.flagOAMInterrupt.write(*stat.Oami)
				}
				if stat.Vbli != nil {
					lcd.flagVBlankInterrupt.write(*stat.Vbli)
				}
				if stat.Hbli != nil {
					lcd.flagHBlankInterrupt.write(*stat.Hbli)
				}
				if stat.Lycf != nil {
					lcd.flagLyc.write(*stat.Lycf)
				}
				if stat.Mod1 != nil {
					lcd.flagMode1.write(*stat.Mod1)
				}
				if stat.Mod0 != nil {
					lcd.flagMode0.write(*stat.Mod0)
				}
			}

			if flags.Lcdc != nil {
				lcdc := flags.Lcdc
				if lcdc.Lcde != nil {
					lcd.flagLcdEnabled.write(*lcdc.Lcde)
				}
				if lcdc.Wmap != nil {
					lcd.flagWindowmapSelect.write(*lcdc.Wmap)
				}
				if lcdc.Wien != nil {
					lcd.flagWindowEnabled.write(*lcdc.Wien)
				}
				if lcdc.Tida != nil {
					lcd.flagTiledataSelect.write(*lcdc.Tida)
				}
				if lcdc.Bmap != nil {
					lcd.flagBackgroundMapSelect.write(*lcdc.Bmap)
				}
				if lcdc.Spht != nil {
					lcd.flagSpriteHeight.write(*lcdc.Spht)
				}
				if lcdc.Spen != nil {
					lcd.flagSpriteEnabled.write(*lcdc.Spen)
				}
				if lcdc.Bgen != nil {
					lcd.flagBackgroundEnabled.write(*lcdc.Bgen)
				}
			}
		}
	}

	return mb
}

func assertTestOutput(t *testing.T, mb *Motherboard, out *TestOutput) {
	if out.Cpu != nil {
		if out.Cpu.MasterInterruptsEnabled != nil {
			assert.Equal(t, *out.Cpu.MasterInterruptsEnabled, mb.cpu.masterInterruptsEnabled, "cpu.masterInterruptsEnabled")
		}
		if out.BootROMEnabled != nil {
			assert.Equal(t, *out.BootROMEnabled, mb.bootROMEnabled, "mb.bootROMEnabled")
		}
		if out.Cpu.Registers != nil {
			regs := out.Cpu.Registers

			if regs.A != nil {
				assert.Equal(t, *regs.A, mb.cpu.a.read(), "cpu.registers.a")
			}
			if regs.B != nil {
				assert.Equal(t, *regs.B, mb.cpu.b.read(), "cpu.registers.b")
			}
			if regs.C != nil {
				assert.Equal(t, *regs.C, mb.cpu.c.read(), "cpu.registers.c")
			}
			if regs.D != nil {
				assert.Equal(t, *regs.D, mb.cpu.d.read(), "cpu.registers.d'")
			}
			if regs.E != nil {
				assert.Equal(t, *regs.E, mb.cpu.e.read(), "cpu.registers.e")
			}
			if regs.F != nil {
				assert.Equal(t, *regs.F, mb.cpu.f.read(), "cpu.registers.f")
			}
			if regs.H != nil {
				assert.Equal(t, *regs.H, mb.cpu.h.read(), "cpu.registers.h")
			}
			if regs.L != nil {
				assert.Equal(t, *regs.L, mb.cpu.l.read(), "cpu.registers.l")
			}

			if regs.Pc != nil {
				assert.Equal(t, *regs.Pc, mb.cpu.pc.read(), "cpu.regsisters.pc")
			}
			if regs.Sp != nil {
				assert.Equal(t, *regs.Sp, mb.cpu.sp.read(), "cpu.registers.sp")
			}
		}

		if out.Cpu.Flags != nil {
			flags := out.Cpu.Flags

			if out.Cpu.Flags.Common != nil {
				if flags.Common.Z != nil {
					assert.Equal(t, *flags.Common.Z, mb.cpu.zFlag.read(), "cpu.flags.common.z")
				}
				if flags.Common.N != nil {
					assert.Equal(t, *flags.Common.N, mb.cpu.nFlag.read(), "cpu.flags.common.n")
				}
				if flags.Common.H != nil {
					assert.Equal(t, *flags.Common.H, mb.cpu.hFlag.read(), "cpu.flags.common.h")
				}
				if flags.Common.C != nil {
					assert.Equal(t, *flags.Common.C, mb.cpu.cFlag.read(), "cpu.flags.common.c")
				}
			}

			if out.Cpu.Flags.Common != nil {
				if flags.Interrupts != nil {
					if flags.Interrupts.Triggered != nil {
						itFlags := flags.Interrupts.Triggered

						if itFlags.Vblank != nil {
							assert.Equal(t, itFlags.Vblank, mb.cpu.intTriggeredVBlank.read())
						}
						if itFlags.Stat != nil {
							assert.Equal(t, itFlags.Stat, mb.cpu.intTriggeredStat.read())
						}
						if itFlags.Timer != nil {
							assert.Equal(t, itFlags.Timer, mb.cpu.intTriggeredTimer.read())
						}
						if itFlags.Serial != nil {
							assert.Equal(t, itFlags.Serial, mb.cpu.intTriggeredSerial.read())
						}
						if itFlags.Joypad != nil {
							assert.Equal(t, itFlags.Joypad, mb.cpu.intTriggeredJoypad.read())
						}
					}

					if flags.Interrupts.Enabled != nil {
						ieFlags := flags.Interrupts.Enabled

						if ieFlags.Vblank != nil {
							assert.Equal(t, ieFlags.Vblank, mb.cpu.intEnabledVBlank.read())
						}
						if ieFlags.Stat != nil {
							assert.Equal(t, ieFlags.Stat, mb.cpu.intEnabledStat.read())
						}
						if ieFlags.Timer != nil {
							assert.Equal(t, ieFlags.Timer, mb.cpu.intEnabledTimer.read())
						}
						if ieFlags.Serial != nil {
							assert.Equal(t, ieFlags.Serial, mb.cpu.intEnabledSerial.read())
						}
						if ieFlags.Joypad != nil {
							assert.Equal(t, ieFlags.Joypad, mb.cpu.intEnabledJoypad.read())
						}
					}
				}
			}
		}
	}

	if out.Timer != nil {
		timerRegs := out.Timer.Registers
		if timerRegs.Div != nil {
			assert.Equal(t, *timerRegs.Div, mb.timer.div.read())
		}
		if timerRegs.Tima != nil {
			assert.Equal(t, *timerRegs.Tima, mb.timer.tima.read())
		}
		if timerRegs.Tma != nil {
			assert.Equal(t, *timerRegs.Tma, mb.timer.tma.read())
		}
		if timerRegs.Tac != nil {
			assert.Equal(t, *timerRegs.Tac, mb.timer.tac.read())
		}
	}

	if out.InternalRAM0 != nil {
		for _, sb := range out.InternalRAM0 {
			assert.Equal(t, *sb.Val, mb.internalRAM0.read(*sb.Offset))
		}
	}
	if out.InternalRAM1 != nil {
		for _, sb := range out.InternalRAM1 {
			assert.Equal(t, *sb.Val, mb.internalRAM1.read(*sb.Offset))
		}
	}

	if out.NonIOInternalRAM0 != nil {
		for _, sb := range out.NonIOInternalRAM0 {
			assert.Equal(t, *sb.Val, mb.nonIOInternalRAM0.read(*sb.Offset))
		}
	}

	if out.NonIOInternalRAM1 != nil {
		for _, sb := range out.NonIOInternalRAM1 {
			assert.Equal(t, *sb.Val, mb.nonIOInternalRAM1.read(*sb.Offset))
		}
	}

	if out.Ppu != nil {
		for _, sb := range out.Ppu.Vram {
			assert.Equal(t, *sb.Val, mb.ppu.vRAM.read(*sb.Offset))
		}
		for _, sb := range out.Ppu.Oam {
			assert.Equal(t, *sb.Val, mb.ppu.oam.read(*sb.Offset))
		}
	}

	if out.Lcd != nil {
		lcd := mb.lcd
		if out.Lcd.Clock != nil {
			assert.Equal(t, *out.Lcd.Clock, lcd.clock)
		}

		if out.Lcd.Registers != nil {
			regs := out.Lcd.Registers
			if regs.Scy != nil {
				assert.Equal(t, *regs.Scy, lcd.scy.read())
			}
			if regs.Scx != nil {
				assert.Equal(t, *regs.Scx, lcd.scx.read())
			}
			if regs.Ly != nil {
				assert.Equal(t, *regs.Ly, lcd.ly.read())
			}
			if regs.Lyc != nil {
				assert.Equal(t, *regs.Lyc, lcd.lyc.read())
			}
			if regs.Dma != nil {
				assert.Equal(t, *regs.Dma, lcd.dma.read())
			}
			if regs.Bgp != nil {
				assert.Equal(t, *regs.Bgp, lcd.bgp.read())
			}
			if regs.Obp0 != nil {
				assert.Equal(t, *regs.Obp0, lcd.obp0.read())
			}
			if regs.Obp1 != nil {
				assert.Equal(t, *regs.Obp1, lcd.obp1.read())
			}
			if regs.Wx != nil {
				assert.Equal(t, *regs.Wx, lcd.wx.read())
			}
			if regs.Wy != nil {
				assert.Equal(t, *regs.Wy, lcd.wy.read())
			}
		}
		if out.Lcd.Flags != nil {
			flags := out.Lcd.Flags
			if flags.Stat != nil {
				stat := flags.Stat
				if stat.Lyci != nil {
					assert.Equal(t, *stat.Lyci, lcd.flagLycInterrupt.read())
				}
				if stat.Oami != nil {
					assert.Equal(t, *stat.Oami, lcd.flagOAMInterrupt.read())
				}
				if stat.Vbli != nil {
					assert.Equal(t, *stat.Vbli, lcd.flagVBlankInterrupt.read())
				}
				if stat.Hbli != nil {
					assert.Equal(t, *stat.Hbli, lcd.flagHBlankInterrupt.read())
				}
				if stat.Lycf != nil {
					assert.Equal(t, *stat.Lycf, lcd.flagLyc.read())
				}
				if stat.Mod1 != nil {
					assert.Equal(t, *stat.Mod1, lcd.flagMode1.read())
				}
				if stat.Mod0 != nil {
					assert.Equal(t, *stat.Mod0, lcd.flagMode0.read())
				}
			}

			if flags.Lcdc != nil {
				lcdc := flags.Lcdc
				if lcdc.Lcde != nil {
					assert.Equal(t, *lcdc.Lcde, lcd.flagLcdEnabled.read())
				}
				if lcdc.Wmap != nil {
					assert.Equal(t, *lcdc.Wmap, lcd.flagWindowmapSelect.read())
				}
				if lcdc.Wien != nil {
					assert.Equal(t, *lcdc.Wien, lcd.flagWindowEnabled.read())
				}
				if lcdc.Tida != nil {
					assert.Equal(t, *lcdc.Tida, lcd.flagTiledataSelect.read())
				}
				if lcdc.Bmap != nil {
					assert.Equal(t, *lcdc.Bmap, lcd.flagBackgroundMapSelect.read())
				}
				if lcdc.Spht != nil {
					assert.Equal(t, *lcdc.Spht, lcd.flagSpriteHeight.read())
				}
				if lcdc.Spen != nil {
					assert.Equal(t, *lcdc.Spen, lcd.flagSpriteEnabled.read())
				}
				if lcdc.Bgen != nil {
					assert.Equal(t, *lcdc.Bgen, lcd.flagBackgroundEnabled.read())
				}
			}
		}
	}
}

func TestSetupEnv(t *testing.T) {
	files1, err := filepath.Glob("./tests/*.yaml")
	if err != nil {
		panic(err)
	}
	files2, err := filepath.Glob("./tests/motherboard/*.yaml")
	if err != nil {
		panic(err)
	}

	files := append(files1, files2...)

	for _, fname := range files {
		fmt.Println(fname)
		f, err := ioutil.ReadFile(fname)
		if err != nil {
			panic(err)
		}

		var gamebertTests []Test
		dec := yaml.NewDecoder(bytes.NewReader(f))
		dec.KnownFields(true)

		err = dec.Decode(&gamebertTests)
		if err != nil {
			panic(err)
		}

		for _, gt := range gamebertTests {
			t.Run(fname+" "+gt.Name, func(t *testing.T) {
				mb := setupEnv(gt.Input)
				mb.tick()

				assertTestOutput(t, mb, gt.Output)
			})
		}
	}
}
