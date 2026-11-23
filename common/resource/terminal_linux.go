//go:build linux

package resource

import (
	"os"
	"syscall"
	"unsafe"
)

func IsTerminal(file *os.File) bool {
	var term syscall.Termios
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), syscall.TCGETS, uintptr(unsafe.Pointer(&term)))
	return err == 0
}
