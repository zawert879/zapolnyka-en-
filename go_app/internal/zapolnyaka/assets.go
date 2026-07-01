package zapolnyaka

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"zapolnyaka/pkg/logger"
)

// AssetResult is the outcome of uploading a single asset file.
type AssetResult struct {
	Name string // base file name as stored on en.cx
	URL  string // public d1 URL the file is served from
	Err  error  // non-nil if the upload failed
}

// UploadAssets uploads each file (given by absolute path) to the game's
// "Файлы для игры" storage and returns a result per file. It never aborts the
// whole batch on a single failure — inspect the per-file Err instead.
//
// Files are served from https://d1.endata.cx/data/games/<gameId>/<name>.
func (z *Zapolnyaka) UploadAssets(files []string) []AssetResult {
	ctx := context.Background()
	results := make([]AssetResult, 0, len(files))

	for i, path := range files {
		name := filepath.Base(path)
		res := AssetResult{
			Name: name,
			URL:  fmt.Sprintf("https://d1.endata.cx/data/games/%d/%s", z.gameID, name),
		}

		logger.Printf("  upload %d/%d %s\n", i+1, len(files), name)
		res.Err = z.withSessionRetry(func() error {
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("open %s: %w", path, err)
			}
			defer f.Close()
			return z.client.AdminUploadGameFile(ctx, z.gameID, name, f)
		})
		if res.Err != nil {
			logger.Printf("    ✗ %s — %v\n", name, res.Err)
		} else {
			logger.Printf("    ✔ %s → %s\n", name, res.URL)
		}

		results = append(results, res)
		if i < len(files)-1 {
			sleep(z.delays.BetweenHints)
		}
	}

	return results
}
