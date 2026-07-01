package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zapolnyaka/internal/config"
	"zapolnyaka/internal/zapolnyaka"
	"zapolnyaka/pkg/logger"

	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
)

// maxAssetSize is the per-file limit enforced by en.cx FileUploader.aspx (48 MB).
const maxAssetSize = 48 * 1024 * 1024

var (
	assetOkStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	assetErrStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	assetInfoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
)

// RunAssets uploads the game's assets (css/js/images) from the assets folder to
// the en.cx "Файлы для игры" storage. The folder is "assets" next to the game
// file by default, overridable via the assetsDir field in the game config.
func RunAssets(gamePath string) error {
	login, password, err := resolveCredentials()
	if err != nil {
		return err
	}

	game, err := config.LoadGame(gamePath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	assetsDir := assetsDirFor(gamePath, game)
	if info, err := os.Stat(assetsDir); err != nil || !info.IsDir() {
		return fmt.Errorf("папка ассетов не найдена: %s", assetsDir)
	}

	files, err := collectAssetFiles(assetsDir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("в папке %s нет файлов для загрузки", assetsDir)
	}

	existing, err := loadManifest(manifestPath(assetsDir))
	if err != nil {
		return err
	}
	plan, err := planUploads(files, existing)
	if err != nil {
		return err
	}

	logger.Printf("📦 Ассеты: %s  файлов: %d  → игра %d (%s)\n", assetsDir, len(plan), game.GameID, game.Domain)
	fmt.Println(assetInfoStyle.Render(fmt.Sprintf("  📦 %s — файлов: %d → игра %d (%s)", assetsDir, len(plan), game.GameID, game.Domain)))

	z, err := zapolnyaka.New(login, password, game.Domain, game.GameID, game.Delays)
	if err != nil {
		return fmt.Errorf("init client: %w", err)
	}

	logger.Println("🔑 Авторизация...")
	if err := z.Auth(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	uploads := make([]zapolnyaka.AssetUpload, len(plan))
	for i, pa := range plan {
		uploads[i] = zapolnyaka.AssetUpload{Path: pa.Path, Name: pa.UploadName}
	}
	results := z.UploadAssets(uploads)

	// Record successful uploads in the manifest (stable name → uploadName map).
	manifest := map[string]string{}
	failed := 0
	for i, r := range results {
		if r.Err != nil {
			failed++
			fmt.Println(assetErrStyle.Render(fmt.Sprintf("  ❌ %s — %v", plan[i].Ref, r.Err)))
			continue
		}
		manifest[plan[i].Ref] = plan[i].UploadName
		tag := ""
		if plan[i].Ref == plan[i].UploadName {
			tag = "  (имя сохранено, без uuid)"
		}
		fmt.Println(assetOkStyle.Render(fmt.Sprintf("  ✔ {{%s}} → %s%s", plan[i].Ref, r.URL, tag)))
	}

	if err := saveManifest(manifestPath(assetsDir), manifest); err != nil {
		return fmt.Errorf("сохранение манифеста: %w", err)
	}

	if failed > 0 {
		return fmt.Errorf("не удалось загрузить %d из %d файлов", failed, len(results))
	}
	logger.Printf("🎉 Загружено %d файлов\n", len(results))
	fmt.Println(assetOkStyle.Render(fmt.Sprintf("  🎉 Загружено %d файлов · манифест: %s", len(results), manifestPath(assetsDir))))
	return nil
}

// collectAssetFiles returns absolute paths of regular, non-hidden files directly
// inside dir (non-recursive), validating each against the 48 MB limit.
func collectAssetFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read assets dir: %w", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", e.Name(), err)
		}
		if fi.Size() > maxAssetSize {
			return nil, fmt.Errorf("файл %s больше лимита 48 МБ (%d байт)", e.Name(), fi.Size())
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	return files, nil
}

// resolveCredentials returns the en.cx login/password from the history file,
// falling back to .env (EN_LOGIN / EN_PASSWORD).
func resolveCredentials() (string, string, error) {
	hist := LoadHistory()
	login, password := hist.Login, hist.Password
	if login == "" || password == "" {
		_ = godotenv.Load()
		login = os.Getenv("EN_LOGIN")
		password = os.Getenv("EN_PASSWORD")
	}
	if login == "" || password == "" {
		return "", "", fmt.Errorf("учётные данные не заданы — используйте пункт «Авторизоваться» или .env (EN_LOGIN / EN_PASSWORD)")
	}
	return login, password, nil
}
