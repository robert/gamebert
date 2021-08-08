package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

type Opcodes struct {
	Unprefixed map[uint8]*Opcode
	Cbprefixed map[uint8]*Opcode
}
type Opcode struct {
	Cycles []int
	Addr   uint8
	Length uint8
}

func (o *Opcodes) GetUnprefixed(op uint8) *Opcode {
	opcode, ok := o.Unprefixed[op]
	if !ok {
		panic("Unrecognized unprefixed opcode")
	}

	return opcode
}
func (o *Opcodes) GetCbPrefixed(op uint8) *Opcode {
	opcode, ok := o.Cbprefixed[op]
	if !ok {
		panic("Unrecognized unprefixed opcode")
	}

	return opcode
}

func LoadOpcodes() *Opcodes {
	// TODO: don't load relatively
	f, err := os.Open("./opcodes.json")
	if err != nil {
		panic(err)
	}

	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()

	opsJSON := OpcodesJSON{}
	if err := dec.Decode(&opsJSON); err != nil {
		panic(err)
	}

	unprefixed := make(map[uint8]*Opcode, 0)
	for _, oj := range opsJSON.Unprefixed {
		op := formatOpcodeJSON(&oj)
		unprefixed[op.Addr] = op
	}
	prefixed := make(map[uint8]*Opcode, 0)
	for _, oj := range opsJSON.Cbprefixed {
		op := formatOpcodeJSON(&oj)
		prefixed[op.Addr] = op
	}

	return &Opcodes{
		Unprefixed: unprefixed,
		Cbprefixed: prefixed,
	}
}

func formatOpcodeJSON(oj *OpcodeJSON) *Opcode {
	addr, err := strconv.ParseUint(
		strings.Replace(oj.Addr, "0x", "", -1),
		16, 64)
	if err != nil {
		panic(err)
	}

	op := Opcode{
		Addr:   uint8(addr),
		Length: uint8(oj.Length),
		Cycles: oj.Cycles,
	}
	return &op
}

type OpcodesJSON struct {
	Unprefixed map[string]OpcodeJSON `json:"unprefixed"`
	Cbprefixed map[string]OpcodeJSON `json:"cbprefixed"`
}
type OpcodeJSON struct {
	Mnemonic string   `json:"mnemonic"`
	Length   int      `json:"length"`
	Cycles   []int    `json:"cycles"`
	Flags    []string `json:"flags"`
	Addr     string   `json:"addr"`
	Group    string   `json:"group"`
	Operand1 string   `json:"operand1"`
	Operand2 string   `json:"operand2"`
}
