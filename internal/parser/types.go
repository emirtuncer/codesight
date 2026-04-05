package parser

import (
	"crypto/sha256"
	"fmt"
)

// Symbol represents a parsed code symbol (function, class, type, etc.)
type Symbol struct {
	Name          string            `json:"name"`
	QualifiedName string            `json:"qualified_name"`
	Kind          string            `json:"kind"`
	LineStart     uint32            `json:"line_start"`
	LineEnd       uint32            `json:"line_end"`
	ColStart      uint32            `json:"col_start"`
	ColEnd        uint32            `json:"col_end"`
	ParentName    string            `json:"parent_name,omitempty"`
	Signature     string            `json:"signature,omitempty"`
	Visibility    string            `json:"visibility,omitempty"`
	IsExported    bool              `json:"is_exported"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// Dependency represents a parsed dependency (import, call, extends, etc.)
type Dependency struct {
	Kind         string `json:"kind"`
	TargetModule string `json:"target_module,omitempty"`
	TargetName   string `json:"target_name,omitempty"`
	Line         uint32 `json:"line"`
	Col          uint32 `json:"col"`
	SourceSymbol string `json:"source_symbol,omitempty"`
}

// ParseResult holds everything extracted from a single file.
type ParseResult struct {
	FilePath     string       `json:"file_path"`
	Language     string       `json:"language"`
	Symbols      []Symbol     `json:"symbols"`
	Dependencies []Dependency `json:"dependencies"`
}

func (s *Symbol) ComputeSignatureHash() string {
	data := fmt.Sprintf("%s:%s:%s", s.Name, s.Kind, s.Signature)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(data)))
}

func (s *Symbol) ComputeContentHash(content []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(content))
}
