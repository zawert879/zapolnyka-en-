package cmd

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"zapolnyaka/internal/config"
)

// manifestName is the per-game asset manifest (original name → uploaded name).
// It lives inside the assets dir as a hidden file so the upload scan skips it.
const manifestName = ".manifest.json"

// placeholderRe matches {{ name }} asset references in level content.
var placeholderRe = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

// assetsDirFor resolves the assets folder for a game (assetsDir field, default
// "assets"), relative to the game file's directory.
func assetsDirFor(gamePath string, game *config.Game) string {
	dir := game.AssetsDir
	if dir == "" {
		dir = "assets"
	}
	return filepath.Join(filepath.Dir(gamePath), dir)
}

func manifestPath(assetsDir string) string {
	return filepath.Join(assetsDir, manifestName)
}

// loadManifest reads the name→uploadedName map. A missing file yields an empty map.
func loadManifest(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	m := map[string]string{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	return m, nil
}

func saveManifest(path string, m map[string]string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// newUUID returns a random RFC-4122 v4 UUID string.
func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// plannedAsset describes how one on-disk file maps to its name on en.cx.
type plannedAsset struct {
	Path       string // absolute path on disk
	Ref        string // name used in {{...}} placeholders (disk name without leading ~)
	UploadName string // name on en.cx (uuid.ext, or Ref for ~-prefixed files)
}

// planUploads decides the upload name for each file, reusing existing UUIDs from
// the manifest so links stay stable. A leading "~" on the disk name means
// "keep the readable name" (strip the ~, no uuid).
func planUploads(files []string, existing map[string]string) ([]plannedAsset, error) {
	plan := make([]plannedAsset, 0, len(files))
	seen := map[string]string{} // ref → disk path, for collision detection

	for _, path := range files {
		disk := filepath.Base(path)
		var ref, upload string
		if strings.HasPrefix(disk, "~") {
			ref = disk[len("~"):]
			upload = ref // keep readable name, no uuid
		} else {
			ref = disk
			if prev, ok := existing[ref]; ok {
				upload = prev // reuse stable uuid
			} else {
				u, err := newUUID()
				if err != nil {
					return nil, err
				}
				upload = u + strings.ToLower(filepath.Ext(ref))
			}
		}
		if other, dup := seen[ref]; dup {
			return nil, fmt.Errorf("конфликт имён ассетов: %q и %q дают одно имя %q", filepath.Base(other), disk, ref)
		}
		seen[ref] = path
		plan = append(plan, plannedAsset{Path: path, Ref: ref, UploadName: upload})
	}
	return plan, nil
}

// expandPlaceholders replaces {{name}} with the full d1 asset URL using the
// manifest. Returns the rewritten text and any placeholder names missing from
// the manifest (left untouched in the output).
func expandPlaceholders(text string, manifest map[string]string, gameID int) (string, []string) {
	if !strings.Contains(text, "{{") {
		return text, nil
	}
	var missing []string
	seen := map[string]bool{}
	out := placeholderRe.ReplaceAllStringFunc(text, func(match string) string {
		name := strings.TrimSpace(placeholderRe.FindStringSubmatch(match)[1])
		upload, ok := manifest[name]
		if !ok {
			if !seen[name] {
				seen[name] = true
				missing = append(missing, name)
			}
			return match
		}
		return fmt.Sprintf("https://d1.endata.cx/data/games/%d/%s", gameID, upload)
	})
	return out, missing
}

// substituteAssets expands {{name}} placeholders in every level's content (task
// body, bonus help, hint texts) in place. It returns an error listing any
// placeholders that have no manifest entry.
func substituteAssets(prepared []config.PreparedLevel, manifest map[string]string, gameID int) error {
	missing := map[string]bool{}
	collect := func(names []string) {
		for _, n := range names {
			missing[n] = true
		}
	}

	for li := range prepared {
		p := &prepared[li]

		var m []string
		p.Body, m = expandPlaceholders(p.Body, manifest, gameID)
		collect(m)

		for ci := range p.Codes {
			if p.Codes[ci].Help != nil {
				s, mm := expandPlaceholders(*p.Codes[ci].Help, manifest, gameID)
				p.Codes[ci].Help = &s
				collect(mm)
			}
		}

		if p.Conf != nil {
			for hi := range p.Conf.Hints {
				var mm []string
				p.Conf.Hints[hi].Text, mm = expandPlaceholders(p.Conf.Hints[hi].Text, manifest, gameID)
				collect(mm)
			}
			for hi := range p.Conf.PenaltyHints {
				var mm []string
				p.Conf.PenaltyHints[hi].Text, mm = expandPlaceholders(p.Conf.PenaltyHints[hi].Text, manifest, gameID)
				collect(mm)
			}
		}
	}

	if len(missing) > 0 {
		names := make([]string, 0, len(missing))
		for n := range missing {
			names = append(names, n)
		}
		sort.Strings(names)
		return fmt.Errorf("нет ассетов для плейсхолдеров: %s — сначала запусти `assets`", strings.Join(names, ", "))
	}
	return nil
}
