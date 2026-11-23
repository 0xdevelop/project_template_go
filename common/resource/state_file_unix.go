//go:build unix

package resource

import (
	"errors"
	"os"
	"syscall"
)

// Internal staging files are not user resource aliases. Never follow a link,
// truncate a shared inode, or block on a FIFO placed in a resumed staging dir.
func openStateFile(name string, flags int) (*os.File, error) {
	f, err := os.OpenFile(name, (flags&^os.O_TRUNC)|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err == nil {
		st, ok := info.Sys().(*syscall.Stat_t)
		if !info.Mode().IsRegular() || !ok || st.Nlink != 1 {
			err = errors.New("unsafe internal resource staging file")
		}
	}
	if err == nil && flags&os.O_TRUNC != 0 {
		err = f.Truncate(0)
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func openResourceFile(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
