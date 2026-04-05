package parser

import (
	"testing"
)

func TestSymbolSignatureHash(t *testing.T) {
	s := Symbol{
		Name:      "Add",
		Kind:      "function",
		Signature: "func Add(a int, b int) int",
	}
	h1 := s.ComputeSignatureHash()
	if h1 == "" {
		t.Fatal("signature hash should not be empty")
	}

	s2 := Symbol{
		Name:      "Add",
		Kind:      "function",
		Signature: "func Add(a int, b int) int",
	}
	if s2.ComputeSignatureHash() != h1 {
		t.Error("same signature should produce same hash")
	}

	s3 := Symbol{
		Name:      "Add",
		Kind:      "function",
		Signature: "func Add(a int, b int) string",
	}
	if s3.ComputeSignatureHash() == h1 {
		t.Error("different signature should produce different hash")
	}
}

func TestSymbolContentHash(t *testing.T) {
	s := Symbol{
		Name:      "Add",
		Kind:      "function",
		Signature: "func Add(a int, b int) int",
	}
	h1 := s.ComputeContentHash([]byte("func Add(a, b int) int { return a + b }"))
	if h1 == "" {
		t.Fatal("content hash should not be empty")
	}

	h2 := s.ComputeContentHash([]byte("func Add(a, b int) int { return a - b }"))
	if h1 == h2 {
		t.Error("different content should produce different hash")
	}
}
