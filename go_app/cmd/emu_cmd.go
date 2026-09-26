package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"zapolnyaka/internal/emu"
	"zapolnyaka/pkg/logger"

	"github.com/charmbracelet/lipgloss"
)

var emuInfoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("81"))

// EmuOptions — параметры команды emu.
type EmuOptions struct {
	Port        int
	Dev         bool
	Offline     bool // CSS/JS движка из встроенной копии, а не с world.en.cx
	OpenBrowser bool
}

// StartEmu запускает сервер эмулятора в фоне. Возвращает сервер, функцию остановки
// и канал с ошибкой завершения.
func StartEmu(gamePath string, o EmuOptions) (*emu.Server, func(), <-chan error, error) {
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
		return nil, nil, nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe(ctx) }()

	logger.Printf("🧪 Эмулятор: %s  игра: %s\n", srv.URL(), gamePath)
	fmt.Println(emuInfoStyle.Render(fmt.Sprintf("  🧪 Эмулятор запущен: %s", srv.URL())))
	fmt.Println(emuInfoStyle.Render(fmt.Sprintf("     конфиг: %s", gamePath)))
	fmt.Println(emuInfoStyle.Render("     панель: справа (Ctrl+`), конфиги и ассеты перечитываются на каждый запрос"))
	if o.OpenBrowser {
		OpenBrowser(srv.URL())
	}
	return srv, cancel, errc, nil
}

// RunEmu запускает эмулятор и ждёт Ctrl+C.
func RunEmu(gamePath string, o EmuOptions) error {
	_, stop, errc, err := StartEmu(gamePath, o)
	if err != nil {
		return err
	}
	defer stop()
	fmt.Println(emuInfoStyle.Render("     остановить: Ctrl+C"))

	sigCtx, sigStop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer sigStop()
	select {
	case <-sigCtx.Done():
		fmt.Println(emuInfoStyle.Render("  ⏹ Эмулятор остановлен"))
		return nil
	case err := <-errc:
		return err
	}
}

// OpenBrowser открывает URL в браузере по умолчанию (ошибки игнорируются).
func OpenBrowser(url string) {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		c = exec.Command("open", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	_ = c.Start()
}
