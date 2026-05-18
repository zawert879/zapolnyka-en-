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
	for i, c := range codes {
		if len(c.Answers) == 0 {
			return nil, fmt.Errorf("codes[%d]: answers is required", i)
		}
		if c.Type != CodeTypeSector && c.Time == nil {
			return nil, fmt.Errorf("codes[%d] (type=%s): time is required", i, c.Type)
		}
	}
	return codes, nil
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
