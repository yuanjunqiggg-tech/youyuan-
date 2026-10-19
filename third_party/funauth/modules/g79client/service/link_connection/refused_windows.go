package link_connection

import (
	"errors"
	"syscall"
)

func isConnectionRefused(err error) bool {
	// WinSock WSAECONNREFUSED is 10061; syscall.ECONNREFUSED is a separate
	// application error on Windows and does not match an actual connectex error.
	return errors.Is(err, syscall.Errno(10061)) || errors.Is(err, syscall.ECONNREFUSED)
}
