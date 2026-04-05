package markdown

import (
	"fmt"
	"os"
	"path/filepath"
)

// NextSymbolID increments LastSymbolID and returns a formatted "SYM-NNN" string.
func NextSymbolID(cfg *ConfigData) string {
	cfg.LastSymbolID++
	return fmt.Sprintf("SYM-%03d", cfg.LastSymbolID)
}

// NextFeatureID increments LastFeatureID and returns a formatted "FEAT-NNN" string.
func NextFeatureID(cfg *ConfigData) string {
	cfg.LastFeatureID++
	return fmt.Sprintf("FEAT-%03d", cfg.LastFeatureID)
}

// NextTaskID increments LastTaskID and returns a formatted "TASK-NNN" string.
func NextTaskID(cfg *ConfigData) string {
	cfg.LastTaskID++
	return fmt.Sprintf("TASK-%03d", cfg.LastTaskID)
}

// LoadConfig reads _config.md from codesightDir and parses its frontmatter.
// Returns a zero-value ConfigData if the file does not exist.
func LoadConfig(codesightDir string) (*ConfigData, error) {
	path := filepath.Join(codesightDir, "_config.md")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ConfigData{}, nil
		}
		return nil, fmt.Errorf("load config: %w", err)
	}

	fm, _, err := ParseFrontmatter(data)
	if err != nil {
		return nil, fmt.Errorf("load config parse: %w", err)
	}

	cfg := &ConfigData{}

	if v, ok := fm["last_symbol_id"].(int); ok {
		cfg.LastSymbolID = v
	}
	if v, ok := fm["last_feature_id"].(int); ok {
		cfg.LastFeatureID = v
	}
	if v, ok := fm["last_task_id"].(int); ok {
		cfg.LastTaskID = v
	}
	if v, ok := fm["projects"].([]string); ok {
		cfg.Projects = v
	}
	if v, ok := fm["feature_patterns"].([]string); ok {
		cfg.FeaturePatterns = v
	}
	if v, ok := fm["ignore_patterns"].([]string); ok {
		cfg.IgnorePatterns = v
	}

	return cfg, nil
}

// SaveConfig writes cfg to _config.md in codesightDir.
func SaveConfig(codesightDir string, cfg *ConfigData) error {
	path := filepath.Join(codesightDir, "_config.md")

	content := WriteConfig(*cfg)

	if err := os.MkdirAll(codesightDir, 0o755); err != nil {
		return fmt.Errorf("save config mkdir: %w", err)
	}

	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("save config write: %w", err)
	}

	return nil
}
