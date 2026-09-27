//go:build darwin

package cmd

import (
	"bufio"
	"io"
	"strings"
	"testing"
	"time"
)

// Браузер для watchWindows: события пишем в его «выход», команды читаем из «входа».
func TestWatchWindows(t *testing.T) {
	toBrowserR, toBrowserW := io.Pipe()
	fromBrowserR, fromBrowserW := io.Pipe()
	done := make(chan struct{})
	go func() {
		watchWindows(toBrowserW, fromBrowserR)
		close(done)
	}()

	cmds := make(chan string, 8)
	go func() {
		br := bufio.NewReader(toBrowserR)
		for {
			msg, err := br.ReadString(0)
			if err != nil {
				close(cmds)
				return
			}
			cmds <- strings.TrimSuffix(msg, "\x00")
		}
	}()
	next := func() string {
		t.Helper()
		select {
		case c, ok := <-cmds:
			if !ok {
				t.Fatal("пайп команд закрыт")
			}
			return c
		case <-time.After(2 * time.Second):
			t.Fatal("команда не пришла")
		}
		return ""
	}
	send := func(ev string) {
		t.Helper()
		if _, err := io.WriteString(fromBrowserW, ev+"\x00"); err != nil {
			t.Fatal(err)
		}
	}

	if c := next(); !strings.Contains(c, "Target.setDiscoverTargets") {
		t.Fatalf("первой должна быть подписка на цели, пришло %s", c)
	}

	send(`{"method":"Target.targetCreated","params":{"targetInfo":{"targetId":"ui","type":"page"}}}`)
	send(`{"method":"Target.targetCreated","params":{"targetInfo":{"targetId":"popup","type":"page"}}}`)
	send(`{"method":"Target.targetCreated","params":{"targetInfo":{"targetId":"omnibox","type":"browser_ui"}}}`)
	send(`{"id":1,"result":{}}`)
	send(`не json`)
	// Служебная цель и одна из двух страниц — браузер ещё нужен.
	send(`{"method":"Target.targetDestroyed","params":{"targetId":"omnibox"}}`)
	send(`{"method":"Target.targetDestroyed","params":{"targetId":"popup"}}`)
	send(`{"method":"Target.targetDestroyed","params":{"targetId":"popup"}}`)
	select {
	case c := <-cmds:
		t.Fatalf("окно ещё открыто, а пришла команда %s", c)
	case <-time.After(100 * time.Millisecond):
	}

	send(`{"method":"Target.targetDestroyed","params":{"targetId":"ui"}}`)
	if c := next(); !strings.Contains(c, "Browser.close") {
		t.Fatalf("после последнего окна ждём Browser.close, пришло %s", c)
	}

	// Браузер вышел — пайп закрыт, наблюдатель завершается.
	_ = fromBrowserW.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchWindows не завершился после закрытия пайпа")
	}
}
