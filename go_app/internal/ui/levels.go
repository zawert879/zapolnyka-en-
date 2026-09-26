package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"zapolnyaka/internal/assets"
	"zapolnyaka/internal/config"
)

// GameInfo — игра в списке выбора.
type GameInfo struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Domain  string `json:"domain"`
	GameID  int    `json:"gameId"`
	Levels  int    `json:"levels"`
	Current bool   `json:"current"`
}

// LevelInfo — уровень в боковом списке.
type LevelInfo struct {
	Number   int    `json:"number"`
	Name     string `json:"name"`
	Dir      string `json:"dir"`
	ConfRel  string `json:"conf"`
	Codes    int    `json:"codes"`
	Sectors  int    `json:"sectors"`
	Bonuses  int    `json:"bonuses"`
	HasBody  bool   `json:"hasBody"`
	Error    string `json:"error,omitempty"`
}

// AssetInfo — файл в папке ассетов.
type AssetInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Uploaded string `json:"uploaded,omitempty"` // имя на сервере по манифесту
}

// LevelFiles — пути файлов уровня.
type LevelFiles struct {
	Dir   string `json:"dir"`
	Conf  string `json:"conf"`
	Codes string `json:"codes,omitempty"`
	Body  string `json:"body,omitempty"`
}

// LevelData — всё для вкладок «Коды» и «Редактор».
type LevelData struct {
	Number int            `json:"number"`
	Files  LevelFiles     `json:"files"`
	Conf   *config.Level  `json:"conf"`
	Codes  []config.Code  `json:"codes"`
	Body   string         `json:"body"`
	Raw    map[string]string `json:"raw"` // сырые тексты: conf, codes, body
	Error  string         `json:"error,omitempty"`
}

// gameDir — папка игры.
func gameDir(gamePath string) string { return filepath.Dir(gamePath) }

// insideGame проверяет, что path лежит внутри папки игры (или её папки ассетов).
func insideGame(gamePath, p string) bool {
	dir, _ := filepath.Abs(gameDir(gamePath))
	abs, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(dir, abs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// listLevels читает game.yml и описывает уровни (ошибки конфигов — в поле Error).
func listLevels(gamePath string) (*config.Game, []LevelInfo, error) {
	game, err := config.LoadGame(gamePath)
	if err != nil {
		return nil, nil, err
	}
	var out []LevelInfo
	for _, rel := range game.Levels {
		confPath := filepath.Join(gameDir(gamePath), rel)
		info := LevelInfo{Dir: filepath.ToSlash(filepath.Dir(rel)), ConfRel: filepath.ToSlash(rel)}
		lvl, err := config.LoadLevel(confPath)
		if err != nil {
			info.Error = err.Error()
			out = append(out, info)
			continue
		}
		info.Number = lvl.Level
		if lvl.Name != nil {
			info.Name = *lvl.Name
		}
		if lvl.Codes != nil {
			codes, err := config.LoadCodes(filepath.Join(filepath.Dir(confPath), *lvl.Codes))
			if err != nil {
				info.Error = err.Error()
			}
			info.Codes = len(codes)
			for _, c := range codes {
				if c.Type.HasSector() {
					info.Sectors++
				}
				if c.Type.HasBonus() {
					info.Bonuses++
				}
			}
		}
		if lvl.Body != nil {
			if fi, err := os.Stat(filepath.Join(filepath.Dir(confPath), *lvl.Body)); err == nil && fi.Size() > 0 {
				info.HasBody = true
			}
		}
		out = append(out, info)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return game, out, nil
}

// listAssets — файлы папки ассетов с отметкой «залит» по манифесту.
func listAssets(gamePath string, game *config.Game) []AssetInfo {
	dir := assets.DirFor(gamePath, game)
	files, err := assets.CollectFiles(dir)
	if err != nil {
		return nil
	}
	manifest, _ := assets.LoadManifest(assets.ManifestPath(dir))
	var out []AssetInfo
	for _, f := range files {
		fi, err := os.Stat(f)
		if err != nil {
			continue
		}
		ref := assets.RefName(filepath.Base(f))
		out = append(out, AssetInfo{Name: ref, Size: fi.Size(), Uploaded: manifest[ref]})
	}
	return out
}

// loadLevel читает файлы уровня n.
func loadLevel(gamePath string, n int) (*LevelData, error) {
	game, err := config.LoadGame(gamePath)
	if err != nil {
		return nil, err
	}
	for _, rel := range game.Levels {
		confPath := filepath.Join(gameDir(gamePath), rel)
		lvl, err := config.LoadLevel(confPath)
		if err != nil || lvl.Level != n {
			continue
		}
		levelDir := filepath.Dir(confPath)
		d := &LevelData{Number: n, Conf: lvl, Raw: map[string]string{}, Codes: []config.Code{}}
		d.Files.Dir = filepath.ToSlash(levelDir)
		d.Files.Conf = filepath.ToSlash(confPath)
		if raw, err := os.ReadFile(confPath); err == nil {
			d.Raw["conf"] = string(raw)
		}
		if lvl.Codes != nil {
			cp := filepath.Join(levelDir, *lvl.Codes)
			d.Files.Codes = filepath.ToSlash(cp)
			if raw, err := os.ReadFile(cp); err == nil {
				d.Raw["codes"] = string(raw)
			}
			codes, err := config.LoadCodes(cp)
			if err != nil {
				d.Error = err.Error()
			}
			if codes != nil {
				d.Codes = codes
			}
		}
		if lvl.Body != nil {
			bp := filepath.Join(levelDir, *lvl.Body)
			d.Files.Body = filepath.ToSlash(bp)
			if raw, err := os.ReadFile(bp); err == nil {
				d.Body = string(raw)
				d.Raw["body"] = string(raw)
			}
		}
		return d, nil
	}
	return nil, fmt.Errorf("уровень %d не найден в %s", n, gamePath)
}

// writeFileAtomic пишет файл через временный рядом.
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// marshalByExt кодирует v в YAML или JSON по расширению файла.
func marshalByExt(path string, v any) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".json" {
		return json.MarshalIndent(v, "", "  ")
	}
	var b strings.Builder
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	_ = enc.Close()
	return []byte(b.String()), nil
}

// saveCodes проверяет и записывает codes-файл уровня.
func saveCodes(gamePath string, n int, codes []config.Code) error {
	d, err := loadLevel(gamePath, n)
	if err != nil {
		return err
	}
	if d.Files.Codes == "" {
		return fmt.Errorf("у уровня %d не задан файл codes", n)
	}
	for i := range codes {
		codes[i] = normalizeCode(codes[i])
	}
	data, err := marshalByExt(d.Files.Codes, codes)
	if err != nil {
		return err
	}
	// Проверка через штатный загрузчик: пишем во временный файл с тем же расширением.
	tmp := d.Files.Codes + ".check" + filepath.Ext(d.Files.Codes)
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	_, verr := config.LoadCodes(tmp)
	_ = os.Remove(tmp)
	if verr != nil {
		return fmt.Errorf("проверка: %s", strings.ReplaceAll(verr.Error(), tmp, filepath.Base(d.Files.Codes)))
	}
	header := "# Коды уровня — сохранено веб-интерфейсом zapolnyaka; комментарии при сохранении не сохраняются.\n"
	if strings.ToLower(filepath.Ext(d.Files.Codes)) == ".json" {
		header = ""
	}
	return writeFileAtomic(d.Files.Codes, append([]byte(header), data...))
}

// normalizeCode чистит пустые необязательные поля, чтобы YAML не засорялся.
func normalizeCode(c config.Code) config.Code {
	empty := func(p *string) *string {
		if p == nil || strings.TrimSpace(*p) == "" {
			return nil
		}
		return p
	}
	c.SectorName, c.BonusName, c.Task, c.Help, c.Levels = empty(c.SectorName), empty(c.BonusName), empty(c.Task), empty(c.Help), empty(c.Levels)
	var answers []string
	for _, a := range c.Answers {
		if strings.TrimSpace(a) != "" {
			answers = append(answers, strings.TrimSpace(a))
		}
	}
	c.Answers = answers
	if c.Type == config.CodeTypeSector {
		c.Time = nil
	}
	return c
}

// saveConf записывает conf-файл уровня (номер уровня и ссылки на файлы сохраняются).
func saveConf(gamePath string, n int, conf *config.Level) error {
	d, err := loadLevel(gamePath, n)
	if err != nil {
		return err
	}
	conf.Level = n
	conf.Codes, conf.Body = d.Conf.Codes, d.Conf.Body
	if conf.Name != nil && strings.TrimSpace(*conf.Name) == "" {
		conf.Name = nil
	}
	if conf.Comment != nil && strings.TrimSpace(*conf.Comment) == "" {
		conf.Comment = nil
	}
	if conf.SectorsToClose != nil && *conf.SectorsToClose <= 0 {
		conf.SectorsToClose = nil
	}
	if conf.AutopassPenalty != nil && *conf.AutopassPenalty <= 0 {
		conf.AutopassPenalty = nil
	}
	data, err := marshalByExt(d.Files.Conf, conf)
	if err != nil {
		return err
	}
	header := "# Настройки уровня — сохранено веб-интерфейсом zapolnyaka.\n"
	if strings.ToLower(filepath.Ext(d.Files.Conf)) == ".json" {
		header = ""
	}
	return writeFileAtomic(d.Files.Conf, append([]byte(header), data...))
}

// saveRaw записывает сырой текст одного из файлов уровня ("conf" | "codes" | "body")
// и проверяет результат штатным загрузчиком.
func saveRaw(gamePath string, n int, which, text string) error {
	d, err := loadLevel(gamePath, n)
	if err != nil {
		return err
	}
	var path string
	switch which {
	case "conf":
		path = d.Files.Conf
	case "codes":
		path = d.Files.Codes
	case "body":
		path = d.Files.Body
		if path == "" {
			path = filepath.Join(d.Files.Dir, "task.html")
		}
	default:
		return fmt.Errorf("неизвестный файл %q", which)
	}
	if path == "" {
		return fmt.Errorf("у уровня %d нет файла %s", n, which)
	}
	if !insideGame(gamePath, path) {
		return fmt.Errorf("файл вне папки игры")
	}
	if which != "body" {
		tmp := path + ".check" + filepath.Ext(path)
		if err := os.WriteFile(tmp, []byte(text), 0o644); err != nil {
			return err
		}
		var verr error
		if which == "conf" {
			_, verr = config.LoadLevel(tmp)
		} else {
			_, verr = config.LoadCodes(tmp)
		}
		_ = os.Remove(tmp)
		if verr != nil {
			return fmt.Errorf("проверка: %s", strings.ReplaceAll(verr.Error(), tmp, filepath.Base(path)))
		}
	}
	return writeFileAtomic(path, []byte(text))
}
