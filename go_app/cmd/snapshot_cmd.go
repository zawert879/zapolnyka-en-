package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"zapolnyaka/internal/config"
	"zapolnyaka/internal/zapolnyaka"
	"zapolnyaka/pkg/logger"
)

// SnapshotOptions — параметры команды snapshot.
type SnapshotOptions struct {
	Name string // имя снимка (файлы <name>.html / <name>.json)
	Send string // ввести этот код: снимается страница-ответ на POST формы
	PID  int    // взять штрафную подсказку с этим HelpId: снимается ответ на ?pid=&pact=
	Pact int    // pact для штрафной подсказки (по умолчанию 1)
}

var snapshotNameRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,80}$`)

// RunSnapshot логинится на en.cx и сохраняет play-страницу игры (HTML + ?json=1)
// в <папка game.yml>/snapshots/. С --send/--pid снимается именно страница-ответ
// на действие (в ней живут одноразовые маркеры вроде «неверный ответ»), затем
// отдельно — JSON-модель после действия.
func RunSnapshot(gamePath string, o SnapshotOptions) error {
	if o.Name == "" {
		o.Name = "snapshot"
	}
	if !snapshotNameRe.MatchString(o.Name) {
		return fmt.Errorf("плохое имя снимка %q (буквы, цифры, _ . -)", o.Name)
	}
	login, password, err := resolveCredentials()
	if err != nil {
		return err
	}
	game, err := config.LoadGame(gamePath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	z, err := zapolnyaka.New(login, password, game.Domain, game.GameID, config.Delays{})
	if err != nil {
		return fmt.Errorf("init client: %w", err)
	}
	logger.Println("🔑 Авторизация...")
	if err := z.Auth(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	var html string
	switch {
	case o.Send != "":
		fmt.Println(emuInfoStyle.Render(fmt.Sprintf("  ⌨ ввожу код %q", o.Send)))
		var status int
		html, status, err = z.PlaySendCode(o.Send)
		if err != nil {
			return err
		}
		fmt.Println(emuInfoStyle.Render(fmt.Sprintf("     POST → HTTP %d", status)))
	case o.PID > 0:
		fmt.Println(emuInfoStyle.Render(fmt.Sprintf("  ⚠ беру штрафную подсказку %d (pact=%d)", o.PID, max(o.Pact, 1))))
		html, err = z.PlayPenaltyHint(o.PID, o.Pact)
		if err != nil {
			return err
		}
	default:
		html, err = z.PlayPage()
		if err != nil {
			return err
		}
	}
	raw, model, err := z.PlayModelRaw()
	if err != nil {
		return err
	}

	dir := filepath.Join(filepath.Dir(gamePath), "snapshots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	htmlPath := filepath.Join(dir, o.Name+".html")
	if err := os.WriteFile(htmlPath, []byte(html), 0o644); err != nil {
		return err
	}
	// JSON сохраняем как отдал сервер (побайтово), только с отступами для чтения.
	jsonPath := filepath.Join(dir, o.Name+".json")
	var pretty bytes.Buffer
	if json.Indent(&pretty, raw, "", "  ") != nil {
		pretty.Reset()
		pretty.Write(raw)
	}
	if err := os.WriteFile(jsonPath, pretty.Bytes(), 0o644); err != nil {
		return err
	}
	levelInfo := "уровень не определён"
	if model != nil && model.Level != nil {
		l := model.Level
		levelInfo = fmt.Sprintf("уровень %d (%s), секторов %d/%d, бонусов %d, подсказок %d+%d, история %d",
			l.Number, l.Name, l.PassedSectorsCount, len(l.Sectors), len(l.Bonuses), len(l.Helps), len(l.PenaltyHelps), len(l.MixedActions))
	}
	logger.Printf("📸 Снимок %s: %s\n", o.Name, levelInfo)
	fmt.Println(emuInfoStyle.Render(fmt.Sprintf("  📸 %s — %s", o.Name, levelInfo)))
	fmt.Println(emuInfoStyle.Render(fmt.Sprintf("     %s (%d байт), %s", htmlPath, len(html), filepath.Base(jsonPath))))
	return nil
}
