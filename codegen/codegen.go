package codegen

import (
	"bytes"
	"fmt"

	"github.com/arc-language/core-builder/ir"
	"github.com/arc-language/core-codegen/arch/amd64"
	"github.com/arc-language/core-codegen/format/elf"
)

// GenerateObject compiles an IR module to an ELF object file for AMD64
func GenerateObject(m *ir.Module) ([]byte, error) {
	// 1. Compile IR to machine code
	artifact, err := amd64.Compile(m)
	if err != nil {
		return nil, fmt.Errorf("compilation failed: %w", err)
	}

	// 2. Create ELF object file
	f := elf.NewFile()

	// Set target triple info if available
	if m.TargetTriple != "" {
		// Could parse and validate target triple
	}

	// 3. Add .text section (executable code)
	textSec := f.AddSection(".text", elf.SHT_PROGBITS, elf.SHF_ALLOC|elf.SHF_EXECINSTR, artifact.TextBuffer)
	textSec.Addralign = 16

	// 4. Add .data section (initialized global data)
	var dataSec *elf.Section
	if len(artifact.DataBuffer) > 0 {
		dataSec = f.AddSection(".data", elf.SHT_PROGBITS, elf.SHF_WRITE|elf.SHF_ALLOC, artifact.DataBuffer)
		dataSec.Addralign = 8
	}

	// 5. Add .bss section for uninitialized data (if needed)
	// For now we initialize everything, but could optimize later

	// 6. Add .rodata section for read-only data (if needed)
	// Could separate string literals and constants here

	// 7. Build symbol table
	// Add file symbol
	f.AddSymbol(m.Name, elf.MakeSymbolInfo(elf.STB_LOCAL, elf.STT_FILE), nil, 0, 0)

	// Track symbol objects for relocations
	symbolMap := make(map[string]*elf.Symbol)

	// Add section symbols (required by some linkers)
	if textSec != nil {
		sym := f.AddSymbol("", elf.MakeSymbolInfo(elf.STB_LOCAL, elf.STT_SECTION), textSec, 0, 0)
		symbolMap[".text"] = sym
	}
	if dataSec != nil {
		sym := f.AddSymbol("", elf.MakeSymbolInfo(elf.STB_LOCAL, elf.STT_SECTION), dataSec, 0, 0)
		symbolMap[".data"] = sym
	}

	// Add symbols from compilation
	for _, sym := range artifact.Symbols {
		var section *elf.Section
		var symType byte

		if sym.IsFunc {
			section = textSec
			symType = elf.STT_FUNC
		} else if sym.IsGlobal {
			section = dataSec
			symType = elf.STT_OBJECT
		}

		// Determine binding (local vs global)
		binding := elf.STB_GLOBAL // Default to global export
		
		// TODO: Could check function/global linkage from IR to determine
		// if it should be local, weak, etc.

		info := elf.MakeSymbolInfo(byte(binding), symType)
		elfSym := f.AddSymbol(sym.Name, info, section, sym.Offset, sym.Size)
		symbolMap[sym.Name] = elfSym
	}

	// 8. Add relocations
	if len(artifact.Relocations) > 0 {
		relaBuf := new(bytes.Buffer)

		for _, rel := range artifact.Relocations {
			// Find the symbol
			sym, ok := symbolMap[rel.SymbolName]
			if !ok {
				// External symbol - add as undefined
				info := elf.MakeSymbolInfo(elf.STB_GLOBAL, elf.STT_NOTYPE)
				sym = f.AddSymbol(rel.SymbolName, info, nil, 0, 0)
				symbolMap[rel.SymbolName] = sym
			}

			// Find symbol index
			symIdx := findSymbolIndex(f.Symbols, sym)

			// Write Elf64_Rela entry
			writeRela(relaBuf, rel.Offset, uint32(symIdx), uint32(rel.Type), rel.Addend)
		}

		// Add .rela.text section
		relaSec := f.AddSection(".rela.text", elf.SHT_RELA, elf.SHF_ALLOC, relaBuf.Bytes())
		relaSec.Link = uint32(len(f.Sections) - 1) // Link to .symtab (will be added later)
		relaSec.Info = uint32(textSec.Index)       // Applies to .text section
		relaSec.Entsize = 24                       // sizeof(Elf64_Rela)
		relaSec.Addralign = 8
	}

	// 9. Write to buffer
	buf := new(bytes.Buffer)
	if err := f.WriteTo(buf); err != nil {
		return nil, fmt.Errorf("ELF generation failed: %w", err)
	}

	return buf.Bytes(), nil
}

// GenerateExecutable compiles an IR module to an executable ELF binary
// This is more complex as it requires linking and setting up program headers
func GenerateExecutable(m *ir.Module, entryPoint string) ([]byte, error) {
	// For a simple executable:
	// 1. Generate object file
	// 2. Add program headers for loadable segments
	// 3. Set entry point
	// 4. Potentially link with libc/runtime
	
	// This is a more advanced feature - for now return error
	return nil, fmt.Errorf("executable generation not yet implemented - use object files with external linker")
}

// Helper to find symbol index
func findSymbolIndex(symbols []*elf.Symbol, target *elf.Symbol) int {
	for i, sym := range symbols {
		if sym == target {
			return i + 1 // +1 because null symbol is at index 0
		}
	}
	return 0
}

// Helper to write relocation entry
func writeRela(buf *bytes.Buffer, offset uint64, symIdx, relType uint32, addend int64) {
	// Elf64_Rela structure:
	// uint64 r_offset
	// uint64 r_info (sym << 32 | type)
	// int64  r_addend

	rinfo := (uint64(symIdx) << 32) | uint64(relType)

	buf.Write(encodeUint64(offset))
	buf.Write(encodeUint64(rinfo))
	buf.Write(encodeInt64(addend))
}

func encodeUint64(v uint64) []byte {
	b := make([]byte, 8)
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
	b[4] = byte(v >> 32)
	b[5] = byte(v >> 40)
	b[6] = byte(v >> 48)
	b[7] = byte(v >> 56)
	return b
}

func encodeInt64(v int64) []byte {
	return encodeUint64(uint64(v))
}

// GenerateAssembly generates human-readable assembly for debugging
func GenerateAssembly(m *ir.Module) (string, error) {
	// This would disassemble the machine code
	// For now, just return the IR string representation
	return m.String(), nil
}

// Optimize performs architecture-specific optimizations
func Optimize(m *ir.Module, level int) error {
	// Future: implement peephole optimizations, instruction selection improvements
	// Level 0: no optimization
	// Level 1: basic optimizations
	// Level 2: aggressive optimizations
	// Level 3: maximum optimizations (may increase compile time)
	return nil
}