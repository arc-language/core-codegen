package amd64

import "github.com/arc-language/core-builder/types"

func SizeOf(t types.Type) int {
	switch t.Kind() {
	case types.IntegerKind:
		if t.BitSize() <= 8 { return 1 }
		if t.BitSize() <= 16 { return 2 }
		if t.BitSize() <= 32 { return 4 }
		return 8
	case types.PointerKind: return 8
	case types.ArrayKind:
		at := t.(*types.ArrayType)
		return int(at.Length) * SizeOf(at.ElementType)
	case types.StructKind:
		st := t.(*types.StructType)
		sz := 0
		for _, f := range st.Fields {
			fs := SizeOf(f)
			if sz%8 != 0 { sz += (8-(sz%8)) } // Simple 8-byte align
			sz += fs
		}
		if sz%8 != 0 { sz += (8-(sz%8)) }
		return sz
	default: return 8
	}
}

func GetStructFieldOffset(st *types.StructType, idx int) int {
	off := 0
	for i := 0; i < idx; i++ {
		off += SizeOf(st.Fields[i])
		if off%8 != 0 { off += (8-(off%8)) }
	}
	if off%8 != 0 { off += (8-(off%8)) }
	return off
}