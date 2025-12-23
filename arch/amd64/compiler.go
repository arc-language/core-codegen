package amd64

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/arc-language/core-builder/ir"
	"github.com/arc-language/core-builder/types"
)

type Artifact struct {
	TextBuffer  []byte
	DataBuffer  []byte
	Symbols     []SymbolDef
	Relocations []Relocation
}

type SymbolDef struct {
	Name     string
	Offset   uint64
	Size     uint64
	IsFunc   bool
	IsGlobal bool
}

type Relocation struct {
	Offset     uint64
	SymbolName string
	Type       RelocationType
	Addend     int64
}

type RelocationType int

const (
	R_X86_64_PC32  RelocationType = 2
	R_X86_64_PLT32 RelocationType = 4
)

type compiler struct {
	text         *bytes.Buffer
	data         *bytes.Buffer
	currentFunc  *ir.Function
	stackMap     map[ir.Value]int // Value -> RBP offset (negative)
	blockOffsets map[*ir.BasicBlock]int
	fixups       []jumpFixup
	relocations  []Relocation
	currentFrame int
	nextTemp     int
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

	// Compile global variables first
	for _, g := range m.Globals {
		offset := c.data.Len()
		
		if err := c.compileGlobal(g); err != nil {
			return nil, fmt.Errorf("in global %s: %w", g.Name(), err)
		}
		
		size := c.data.Len() - offset
		symbols = append(symbols, SymbolDef{
			Name:     g.Name(),
			Offset:   uint64(offset),
			Size:     uint64(size),
			IsGlobal: true,
			IsFunc:   false,
		})
	}

	// Compile functions
	for _, fn := range m.Functions {
		if len(fn.Blocks) == 0 {
			continue // External declaration
		}

		startOff := c.text.Len()
		if err := c.compileFunction(fn); err != nil {
			return nil, fmt.Errorf("in function %s: %w", fn.Name(), err)
		}
		
		endOff := c.text.Len()

		symbols = append(symbols, SymbolDef{
			Name:     fn.Name(),
			Offset:   uint64(startOff),
			Size:     uint64(endOff - startOff),
			IsFunc:   true,
			IsGlobal: false, // Will be determined by linkage
		})
	}

	return &Artifact{
		TextBuffer:  c.text.Bytes(),
		DataBuffer:  c.data.Bytes(),
		Symbols:     symbols,
		Relocations: c.relocations,
	}, nil
}

func (c *compiler) compileGlobal(g *ir.Global) error {
	// Align to 8 bytes
	for c.data.Len()%8 != 0 {
		c.data.WriteByte(0)
	}

	if g.Initializer == nil {
		// Zero-initialized
		size := SizeOf(g.Type())
		c.data.Write(make([]byte, size))
		return nil
	}

	return c.emitConstant(g.Initializer)
}

func (c *compiler) emitConstant(constant ir.Constant) error {
	switch v := constant.(type) {
	case *ir.ConstantInt:
		size := SizeOf(v.Type())
		switch size {
		case 1:
			c.data.WriteByte(byte(v.Value))
		case 2:
			binary.Write(c.data, binary.LittleEndian, uint16(v.Value))
		case 4:
			binary.Write(c.data, binary.LittleEndian, uint32(v.Value))
		case 8:
			binary.Write(c.data, binary.LittleEndian, uint64(v.Value))
		}
	case *ir.ConstantFloat:
		if v.Type().(*types.FloatType).BitWidth == 32 {
			binary.Write(c.data, binary.LittleEndian, float32(v.Value))
		} else {
			binary.Write(c.data, binary.LittleEndian, v.Value)
		}
	case *ir.ConstantZero:
		size := SizeOf(v.Type())
		c.data.Write(make([]byte, size))
	case *ir.ConstantArray:
		for _, elem := range v.Elements {
			if err := c.emitConstant(elem); err != nil {
				return err
			}
		}
	case *ir.ConstantStruct:
		st := v.Type().(*types.StructType)
		offset := 0
		for i, field := range v.Fields {
			// Add padding
			fieldOffset := GetStructFieldOffset(st, i)
			for offset < fieldOffset {
				c.data.WriteByte(0)
				offset++
			}
			if err := c.emitConstant(field); err != nil {
				return err
			}
			offset += SizeOf(field.Type())
		}
	default:
		return fmt.Errorf("unsupported constant type: %T", constant)
	}
	return nil
}

func (c *compiler) compileFunction(fn *ir.Function) error {
	c.currentFunc = fn
	c.stackMap = make(map[ir.Value]int)
	c.blockOffsets = make(map[*ir.BasicBlock]int)
	c.fixups = nil
	c.nextTemp = 0

	// 1. Analyze and allocate stack space
	offset := 0
	alloc := func(v ir.Value, sz int) {
		if sz < 8 {
			sz = 8 // Minimum slot size
		}
		// Align to natural alignment
		if offset%sz != 0 {
			offset += (sz - (offset % sz))
		}
		offset += sz
		c.stackMap[v] = -offset
	}

	// Allocate space for arguments (they'll be copied from registers/stack)
	for _, arg := range fn.Arguments {
		alloc(arg, SizeOf(arg.Type()))
	}

	// Allocate space for all instructions that produce values
	for _, block := range fn.Blocks {
		for _, inst := range block.Instructions {
			if inst.Type() != nil && inst.Type().Kind() != types.VoidKind {
				// Special handling for alloca - it needs pointer-sized space
				if _, ok := inst.(*ir.AllocaInst); ok {
					alloc(inst, 8) // Store the pointer
				} else {
					alloc(inst, SizeOf(inst.Type()))
				}
			}
		}
	}

	// Handle alloca instructions - allocate their actual space
	allocaOffset := offset
	for _, block := range fn.Blocks {
		for _, inst := range block.Instructions {
			if allocaInst, ok := inst.(*ir.AllocaInst); ok {
				size := SizeOf(allocaInst.AllocatedType)
				if allocaInst.NumElements != nil {
					// For array allocas
					if constInt, ok := allocaInst.NumElements.(*ir.ConstantInt); ok {
						size *= int(constInt.Value)
					}
				}
				if size < 8 {
					size = 8
				}
				allocaOffset += size
				// The alloca instruction itself stores the address
				// We'll generate code to compute this address
			}
		}
	}

	// Align stack frame to 16 bytes (required by System V ABI)
	if allocaOffset%16 != 0 {
		allocaOffset += (16 - (allocaOffset % 16))
	}
	c.currentFrame = allocaOffset

	// 2. Function prologue
	c.emitPrologue()

	// 3. Save register arguments to stack
	c.emitArgSave(fn)

	// 4. Compile basic blocks
	for _, block := range fn.Blocks {
		c.blockOffsets[block] = c.text.Len()
		for _, inst := range block.Instructions {
			if err := c.compileInstruction(inst); err != nil {
				return fmt.Errorf("in block %s: %w", block.Name(), err)
			}
		}
	}

	// 5. Apply jump fixups
	c.applyFixups()

	return nil
}

func (c *compiler) emitPrologue() {
	// push rbp
	c.emitBytes(0x55)
	// mov rbp, rsp
	c.emitBytes(0x48, 0x89, 0xE5)
	// sub rsp, frame_size
	if c.currentFrame > 0 {
		if c.currentFrame <= 127 {
			c.emitBytes(0x48, 0x83, 0xEC, byte(c.currentFrame))
		} else {
			c.emitBytes(0x48, 0x81, 0xEC)
			c.emitUint32(uint32(c.currentFrame))
		}
	}
}

func (c *compiler) emitArgSave(fn *ir.Function) {
	// System V AMD64 ABI: RDI, RSI, RDX, RCX, R8, R9
	argRegs := []int{RDI, RSI, RDX, RCX, R8, R9}

	for i, arg := range fn.Arguments {
		offset := c.stackMap[arg]
		size := SizeOf(arg.Type())

		if i < len(argRegs) {
			// Load from register and store to stack
			reg := argRegs[i]
			if size <= 8 {
				c.emitStoreReg(reg, offset, size)
			}
		} else {
			// Arguments beyond 6 are on the caller's stack
			// They are at [rbp + 16 + (i-6)*8]
			srcOffset := 16 + (i-6)*8

			// Load with appropriate size
			if size == 4 {
				// mov eax, [rbp + srcOffset]  (32-bit load, zero-extends)
				c.emitBytes(0x8B, 0x85)
				c.emitInt32(int32(srcOffset))
			} else {
				// mov rax, [rbp + srcOffset]
				c.emitBytes(0x48, 0x8B, 0x85)
				c.emitInt32(int32(srcOffset))
			}

			// Store to local stack slot with appropriate size
			c.emitStoreToStack(RAX, offset, size)
		}
	}
}

func (c *compiler) applyFixups() {
	text := c.text.Bytes()
	for _, fix := range c.fixups {
		targetOff, ok := c.blockOffsets[fix.target]
		if !ok {
			// Should not happen - all blocks should have offsets
			continue
		}
		// Calculate relative offset from end of instruction
		rel := targetOff - (fix.offset + 4)
		binary.LittleEndian.PutUint32(text[fix.offset:], uint32(rel))
	}
}

func (c *compiler) emitBytes(b ...byte) {
	c.text.Write(b)
}

func (c *compiler) emitUint32(v uint32) {
	binary.Write(c.text, binary.LittleEndian, v)
}

func (c *compiler) emitInt32(v int32) {
	binary.Write(c.text, binary.LittleEndian, v)
}

func (c *compiler) emitUint64(v uint64) {
	binary.Write(c.text, binary.LittleEndian, v)
}

// Register constants
const (
	RAX = 0
	RCX = 1
	RDX = 2
	RBX = 3
	RSP = 4
	RBP = 5
	RSI = 6
	RDI = 7
	R8  = 8
	R9  = 9
	R10 = 10
	R11 = 11
	R12 = 12
	R13 = 13
	R14 = 14
	R15 = 15
)