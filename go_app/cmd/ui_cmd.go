package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"zapolnyaka/internal/emu"
	"zapolnyaka/internal/ui"
	"zapolnyaka/pkg/logger"
)

// uiActions — команды приложения для веб-интерфейса (реализация ui.Actions).
// Лог команды дублируется в задание через logger.Tee, вывод validate/check — напрямую.
type uiActions struct{}

func (uiActions) ScanGames() []string { return ScanGames("data") }
func (uiActions) Login() string        { return LoadHistory().Login }

func (uiActions) Go(gamePath string, levels []int, log io.Writer) error {
	defer logger.Tee(log)()
	return RunGoLevels(gamePath, levels)
}

func (uiActions) Assets(gamePath string, log io.Writer) error {
	defer logger.Tee(log)()
	return RunAssets(gamePath)
}

func (uiActions) Validate(gamePath string, log io.Writer) error { return RunValidateTo(log, gamePath) }

func (uiActions) Check(gamePath string, log io.Writer) error {
	defer logger.Tee(log)()
	return RunCheckTo(log, gamePath)
}

func (uiActions) Snapshot(gamePath, name, send string, pid, pact int, log io.Writer) error {
	defer logger.Tee(log)()
	return RunSnapshot(gamePath, SnapshotOptions{Name: name, Send: send, PID: pid, Pact: pact})
}

func (uiActions) Auth(login, password string) error { return ActionAuth(login, password) }

func (uiActions) NewGame(name, domain string, gameID int) (string, error) {
	if err := RunGame(name, domain, gameID); err != nil {
		return "", err
	}
	return filepath.Join("data", name, "game.yml"), nil
}

func (uiActions) NewLevel(gamePath, dir string, num int) error { return RunLevel(gamePath, dir, num, "") }

func (uiActions) SaveLastGame(gamePath string) {
	hist := LoadHistory()
	hist.LastGame = gamePath
	SaveHistory(hist)
}

// StartUI запускает эмулятор с веб-интерфейсом. Возвращает URL интерфейса,
// функцию остановки и канал ошибки сервера.
func StartUI(gamePath string, o EmuOptions, version string) (string, func(), <-chan error, error) {
	if o.Port == 0 {
		o.Port = 8090
	}
	srv, err := emu.New(emu.Options{
		GamePath: gamePath,
		Addr:     fmt.Sprintf("127.0.0.1:%d", o.Port),
		Dev:      o.Dev,
		Offline:  o.Offline,
		Login:    LoadHistory().Login,
		Logf:     logger.Printf,
	})
	if err != nil {
		return "", nil, nil, err
	}
	devDir := ""
	if o.Dev {
		if _, file, _, ok := runtime.Caller(0); ok {
			devDir = filepath.Join(filepath.Dir(file), "..", "internal", "ui")
		}
	}
	ui.Mount(srv.Mux(), ui.Deps{Emu: srv, Actions: uiActions{}, Version: version, Dev: o.Dev, DevDir: devDir})

	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe(ctx) }()
	url := "http://" + srv.Addr() + "/ui/"
	logger.Printf("🖥 Веб-интерфейс: %s  игра: %s\n", url, gamePath)
	fmt.Println(emuInfoStyle.Render(fmt.Sprintf("  🖥 Веб-интерфейс запущен: %s", url)))
	fmt.Println(emuInfoStyle.Render(fmt.Sprintf("     эмулятор: %s", srv.URL())))
	if o.OpenBrowser {
		OpenBrowser(url)
	}
	return url, cancel, errc, nil
}

// RunUI запускает веб-интерфейс и ждёт Ctrl+C.
func RunUI(gamePath string, o EmuOptions, version string) error {
	_, stop, errc, err := StartUI(gamePath, o, version)
	if err != nil {
		return err
	}
	defer stop()
	fmt.Println(emuInfoStyle.Render("     остановить: Ctrl+C"))
	sigCtx, sigStop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer sigStop()
	select {
	case <-sigCtx.Done():
		fmt.Println(emuInfoStyle.Render("  ⏹ Веб-интерфейс остановлен"))
		return nil
	case err := <-errc:
		return err
	}
}
