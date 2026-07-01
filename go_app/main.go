package main

import (
	"zapolnyaka/pkg/logger"
	"zapolnyaka/tui"
)

// version is the build version, injected at release time via
// -ldflags="-X main.version=<tag>". Defaults to "dev" for local builds.
var version = "dev"

func main() {
	closeLog := logger.Init("zapolnyaka.log")
	defer closeLog()

	if isCLIMode() {
		runCLI()
	} else {
		tui.Run()
	}
}
