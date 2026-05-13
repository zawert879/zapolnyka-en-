package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"zapolnyaka/internal/config"

	"gopkg.in/yaml.v3"
)

// RunCode appends a single code entry to the level's codes file.
func RunCode(gamePath, levelRelPath string, code config.Code) error {
	gameDir := filepath.Dir(gamePath)
	confPath := filepath.Join(gameDir, levelRelPath)
	level, err := config.LoadLevel(confPath)
	if err != nil {
		return fmt.Errorf("load level: %w", err)
	}
	if level.Codes == nil {
		return fmt.Errorf("у уровня нет поля codes в конфиге")
	}
	levelDir := filepath.Dir(confPath)
	codesPath := filepath.Join(levelDir, *level.Codes)

	ext := strings.ToLower(filepath.Ext(codesPath))
	if ext == ".yml" || ext == ".yaml" {
		if err := appendCodeYAML(codesPath, code); err != nil {
			return err
		}
	} else {
		if err := appendCodeJSON(codesPath, code); err != nil {
			return err
		}
	}
	fmt.Printf("Код добавлен в %s\n", codesPath)
	return nil
}

// appendCodeYAML appends the code to a YAML codes file preserving existing content.
func appendCodeYAML(path string, code config.Code) error {
	newEntry, err := yaml.Marshal([]config.Code{code})
	if err != nil {
		return err
	}
	existing, _ := os.ReadFile(path)
	content := string(existing)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content+string(newEntry)), 0o644)
}

// appendCodeJSON parses the existing JSON array, appends the code, and writes back.
func appendCodeJSON(path string, code config.Code) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var codes []config.Code
	if err := json.Unmarshal(existing, &codes); err != nil {
		codes = nil
	}
	codes = append(codes, code)
	data, err := json.MarshalIndent(codes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
