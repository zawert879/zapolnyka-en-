package logger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"
)

// W is the active output writer. Before Init it discards output.
// After Init it writes only to the log file (not stdout).
var W io.Writer = io.Discard

// timestampWriter wraps a writer and prepends HH:MM:SS to every line.
type timestampWriter struct {
	out io.Writer
	buf []byte
}

func (w *timestampWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := w.buf[:idx+1]
		ts := time.Now().Format("15:04:05 ")
		if _, err := fmt.Fprintf(w.out, "%s%s", ts, line); err != nil {
			return 0, err
		}
		w.buf = w.buf[idx+1:]
	}
	return len(p), nil
}

// Init opens (or creates) logPath in append mode and sets W to write to both
// os.Stdout and the file. Returns a close function to flush and close the file.
func Init(logPath string) func() {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: не удалось открыть %s: %v\n", logPath, err)
		return func() {}
	}
	fmt.Fprintf(f, "\n=== %s ===\n", time.Now().Format("2006-01-02 15:04:05"))
	W = &timestampWriter{out: f}
	return func() { _ = f.Close() }
}

func Printf(format string, a ...any) { fmt.Fprintf(W, format, a...) }
func Println(a ...any)               { fmt.Fprintln(W, a...) }
func Print(a ...any)                 { fmt.Fprint(W, a...) }

// Alert writes to both the log file and os.Stdout so the message is always
// visible in the terminal even when stdout is not the log destination.
func Alert(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprint(W, msg)
	fmt.Fprint(os.Stdout, msg)
}
