package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func parseFile(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yml" || ext == ".yaml" {
		if err := yaml.Unmarshal(data, out); err != nil {
			return fmt.Errorf("parse yaml %s: %w", path, err)
		}
	} else {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("parse json %s: %w", path, err)
		}
	}
	return nil
}

func LoadGame(path string) (*Game, error) {
	var g Game
	if err := parseFile(path, &g); err != nil {
		return nil, err
	}
	if g.Domain == "" {
		return nil, fmt.Errorf("game %s: domain is required", path)
	}
	if g.GameID == 0 {
		return nil, fmt.Errorf("game %s: gameId is required", path)
	}
	return &g, nil
}

func LoadLevel(path string) (*Level, error) {
	var l Level
	if err := parseFile(path, &l); err != nil {
		return nil, err
	}
	if l.Level == 0 {
		return nil, fmt.Errorf("level %s: level number is required", path)
	}
	return &l, nil
}

func LoadCodes(path string) ([]Code, error) {
	var codes []Code
	if err := parseFile(path, &codes); err != nil {
		return nil, err
	}
	lines := codeLines(path)
	where := func(i int) string {
		if i < len(lines) && lines[i] > 0 {
			return fmt.Sprintf("%s:%d (запись %d)", path, lines[i], i+1)
		}
		return fmt.Sprintf("%s (запись %d)", path, i+1)
	}
	for i, c := range codes {
		switch c.Type {
		case CodeTypeSector, CodeTypeBonus, CodeTypePenalty, CodeTypeSectorBonus, CodeTypeSectorPenalty:
		default:
			return nil, fmt.Errorf("%s: неизвестный type %q (ожидается сектор | бонус | штраф | секторбонус | секторштраф)", where(i), string(c.Type))
		}
		if len(c.Answers) == 0 {
			return nil, fmt.Errorf("%s: нужно поле answers (минимум один ответ)", where(i))
		}
		if c.Type != CodeTypeSector && c.Time == nil {
			return nil, fmt.Errorf("%s, type=%s: нужно поле time (секунды)", where(i), c.Type)
		}
		if c.Levels != nil {
			if !c.Type.HasBonus() {
				return nil, fmt.Errorf("%s, type=%s: levels допустимо только для бонусных типов", where(i), c.Type)
			}
			if _, err := ParseLevelSpec(*c.Levels); err != nil {
				return nil, fmt.Errorf("%s: %w", where(i), err)
			}
		}
	}
	return codes, nil
}

// codeLines returns the 1-based source line of every top-level list item in a YAML
// codes file, so validation errors can point at the record. Returns nil for JSON
// files or when the file is not a plain sequence.
func codeLines(path string) []int {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".yml" && ext != ".yaml" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil || len(doc.Content) == 0 {
		return nil
	}
	seq := doc.Content[0]
	if seq.Kind != yaml.SequenceNode {
		return nil
	}
	lines := make([]int, len(seq.Content))
	for i, item := range seq.Content {
		lines[i] = item.Line
	}
	return lines
}

// LoadGame fully loads a game and all its levels into PreparedLevels.
// gameDir is the base directory (directory of the game file).
// If isDev, uses devLevel instead of level where available.
func LoadAll(gamePath string) (*Game, []PreparedLevel, error) {
	game, err := LoadGame(gamePath)
	if err != nil {
		return nil, nil, err
	}
	gameDir := filepath.Dir(gamePath)

	var prepared []PreparedLevel
	for _, relPath := range game.Levels {
		confPath := filepath.Join(gameDir, relPath)
		level, err := LoadLevel(confPath)
		if err != nil {
			return nil, nil, err
		}
		levelDir := filepath.Dir(confPath)
		p := PreparedLevel{Conf: level}

		// Load codes
		if level.Codes != nil {
			codesPath := filepath.Join(levelDir, *level.Codes)
			codes, err := LoadCodes(codesPath)
			if err != nil {
				return nil, nil, err
			}
			p.Codes = codes
		}

		// Load body
		if level.Body != nil {
			bodyPath := filepath.Join(levelDir, *level.Body)
			data, err := os.ReadFile(bodyPath)
			if err != nil {
				return nil, nil, fmt.Errorf("read body %s: %w", bodyPath, err)
			}
			p.Body = string(data)
		}

		prepared = append(prepared, p)
	}
	return game, prepared, nil
}

// DefaultFormat returns the file extension to use for new level files.
func (g *Game) DefaultFormatExt() string {
	if g.DefaultFormat == "json" {
		return "json"
	}
	return "yml"
}
