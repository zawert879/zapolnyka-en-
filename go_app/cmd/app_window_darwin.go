//go:build darwin

package cmd

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
)

// quitWithLastWindow заставляет Chromium завершиться вместе с последним окном.
// На macOS приложение без окон продолжает работать, и RunApp не узнал бы, что
// окно закрыли: сервер висел бы дальше, а повторный запуск .app ничего бы не
// открывал (система считает приложение запущенным). За окнами следим по
// DevTools-протоколу через пайп (--remote-debugging-pipe: fd 3 Chromium читает,
// в fd 4 пишет) — сетевой порт при этом не открывается.
// Вызывать до c.Start(), а возвращённую функцию — после.
func quitWithLastWindow(c *exec.Cmd) (started func()) {
	toBrowserR, toBrowserW, err := os.Pipe()
	if err != nil {
		return func() {}
	}
	fromBrowserR, fromBrowserW, err := os.Pipe()
	if err != nil {
		_ = toBrowserR.Close()
		_ = toBrowserW.Close()
		return func() {}
	}
	c.Args = append(c.Args, "--remote-debugging-pipe")
	c.ExtraFiles = []*os.File{toBrowserR, fromBrowserW}
	return func() {
		// Концы, отданные браузеру, в нашем процессе больше не нужны: иначе
		// после его выхода чтение не получит EOF.
		_ = toBrowserR.Close()
		_ = fromBrowserW.Close()
		go watchWindows(toBrowserW, fromBrowserR)
	}
}

// watchWindows читает события DevTools (JSON, разделитель — нулевой байт) и,
// когда закрыта последняя страница, просит браузер завершиться. Возвращается,
// когда браузер закрыл пайп.
func watchWindows(w io.WriteCloser, r io.ReadCloser) {
	defer w.Close()
	defer r.Close()
	if _, err := io.WriteString(w, `{"id":1,"method":"Target.setDiscoverTargets","params":{"discover":true}}`+"\x00"); err != nil {
		return
	}
	pages := map[string]bool{}
	br := bufio.NewReader(r)
	for {
		raw, err := br.ReadBytes(0)
		if err != nil {
			return
		}
		var ev struct {
			Method string `json:"method"`
			Params struct {
				TargetID   string `json:"targetId"`
				TargetInfo struct {
					TargetID string `json:"targetId"`
					Type     string `json:"type"`
				} `json:"targetInfo"`
			} `json:"params"`
		}
		if json.Unmarshal(raw[:len(raw)-1], &ev) != nil {
			continue
		}
		switch ev.Method {
		case "Target.targetCreated":
			if ev.Params.TargetInfo.Type == "page" {
				pages[ev.Params.TargetInfo.TargetID] = true
			}
		case "Target.targetDestroyed":
			if !pages[ev.Params.TargetID] {
				continue
			}
			delete(pages, ev.Params.TargetID)
			if len(pages) == 0 {
				_, _ = io.WriteString(w, `{"id":2,"method":"Browser.close"}`+"\x00")
			}
		}
	}
}
