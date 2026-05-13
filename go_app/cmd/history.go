package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// History stores the last used game/level paths and saved credentials.
type History struct {
	Login            string `json:"login"`
	Password         string `json:"password"`
	LastGame         string `json:"lastGame"`
	LastLevel        string `json:"lastLevel"`
	SelfTestDevLevel int    `json:"selfTestDevLevel,omitempty"`
}

const historyFile = ".zapolnyaka.json"

func LoadHistory() History {
	data, err := os.ReadFile(historyFile)
	if err != nil {
		return History{}
	}
	var h History
	_ = json.Unmarshal(data, &h)
	return h
}

// SaveHistory merges h into the existing history file so that fields not present
// in h are never silently erased. Credentials (Login/Password) are preserved even
// if the caller passes a partial struct without them.
func SaveHistory(h History) {
	existing := LoadHistory()
	if h.Login == "" {
		h.Login = existing.Login
	}
	if h.Password == "" {
		h.Password = existing.Password
	}
	if h.LastGame == "" {
		h.LastGame = existing.LastGame
	}
	if h.LastLevel == "" {
		h.LastLevel = existing.LastLevel
	}
	if h.SelfTestDevLevel == 0 {
		h.SelfTestDevLevel = existing.SelfTestDevLevel
	}
	data, _ := json.MarshalIndent(h, "", "  ")
	_ = os.WriteFile(historyFile, data, 0o644)
}

// ScanGames finds game.yml/yaml/json files inside each subdirectory of dataDir.
func ScanGames(dataDir string) []string {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return nil
	}
	var games []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		for _, name := range []string{"game.yml", "game.yaml", "game.json"} {
			p := filepath.Join(dataDir, e.Name(), name)
			if _, err := os.Stat(p); err == nil {
				games = append(games, p)
				break
			}
		}
	}
	return games
}

// ScanLevelFolders returns names of subdirectories inside gameDir.
func ScanLevelFolders(gameDir string) []string {
	entries, err := os.ReadDir(gameDir)
	if err != nil {
		return nil
	}
	var folders []string
	for _, e := range entries {
		if e.IsDir() {
			folders = append(folders, e.Name())
		}
	}
	return folders
}
