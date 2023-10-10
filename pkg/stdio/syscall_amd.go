//go:build !arm && !arm64

package stdio

import "syscall"

func dup2(oldfd int, newfd int) error {
	return syscall.Dup2(oldfd, newfd)
}
