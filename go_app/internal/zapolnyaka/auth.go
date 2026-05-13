package zapolnyaka

import (
	"fmt"
	"strings"
	"time"
	"zapolnyaka/pkg/logger"

	"github.com/go-rod/rod/lib/proto"
)

// isConnectError returns true for low-level CDP/WebSocket connectivity errors.
func isConnectError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "use of closed network connection") ||
		strings.Contains(s, "connection reset by peer") ||
		strings.Contains(s, "EOF") && strings.Contains(s, "websocket") ||
		strings.Contains(s, "broken pipe")
}

// Auth navigates to the en.cx login page and signs in.
func (z *Zapolnyaka) Auth() error {
	loginURL := z.url("Login.aspx?return=%2fDefault.aspx&lang=ru")
	logger.Printf("  auth: открываем %s\n", loginURL)
	if err := z.page.Navigate(loginURL); err != nil {
		return fmt.Errorf("navigate to login: %w", err)
	}
	if err := z.page.Timeout(15 * time.Second).WaitLoad(); err != nil {
		return fmt.Errorf("waitLoad login page: %w", err)
	}

	loginEl, err := z.page.Timeout(10 * time.Second).Element(`[placeholder="логин или id"]`)
	if err != nil {
		return fmt.Errorf("find login field: %w", err)
	}
	if err := loginEl.Input(z.login); err != nil {
		return fmt.Errorf("fill login: %w", err)
	}

	pwEl, err := z.page.Timeout(5 * time.Second).Element(`[placeholder="пароль"]`)
	if err != nil {
		return fmt.Errorf("find password field: %w", err)
	}
	if err := pwEl.Input(z.password); err != nil {
		return fmt.Errorf("fill password: %w", err)
	}

	btnEl, err := z.page.Timeout(5 * time.Second).ElementR("button,input[type=submit]", "Вход")
	if err != nil {
		return fmt.Errorf("find login button: %w", err)
	}
	waitNav := z.page.Timeout(30 * time.Second).WaitNavigation(proto.PageLifecycleEventNameLoad)
	if err := btnEl.Click("left", 1); err != nil {
		return fmt.Errorf("click login button: %w", err)
	}
	waitNav()
	logger.Printf("  auth: авторизация успешна\n")
	return nil
}

// gotoSafe navigates to url, handling:
//   - CDP connection drop: relaunch browser + re-auth + retry
//   - NotHumanRequest redirect: re-auth + 30 min wait (up to 3 tries)
//   - Login.aspx redirect: re-auth + retry (session expired)
func (z *Zapolnyaka) gotoSafe(url string) error {
	logger.Printf("  GET %s\n", url)
	const maxAttempts = 3
	for i := 0; i < maxAttempts; i++ {
		if err := z.page.Navigate(url); err != nil {
			if isConnectError(err) {
				logger.Alert("  💔 соединение с браузером разорвано: %v\n", err)
				if rerr := z.reconnect(); rerr != nil {
					return fmt.Errorf("reconnect: %w", rerr)
				}
				continue
			}
			return fmt.Errorf("navigate %s: %w", url, err)
		}
		if err := z.page.Timeout(navTimeout).WaitLoad(); err != nil {
			if isConnectError(err) {
				logger.Alert("  💔 соединение с браузером разорвано при ожидании: %v\n", err)
				if rerr := z.reconnect(); rerr != nil {
					return fmt.Errorf("reconnect: %w", rerr)
				}
				continue
			}
			return fmt.Errorf("waitLoad %s: %w", url, err)
		}

		info, err := z.page.Info()
		if err != nil {
			return fmt.Errorf("get page info: %w", err)
		}
		current := info.URL

		if strings.Contains(current, "NotHumanRequest") {
			logger.Alert("  ⚠️  бот-детект, повторная авторизация через 30 мин...\n")
			if err := z.Auth(); err != nil {
				return fmt.Errorf("re-auth: %w", err)
			}
			if i < maxAttempts-1 {
				time.Sleep(30 * time.Minute)
			}
			continue
		}

		// Session expired — page redirected to Login.aspx
		if strings.Contains(current, "Login.aspx") {
			logger.Alert("  🔒 сессия устарела (Login.aspx) — переавторизация...\n")
			if err := z.Auth(); err != nil {
				return fmt.Errorf("re-auth after session expiry: %w", err)
			}
			continue
		}

		return nil
	}
	return fmt.Errorf("не удалось открыть %s после %d попыток", url, maxAttempts)
}
