package amd64

import (
	"unsafe"

	"github.com/arc-language/core-builder/ir"
	"github.com/arc-language/core-builder/types"
)

// Load a value into a register
func (c *compiler) loadToReg(reg int, value ir.Value) {
	// Handle constants
	switch v := value.(type) {
	case *ir.ConstantInt:
		c.loadConstInt(reg, v.Value)
		return
	case *ir.ConstantNull:
		// xor reg, reg
		c.emitXorReg(reg, reg)
		return
	case *ir.ConstantUndef:
		// Leave undefined - just xor to zero
		c.emitXorReg(reg, reg)
		return
	case *ir.Global:
		// Load address of global
		// lea reg, [rip + offset]
		// This requires a relocation
		c.emitLeaRipRelative(reg, v.Name())
		return
	}

	// Load from stack location
	offset, ok := c.stackMap[value]
	if !ok {
		// This shouldn't happen - all values should be allocated
		// Fall back to zero
		c.emitXorReg(reg, reg)
		return
	}

	size := SizeOf(value.Type())
	c.emitLoadFromStack(reg, offset, size)
}

// Load a floating point value into an XMM register
func (c *compiler) loadToFpReg(xmmReg int, value ir.Value) {
	// Handle constants
	switch v := value.(type) {
	case *ir.ConstantFloat:
		c.loadConstFloat(xmmReg, v.Value, v.Type().(*types.FloatType).BitWidth)
		return
	}

	// Load from stack location
	offset, ok := c.stackMap[value]
	if !ok {
		// XOR to zero
		c.emitXorps(xmmReg, xmmReg)
		return
	}

	fpType := value.Type().(*types.FloatType)
	if fpType.BitWidth == 32 {
		// movss xmm, [rbp + offset]
		c.emitFpLoadFromStack(xmmReg, offset, false)
	} else {
		// movsd xmm, [rbp + offset]
		c.emitFpLoadFromStack(xmmReg, offset, true)
	}
}

// Store a register value
func (c *compiler) storeFromReg(reg int, dest ir.Value) {
	offset, ok := c.stackMap[dest]
	if !ok {
		return // Nowhere to store
	}

	size := SizeOf(dest.Type())
	c.emitStoreToStack(reg, offset, size)
}

// Store an XMM register value
func (c *compiler) storeFromFpReg(xmmReg int, dest ir.Value) {
	offset, ok := c.stackMap[dest]
	if !ok {
		return
	}

	fpType := dest.Type().(*types.FloatType)
	if fpType.BitWidth == 32 {
		// movss [rbp + offset], xmm
		c.emitFpStoreToStack(xmmReg, offset, false)
	} else {
		// movsd [rbp + offset], xmm
		c.emitFpStoreToStack(xmmReg, offset, true)
	}
}

// Load constant integer into register
func (c *compiler) loadConstInt(reg int, value int64) {
	if value == 0 {
		c.emitXorReg(reg, reg)
		return
	}

	// mov reg, imm64
	rex := byte(0x48)
	if reg >= 8 {
		rex |= 0x01
		reg -= 8
	}

	c.emitBytes(rex, byte(0xB8+reg))
	c.emitUint64(uint64(value))
}

// Load constant float into XMM register
func (c *compiler) loadConstFloat(xmmReg int, value float64, bits int) {
	// We need to materialize the constant in memory first
	// For now, use a simple approach: load via integer register

	if bits == 32 {
		// Load as 32-bit int, then movd to xmm
		bits32 := *(*uint32)(unsafe.Pointer(&value))
		c.loadConstInt(RAX, int64(bits32))
		c.emitMovdToXmm(xmmReg, RAX)
	} else {
		// Load as 64-bit int, then movq to xmm
		bits64 := *(*uint64)(unsafe.Pointer(&value))
		c.loadConstInt(RAX, int64(bits64))
		c.emitMovqToXmm(xmmReg, RAX)
	}
}

// Emit XOR reg, reg
func (c *compiler) emitXorReg(dst, src int) {
	rex := byte(0x48)
	dstReg := dst
	srcReg := src
	
	if dstReg >= 8 {
		rex |= 0x04
		dstReg -= 8
	}
	if srcReg >= 8 {
		rex |= 0x01
		srcReg -= 8
	}

	c.emitBytes(rex, 0x31, byte(0xC0|(srcReg<<3)|dstReg))
}

// Emit load from stack: mov reg, [rbp + offset]
func (c *compiler) emitLoadFromStack(reg int, offset int, size int) {
	regNum := reg
	needsREX := false
	rex := byte(0x40) // Base REX prefix
	
	if regNum >= 8 {
		rex |= 0x04 // REX.R bit
		needsREX = true
		regNum -= 8
	}

	switch size {
	case 1:
		// movzx r32, byte ptr [rbp + offset] (zero-extends to 64)
		// We avoid REX.W to keep encoding standard for movzbl
		if needsREX {
			c.emitBytes(rex, 0x0F, 0xB6, byte(0x85|(regNum<<3)))
		} else {
			c.emitBytes(0x0F, 0xB6, byte(0x85|(regNum<<3)))
		}
		c.emitInt32(int32(offset))

	case 2:
		// movzx r32, word ptr [rbp + offset] (zero-extends to 64)
		// We avoid REX.W to keep encoding standard for movzwl
		if needsREX {
			c.emitBytes(rex, 0x0F, 0xB7, byte(0x85|(regNum<<3)))
		} else {
			c.emitBytes(0x0F, 0xB7, byte(0x85|(regNum<<3)))
		}
		c.emitInt32(int32(offset))

	case 4:
		// mov r32, [rbp + offset] (zero-extends to 64)
		if needsREX {
			c.emitBytes(rex, 0x8B, byte(0x85|(regNum<<3)))
		} else {
			c.emitBytes(0x8B, byte(0x85|(regNum<<3)))
		}
		c.emitInt32(int32(offset))

	case 8:
		// mov r64, [rbp + offset]
		rex |= 0x08 // REX.W for 64-bit operand
		c.emitBytes(rex, 0x8B, byte(0x85|(regNum<<3)))
		c.emitInt32(int32(offset))

	default:
		// Fallback to 8-byte load
		rex |= 0x08 // REX.W
		c.emitBytes(rex, 0x8B, byte(0x85|(regNum<<3)))
		c.emitInt32(int32(offset))
	}
}

// Emit store to stack: mov [rbp + offset], reg
func (c *compiler) emitStoreToStack(reg int, offset int, size int) {
	regNum := reg
	needsREX := false
	rex := byte(0x40) // Base REX prefix
	
	if regNum >= 8 {
		rex |= 0x04 // REX.R bit
		needsREX = true
		regNum -= 8
	}

	switch size {
	case 1:
		// mov byte ptr [rbp + offset], r8
		if needsREX || reg >= 4 { // Need REX for spl, bpl, sil, dil or R8-R15
			c.emitBytes(rex, 0x88, byte(0x85|(regNum<<3)))
		} else {
			c.emitBytes(0x88, byte(0x85|(regNum<<3)))
		}
		c.emitInt32(int32(offset))

	case 2:
		// mov word ptr [rbp + offset], r16
		if needsREX {
			c.emitBytes(0x66, rex, 0x89, byte(0x85|(regNum<<3)))
		} else {
			c.emitBytes(0x66, 0x89, byte(0x85|(regNum<<3)))
		}
		c.emitInt32(int32(offset))

	case 4:
		// mov dword ptr [rbp + offset], r32d
		if needsREX {
			// For R8-R15, we still need REX but NOT REX.W (which would make it 64-bit)
			c.emitBytes(rex, 0x89, byte(0x85|(regNum<<3)))
		} else {
			c.emitBytes(0x89, byte(0x85|(regNum<<3)))
		}
		c.emitInt32(int32(offset))

	case 8:
		// mov qword ptr [rbp + offset], r64
		rex |= 0x08 // REX.W bit for 64-bit operand
		c.emitBytes(rex, 0x89, byte(0x85|(regNum<<3)))
		c.emitInt32(int32(offset))

	default:
		// Fallback to 8-byte
		rex |= 0x08 // REX.W bit
		c.emitBytes(rex, 0x89, byte(0x85|(regNum<<3)))
		c.emitInt32(int32(offset))
	}
}

// Floating point load from stack
func (c *compiler) emitFpLoadFromStack(xmmReg int, offset int, isDouble bool) {
	prefix := byte(0xF3) // movss
	if isDouble {
		prefix = 0xF2 // movsd
	}

	rex := byte(0)
	regNum := xmmReg
	
	if regNum >= 8 {
		rex = 0x44
		regNum -= 8
	}

	if rex != 0 {
		c.emitBytes(prefix, rex, 0x0F, 0x10, byte(0x85|(regNum<<3)))
	} else {
		c.emitBytes(prefix, 0x0F, 0x10, byte(0x85|(regNum<<3)))
	}
	c.emitInt32(int32(offset))
}

// Floating point store to stack
func (c *compiler) emitFpStoreToStack(xmmReg int, offset int, isDouble bool) {
	prefix := byte(0xF3) // movss
	if isDouble {
		prefix = 0xF2 // movsd
	}

	rex := byte(0)
	regNum := xmmReg
	
	if regNum >= 8 {
		rex = 0x44
		regNum -= 8
	}

	if rex != 0 {
		c.emitBytes(prefix, rex, 0x0F, 0x11, byte(0x85|(regNum<<3)))
	} else {
		c.emitBytes(prefix, 0x0F, 0x11, byte(0x85|(regNum<<3)))
	}
	c.emitInt32(int32(offset))
}

// Emit LEA with RIP-relative addressing (for globals)
func (c *compiler) emitLeaRipRelative(reg int, symbolName string) {
	rex := byte(0x48)
	regNum := reg
	
	if regNum >= 8 {
		rex |= 0x04
		regNum -= 8
	}

	// lea reg, [rip + disp32]
	c.emitBytes(rex, 0x8D, byte(0x05|(regNum<<3)))

	// Add relocation
	c.relocations = append(c.relocations, Relocation{
		Offset:     uint64(c.text.Len()),
		SymbolName: symbolName,
		Type:       R_X86_64_PC32,
		Addend:     -4,
	})
	c.emitUint32(0) // Placeholder
}

// Move GPR to XMM
func (c *compiler) emitMovdToXmm(xmmReg, gprReg int) {
	// movd xmm, reg
	rex := byte(0x48)
	xmmNum := xmmReg
	gprNum := gprReg
	
	if xmmNum >= 8 {
		rex |= 0x04
		xmmNum -= 8
	}
	if gprNum >= 8 {
		rex |= 0x01
		gprNum -= 8
	}

	c.emitBytes(0x66, rex, 0x0F, 0x6E, byte(0xC0|(xmmNum<<3)|gprNum))
}

// Move GPR to XMM (64-bit)
func (c *compiler) emitMovqToXmm(xmmReg, gprReg int) {
	// movq xmm, reg
	rex := byte(0x48)
	xmmNum := xmmReg
	gprNum := gprReg
	
	if xmmNum >= 8 {
		rex |= 0x04
		xmmNum -= 8
	}
	if gprNum >= 8 {
		rex |= 0x01
		gprNum -= 8
	}

	c.emitBytes(0x66, rex, 0x0F, 0x6E, byte(0xC0|(xmmNum<<3)|gprNum))
}

// XOR XMM registers
func (c *compiler) emitXorps(dst, src int) {
	rex := byte(0)
	dstNum := dst
	srcNum := src
	
	if dstNum >= 8 {
		rex |= 0x04
		dstNum -= 8
	}
	if srcNum >= 8 {
		rex |= 0x01
		srcNum -= 8
	}

	if rex != 0 {
		c.emitBytes(rex, 0x0F, 0x57, byte(0xC0|(dstNum<<3)|srcNum))
	} else {
		c.emitBytes(0x0F, 0x57, byte(0xC0|(dstNum<<3)|srcNum))
	}
}

// Store register with appropriate size encoding
func (c *compiler) emitStoreReg(reg, offset int, size int) {
	c.emitStoreToStack(reg, offset, size)
}

// Load register with appropriate size encoding  
func (c *compiler) emitLoadReg(reg, offset int) {
	c.emitLoadFromStack(reg, offset, 8)
}
