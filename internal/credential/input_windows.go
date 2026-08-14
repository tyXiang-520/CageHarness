//go:build windows

package credential

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	enableEchoInput      = 0x0004
	enableLineInput      = 0x0002
	enableProcessedInput = 0x0001
)

// readPasswordSecure reads a password from stdin without echo on Windows.
// Uses the Windows Console API via kernel32.dll to disable echo.
func readPasswordSecure() (string, error) {
	handle := syscall.Handle(os.Stdin.Fd())

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode := kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode := kernel32.NewProc("SetConsoleMode")

	// Get current console mode
	var oldMode uint32
	ret, _, err := procGetConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&oldMode)))
	if ret == 0 {
		return "", fmt.Errorf("GetConsoleMode failed: %v", err)
	}

	// Restore console mode on exit
	defer procSetConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&oldMode)))

	// Disable echo input, keep line input for backspace handling
	newMode := oldMode
	newMode &^= enableEchoInput

	ret, _, err = procSetConsoleMode.Call(uintptr(handle), uintptr(newMode))
	if ret == 0 {
		return "", fmt.Errorf("SetConsoleMode failed: %v", err)
	}

	// Read password byte by byte from stdin
	var password []byte
	buf := make([]byte, 1)
	for {
		n, rerr := os.Stdin.Read(buf)
		if rerr != nil {
			os.Stderr.Write([]byte{13, 10})
			return "", fmt.Errorf("read stdin: %w", rerr)
		}
		if n == 0 {
			continue
		}

		ch := buf[0]

		// Enter key (CR = 13)
		if ch == 13 {
			// Write CRLF to move to next line
			os.Stderr.Write([]byte{13, 10})
			break
		}

		// Backspace (8) or DEL (127) — remove last character
		if ch == 8 || ch == 127 {
			if len(password) > 0 {
				password = password[:len(password)-1]
			}
			continue
		}

		// Ctrl+C (3) — abort
		if ch == 3 {
			os.Stderr.Write([]byte{13, 10})
			return "", fmt.Errorf("input cancelled")
		}

		password = append(password, ch)
	}

	return string(password), nil
}