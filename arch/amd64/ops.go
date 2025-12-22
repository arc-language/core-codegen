package amd64

import (
	"fmt"
	"github.com/arc-language/core-builder/ir"
	"github.com/arc-language/core-builder/types"
)

// Regs
const (
	RAX = 0; RCX = 1; RDX = 2; RBX = 3
	RSP = 4; RBP = 5; RSI = 6; RDI = 7
)

func (c *compiler) compileInstruction(inst ir.Instruction) error {
	switch inst.Opcode() {
	case ir.OpAdd: return c.binOp(inst, 0x01)
	case ir.OpSub: return c.binOp(inst, 0x29)
	case ir.OpAnd: return c.binOp(inst, 0x21)
	case ir.OpOr:  return c.binOp(inst, 0x09)
	case ir.OpXor: return c.binOp(inst, 0x31)
	case ir.OpMul: return c.mulOp(inst)
	case ir.OpUDiv, ir.OpSDiv, ir.OpURem, ir.OpSRem: return c.divOp(inst)
	
	case ir.OpLoad:  return c.loadOp(inst.(*ir.LoadInst))
	case ir.OpStore: return c.storeOp(inst.(*ir.StoreInst))
	case ir.OpAlloca: return nil // Pre-allocated
	case ir.OpGetElementPtr: return c.gepOp(inst.(*ir.GetElementPtrInst))
	
	case ir.OpRet: return c.retOp(inst.(*ir.RetInst))
	case ir.OpBr:  return c.brOp(inst.(*ir.BrInst))
	case ir.OpCondBr: return c.condBrOp(inst.(*ir.CondBrInst))
	case ir.OpCall: return c.callOp(inst.(*ir.CallInst))
	
	case ir.OpICmp: return c.icmpOp(inst.(*ir.ICmpInst))
	case ir.OpZExt, ir.OpSExt, ir.OpTrunc: return c.castOp(inst.(*ir.CastInst))
	
	default: return fmt.Errorf("unsupported opcode: %s", inst.Opcode())
	}
}

func (c *compiler) binOp(inst ir.Instruction, op byte) error {
	c.emitLoadReg(RAX, c.stackMap[inst.Operands()[0]])
	c.emitLoadReg(RCX, c.stackMap[inst.Operands()[1]])
	c.emitBytes(0x48, op, 0xC8) // OP RAX, RCX
	c.emitStoreReg(RAX, c.stackMap[inst])
	return nil
}

func (c *compiler) mulOp(inst ir.Instruction) error {
	c.emitLoadReg(RAX, c.stackMap[inst.Operands()[0]])
	c.emitLoadReg(RCX, c.stackMap[inst.Operands()[1]])
	c.emitBytes(0x48, 0x0F, 0xAF, 0xC1) // IMUL RAX, RCX
	c.emitStoreReg(RAX, c.stackMap[inst])
	return nil
}

func (c *compiler) divOp(inst ir.Instruction) error {
	signed := inst.Opcode() == ir.OpSDiv || inst.Opcode() == ir.OpSRem
	rem := inst.Opcode() == ir.OpURem || inst.Opcode() == ir.OpSRem
	
	c.emitLoadReg(RAX, c.stackMap[inst.Operands()[0]])
	c.emitLoadReg(RCX, c.stackMap[inst.Operands()[1]])
	
	if signed {
		c.emitBytes(0x48, 0x99) // CQO
		c.emitBytes(0x48, 0xF7, 0xF9) // IDIV RCX
	} else {
		c.emitBytes(0x31, 0xD2) // XOR RDX, RDX
		c.emitBytes(0x48, 0xF7, 0xF1) // DIV RCX
	}
	
	if rem { c.emitStoreReg(RDX, c.stackMap[inst]) } else { c.emitStoreReg(RAX, c.stackMap[inst]) }
	return nil
}

func (c *compiler) loadOp(inst *ir.LoadInst) error {
	c.emitLoadReg(RAX, c.stackMap[inst.Operands()[0]]) // Ptr
	c.emitBytes(0x48, 0x8B, 0x00) // MOV RAX, [RAX]
	c.emitStoreReg(RAX, c.stackMap[inst])
	return nil
}

func (c *compiler) storeOp(inst *ir.StoreInst) error {
	c.emitLoadReg(RAX, c.stackMap[inst.Operands()[0]]) // Val
	c.emitLoadReg(RCX, c.stackMap[inst.Operands()[1]]) // Ptr
	c.emitBytes(0x48, 0x89, 0x01) // MOV [RCX], RAX
	return nil
}

func (c *compiler) gepOp(inst *ir.GetElementPtrInst) error {
	c.emitLoadReg(RAX, c.stackMap[inst.Operands()[0]]) // Base Ptr
	curTy := inst.SourceElementType
	
	for i, idx := range inst.Operands()[1:] {
		c.emitLoadReg(RCX, c.stackMap[idx])
		
		sz := 0
		if i == 0 {
			sz = SizeOf(curTy)
		} else {
			if st, ok := curTy.(*types.StructType); ok {
				if cInt, ok := idx.(*ir.ConstantInt); ok {
					offset := GetStructFieldOffset(st, int(cInt.Value))
					c.emitBytes(0x48, 0x05) // ADD RAX, imm32
					c.emitUint32(uint32(offset))
					curTy = st.Fields[cInt.Value]
					continue
				}
				return fmt.Errorf("struct index must be constant")
			} else if at, ok := curTy.(*types.ArrayType); ok {
				curTy = at.ElementType
				sz = SizeOf(curTy)
			} else if pt, ok := curTy.(*types.PointerType); ok {
				curTy = pt.ElementType
				sz = SizeOf(curTy)
			}
		}
		
		c.emitBytes(0x48, 0x69, 0xC9) // IMUL RCX, imm32
		c.emitUint32(uint32(sz))
		c.emitBytes(0x48, 0x01, 0xC8) // ADD RAX, RCX
	}
	c.emitStoreReg(RAX, c.stackMap[inst])
	return nil
}

func (c *compiler) retOp(inst *ir.RetInst) error {
	if inst.NumOperands() > 0 && inst.Operands()[0] != nil {
		c.emitLoadReg(RAX, c.stackMap[inst.Operands()[0]])
	}
	c.emitBytes(0xC9, 0xC3) // LEAVE; RET
	return nil
}

func (c *compiler) brOp(inst *ir.BrInst) error {
	c.emitBytes(0xE9); c.fixups = append(c.fixups, jumpFixup{c.text.Len(), inst.Target}); c.emitUint32(0)
	return nil
}

func (c *compiler) condBrOp(inst *ir.CondBrInst) error {
	c.emitLoadReg(RAX, c.stackMap[inst.Condition])
	c.emitBytes(0x48, 0x85, 0xC0) // TEST RAX, RAX
	c.emitBytes(0x0F, 0x85); c.fixups = append(c.fixups, jumpFixup{c.text.Len(), inst.TrueBlock}); c.emitUint32(0)
	c.emitBytes(0xE9); c.fixups = append(c.fixups, jumpFixup{c.text.Len(), inst.FalseBlock}); c.emitUint32(0)
	return nil
}

func (c *compiler) callOp(inst *ir.CallInst) error {
	// SysV regs: RDI, RSI, RDX, RCX, R8, R9
	regs := []int{RDI, RSI, RDX, RCX, 8, 9}
	for i, arg := range inst.Operands() {
		if i < 6 { c.emitLoadReg(regs[i], c.stackMap[arg]) }
	}
	c.emitBytes(0xE8); c.emitUint32(0) // CALL rel32 (Placeholder, needs ELF relocation for real linking)
	if inst.Type().Kind() != types.VoidKind { c.emitStoreReg(RAX, c.stackMap[inst]) }
	return nil
}

func (c *compiler) icmpOp(inst *ir.ICmpInst) error {
	c.emitLoadReg(RAX, c.stackMap[inst.Operands()[0]])
	c.emitLoadReg(RCX, c.stackMap[inst.Operands()[1]])
	c.emitBytes(0x48, 0x39, 0xC8) // CMP RAX, RCX
	
	var op byte
	switch inst.Predicate {
	case ir.ICmpEQ: op = 0x94
	case ir.ICmpNE: op = 0x95
	case ir.ICmpSLT: op = 0x9C
	case ir.ICmpSGT: op = 0x9F
	default: op = 0x94
	}
	c.emitBytes(0x0F, op, 0xC0) // SETcc AL
	c.emitBytes(0x48, 0x0F, 0xB6, 0xC0) // MOVZX RAX, AL
	c.emitStoreReg(RAX, c.stackMap[inst])
	return nil
}

func (c *compiler) castOp(inst *ir.CastInst) error {
	c.emitLoadReg(RAX, c.stackMap[inst.Operands()[0]])
	// Naive: Assuming implicit truncation/extension via 64-bit registers
	if inst.Opcode() == ir.OpZExt {
		// Clear high bits if src < 64
		// (Simplified logic)
		c.emitBytes(0x48, 0x63, 0xC0) // MOVSXD (Example, actually complex)
	}
	c.emitStoreReg(RAX, c.stackMap[inst])
	return nil
}

func (c *compiler) emitLoadReg(reg, off int) {
	rex := byte(0x48); if reg >= 8 { rex |= 4; reg -= 8 }
	c.emitBytes(rex, 0x8B, byte(0x80|(reg<<3)|5)); c.emitUint32(uint32(off))
}

func (c *compiler) emitStoreReg(reg, off int) {
	rex := byte(0x48); if reg >= 8 { rex |= 4; reg -= 8 }
	c.emitBytes(rex, 0x89, byte(0x80|(reg<<3)|5)); c.emitUint32(uint32(off))
}