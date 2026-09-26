package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"zapolnyaka/internal/assets"
	"zapolnyaka/internal/config"
)

// ---------------------------------------------------------------- вкл/выкл уровня

// handleLevelEnabled комментирует/раскомментирует элемент levels в game.yml.
// Тело: {"conf": "4doezd/conf.yml", "enabled": true}.
func (u *UI) handleLevelEnabled(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Conf    string `json:"conf"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Conf) == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("нужны conf и enabled"))
		return
	}
	if err := config.SetLevelEnabled(u.deps.Emu.GamePath(), strings.TrimSpace(req.Conf), req.Enabled); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ---------------------------------------------------------------- ассеты

// assetName проверяет имя файла ассета: простое имя без путей и скрытых файлов.
func assetName(name string) (string, error) {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, `\`, "/")))
	if name == "" || name == "." || name == ".." || strings.HasPrefix(name, ".") || strings.ContainsAny(name, "/\\{}") {
		return "", fmt.Errorf("плохое имя файла %q", name)
	}
	return name, nil
}

func (u *UI) assetsDir() (string, error) {
	gamePath := u.deps.Emu.GamePath()
	game, err := config.LoadGame(gamePath)
	if err != nil {
		return "", err
	}
	return assets.DirFor(gamePath, game), nil
}

// handleAssetUpload принимает multipart-форму с полями files (несколько) и кладёт
// их в папку ассетов игры. Существующие файлы с тем же именем перезаписываются
// (uuid в манифесте остаётся, ссылки в игре не меняются — достаточно снова
// выполнить «Залить ассеты»).
func (u *UI) handleAssetUpload(w http.ResponseWriter, r *http.Request) {
	dir, err := u.assetsDir()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("multipart: %w", err))
		return
	}
	var saved []string
	for _, fh := range r.MultipartForm.File["files"] {
		name, err := assetName(fh.Filename)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if fh.Size > assets.MaxFileSize {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("файл %s больше лимита 48 МБ", name))
			return
		}
		src, err := fh.Open()
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		data, err := io.ReadAll(io.LimitReader(src, assets.MaxFileSize+1))
		_ = src.Close()
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if err := writeFileAtomic(filepath.Join(dir, name), data); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		saved = append(saved, assets.RefName(name))
	}
	if len(saved) == 0 {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("нет файлов (поле files)"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "saved": saved, "dir": filepath.ToSlash(dir)})
}

// handleAssetDelete удаляет файл из папки ассетов (по имени плейсхолдера или по
// имени на диске, с учётом префикса ~). Манифест не трогаем: залитый на en.cx файл
// остаётся, а при следующем «Залить ассеты» запись просто не используется.
func (u *UI) handleAssetDelete(w http.ResponseWriter, r *http.Request) {
	name, err := assetName(r.PathValue("name"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	dir, err := u.assetsDir()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	for _, candidate := range []string{name, "~" + name} {
		p := filepath.Join(dir, candidate)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			if err := os.Remove(p); err != nil {
				writeErr(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "removed": candidate})
			return
		}
	}
	writeErr(w, http.StatusNotFound, fmt.Errorf("ассет %s не найден", name))
}
