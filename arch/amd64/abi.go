package amd64

import "github.com/arc-language/core-builder/types"

// SizeOf returns the size in bytes of a type following AMD64 ABI
func SizeOf(t types.Type) int {
	switch t.Kind() {
	case types.VoidKind:
		return 0

	case types.IntegerKind:
		bits := t.(*types.IntType).BitWidth
		if bits <= 8 {
			return 1
		}
		if bits <= 16 {
			return 2
		}
		if bits <= 32 {
			return 4
		}
		if bits <= 64 {
			return 8
		}
		// For larger integers, round up to 8-byte boundary
		return ((bits + 63) / 64) * 8

	case types.FloatKind:
		bits := t.(*types.FloatType).BitWidth
		if bits == 16 {
			return 2
		}
		if bits == 32 {
			return 4
		}
		if bits == 64 {
			return 8
		}
		if bits == 128 {
			return 16
		}
		return 8

	case types.PointerKind:
		return 8 // Always 8 bytes on AMD64

	case types.ArrayKind:
		at := t.(*types.ArrayType)
		elemSize := SizeOf(at.ElementType)
		return int(at.Length) * elemSize

	case types.StructKind:
		st := t.(*types.StructType)
		if st.Packed {
			// Packed struct - no padding
			size := 0
			for _, field := range st.Fields {
				size += SizeOf(field)
			}
			return size
		}
		// Normal struct - with alignment
		return GetStructSize(st)

	case types.VectorKind:
		vt := t.(*types.VectorType)
		if vt.Scalable {
			// Scalable vectors are runtime-determined
			return 0
		}
		elemSize := SizeOf(vt.ElementType)
		totalSize := elemSize * vt.Length
		// Vectors are typically aligned to their size (up to 16/32 bytes)
		if totalSize < 16 {
			return totalSize
		}
		return ((totalSize + 15) / 16) * 16

	case types.FunctionKind:
		return 8 // Function pointers

	case types.LabelKind:
		return 0

	default:
		return 8 // Default fallback
	}
}

// AlignOf returns the alignment requirement in bytes
func AlignOf(t types.Type) int {
	switch t.Kind() {
	case types.VoidKind, types.LabelKind:
		return 1

	case types.IntegerKind:
		bits := t.(*types.IntType).BitWidth
		if bits <= 8 {
			return 1
		}
		if bits <= 16 {
			return 2
		}
		if bits <= 32 {
			return 4
		}
		return 8

	case types.FloatKind:
		bits := t.(*types.FloatType).BitWidth
		if bits == 16 {
			return 2
		}
		if bits == 32 {
			return 4
		}
		if bits == 64 {
			return 8
		}
		if bits == 128 {
			return 16
		}
		return 8

	case types.PointerKind:
		return 8

	case types.ArrayKind:
		at := t.(*types.ArrayType)
		return AlignOf(at.ElementType)

	case types.StructKind:
		st := t.(*types.StructType)
		if st.Packed {
			return 1
		}
		// Struct alignment is the maximum of its field alignments
		maxAlign := 1
		for _, field := range st.Fields {
			align := AlignOf(field)
			if align > maxAlign {
				maxAlign = align
			}
		}
		return maxAlign

	case types.VectorKind:
		vt := t.(*types.VectorType)
		totalSize := SizeOf(vt.ElementType) * vt.Length
		if totalSize <= 16 {
			return totalSize
		}
		return 16

	case types.FunctionKind:
		return 8

	default:
		return 8
	}
}

// GetStructSize returns the total size of a struct with proper alignment
func GetStructSize(st *types.StructType) int {
	if st.Packed {
		size := 0
		for _, field := range st.Fields {
			size += SizeOf(field)
		}
		return size
	}

	offset := 0
	for _, field := range st.Fields {
		fieldAlign := AlignOf(field)
		// Align offset to field alignment
		if offset%fieldAlign != 0 {
			offset += fieldAlign - (offset % fieldAlign)
		}
		offset += SizeOf(field)
	}

	// Align struct size to its own alignment
	structAlign := AlignOf(st)
	if offset%structAlign != 0 {
		offset += structAlign - (offset % structAlign)
	}

	return offset
}

// GetStructFieldOffset returns the byte offset of a field in a struct
func GetStructFieldOffset(st *types.StructType, fieldIndex int) int {
	if fieldIndex < 0 || fieldIndex >= len(st.Fields) {
		return 0
	}

	if st.Packed {
		// Packed - just sum sizes
		offset := 0
		for i := 0; i < fieldIndex; i++ {
			offset += SizeOf(st.Fields[i])
		}
		return offset
	}

	// Normal struct - account for alignment
	offset := 0
	for i := 0; i < fieldIndex; i++ {
		field := st.Fields[i]
		fieldAlign := AlignOf(field)

		// Align to field alignment
		if offset%fieldAlign != 0 {
			offset += fieldAlign - (offset % fieldAlign)
		}
		offset += SizeOf(field)
	}

	// Align to the target field's alignment
	if fieldIndex < len(st.Fields) {
		fieldAlign := AlignOf(st.Fields[fieldIndex])
		if offset%fieldAlign != 0 {
			offset += fieldAlign - (offset % fieldAlign)
		}
	}

	return offset
}

// GetArrayElementOffset returns the byte offset of an element in an array
func GetArrayElementOffset(at *types.ArrayType, index int64) int {
	elemSize := SizeOf(at.ElementType)
	return int(index) * elemSize
}

// IsPassedInRegisters determines if a type should be passed in registers
// following System V AMD64 ABI
func IsPassedInRegisters(t types.Type) bool {
	size := SizeOf(t)

	// Types larger than 16 bytes are passed by reference
	if size > 16 {
		return false
	}

	switch t.Kind() {
	case types.IntegerKind, types.PointerKind:
		return size <= 8

	case types.FloatKind:
		return size <= 16

	case types.StructKind:
		// Small structs can be passed in registers
		// This is simplified - full ABI is more complex
		return size <= 16

	case types.ArrayKind:
		// Arrays are typically passed by reference
		return false

	default:
		return false
	}
}

// ClassifyParameter classifies a parameter for System V calling convention
type ParamClass int

const (
	ParamInteger ParamClass = iota // Pass in integer register
	ParamSSE                        // Pass in XMM register
	ParamMemory                     // Pass on stack
	ParamX87                        // x87 FPU (rarely used)
)

func ClassifyParameter(t types.Type) ParamClass {
	switch t.Kind() {
	case types.IntegerKind, types.PointerKind:
		if SizeOf(t) <= 8 {
			return ParamInteger
		}
		return ParamMemory

	case types.FloatKind:
		return ParamSSE

	case types.StructKind, types.ArrayKind:
		size := SizeOf(t)
		if size > 16 {
			return ParamMemory
		}
		// Complex rules for struct classification
		// Simplified: small structs in integer registers
		return ParamInteger

	default:
		return ParamMemory
	}
}