package markdown

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNextSymbolID_Sequential(t *testing.T) {
	cfg := &ConfigData{}
	id1 := NextSymbolID(cfg)
	id2 := NextSymbolID(cfg)
	id3 := NextSymbolID(cfg)

	if id1 != "SYM-001" {
		t.Errorf("id1: got %q, want SYM-001", id1)
	}
	if id2 != "SYM-002" {
		t.Errorf("id2: got %q, want SYM-002", id2)
	}
	if id3 != "SYM-003" {
		t.Errorf("id3: got %q, want SYM-003", id3)
	}
	if cfg.LastSymbolID != 3 {
		t.Errorf("LastSymbolID: got %d, want 3", cfg.LastSymbolID)
	}
}

func TestNextFeatureID_Format(t *testing.T) {
	cfg := &ConfigData{LastFeatureID: 9}
	id := NextFeatureID(cfg)
	if id != "FEAT-010" {
		t.Errorf("got %q, want FEAT-010", id)
	}
}

func TestNextTaskID_Format(t *testing.T) {
	cfg := &ConfigData{LastTaskID: 99}
	id := NextTaskID(cfg)
	if id != "TASK-100" {
		t.Errorf("got %q, want TASK-100", id)
	}
}

func TestNextIDs_LargeNumbers(t *testing.T) {
	cfg := &ConfigData{LastSymbolID: 999}
	id := NextSymbolID(cfg)
	if id != "SYM-1000" {
		t.Errorf("got %q, want SYM-1000", id)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LastSymbolID != 0 {
		t.Errorf("LastSymbolID: got %d, want 0", cfg.LastSymbolID)
	}
	if cfg.LastFeatureID != 0 {
		t.Errorf("LastFeatureID: got %d, want 0", cfg.LastFeatureID)
	}
	if cfg.LastTaskID != 0 {
		t.Errorf("LastTaskID: got %d, want 0", cfg.LastTaskID)
	}
}

func TestSaveAndLoadConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	original := &ConfigData{
		LastSymbolID:  42,
		LastFeatureID: 7,
		LastTaskID:    15,
		Projects:      []string{"alpha", "beta"},
	}

	if err := SaveConfig(dir, original); err != nil {
		t.Fatalf("SaveConfig error: %v", err)
	}

	// Verify file was created
	path := filepath.Join(dir, "_config.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("_config.md not created: %v", err)
	}

	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if loaded.LastSymbolID != 42 {
		t.Errorf("LastSymbolID: got %d, want 42", loaded.LastSymbolID)
	}
	if loaded.LastFeatureID != 7 {
		t.Errorf("LastFeatureID: got %d, want 7", loaded.LastFeatureID)
	}
	if loaded.LastTaskID != 15 {
		t.Errorf("LastTaskID: got %d, want 15", loaded.LastTaskID)
	}
	if len(loaded.Projects) != 2 {
		t.Errorf("Projects: got %v, want [alpha beta]", loaded.Projects)
	} else {
		if loaded.Projects[0] != "alpha" || loaded.Projects[1] != "beta" {
			t.Errorf("Projects: got %v", loaded.Projects)
		}
	}
}

func TestSaveConfig_CreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "subdir", "codesight")
	cfg := &ConfigData{LastSymbolID: 1}
	if err := SaveConfig(dir, cfg); err != nil {
		t.Fatalf("SaveConfig should create dirs: %v", err)
	}
	path := filepath.Join(dir, "_config.md")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("_config.md not found: %v", err)
	}
}

func TestNextID_StartingFromNonZero(t *testing.T) {
	cfg := &ConfigData{LastSymbolID: 5, LastFeatureID: 2, LastTaskID: 10}

	sym := NextSymbolID(cfg)
	feat := NextFeatureID(cfg)
	task := NextTaskID(cfg)

	if sym != "SYM-006" {
		t.Errorf("sym: got %q", sym)
	}
	if feat != "FEAT-003" {
		t.Errorf("feat: got %q", feat)
	}
	if task != "TASK-011" {
		t.Errorf("task: got %q", task)
	}
}
