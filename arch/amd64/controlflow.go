package amd64

import (
	"fmt"

	"github.com/arc-language/core-builder/ir"
	"github.com/arc-language/core-builder/types"
)

// Return instruction
func (c *compiler) retOp(inst *ir.RetInst) error {
	if inst.NumOperands() > 0 && inst.Operands()[0] != nil {
		retVal := inst.Operands()[0]

		// Check if it's a float return
		if types.IsFloat(retVal.Type()) {
			c.loadToFpReg(0, retVal) // Return in XMM0
		} else {
			c.loadToReg(RAX, retVal) // Return in RAX
		}
	}

	// Epilogue
	// leave (equivalent to: mov rsp, rbp; pop rbp)
	c.emitBytes(0xC9)
	// ret
	c.emitBytes(0xC3)

	return nil
}

// Unconditional branch
func (c *compiler) brOp(inst *ir.BrInst) error {
	// jmp rel32
	c.emitBytes(0xE9)
	c.fixups = append(c.fixups, jumpFixup{
		offset: c.text.Len(),
		target: inst.Target,
	})
	c.emitUint32(0) // Placeholder

	return nil
}

// Conditional branch
func (c *compiler) condBrOp(inst *ir.CondBrInst) error {
	c.loadToReg(RAX, inst.Condition)

	// test rax, rax
	c.emitBytes(0x48, 0x85, 0xC0)

	// jnz trueBlock (jump if not zero)
	c.emitBytes(0x0F, 0x85)
	c.fixups = append(c.fixups, jumpFixup{
		offset: c.text.Len(),
		target: inst.TrueBlock,
	})
	c.emitUint32(0) // Placeholder

	// jmp falseBlock
	c.emitBytes(0xE9)
	c.fixups = append(c.fixups, jumpFixup{
		offset: c.text.Len(),
		target: inst.FalseBlock,
	})
	c.emitUint32(0) // Placeholder

	return nil
}

// Switch instruction
func (c *compiler) switchOp(inst *ir.SwitchInst) error {
	c.loadToReg(RAX, inst.Condition)

	// Generate comparison chain
	for _, switchCase := range inst.Cases {
		// cmp rax, case_value
		if switchCase.Value.Value >= -128 && switchCase.Value.Value <= 127 {
			c.emitBytes(0x48, 0x83, 0xF8, byte(switchCase.Value.Value))
		} else {
			c.emitBytes(0x48, 0x3D)
			c.emitInt32(int32(switchCase.Value.Value))
		}

		// je case_block
		c.emitBytes(0x0F, 0x84)
		c.fixups = append(c.fixups, jumpFixup{
			offset: c.text.Len(),
			target: switchCase.Block,
		})
		c.emitUint32(0)
	}

	// Jump to default block
	c.emitBytes(0xE9)
	c.fixups = append(c.fixups, jumpFixup{
		offset: c.text.Len(),
		target: inst.DefaultBlock,
	})
	c.emitUint32(0)

	return nil
}

// Phi node - handled specially
func (c *compiler) phiOp(inst *ir.PhiInst) error {
	// Phi nodes are typically handled by the register allocator
	// For our simple implementation, we just pick the first incoming value
	// A proper implementation would track which predecessor we came from
	// and load the appropriate value

	// For now, this is a placeholder - phi handling requires more sophisticated
	// control flow analysis
	return nil
}

// Select (ternary operator)
func (c *compiler) selectOp(inst *ir.SelectInst) error {
	ops := inst.Operands()
	cond := ops[0]
	trueVal := ops[1]
	falseVal := ops[2]

	c.loadToReg(RAX, cond)
	c.loadToReg(RCX, trueVal)
	c.loadToReg(RDX, falseVal)

	// test rax, rax
	c.emitBytes(0x48, 0x85, 0xC0)

	// cmovz rcx, rdx (move rdx to rcx if zero)
	c.emitBytes(0x48, 0x0F, 0x44, 0xCA)

	// Result in RCX
	c.storeFromReg(RCX, inst)
	return nil
}

// Function call
func (c *compiler) callOp(inst *ir.CallInst) error {
	ops := inst.Operands()

	// System V AMD64 ABI calling convention
	// Integer/pointer args: RDI, RSI, RDX, RCX, R8, R9, then stack
	// Float args: XMM0-XMM7, then stack
	// Return: RAX (integer), XMM0 (float)

	intArgRegs := []int{RDI, RSI, RDX, RCX, R8, R9}
	fpArgRegs := []int{0, 1, 2, 3, 4, 5, 6, 7} // XMM0-XMM7

	intArgIdx := 0
	fpArgIdx := 0
	stackArgs := []ir.Value{}

	// Classify and place arguments
	for _, arg := range ops {
		if types.IsFloat(arg.Type()) {
			if fpArgIdx < len(fpArgRegs) {
				c.loadToFpReg(fpArgRegs[fpArgIdx], arg)
				fpArgIdx++
			} else {
				stackArgs = append(stackArgs, arg)
			}
		} else {
			if intArgIdx < len(intArgRegs) {
				c.loadToReg(intArgRegs[intArgIdx], arg)
				intArgIdx++
			} else {
				stackArgs = append(stackArgs, arg)
			}
		}
	}

	// Push stack arguments in reverse order
	for i := len(stackArgs) - 1; i >= 0; i-- {
		c.loadToReg(RAX, stackArgs[i])
		// push rax
		c.emitBytes(0x50)
	}

	// Align stack to 16 bytes if needed (ABI requirement)
	stackAdjust := len(stackArgs) * 8
	if stackAdjust%16 != 0 {
		// sub rsp, 8
		c.emitBytes(0x48, 0x83, 0xEC, 0x08)
		stackAdjust += 8
	}

	// Emit call
	calleeName := inst.CalleeName
	if inst.Callee != nil {
		calleeName = inst.Callee.Name()
	}

	// call rel32
	c.emitBytes(0xE8)

	// Add relocation for the call
	c.relocations = append(c.relocations, Relocation{
		Offset:     uint64(c.text.Len()),
		SymbolName: calleeName,
		Type:       R_X86_64_PLT32,
		Addend:     -4,
	})
	c.emitUint32(0) // Placeholder

	// Clean up stack
	if stackAdjust > 0 {
		if stackAdjust <= 127 {
			c.emitBytes(0x48, 0x83, 0xC4, byte(stackAdjust))
		} else {
			c.emitBytes(0x48, 0x81, 0xC4)
			c.emitUint32(uint32(stackAdjust))
		}
	}

	// Store return value
	if inst.Type() != nil && inst.Type().Kind() != types.VoidKind {
		if types.IsFloat(inst.Type()) {
			c.storeFromFpReg(0, inst)
		} else {
			c.storeFromReg(RAX, inst)
		}
	}

	return nil
}

// Extract value from aggregate
func (c *compiler) extractValueOp(inst *ir.ExtractValueInst) error {
	agg := inst.Operands()[0]
	c.loadToReg(RAX, agg)

	// Calculate offset based on indices
	currentType := agg.Type()
	offset := 0

	for _, idx := range inst.Indices {
		switch ty := currentType.(type) {
		case *types.StructType:
			offset += GetStructFieldOffset(ty, idx)
			currentType = ty.Fields[idx]
		case *types.ArrayType:
			elemSize := SizeOf(ty.ElementType)
			offset += idx * elemSize
			currentType = ty.ElementType
		default:
			return fmt.Errorf("extractvalue on non-aggregate type: %T", ty)
		}
	}

	// Load from aggregate + offset
	if offset > 0 {
		if offset <= 127 {
			c.emitBytes(0x48, 0x83, 0xC0, byte(offset))
		} else {
			c.emitBytes(0x48, 0x05)
			c.emitInt32(int32(offset))
		}
	}

	// Load the value
	size := SizeOf(inst.Type())
	switch size {
	case 1:
		c.emitBytes(0x48, 0x0F, 0xB6, 0x00) // movzx rax, byte ptr [rax]
	case 2:
		c.emitBytes(0x48, 0x0F, 0xB7, 0x00) // movzx rax, word ptr [rax]
	case 4:
		c.emitBytes(0x8B, 0x00) // mov eax, [rax]
	case 8:
		c.emitBytes(0x48, 0x8B, 0x00) // mov rax, [rax]
	}

	c.storeFromReg(RAX, inst)
	return nil
}

// Insert value into aggregate
func (c *compiler) insertValueOp(inst *ir.InsertValueInst) error {
	ops := inst.Operands()
	agg := ops[0]
	value := ops[1]

	// This is complex - need to copy aggregate and modify one field
	// For simplicity, we'll load the aggregate, modify it, and store back
	// A proper implementation would use temporary storage

	c.loadToReg(RCX, agg) // Aggregate address/value
	c.loadToReg(RAX, value)

	// Calculate offset
	currentType := agg.Type()
	offset := 0

	for _, idx := range inst.Indices {
		switch ty := currentType.(type) {
		case *types.StructType:
			offset += GetStructFieldOffset(ty, idx)
			currentType = ty.Fields[idx]
		case *types.ArrayType:
			elemSize := SizeOf(ty.ElementType)
			offset += idx * elemSize
			currentType = ty.ElementType
		}
	}

	// Store value at aggregate + offset
	if offset > 0 {
		if offset <= 127 {
			c.emitBytes(0x48, 0x83, 0xC1, byte(offset))
		} else {
			c.emitBytes(0x48, 0x81, 0xC1)
			c.emitInt32(int32(offset))
		}
	}

	size := SizeOf(value.Type())
	switch size {
	case 1:
		c.emitBytes(0x88, 0x01) // mov byte ptr [rcx], al
	case 2:
		c.emitBytes(0x66, 0x89, 0x01) // mov word ptr [rcx], ax
	case 4:
		c.emitBytes(0x89, 0x01) // mov dword ptr [rcx], eax
	case 8:
		c.emitBytes(0x48, 0x89, 0x01) // mov qword ptr [rcx], rax
	}

	c.storeFromReg(RCX, inst)
	return nil
}

// Integer cast operations
func (c *compiler) intCastOp(inst *ir.CastInst) error {
	src := inst.Operands()[0]
	c.loadToReg(RAX, src)

	srcSize := SizeOf(src.Type())

	switch inst.Opcode() {
	case ir.OpTrunc:
		// Truncation - just take lower bits (already in RAX)
		// No operation needed, storing will handle it

	case ir.OpZExt:
		// Zero extension
		switch srcSize {
		case 1:
			c.emitBytes(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al
		case 2:
			c.emitBytes(0x48, 0x0F, 0xB7, 0xC0) // movzx rax, ax
		case 4:
			c.emitBytes(0x89, 0xC0) // mov eax, eax (zero-extends)
		}

	case ir.OpSExt:
		// Sign extension
		switch srcSize {
		case 1:
			c.emitBytes(0x48, 0x0F, 0xBE, 0xC0) // movsx rax, al
		case 2:
			c.emitBytes(0x48, 0x0F, 0xBF, 0xC0) // movsx rax, ax
		case 4:
			c.emitBytes(0x48, 0x63, 0xC0) // movsxd rax, eax
		}
	}

	c.storeFromReg(RAX, inst)
	return nil
}

// Floating point cast operations
func (c *compiler) fpCastOp(inst *ir.CastInst) error {
	src := inst.Operands()[0]
	srcType := src.Type().(*types.FloatType)
	dstType := inst.Type().(*types.FloatType)

	c.loadToFpReg(0, src)

	if srcType.BitWidth == 32 && dstType.BitWidth == 64 {
		// cvtss2sd xmm0, xmm0
		c.emitBytes(0xF3, 0x0F, 0x5A, 0xC0)
	} else if srcType.BitWidth == 64 && dstType.BitWidth == 32 {
		// cvtsd2ss xmm0, xmm0
		c.emitBytes(0xF2, 0x0F, 0x5A, 0xC0)
	}

	c.storeFromFpReg(0, inst)
	return nil
}

// Float to integer conversion
func (c *compiler) fpToIntOp(inst *ir.CastInst) error {
	src := inst.Operands()[0]
	srcType := src.Type().(*types.FloatType)

	c.loadToFpReg(0, src)

	if srcType.BitWidth == 32 {
		// cvttss2si rax, xmm0
		c.emitBytes(0xF3, 0x48, 0x0F, 0x2C, 0xC0)
	} else {
		// cvttsd2si rax, xmm0
		c.emitBytes(0xF2, 0x48, 0x0F, 0x2C, 0xC0)
	}

	c.storeFromReg(RAX, inst)
	return nil
}

// Integer to float conversion
func (c *compiler) intToFpOp(inst *ir.CastInst) error {
	src := inst.Operands()[0]
	dstType := inst.Type().(*types.FloatType)

	c.loadToReg(RAX, src)

	if dstType.BitWidth == 32 {
		// cvtsi2ss xmm0, rax
		c.emitBytes(0xF3, 0x48, 0x0F, 0x2A, 0xC0)
	} else {
		// cvtsi2sd xmm0, rax
		c.emitBytes(0xF2, 0x48, 0x0F, 0x2A, 0xC0)
	}

	c.storeFromFpReg(0, inst)
	return nil
}

// Bitcast and pointer casts
func (c *compiler) bitcastOp(inst *ir.CastInst) error {
	src := inst.Operands()[0]

	// For bitcast, just copy the bits
	// For pointer/int conversions, also just copy
	c.loadToReg(RAX, src)
	c.storeFromReg(RAX, inst)

	return nil
}