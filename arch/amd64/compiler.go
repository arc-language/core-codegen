package amd64

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/arc-language/core-builder/ir"
	"github.com/arc-language/core-builder/types"
)

type Artifact struct {
	TextBuffer []byte
	DataBuffer []byte
	Symbols    []SymbolDef
}

type SymbolDef struct {
	Name     string
	Offset   uint64
	Size     uint64
	IsFunc   bool
	IsGlobal bool
}

type compiler struct {
	text         *bytes.Buffer
	data         *bytes.Buffer
	currentFunc  *ir.Function
	stackMap     map[ir.Value]int // Value -> RBP offset (negative)
	blockOffsets map[*ir.BasicBlock]int
	fixups       []jumpFixup
	currentFrame int
}

type jumpFixup struct {
	offset int
	target *ir.BasicBlock
}

func Compile(m *ir.Module) (*Artifact, error) {
	c := &compiler{
		text: new(bytes.Buffer),
		data: new(bytes.Buffer),
	}

	var symbols []SymbolDef

	for _, fn := range m.Functions {
		if len(fn.Blocks) == 0 {
			continue
		}

		startOff := c.text.Len()
		if err := c.compileFunction(fn); err != nil {
			return nil, fmt.Errorf("in function %s: %w", fn.Name(), err)
		}
		
		symbols = append(symbols, SymbolDef{
			Name:   fn.Name(),
			Offset: uint64(startOff),
			Size:   uint64(c.text.Len() - startOff),
			IsFunc: true,
		})
	}

	return &Artifact{
		TextBuffer: c.text.Bytes(),
		DataBuffer: c.data.Bytes(),
		Symbols:    symbols,
	}, nil
}

func (c *compiler) compileFunction(fn *ir.Function) error {
	c.currentFunc = fn
	c.stackMap = make(map[ir.Value]int)
	c.blockOffsets = make(map[*ir.BasicBlock]int)
	c.fixups = nil

	// 1. Stack Allocation (Simple Spill-All)
	offset := 0
	alloc := func(v ir.Value, sz int) {
		if sz < 8 { sz = 8 } // Minimum slot size
		offset += sz
		c.stackMap[v] = -offset
	}

	for _, arg := range fn.Arguments {
		alloc(arg, 8)
	}

	for _, block := range fn.Blocks {
		for _, inst := range block.Instructions {
			if inst.Type().Kind() != types.VoidKind {
				alloc(inst, SizeOf(inst.Type()))
			}
		}
	}

	// Align stack frame to 16 bytes
	if offset%16 != 0 {
		offset += (16 - (offset % 16))
	}
	c.currentFrame = offset

	// 2. Prologue
	c.emitBytes(0x55)             // push rbp
	c.emitBytes(0x48, 0x89, 0xE5) // mov rbp, rsp
	if c.currentFrame > 0 {
		c.emitBytes(0x48, 0x81, 0xEC) // sub rsp, imm32
		c.emitUint32(uint32(c.currentFrame))
	}

	// 3. Move Register Args to Stack (SysV: DI, SI, DX, CX, R8, R9)
	regArgs := []int{7, 6, 2, 1, 8, 9}
	for i, arg := range fn.Arguments {
		if i < len(regArgs) {
			c.emitStoreReg(regArgs[i], c.stackMap[arg])
		}
	}

	// 4. Body
	for _, block := range fn.Blocks {
		c.blockOffsets[block] = c.text.Len()
		for _, inst := range block.Instructions {
			if err := c.compileInstruction(inst); err != nil {
				return err
			}
		}
	}

	// 5. Jump Fixups
	text := c.text.Bytes()
	for _, fix := range c.fixups {
		targetOff, ok := c.blockOffsets[fix.target]
		if !ok {
			return fmt.Errorf("missing label for block %s", fix.target.Name())
		}
		// Calculate relative offset (Target - EndOfInstruction)
		rel := targetOff - (fix.offset + 4)
		binary.LittleEndian.PutUint32(text[fix.offset:], uint32(rel))
	}

	return nil
}

func (c *compiler) emitBytes(b ...byte) { c.text.Write(b) }
func (c *compiler) emitUint32(v uint32) { binary.Write(c.text, binary.LittleEndian, v) }