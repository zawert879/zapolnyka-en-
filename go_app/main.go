package main

import (
	"zapolnyaka/pkg/logger"
	"zapolnyaka/tui"
)

func main() {
	closeLog := logger.Init("zapolnyaka.log")
	defer closeLog()

	if isCLIMode() {
		runCLI()
	} else {
		tui.Run()
	}
}
