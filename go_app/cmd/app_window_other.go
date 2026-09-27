//go:build !darwin

package cmd

import "os/exec"

// quitWithLastWindow нужен только на macOS: на Windows и Linux Chromium сам
// завершается, когда закрыто последнее окно.
func quitWithLastWindow(c *exec.Cmd) (started func()) {
	return func() {}
}
