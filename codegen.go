package codegen

import (
	"bytes"
	"fmt"

	"github.com/arc-language/core-builder/ir"
	"github.com/arc-language/core-codegen/arch/amd64"
	"github.com/arc-language/core-codegen/format/elf"
)

// GenerateObject compiles the module to an ELF object file (.o) for AMD64.
func GenerateObject(m *ir.Module) ([]byte, error) {
	// 1. Compile IR to Machine Code (Architecture specific)
	artifact, err := amd64.Compile(m)
	if err != nil {
		return nil, fmt.Errorf("amd64 compilation failed: %w", err)
	}

	// 2. Wrap Machine Code in Container Format (OS/Linker specific)
	// We are generating an ELF64 Relocatable Object file.
	f := elf.NewFile()

	// Add .text section (Machine code)
	textSec := f.AddSection(".text", elf.SHT_PROGBITS, elf.SHF_ALLOC|elf.SHF_EXECINSTR, artifact.TextBuffer)
	textSec.Addralign = 16

	// Add .data section (Global variables)
	dataSec := f.AddSection(".data", elf.SHT_PROGBITS, elf.SHF_WRITE|elf.SHF_ALLOC, artifact.DataBuffer)
	dataSec.Addralign = 8

	// Add Symbol Table entries
	// First, the FILE symbol
	f.AddSymbol(m.Name, elf.STT_FILE, nil, 0, 0)

	for _, sym := range artifact.Symbols {
		var sec *elf.Section
		var info byte

		if sym.IsFunc {
			sec = textSec
			info = elf.STT_FUNC
		} else {
			sec = dataSec
			info = elf.STT_OBJECT
		}

		// Set Global binding (High nibble = 1) if needed. 
		// For now we assume all compiled symbols are global/exported.
		info |= (1 << 4)

		f.AddSymbol(sym.Name, info, sec, sym.Offset, sym.Size)
	}

	// 3. Serialize to bytes
	buf := new(bytes.Buffer)
	if err := f.WriteTo(buf); err != nil {
		return nil, fmt.Errorf("elf serialization failed: %w", err)
	}

	return buf.Bytes(), nil
}