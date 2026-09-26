//go:build windows

package cmd

import (
	"syscall"
	"unsafe"
)

// MessageBox показывает системное окно с сообщением (для GUI-сборки без консоли).
func MessageBox(title, text string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	proc := user32.NewProc("MessageBoxW")
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString(title)
	const mbIconError = 0x10
	_, _, _ = proc.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), mbIconError)
}
