package elf

import (
	"bytes"
	"encoding/binary"
	"io"
)

// ELF64 Constants
const (
	// ELF Header
	EI_NIDENT   = 16
	EI_MAG0     = 0
	ELFMAG0     = 0x7f
	ELFMAG1     = 'E'
	ELFMAG2     = 'L'
	ELFMAG3     = 'F'
	EI_CLASS    = 4
	ELFCLASS64  = 2
	EI_DATA     = 5
	ELFDATA2LSB = 1
	EI_VERSION  = 6
	EV_CURRENT  = 1

	// Object file types
	ET_NONE = 0
	ET_REL  = 1
	ET_EXEC = 2
	ET_DYN  = 3
	ET_CORE = 4

	// Machine types
	EM_X86_64 = 62

	// Section types
	SHT_NULL     = 0
	SHT_PROGBITS = 1
	SHT_SYMTAB   = 2
	SHT_STRTAB   = 3
	SHT_RELA     = 4
	SHT_HASH     = 5
	SHT_DYNAMIC  = 6
	SHT_NOTE     = 7
	SHT_NOBITS   = 8
	SHT_REL      = 9

	// Section flags
	SHF_WRITE     = 0x1
	SHF_ALLOC     = 0x2
	SHF_EXECINSTR = 0x4
	SHF_MERGE     = 0x10
	SHF_STRINGS   = 0x20

	// Symbol binding
	STB_LOCAL  = 0
	STB_GLOBAL = 1
	STB_WEAK   = 2

	// Symbol types
	STT_NOTYPE  = 0
	STT_OBJECT  = 1
	STT_FUNC    = 2
	STT_SECTION = 3
	STT_FILE    = 4
	STT_COMMON  = 5
	STT_TLS     = 6

	// Symbol visibility
	STV_DEFAULT   = 0
	STV_INTERNAL  = 1
	STV_HIDDEN    = 2
	STV_PROTECTED = 3

	// Special section indices
	SHN_UNDEF = 0
	SHN_ABS   = 0xfff1

	// Relocation types for x86-64
	R_X86_64_NONE   = 0
	R_X86_64_64     = 1
	R_X86_64_PC32   = 2
	R_X86_64_GOT32  = 3
	R_X86_64_PLT32  = 4
	R_X86_64_COPY   = 5
	R_X86_64_32     = 10
	R_X86_64_32S    = 11
	R_X86_64_16     = 12
	R_X86_64_PC16   = 13
	R_X86_64_8      = 14
	R_X86_64_PC8    = 15
	R_X86_64_PC64   = 24
)

// File represents an ELF object file
type File struct {
	Sections   []*Section
	Symbols    []*Symbol
	StrTab     *StringTable
	ShStrTab   *StringTable
	DataLayout string
	Machine    uint16
}

// Section represents an ELF section
type Section struct {
	Name      string
	Type      uint32
	Flags     uint64
	Addr      uint64
	Addralign uint64
	Entsize   uint64
	Link      uint32
	Info      uint32
	Content   []byte

	// Internal
	index    uint16
	nameIdx  uint32
	offset   uint64
	size     uint64
}

// Symbol represents an ELF symbol
type Symbol struct {
	Name    string
	Info    byte // Binding (high 4 bits) | Type (low 4 bits)
	Other   byte // Visibility
	Section *Section
	Value   uint64
	Size    uint64

	// Internal
	nameIdx uint32
}

// Relocation represents a relocation entry
type Relocation struct {
	Offset uint64
	Symbol *Symbol
	Type   uint32
	Addend int64
}

// StringTable manages string storage
type StringTable struct {
	Data []byte
	strs map[string]uint32 // Deduplication
}

func NewStringTable() *StringTable {
	return &StringTable{
		Data: []byte{0}, // Null string at index 0
		strs: make(map[string]uint32),
	}
}

func (st *StringTable) Add(s string) uint32 {
	if s == "" {
		return 0
	}

	// Check if already exists
	if idx, exists := st.strs[s]; exists {
		return idx
	}

	idx := uint32(len(st.Data))
	st.Data = append(st.Data, []byte(s)...)
	st.Data = append(st.Data, 0)
	st.strs[s] = idx
	return idx
}

// NewFile creates a new ELF object file
func NewFile() *File {
	f := &File{
		StrTab:   NewStringTable(),
		ShStrTab: NewStringTable(),
		Machine:  EM_X86_64,
	}

	// Section 0 is always the null section
	f.Sections = append(f.Sections, &Section{
		Name: "",
		Type: SHT_NULL,
	})

	return f
}

// AddSection adds a new section
func (f *File) AddSection(name string, typ uint32, flags uint64, content []byte) *Section {
	s := &Section{
		Name:    name,
		Type:    typ,
		Flags:   flags,
		Content: content,
		index:   uint16(len(f.Sections)),
	}

	f.Sections = append(f.Sections, s)
	return s
}

// AddSymbol adds a new symbol
func (f *File) AddSymbol(name string, info byte, section *Section, value, size uint64) *Symbol {
	sym := &Symbol{
		Name:    name,
		Info:    info,
		Other:   STV_DEFAULT,
		Section: section,
		Value:   value,
		Size:    size,
	}

	f.Symbols = append(f.Symbols, sym)
	return sym
}

// AddRelocation adds a relocation for a section
func (f *File) AddRelocation(section *Section, offset uint64, symbol *Symbol, relType uint32, addend int64) {
	// Relocations are stored with the section they apply to
	// We'll need to track them separately and create .rela sections later
}

// WriteTo writes the complete ELF file
func (f *File) WriteTo(w io.Writer) error {
	// 1. Build string tables
	for _, sec := range f.Sections {
		sec.nameIdx = f.ShStrTab.Add(sec.Name)
	}

	for _, sym := range f.Symbols {
		sym.nameIdx = f.StrTab.Add(sym.Name)
	}

	// 2. Add string table sections
	f.AddSection(".shstrtab", SHT_STRTAB, 0, f.ShStrTab.Data)
	strTabSec := f.AddSection(".strtab", SHT_STRTAB, 0, f.StrTab.Data)
	strTabSec.Addralign = 1

	// 3. Build symbol table
	symBuf := new(bytes.Buffer)

	// First symbol is always null
	f.writeSymbol(symBuf, &Symbol{})

	// Track first global symbol index
	firstGlobal := 1
	localCount := 0

	// Write local symbols first
	for _, sym := range f.Symbols {
		binding := sym.Info >> 4
		if binding == STB_LOCAL {
			f.writeSymbol(symBuf, sym)
			localCount++
		}
	}

	firstGlobal = localCount + 1

	// Write global symbols
	for _, sym := range f.Symbols {
		binding := sym.Info >> 4
		if binding != STB_LOCAL {
			f.writeSymbol(symBuf, sym)
		}
	}

	symTabSec := f.AddSection(".symtab", SHT_SYMTAB, 0, symBuf.Bytes())
	symTabSec.Link = uint32(strTabSec.index)
	symTabSec.Info = uint32(firstGlobal) // Index of first global symbol
	symTabSec.Addralign = 8
	symTabSec.Entsize = 24 // sizeof(Elf64_Sym)

	// 4. Calculate section offsets
	headerSize := uint64(64) // sizeof(Elf64_Ehdr)
	currentOffset := headerSize

	for _, sec := range f.Sections {
		// Align section
		if sec.Addralign > 0 {
			if currentOffset%sec.Addralign != 0 {
				currentOffset += sec.Addralign - (currentOffset % sec.Addralign)
			}
		}

		sec.offset = currentOffset
		sec.size = uint64(len(sec.Content))
		currentOffset += sec.size
	}

	shdrOffset := currentOffset

	// 5. Write ELF header
	if err := f.writeElfHeader(w, shdrOffset); err != nil {
		return err
	}

	// 6. Write section contents
	written := headerSize
	for _, sec := range f.Sections {
		// Add padding if needed
		if sec.offset > written {
			padding := make([]byte, sec.offset-written)
			if _, err := w.Write(padding); err != nil {
				return err
			}
			written = sec.offset
		}

		if _, err := w.Write(sec.Content); err != nil {
			return err
		}
		written += sec.size
	}

	// 7. Write section headers
	for _, sec := range f.Sections {
		if err := f.writeSectionHeader(w, sec); err != nil {
			return err
		}
	}

	return nil
}

func (f *File) writeElfHeader(w io.Writer, shoff uint64) error {
	var hdr elfHeader

	// Magic number
	hdr.Ident[EI_MAG0] = ELFMAG0
	hdr.Ident[1] = ELFMAG1
	hdr.Ident[2] = ELFMAG2
	hdr.Ident[3] = ELFMAG3
	hdr.Ident[EI_CLASS] = ELFCLASS64
	hdr.Ident[EI_DATA] = ELFDATA2LSB
	hdr.Ident[EI_VERSION] = EV_CURRENT
	// Rest of e_ident is zero

	hdr.Type = ET_REL      // Relocatable object file
	hdr.Machine = f.Machine
	hdr.Version = EV_CURRENT
	hdr.Shoff = shoff
	hdr.Ehsize = 64                        // sizeof(Elf64_Ehdr)
	hdr.Shentsize = 64                     // sizeof(Elf64_Shdr)
	hdr.Shnum = uint16(len(f.Sections))
	hdr.Shstrndx = uint16(len(f.Sections) - 2) // .shstrtab is second-to-last

	return binary.Write(w, binary.LittleEndian, hdr)
}

func (f *File) writeSectionHeader(w io.Writer, sec *Section) error {
	var shdr elfSectionHeader

	shdr.Name = sec.nameIdx
	shdr.Type = sec.Type
	shdr.Flags = sec.Flags
	shdr.Addr = sec.Addr
	shdr.Offset = sec.offset
	shdr.Size = sec.size
	shdr.Link = sec.Link
	shdr.Info = sec.Info
	shdr.Addralign = sec.Addralign
	shdr.Entsize = sec.Entsize

	return binary.Write(w, binary.LittleEndian, shdr)
}

func (f *File) writeSymbol(w io.Writer, sym *Symbol) error {
	shndx := uint16(SHN_UNDEF)
	if sym.Section != nil {
		shndx = sym.Section.index
	}

	// Write in correct order for Elf64_Sym
	binary.Write(w, binary.LittleEndian, sym.nameIdx)  // st_name
	w.Write([]byte{sym.Info})                          // st_info
	w.Write([]byte{sym.Other})                         // st_other
	binary.Write(w, binary.LittleEndian, shndx)        // st_shndx
	binary.Write(w, binary.LittleEndian, sym.Value)    // st_value
	binary.Write(w, binary.LittleEndian, sym.Size)     // st_size

	return nil
}

// MakeSymbolInfo creates the info byte for a symbol
func MakeSymbolInfo(binding, typ byte) byte {
	return (binding << 4) | (typ & 0xf)
}

// ELF structures
type elfHeader struct {
	Ident     [EI_NIDENT]byte
	Type      uint16
	Machine   uint16
	Version   uint32
	Entry     uint64
	Phoff     uint64
	Shoff     uint64
	Flags     uint32
	Ehsize    uint16
	Phentsize uint16
	Phnum     uint16
	Shentsize uint16
	Shnum     uint16
	Shstrndx  uint16
}

type elfSectionHeader struct {
	Name      uint32
	Type      uint32
	Flags     uint64
	Addr      uint64
	Offset    uint64
	Size      uint64
	Link      uint32
	Info      uint32
	Addralign uint64
	Entsize   uint64
}