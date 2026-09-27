//go:build !windows && !darwin

package cmd

import (
	"fmt"
	"os"
)

// MessageBox на Linux и прочих пишет сообщение в stderr.
func MessageBox(title, text string) {
	fmt.Fprintf(os.Stderr, "%s: %s\n", title, text)
}
