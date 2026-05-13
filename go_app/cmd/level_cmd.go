package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"zapolnyaka/internal/config"
	"zapolnyaka/pkg/logger"

	"gopkg.in/yaml.v3"
)

// RunLevel creates a level template and registers it in the game config.
func RunLevel(gamePath, dir string, levelNum int, format string) error {
	game, err := config.LoadGame(gamePath)
	if err != nil {
		return fmt.Errorf("load game: %w", err)
	}
	if format == "" {
		format = game.DefaultFormatExt()
	}
	gameDir := filepath.Dir(gamePath)
	levelDir := filepath.Join(gameDir, dir)

	if _, err := os.Stat(levelDir); err == nil {
		return fmt.Errorf("папка уровня уже существует: %s", levelDir)
	}

	// If level number not provided, try parsing dir as int
	if levelNum == 0 {
		if n, err := strconv.Atoi(dir); err == nil {
			levelNum = n
		}
	}
	if levelNum == 0 {
		return fmt.Errorf("укажи номер уровня (папка %q не число)", dir)
	}

	if err := os.MkdirAll(levelDir, 0o755); err != nil {
		return err
	}

	confName := "conf." + format
	codesName := "codes." + format

	// Write conf file
	confContent := buildConfTemplate(levelNum, codesName, format)
	if err := os.WriteFile(filepath.Join(levelDir, confName), []byte(confContent), 0o644); err != nil {
		return err
	}

	// Write codes file
	codesContent := buildCodesTemplate(format)
	if err := os.WriteFile(filepath.Join(levelDir, codesName), []byte(codesContent), 0o644); err != nil {
		return err
	}

	// Write empty task.html
	if err := os.WriteFile(filepath.Join(levelDir, "task.html"), []byte(""), 0o644); err != nil {
		return err
	}

	// Add to game config (re-marshal, comments are lost — same as TS version)
	relConf := strings.ReplaceAll(filepath.Join(dir, confName), `\`, "/")
	alreadyIn := false
	for _, l := range game.Levels {
		if l == relConf {
			alreadyIn = true
			break
		}
	}
	if !alreadyIn {
		game.Levels = append(game.Levels, relConf)
	}

	gameExt := strings.ToLower(filepath.Ext(gamePath))
	var gameData []byte
	if gameExt == ".yml" || gameExt == ".yaml" {
		gameData, err = yaml.Marshal(game)
	} else {
		gameData, err = jsonMarshalIndent(game)
	}
	if err != nil {
		return err
	}
	if err := os.WriteFile(gamePath, gameData, 0o644); err != nil {
		return err
	}

	logger.Printf("Создан уровень %d в %s\n", levelNum, levelDir)
	logger.Printf("  conf:  %s\n", filepath.Join(levelDir, confName))
	logger.Printf("  codes: %s\n", filepath.Join(levelDir, codesName))
	logger.Printf("  task:  %s\n", filepath.Join(levelDir, "task.html"))
	logger.Printf("  game:  %s обновлён\n", gamePath)
	return nil
}

func buildConfTemplate(level int, codesName, format string) string {
	if format == "yml" {
		return strings.Join([]string{
			"# файлы",
			fmt.Sprintf("codes: %s", codesName),
			"body: task.html",
			"clean: true                  # очистить уровень перед заливкой",
			"",
			"# настройки уровня",
			fmt.Sprintf("level: %d", level),
			"autopass: 0                  # автопереход через N секунд (0 — выкл)",
			"#autopassPenalty: 0          # штраф при автопереходе, сек",
			"#name: \"Название уровня\"     # «Название уровня» в админке",
			"#comment: \"комментарий\"      # «Комментарий к уровню» в админке",
			"#sectorsToClose: 1           # «Условие прохождения»: сколько секторов закрыть (по умолчанию — все)",
			"#hints:                      # обычные подсказки",
			"#  - time: 600",
			"#    text: \"Подсказка 1\"",
			"#penaltyHints:               # штрафные подсказки",
			"#  - time: 0",
			"#    text: \"Спойлер\"",
			"#    penalty: 300",
			"#    comment: \"Откроется со штрафом 5 мин\"",
			"",
		}, "\n")
	}
	data, _ := jsonMarshalIndent(map[string]any{
		"codes":    codesName,
		"body":     "task.html",
		"clean":    true,
		"level":    level,
		"autopass": 0,
	})
	return string(data) + "\n"
}

func buildCodesTemplate(format string) string {
	if format == "yml" {
		return strings.Join([]string{
			"# записи кодов — порядок в массиве = порядок создания в админке",
			"# типы: сектор | бонус | штраф | секторбонус | секторштраф",
			"# поля:",
			"#   type        — см. выше",
			"#   sectorName  — имя сектора в админке (обязательно для типов с сектором)",
			"#   bonusName   — имя бонуса в админке (обязательно для типов с бонусом/штрафом)",
			"#   answers     — массив кодов (минимум 1)",
			"#   time        — секунды, обязательно для бонусных типов",
			"#   help        — подсказка/HTML/скрипт в txtHelp (для бонусных типов)",
			"# sectorName и bonusName видны игроку сразу (в списках секторов/бонусов уровня).",
			"# Не клади туда ответ. Только help показывается после ввода верного кода.",
			"",
			"# --- примеры ---",
			"",
			"# чистый сектор",
			"# - type: сектор",
			"#   sectorName: \"сектор 1\"",
			"#   answers:",
			"#     - ответ1",
			"#     - ответ2",
			"",
			"# бонус (минус время)",
			"# - type: бонус",
			"#   bonusName: \"бонус хлеб\"",
			"#   answers:",
			"#     - хлеб",
			"#   time: 300              # -5 минут",
			"#   help: \"молодец!\"       # показать игроку после ввода (HTML/<script> допустимы)",
			"",
			"# штраф (плюс время)",
			"# - type: штраф",
			"#   bonusName: \"ловушка кот\"",
			"#   answers:",
			"#     - кот",
			"#     - кошка",
			"#   time: 600              # +10 минут",
			"#   help: \"ловушка!\"",
			"",
			"# сектор+бонус: закрывает сектор и даёт бонус одним кодом",
			"# - type: секторбонус",
			"#   sectorName: \"сектор 3\"   # видно сразу — нейтральное имя",
			"#   bonusName: \"ингредиент\"  # видно сразу — не клади ответ",
			"#   answers:",
			"#     - хлеб",
			"#   time: 0",
			"#   help: \"молодец!\"         # показывается только после ввода верного кода",
			"",
			"# сектор+штраф: закрытие сектора штрафит по времени",
			"# - type: секторштраф",
			"#   sectorName: \"обманка\"",
			"#   bonusName: \"ловушка\"",
			"#   answers:",
			"#     - обманка",
			"#   time: 300",
			"#   help: \"это была ловушка\"",
			"",
		}, "\n")
	}
	data, _ := json.MarshalIndent([]map[string]any{
		{"type": "сектор", "sectorName": "сектор 1", "answers": []string{"ответ"}},
	}, "", "  ")
	return string(data) + "\n"
}

func jsonMarshalIndent(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
