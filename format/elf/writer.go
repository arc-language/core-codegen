package elf

import (
	"bytes"
	"encoding/binary"
	"io"
)

// ELF64 Constants
const (
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
	ET_REL      = 1
	EM_X86_64   = 62

	// Section Types
	SHT_NULL     = 0
	SHT_PROGBITS = 1
	SHT_SYMTAB   = 2
	SHT_STRTAB   = 3
	SHT_RELA     = 4

	// Section Flags
	SHF_WRITE     = 0x1
	SHF_ALLOC     = 0x2
	SHF_EXECINSTR = 0x4

	// Symbol Info
	STT_NOTYPE = 0
	STT_OBJECT = 1
	STT_FUNC   = 2
	STT_FILE   = 4
)

// File represents an ELF object file state
type File struct {
	Sections []*Section
	Symbols  []*Symbol
	StrTab   *StringTable
	ShStrTab *StringTable
}

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
	
	// Internal linkage
	index     uint16
	nameIdx   uint32
	offset    uint64
	size      uint64
}

type Symbol struct {
	Name    string
	Info    byte // Binding << 4 | Type
	Other   byte
	Section *Section
	Value   uint64
	Size    uint64
	nameIdx uint32
}

type StringTable struct {
	Data []byte
}

func NewStringTable() *StringTable {
	return &StringTable{Data: []byte{0}}
}

func (st *StringTable) Add(s string) uint32 {
	if s == "" {
		return 0
	}
	idx := uint32(len(st.Data))
	st.Data = append(st.Data, []byte(s)...)
	st.Data = append(st.Data, 0)
	return idx
}

func NewFile() *File {
	f := &File{
		StrTab:   NewStringTable(),
		ShStrTab: NewStringTable(),
	}
	// Index 0: Null Section
	f.Sections = append(f.Sections, &Section{Type: SHT_NULL})
	return f
}

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

func (f *File) AddSymbol(name string, info byte, section *Section, value, size uint64) {
	f.Symbols = append(f.Symbols, &Symbol{
		Name:    name,
		Info:    info,
		Section: section,
		Value:   value,
		Size:    size,
	})
}

// WriteTo writes the ELF binary to the writer
func (f *File) WriteTo(w io.Writer) error {
	// 1. Prepare String Tables
	for _, s := range f.Sections {
		s.nameIdx = f.ShStrTab.Add(s.Name)
	}
	for _, sym := range f.Symbols {
		sym.nameIdx = f.StrTab.Add(sym.Name)
	}

	// 2. Add System Sections
	f.AddSection(".shstrtab", SHT_STRTAB, 0, f.ShStrTab.Data)
	strTabSec := f.AddSection(".strtab", SHT_STRTAB, 0, f.StrTab.Data)

	// Build Symbol Table Content
	symBuf := new(bytes.Buffer)
	f.writeSym(symBuf, &Symbol{}) // Null symbol
	for _, sym := range f.Symbols {
		f.writeSym(symBuf, sym)
	}

	symTabSec := f.AddSection(".symtab", SHT_SYMTAB, 0, symBuf.Bytes())
	symTabSec.Link = uint32(strTabSec.index)
	symTabSec.Entsize = 24
	symTabSec.Info = 1 // Index of first global symbol
	symTabSec.Addralign = 8

	// 3. Calculate Offsets
	headerSize := 64
	currentOffset := uint64(headerSize)

	for _, s := range f.Sections {
		if s.Addralign > 0 && currentOffset%s.Addralign != 0 {
			pad := s.Addralign - (currentOffset % s.Addralign)
			currentOffset += pad
		}
		s.offset = currentOffset
		s.size = uint64(len(s.Content))
		currentOffset += s.size
	}

	shOffset := currentOffset

	// 4. Write Header
	h := elfHeader{
		Type:      ET_REL,
		Machine:   EM_X86_64,
		Version:   EV_CURRENT,
		Shoff:     shOffset,
		Ehsize:    64,
		Shentsize: 64,
		Shnum:     uint16(len(f.Sections)),
		Shstrndx:  uint16(len(f.Sections) - 3), // .shstrtab location
	}
	h.Ident[EI_MAG0] = ELFMAG0
	h.Ident[1] = ELFMAG1
	h.Ident[2] = ELFMAG2
	h.Ident[3] = ELFMAG3
	h.Ident[EI_CLASS] = ELFCLASS64
	h.Ident[EI_DATA] = ELFDATA2LSB
	h.Ident[EI_VERSION] = EV_CURRENT

	if err := binary.Write(w, binary.LittleEndian, h); err != nil {
		return err
	}

	// 5. Write Sections
	written := uint64(headerSize)
	for _, s := range f.Sections {
		if s.offset > written {
			w.Write(make([]byte, s.offset-written)) // Padding
			written = s.offset
		}
		w.Write(s.Content)
		written += s.size
	}

	// 6. Write Section Headers
	for _, s := range f.Sections {
		sh := elfSectionHeader{
			Name:      s.nameIdx,
			Type:      s.Type,
			Flags:     s.Flags,
			Addr:      s.Addr,
			Offset:    s.offset,
			Size:      s.size,
			Link:      s.Link,
			Info:      s.Info,
			Addralign: s.Addralign,
			Entsize:   s.Entsize,
		}
		binary.Write(w, binary.LittleEndian, sh)
	}

	return nil
}

func (f *File) writeSym(w io.Writer, s *Symbol) {
	shndx := uint16(0)
	if s.Section != nil {
		shndx = s.Section.index
	}
	binary.Write(w, binary.LittleEndian, s.nameIdx)
	w.Write([]byte{s.Info})
	w.Write([]byte{s.Other})
	binary.Write(w, binary.LittleEndian, shndx)
	binary.Write(w, binary.LittleEndian, s.Value)
	binary.Write(w, binary.LittleEndian, s.Size)
}

// ELF Structures
type elfHeader struct {
	Ident     [16]byte
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