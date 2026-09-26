//go:build !windows

package cmd

import (
	"fmt"
	"os"
)

// MessageBox на не-Windows пишет сообщение в stderr.
func MessageBox(title, text string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", title, text)
}
