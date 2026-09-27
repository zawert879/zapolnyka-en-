package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"zapolnyaka/pkg/logger"
)

// AppOptions — параметры оконного режима.
type AppOptions struct {
	Port    int  // 0 — свободный порт
	Offline bool // CSS/JS движка из встроенной копии
	Width   int
	Height  int
}

// RunApp запускает веб-интерфейс и показывает его в отдельном окне Chromium
// (Edge или Chrome, режим --app: без адресной строки и вкладок, свой профиль).
// Возвращается, когда окно закрыто; сервер при этом останавливается.
// Если Chromium не найден — открывает системный браузер и живёт до Ctrl+C.
func RunApp(gamePath string, o AppOptions, version string) error {
	if o.Port == 0 {
		o.Port = freePort()
	}
	if o.Width == 0 || o.Height == 0 {
		o.Width, o.Height = 1440, 900
	}
	url, stop, errc, err := StartUI(gamePath, EmuOptions{Port: o.Port, Offline: o.Offline}, version)
	if err != nil {
		return err
	}
	defer stop()

	browser, ok := findChromium()
	if !ok {
		logger.Println("app: Chromium не найден, открываю системный браузер")
		fmt.Println(emuInfoStyle.Render("  Edge/Chrome не найден — открыт системный браузер; остановить: Ctrl+C"))
		OpenBrowser(url)
		sigCtx, sigStop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer sigStop()
		select {
		case <-sigCtx.Done():
			return nil
		case err := <-errc:
			return err
		}
	}

	profile := appProfileDir()
	_ = os.MkdirAll(profile, 0o755)
	args := []string{
		"--app=" + url,
		"--user-data-dir=" + profile,
		fmt.Sprintf("--window-size=%d,%d", o.Width, o.Height),
		"--no-first-run", "--no-default-browser-check", "--disable-sync",
		"--disable-extensions", "--disable-background-mode", "--disable-features=Translate,msEdgeStartupBoost",
	}
	c := exec.Command(browser, args...)
	started := quitWithLastWindow(c)
	if err := c.Start(); err != nil {
		return fmt.Errorf("запуск окна (%s): %w", browser, err)
	}
	started()
	logger.Printf("app: окно %s pid=%d, профиль %s\n", filepath.Base(browser), c.Process.Pid, profile)
	fmt.Println(emuInfoStyle.Render(fmt.Sprintf("  🪟 Окно: %s · закрытие окна завершает приложение", filepath.Base(browser))))

	sigCtx, sigStop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer sigStop()
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	select {
	case err := <-done:
		var exitErr *exec.ExitError
		if err != nil && !errors.As(err, &exitErr) {
			return err
		}
		return nil
	case <-sigCtx.Done():
		_ = c.Process.Kill()
		return nil
	case err := <-errc:
		_ = c.Process.Kill()
		return err
	}
}

// freePort просит у системы свободный TCP-порт на 127.0.0.1.
func freePort() int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 8090
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// appProfileDir — отдельный профиль браузера для окна приложения: так открывается
// новое окно с собственным процессом (и его закрытие видно приложению), а не
// вкладка в уже запущенном браузере пользователя.
func appProfileDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "zapolnyaka", "browser")
}

// findChromium ищет Edge или Chrome (в этом порядке: Edge есть на любом Windows).
func findChromium() (string, bool) {
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		for _, root := range []string{os.Getenv("ProgramFiles(x86)"), os.Getenv("ProgramFiles"), os.Getenv("LocalAppData")} {
			if root == "" {
				continue
			}
			candidates = append(candidates,
				filepath.Join(root, "Microsoft", "Edge", "Application", "msedge.exe"),
				filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(root, "Chromium", "Application", "chrome.exe"),
			)
		}
	case "darwin":
		candidates = []string{
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	default:
		for _, name := range []string{"microsoft-edge", "google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
			if p, err := exec.LookPath(name); err == nil {
				return p, true
			}
		}
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, true
		}
	}
	return "", false
}

// ActionApp — оконный режим с сохранением игры в истории.
func ActionApp(gamePath string, o AppOptions, version string) error {
	hist := LoadHistory()
	hist.LastGame = gamePath
	SaveHistory(hist)
	return RunApp(gamePath, o, version)
}

// DefaultGamePath — игра для запуска без аргументов: последняя из истории или
// первая найденная в data/.
func DefaultGamePath() string {
	if p := LoadHistory().LastGame; p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if games := ScanGames("data"); len(games) > 0 {
		return games[0]
	}
	return ""
}

// portString — для сообщений.
func portString(p int) string { return strconv.Itoa(p) }
