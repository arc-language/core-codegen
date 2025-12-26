package amd64

import (
	"fmt"

	"github.com/arc-language/core-builder/ir"
	"github.com/arc-language/core-builder/types"
)

func (c *compiler) compileInstruction(inst ir.Instruction) error {
	switch inst.Opcode() {
	// Arithmetic
	case ir.OpAdd:
		return c.addOp(inst)
	case ir.OpSub:
		return c.subOp(inst)
	case ir.OpMul:
		return c.mulOp(inst)
	case ir.OpUDiv, ir.OpSDiv:
		return c.divOp(inst, false)
	case ir.OpURem, ir.OpSRem:
		return c.divOp(inst, true)

	// Floating point
	case ir.OpFAdd:
		return c.fpBinOp(inst, 0x58)
	case ir.OpFSub:
		return c.fpBinOp(inst, 0x5C)
	case ir.OpFMul:
		return c.fpBinOp(inst, 0x59)
	case ir.OpFDiv:
		return c.fpBinOp(inst, 0x5E)

	// Bitwise
	case ir.OpAnd:
		return c.andOp(inst)
	case ir.OpOr:
		return c.orOp(inst)
	case ir.OpXor:
		return c.xorOp(inst)
	case ir.OpShl:
		return c.shiftOp(inst, 0x00) // shl uses /4 -> 0xE0
	case ir.OpLShr:
		return c.shiftOp(inst, 0x08) // shr uses /5 -> 0xE8
	case ir.OpAShr:
		return c.shiftOp(inst, 0x18) // sar uses /7 -> 0xF8

	// Memory
	case ir.OpAlloca:
		return c.allocaOp(inst.(*ir.AllocaInst))
	case ir.OpLoad:
		return c.loadOp(inst.(*ir.LoadInst))
	case ir.OpStore:
		return c.storeOp(inst.(*ir.StoreInst))
	case ir.OpGetElementPtr:
		return c.gepOp(inst.(*ir.GetElementPtrInst))

	// Comparison
	case ir.OpICmp:
		return c.icmpOp(inst.(*ir.ICmpInst))
	case ir.OpFCmp:
		return c.fcmpOp(inst.(*ir.FCmpInst))

	// Control flow
	case ir.OpRet:
		return c.retOp(inst.(*ir.RetInst))
	case ir.OpBr:
		return c.brOp(inst.(*ir.BrInst))
	case ir.OpCondBr:
		return c.condBrOp(inst.(*ir.CondBrInst))
	case ir.OpSwitch:
		return c.switchOp(inst.(*ir.SwitchInst))

	// Casts
	case ir.OpTrunc, ir.OpZExt, ir.OpSExt:
		return c.intCastOp(inst.(*ir.CastInst))
	case ir.OpFPTrunc, ir.OpFPExt:
		return c.fpCastOp(inst.(*ir.CastInst))
	case ir.OpFPToUI, ir.OpFPToSI:
		return c.fpToIntOp(inst.(*ir.CastInst))
	case ir.OpUIToFP, ir.OpSIToFP:
		return c.intToFpOp(inst.(*ir.CastInst))
	case ir.OpPtrToInt, ir.OpIntToPtr, ir.OpBitcast:
		return c.bitcastOp(inst.(*ir.CastInst))

	// Other
	case ir.OpPhi:
		return c.phiOp(inst.(*ir.PhiInst))
	case ir.OpSelect:
		return c.selectOp(inst.(*ir.SelectInst))
	case ir.OpCall:
		return c.callOp(inst.(*ir.CallInst))
	case ir.OpSyscall:
		return c.syscallOp(inst.(*ir.SyscallInst))
	case ir.OpExtractValue:
		return c.extractValueOp(inst.(*ir.ExtractValueInst))
	case ir.OpInsertValue:
		return c.insertValueOp(inst.(*ir.InsertValueInst))

	default:
		return fmt.Errorf("unsupported opcode: %s", inst.Opcode())
	}
}

// Addition
func (c *compiler) addOp(inst ir.Instruction) error {
	ops := inst.Operands()
	lhs := ops[0]
	rhs := ops[1]

	c.loadToReg(RAX, lhs)

	// Check if rhs is a constant
	if constInt, ok := rhs.(*ir.ConstantInt); ok {
		if constInt.Value >= -128 && constInt.Value <= 127 {
			// 8-bit immediate: add rax, imm8 (48 83 C0 ib)
			c.emitBytes(0x48, 0x83, 0xC0, byte(constInt.Value))
		} else {
			// 32-bit immediate: add rax, imm32 (48 81 C0 id)
			c.emitBytes(0x48, 0x81, 0xC0)
			c.emitInt32(int32(constInt.Value))
		}
	} else {
		// Register form: add rax, rcx
		c.loadToReg(RCX, rhs)
		c.emitBytes(0x48, 0x01, 0xC8)
	}

	c.storeFromReg(RAX, inst)
	return nil
}

// Subtraction
func (c *compiler) subOp(inst ir.Instruction) error {
	ops := inst.Operands()
	lhs := ops[0]
	rhs := ops[1]

	c.loadToReg(RAX, lhs)

	// Check if rhs is a constant
	if constInt, ok := rhs.(*ir.ConstantInt); ok {
		if constInt.Value >= -128 && constInt.Value <= 127 {
			// 8-bit immediate: sub rax, imm8 (48 83 E8 ib)
			c.emitBytes(0x48, 0x83, 0xE8, byte(constInt.Value))
		} else {
			// 32-bit immediate: sub rax, imm32 (48 81 E8 id)
			c.emitBytes(0x48, 0x81, 0xE8)
			c.emitInt32(int32(constInt.Value))
		}
	} else {
		// Register form: sub rax, rcx
		c.loadToReg(RCX, rhs)
		c.emitBytes(0x48, 0x29, 0xC8)
	}

	c.storeFromReg(RAX, inst)
	return nil
}

// AND operation
func (c *compiler) andOp(inst ir.Instruction) error {
	ops := inst.Operands()
	lhs := ops[0]
	rhs := ops[1]

	c.loadToReg(RAX, lhs)

	// Check if rhs is a constant
	if constInt, ok := rhs.(*ir.ConstantInt); ok {
		if constInt.Value >= -128 && constInt.Value <= 127 {
			// 8-bit immediate: and rax, imm8 (48 83 E0 ib)
			c.emitBytes(0x48, 0x83, 0xE0, byte(constInt.Value))
		} else {
			// 32-bit immediate: and rax, imm32 (48 81 E0 id)
			c.emitBytes(0x48, 0x81, 0xE0)
			c.emitInt32(int32(constInt.Value))
		}
	} else {
		// Register form: and rax, rcx
		c.loadToReg(RCX, rhs)
		c.emitBytes(0x48, 0x21, 0xC8)
	}

	c.storeFromReg(RAX, inst)
	return nil
}

// OR operation
func (c *compiler) orOp(inst ir.Instruction) error {
	ops := inst.Operands()
	lhs := ops[0]
	rhs := ops[1]

	c.loadToReg(RAX, lhs)

	// Check if rhs is a constant
	if constInt, ok := rhs.(*ir.ConstantInt); ok {
		if constInt.Value >= -128 && constInt.Value <= 127 {
			// 8-bit immediate: or rax, imm8 (48 83 C8 ib)
			c.emitBytes(0x48, 0x83, 0xC8, byte(constInt.Value))
		} else {
			// 32-bit immediate: or rax, imm32 (48 81 C8 id)
			c.emitBytes(0x48, 0x81, 0xC8)
			c.emitInt32(int32(constInt.Value))
		}
	} else {
		// Register form: or rax, rcx
		c.loadToReg(RCX, rhs)
		c.emitBytes(0x48, 0x09, 0xC8)
	}

	c.storeFromReg(RAX, inst)
	return nil
}

// XOR operation
func (c *compiler) xorOp(inst ir.Instruction) error {
	ops := inst.Operands()
	lhs := ops[0]
	rhs := ops[1]

	c.loadToReg(RAX, lhs)

	// Check if rhs is a constant
	if constInt, ok := rhs.(*ir.ConstantInt); ok {
		if constInt.Value >= -128 && constInt.Value <= 127 {
			// 8-bit immediate: xor rax, imm8 (48 83 F0 ib)
			c.emitBytes(0x48, 0x83, 0xF0, byte(constInt.Value))
		} else {
			// 32-bit immediate: xor rax, imm32 (48 81 F0 id)
			c.emitBytes(0x48, 0x81, 0xF0)
			c.emitInt32(int32(constInt.Value))
		}
	} else {
		// Register form: xor rax, rcx
		c.loadToReg(RCX, rhs)
		c.emitBytes(0x48, 0x31, 0xC8)
	}

	c.storeFromReg(RAX, inst)
	return nil
}

// Multiplication
func (c *compiler) mulOp(inst ir.Instruction) error {
	ops := inst.Operands()
	c.loadToReg(RAX, ops[0])
	c.loadToReg(RCX, ops[1])

	// imul rax, rcx
	c.emitBytes(0x48, 0x0F, 0xAF, 0xC1)

	c.storeFromReg(RAX, inst)
	return nil
}

// Division and remainder
func (c *compiler) divOp(inst ir.Instruction, remainder bool) error {
	ops := inst.Operands()
	signed := inst.Opcode() == ir.OpSDiv || inst.Opcode() == ir.OpSRem

	c.loadToReg(RAX, ops[0]) // Dividend in RAX
	c.loadToReg(RCX, ops[1]) // Divisor in RCX

	if signed {
		// cqo - sign extend RAX into RDX:RAX
		c.emitBytes(0x48, 0x99)
		// idiv rcx
		c.emitBytes(0x48, 0xF7, 0xF9)
	} else {
		// xor rdx, rdx - zero out RDX
		c.emitBytes(0x48, 0x31, 0xD2)
		// div rcx
		c.emitBytes(0x48, 0xF7, 0xF1)
	}

	// Quotient in RAX, remainder in RDX
	if remainder {
		c.storeFromReg(RDX, inst)
	} else {
		c.storeFromReg(RAX, inst)
	}
	return nil
}

// Floating point binary operations
func (c *compiler) fpBinOp(inst ir.Instruction, opcode byte) error {
	ops := inst.Operands()

	// Load operands to XMM registers
	c.loadToFpReg(0, ops[0]) // XMM0
	c.loadToFpReg(1, ops[1]) // XMM1

	// Determine if single or double precision
	fpType := inst.Type().(*types.FloatType)
	prefix := byte(0xF2) // Default to double (sd)
	if fpType.BitWidth == 32 {
		prefix = 0xF3 // Single precision (ss)
	}

	// Execute operation: XMM0 = XMM0 op XMM1
	c.emitBytes(prefix, 0x0F, opcode, 0xC1)

	c.storeFromFpReg(0, inst)
	return nil
}

// Shift operations
func (c *compiler) shiftOp(inst ir.Instruction, opext byte) error {
	ops := inst.Operands()
	value := ops[0]
	amount := ops[1]

	c.loadToReg(RAX, value)

	if constInt, ok := amount.(*ir.ConstantInt); ok {
		// Immediate shift
		if constInt.Value == 1 {
			// Special encoding for shift by 1: 48 D1 E0+opext
			c.emitBytes(0x48, 0xD1, 0xE0|opext)
		} else {
			// Shift by immediate: 48 C1 E0+opext imm8
			c.emitBytes(0x48, 0xC1, 0xE0|opext, byte(constInt.Value))
		}
	} else {
		// Variable shift (amount in CL): 48 D3 E0+opext
		c.loadToReg(RCX, amount)
		c.emitBytes(0x48, 0xD3, 0xE0|opext)
	}

	c.storeFromReg(RAX, inst)
	return nil
}

// Alloca - stack allocation
func (c *compiler) allocaOp(inst *ir.AllocaInst) error {
	// Retrieve pre-calculated offset
	allocOffset, ok := c.allocaOffsets[inst]
	if !ok {
		return fmt.Errorf("unknown alloca instruction")
	}

	// lea rax, [rbp + allocOffset] (allocOffset is negative)
	c.emitBytes(0x48, 0x8D, 0x85)
	c.emitInt32(int32(allocOffset))

	// Store the address
	c.storeFromReg(RAX, inst)
	return nil
}

// Load from memory
func (c *compiler) loadOp(inst *ir.LoadInst) error {
	ptr := inst.Operands()[0]
	c.loadToReg(RAX, ptr) // Load pointer address

	// Determine size
	size := SizeOf(inst.Type())

	// mov rax, [rax]
	switch size {
	case 1:
		// movzx rax, byte ptr [rax]
		c.emitBytes(0x48, 0x0F, 0xB6, 0x00)
	case 2:
		// movzx rax, word ptr [rax]
		c.emitBytes(0x48, 0x0F, 0xB7, 0x00)
	case 4:
		// mov eax, [rax] (zero-extends to 64-bit)
		c.emitBytes(0x8B, 0x00)
	case 8:
		// mov rax, [rax]
		c.emitBytes(0x48, 0x8B, 0x00)
	default:
		return fmt.Errorf("unsupported load size: %d", size)
	}

	c.storeFromReg(RAX, inst)
	return nil
}

// Store to memory
func (c *compiler) storeOp(inst *ir.StoreInst) error {
	ops := inst.Operands()
	value := ops[0]
	ptr := ops[1]

	c.loadToReg(RAX, value) // Value to store
	c.loadToReg(RCX, ptr)   // Pointer

	size := SizeOf(value.Type())

	// mov [rcx], rax (with appropriate size)
	switch size {
	case 1:
		// mov byte ptr [rcx], al
		c.emitBytes(0x88, 0x01)
	case 2:
		// mov word ptr [rcx], ax
		c.emitBytes(0x66, 0x89, 0x01)
	case 4:
		// mov dword ptr [rcx], eax
		c.emitBytes(0x89, 0x01)
	case 8:
		// mov qword ptr [rcx], rax
		c.emitBytes(0x48, 0x89, 0x01)
	default:
		return fmt.Errorf("unsupported store size: %d", size)
	}

	return nil
}

// GetElementPtr - pointer arithmetic
func (c *compiler) gepOp(inst *ir.GetElementPtrInst) error {
	ops := inst.Operands()
	c.loadToReg(RAX, ops[0]) // Base pointer

	currentType := inst.SourceElementType

	for i, idx := range ops[1:] {
		// Calculate offset for this index
		var elemSize int

		if i == 0 {
			// First index: scale by the size of the base type
			elemSize = SizeOf(currentType)
		} else {
			// Subsequent indices: navigate through the type
			switch ty := currentType.(type) {
			case *types.ArrayType:
				elemSize = SizeOf(ty.ElementType)
				currentType = ty.ElementType
			case *types.StructType:
				// For structs, index must be constant
				if constIdx, ok := idx.(*ir.ConstantInt); ok {
					fieldIdx := int(constIdx.Value)
					offset := GetStructFieldOffset(ty, fieldIdx)

					// add rax, offset
					if offset <= 127 {
						c.emitBytes(0x48, 0x83, 0xC0, byte(offset))
					} else {
						c.emitBytes(0x48, 0x05)
						c.emitInt32(int32(offset))
					}

					currentType = ty.Fields[fieldIdx]
					continue
				}
				return fmt.Errorf("struct GEP requires constant index")
			case *types.PointerType:
				elemSize = SizeOf(ty.ElementType)
				currentType = ty.ElementType
			default:
				return fmt.Errorf("invalid GEP type: %T", ty)
			}
		}

		// Load index and multiply by element size
		if constIdx, ok := idx.(*ir.ConstantInt); ok {
			// Constant offset
			offset := int(constIdx.Value) * elemSize
			if offset != 0 {
				if offset >= -128 && offset <= 127 {
					c.emitBytes(0x48, 0x83, 0xC0, byte(offset))
				} else {
					c.emitBytes(0x48, 0x05)
					c.emitInt32(int32(offset))
				}
			}
		} else {
			// Variable offset
			c.loadToReg(RCX, idx)

			// imul rcx, elemSize
			if elemSize == 1 {
				// No scaling needed
			} else if elemSize <= 127 {
				c.emitBytes(0x48, 0x6B, 0xC9, byte(elemSize))
			} else {
				c.emitBytes(0x48, 0x69, 0xC9)
				c.emitInt32(int32(elemSize))
			}

			// add rax, rcx
			c.emitBytes(0x48, 0x01, 0xC8)
		}
	}

	c.storeFromReg(RAX, inst)
	return nil
}

// Integer comparison
func (c *compiler) icmpOp(inst *ir.ICmpInst) error {
	ops := inst.Operands()
	c.loadToReg(RAX, ops[0])
	c.loadToReg(RCX, ops[1])

	// cmp rax, rcx
	c.emitBytes(0x48, 0x39, 0xC8)

	// SETcc al
	var setcc byte
	switch inst.Predicate {
	case ir.ICmpEQ:
		setcc = 0x94 // sete
	case ir.ICmpNE:
		setcc = 0x95 // setne
	case ir.ICmpSLT:
		setcc = 0x9C // setl
	case ir.ICmpSLE:
		setcc = 0x9E // setle
	case ir.ICmpSGT:
		setcc = 0x9F // setg
	case ir.ICmpSGE:
		setcc = 0x9D // setge
	case ir.ICmpULT:
		setcc = 0x92 // setb
	case ir.ICmpULE:
		setcc = 0x96 // setbe
	case ir.ICmpUGT:
		setcc = 0x97 // seta
	case ir.ICmpUGE:
		setcc = 0x93 // setae
	default:
		return fmt.Errorf("unsupported icmp predicate: %v", inst.Predicate)
	}

	c.emitBytes(0x0F, setcc, 0xC0)

	// movzx rax, al
	c.emitBytes(0x48, 0x0F, 0xB6, 0xC0)

	c.storeFromReg(RAX, inst)
	return nil
}

// Floating point comparison
func (c *compiler) fcmpOp(inst *ir.FCmpInst) error {
	ops := inst.Operands()

	c.loadToFpReg(0, ops[0]) // XMM0
	c.loadToFpReg(1, ops[1]) // XMM1

	fpType := ops[0].Type().(*types.FloatType)
	prefix := byte(0xF2)
	if fpType.BitWidth == 32 {
		prefix = 0xF3
	}

	// ucomiss/ucomisd xmm0, xmm1
	c.emitBytes(prefix, 0x0F, 0x2E, 0xC1)

	// Map FCmp predicates to x86 condition codes
	var setcc byte
	switch inst.Predicate {
	case ir.FCmpOEQ:
		setcc = 0x94 // sete (equal, no parity)
	case ir.FCmpONE:
		setcc = 0x95 // setne
	case ir.FCmpOLT:
		setcc = 0x92 // setb (below)
	case ir.FCmpOLE:
		setcc = 0x96 // setbe
	case ir.FCmpOGT:
		setcc = 0x97 // seta (above)
	case ir.FCmpOGE:
		setcc = 0x93 // setae
	default:
		return fmt.Errorf("unsupported fcmp predicate: %v", inst.Predicate)
	}

	c.emitBytes(0x0F, setcc, 0xC0)
	c.emitBytes(0x48, 0x0F, 0xB6, 0xC0) // movzx rax, al

	c.storeFromReg(RAX, inst)
	return nil
}

// System Call (Linux x86_64)
func (c *compiler) syscallOp(inst *ir.SyscallInst) error {
	ops := inst.Operands()
	if len(ops) == 0 {
		return fmt.Errorf("syscall requires at least a syscall number")
	}

	// Linux x86_64 Syscall Calling Convention
	// Syscall Number: RAX
	// Args: RDI, RSI, RDX, R10, R8, R9
	// Return: RAX
	
	// Registers in order for arguments 1..6
	argRegs := []int{RDI, RSI, RDX, R10, R8, R9}

	// 1. Load Syscall Number into RAX (ops[0])
	c.loadToReg(RAX, ops[0])

	// 2. Load Arguments into specific registers
	// Note: args start at ops[1]
	for i, arg := range ops[1:] {
		if i >= len(argRegs) {
			return fmt.Errorf("too many arguments for syscall (max 6 supported)")
		}
		c.loadToReg(argRegs[i], arg)
	}

	// 3. Emit 'syscall' instruction
	// Opcode: 0F 05
	c.emitBytes(0x0F, 0x05)

	// 4. Store result (RAX) to stack slot allocated for this instruction
	// This captures the return value of the syscall
	c.storeFromReg(RAX, inst)

	return nil
}
