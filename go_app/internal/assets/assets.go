// Package assets — работа с ассетами игры (css/js/картинки): папка ассетов,
// манифест «имя файла → имя на en.cx», планирование загрузки под uuid-именами и
// подстановка плейсхолдеров {{имя}} в контент уровней.
//
// Пакет не зависит от cmd, чтобы его могли использовать и команды заливки, и эмулятор.
package assets

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

// ManifestName — манифест ассетов игры (исходное имя → имя на сервере).
// Лежит внутри папки ассетов скрытым файлом, поэтому скан загрузки его пропускает.
const ManifestName = ".manifest.json"

// MaxFileSize — лимит на файл, который держит FileUploader.aspx en.cx (48 МБ).
const MaxFileSize = 48 * 1024 * 1024

// PlaceholderRe находит ссылки {{ имя }} на ассеты в контенте уровня.
var PlaceholderRe = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

// DirFor возвращает папку ассетов игры (поле assetsDir, по умолчанию "assets")
// относительно папки game-файла.
func DirFor(gamePath string, game *config.Game) string {
	dir := game.AssetsDir
	if dir == "" {
		dir = "assets"
	}
	return filepath.Join(filepath.Dir(gamePath), dir)
}

// ManifestPath — путь к манифесту внутри папки ассетов.
func ManifestPath(assetsDir string) string {
	return filepath.Join(assetsDir, ManifestName)
}

// LoadManifest читает карту «имя → имя на сервере». Отсутствующий файл = пустая карта.
func LoadManifest(path string) (map[string]string, error) {
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

// SaveManifest записывает манифест в JSON.
func SaveManifest(path string, m map[string]string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// Reverse строит обратную карту «имя на сервере → исходное имя».
func Reverse(manifest map[string]string) map[string]string {
	r := make(map[string]string, len(manifest))
	for name, upload := range manifest {
		r[upload] = name
	}
	return r
}

// NewUUID возвращает случайный UUID v4 (RFC 4122).
func NewUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// CollectFiles возвращает абсолютные пути обычных нескрытых файлов прямо в dir
// (без рекурсии), проверяя каждый на лимит 48 МБ.
func CollectFiles(dir string) ([]string, error) {
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
		if fi.Size() > MaxFileSize {
			return nil, fmt.Errorf("файл %s больше лимита 48 МБ (%d байт)", e.Name(), fi.Size())
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	return files, nil
}

// PlannedAsset описывает, как файл на диске отображается в имя на en.cx.
type PlannedAsset struct {
	Path       string // абсолютный путь на диске
	Ref        string // имя в плейсхолдере {{...}} (имя на диске без ведущего ~)
	UploadName string // имя на en.cx (uuid.ext, либо Ref для файлов с ~)
}

// RefName возвращает имя-ссылку для файла на диске: ведущий "~" отбрасывается.
func RefName(diskName string) string {
	return strings.TrimPrefix(diskName, "~")
}

// PlanUploads выбирает имя загрузки для каждого файла, переиспользуя uuid из
// манифеста, чтобы ссылки оставались стабильными. Ведущий "~" в имени файла =
// «оставить читаемое имя» (без uuid).
func PlanUploads(files []string, existing map[string]string) ([]PlannedAsset, error) {
	plan := make([]PlannedAsset, 0, len(files))
	seen := map[string]string{} // ref → путь на диске, для поиска коллизий

	for _, path := range files {
		disk := filepath.Base(path)
		var ref, upload string
		if strings.HasPrefix(disk, "~") {
			ref = disk[len("~"):]
			upload = ref // читаемое имя, без uuid
		} else {
			ref = disk
			if prev, ok := existing[ref]; ok {
				upload = prev // стабильный uuid
			} else {
				u, err := NewUUID()
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
		plan = append(plan, PlannedAsset{Path: path, Ref: ref, UploadName: upload})
	}
	return plan, nil
}

// Resolver превращает имя ассета из плейсхолдера в URL. ok=false — ассет неизвестен.
type Resolver func(name string) (url string, ok bool)

// D1URL — адрес файла игры на d1.endata.cx (отдаётся мгновенно, без кэша).
func D1URL(gameID int, uploadName string) string {
	return fmt.Sprintf("https://d1.endata.cx/data/games/%d/%s", gameID, uploadName)
}

// D1Resolver — резолвер для заливки: имя → URL на d1 по манифесту.
func D1Resolver(manifest map[string]string, gameID int) Resolver {
	return func(name string) (string, bool) {
		upload, ok := manifest[name]
		if !ok {
			return "", false
		}
		return D1URL(gameID, upload), true
	}
}

// Expand заменяет {{name}} на URL из резолвера. Возвращает текст и список имён,
// которых резолвер не знает (в тексте они остаются как есть).
func Expand(text string, resolve Resolver) (string, []string) {
	if !strings.Contains(text, "{{") {
		return text, nil
	}
	var missing []string
	seen := map[string]bool{}
	out := PlaceholderRe.ReplaceAllStringFunc(text, func(match string) string {
		name := strings.TrimSpace(PlaceholderRe.FindStringSubmatch(match)[1])
		url, ok := resolve(name)
		if !ok {
			if !seen[name] {
				seen[name] = true
				missing = append(missing, name)
			}
			return match
		}
		return url
	})
	return out, missing
}

// Substitute раскрывает плейсхолдеры во всём контенте уровней (тело, task/help
// бонусов, тексты подсказок) на месте. Ошибка перечисляет имена без ассета.
func Substitute(prepared []config.PreparedLevel, resolve Resolver) error {
	missing := map[string]bool{}
	collect := func(names []string) {
		for _, n := range names {
			missing[n] = true
		}
	}

	for li := range prepared {
		p := &prepared[li]

		var m []string
		p.Body, m = Expand(p.Body, resolve)
		collect(m)

		for ci := range p.Codes {
			if p.Codes[ci].Task != nil {
				s, mm := Expand(*p.Codes[ci].Task, resolve)
				p.Codes[ci].Task = &s
				collect(mm)
			}
			if p.Codes[ci].Help != nil {
				s, mm := Expand(*p.Codes[ci].Help, resolve)
				p.Codes[ci].Help = &s
				collect(mm)
			}
		}

		if p.Conf != nil {
			for hi := range p.Conf.Hints {
				var mm []string
				p.Conf.Hints[hi].Text, mm = Expand(p.Conf.Hints[hi].Text, resolve)
				collect(mm)
			}
			for hi := range p.Conf.PenaltyHints {
				var mm []string
				p.Conf.PenaltyHints[hi].Text, mm = Expand(p.Conf.PenaltyHints[hi].Text, resolve)
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
