//go:build darwin

package cmd

import (
	"fmt"
	"os"
	"os/exec"
)

// msgBoxScript — заголовок и текст приходят аргументами, а не вклеиваются в
// скрипт: кавычки и переводы строк в сообщении его не ломают.
const msgBoxScript = `on run argv
	activate
	display dialog (item 2 of argv) with title (item 1 of argv) buttons {"OK"} default button 1 with icon stop
end run`

// MessageBox показывает системный диалог (у .app-бандла нет терминала, stderr
// никто не увидит) и дублирует сообщение в stderr — для запуска из терминала.
func MessageBox(title, text string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", title, text)
	_ = exec.Command("osascript", "-e", msgBoxScript, title, text).Run()
}
