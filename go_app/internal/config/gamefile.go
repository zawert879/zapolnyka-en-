package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Выключенные уровни в game.yml — закомментированные элементы списка levels:
//
//	levels:
//	  - 1/conf.yml
//	  # - 2/conf.yml      ← выключен: не заливается, но виден в интерфейсе и эмуляторе
//
// Комментарий должен стоять на своей строке внутри блока levels и выглядеть как
// элемент списка с путём к conf-файлу (.yml/.yaml/.json). Прочие комментарии не
// трогаются. Для JSON-конфигов игры выключенных уровней нет.

var (
	levelsKeyRe    = regexp.MustCompile(`^levels\s*:\s*(#.*)?$`)
	topKeyRe       = regexp.MustCompile(`^[^\s#\-]`)
	disabledItemRe = regexp.MustCompile(`^(\s*)#\s*-\s+(\S+)(\s.*)?$`)
	enabledItemRe  = regexp.MustCompile(`^(\s*)-\s+(\S+)(\s.*)?$`)
)

func isYAMLPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yml" || ext == ".yaml"
}

// levelsBlock возвращает диапазон строк [from, to) блока levels (после ключа и до
// следующего ключа верхнего уровня). from == -1, если блока нет или он в flow-стиле.
func levelsBlock(lines []string) (from, to int) {
	from = -1
	for i, ln := range lines {
		if from < 0 {
			if levelsKeyRe.MatchString(strings.TrimRight(ln, "\r")) {
				from = i + 1
			}
			continue
		}
		if topKeyRe.MatchString(ln) {
			return from, i
		}
	}
	if from < 0 {
		return -1, -1
	}
	return from, len(lines)
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}

func looksLikeConf(s string) bool {
	ext := strings.ToLower(filepath.Ext(s))
	return ext == ".yml" || ext == ".yaml" || ext == ".json"
}

// DisabledLevels возвращает пути закомментированных уровней из текста game.yml в
// порядке файла.
func DisabledLevels(data []byte) []string {
	lines := strings.Split(string(data), "\n")
	from, to := levelsBlock(lines)
	if from < 0 {
		return nil
	}
	var out []string
	for _, ln := range lines[from:to] {
		m := disabledItemRe.FindStringSubmatch(strings.TrimRight(ln, "\r"))
		if m == nil {
			continue
		}
		rel := unquote(m[2])
		if looksLikeConf(rel) {
			out = append(out, rel)
		}
	}
	return out
}

// ToggleLevel включает (раскомментирует) или выключает (комментирует) элемент rel
// в блоке levels, не трогая остальной текст. Возвращает новый текст и флаг, была ли
// найдена строка.
func ToggleLevel(data []byte, rel string, enabled bool) ([]byte, bool, error) {
	text := string(data)
	lines := strings.Split(text, "\n")
	from, to := levelsBlock(lines)
	if from < 0 {
		return nil, false, fmt.Errorf("в game.yml нет блока levels (или он записан в одну строку)")
	}
	// Отступ элементов списка берём с первого включённого элемента блока.
	indent := ""
	for _, ln := range lines[from:to] {
		if m := enabledItemRe.FindStringSubmatch(strings.TrimRight(ln, "\r")); m != nil {
			indent = m[1]
			break
		}
	}
	for i := from; i < to; i++ {
		raw := lines[i]
		ln := strings.TrimRight(raw, "\r")
		cr := raw[len(ln):]
		if enabled {
			m := disabledItemRe.FindStringSubmatch(ln)
			if m == nil || unquote(m[2]) != rel {
				continue
			}
			ind := indent
			if ind == "" {
				ind = m[1]
			}
			lines[i] = ind + "- " + m[2] + m[3] + cr
			return []byte(strings.Join(lines, "\n")), true, nil
		}
		m := enabledItemRe.FindStringSubmatch(ln)
		if m == nil || unquote(m[2]) != rel {
			continue
		}
		lines[i] = m[1] + "# - " + m[2] + m[3] + cr
		return []byte(strings.Join(lines, "\n")), true, nil
	}
	return nil, false, nil
}

// SetLevelEnabled переключает уровень rel в файле игры и записывает файл.
func SetLevelEnabled(gamePath, rel string, enabled bool) error {
	if !isYAMLPath(gamePath) {
		return fmt.Errorf("включение/выключение уровней поддерживается только для YAML-конфига игры")
	}
	data, err := os.ReadFile(gamePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", gamePath, err)
	}
	out, found, err := ToggleLevel(data, rel, enabled)
	if err != nil {
		return err
	}
	if !found {
		state := "выключенных"
		if !enabled {
			state = "включённых"
		}
		return fmt.Errorf("уровень %s не найден среди %s в %s", rel, state, filepath.Base(gamePath))
	}
	tmp := gamePath + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, gamePath)
}
