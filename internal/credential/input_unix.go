//go:build !windows

package credential

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// readPasswordSecure reads a password from stdin without echo on Unix systems.
// Uses stty to disable terminal echo, which is available on all Unix-like systems
// (Linux, macOS, BSD).
func readPasswordSecure() (string, error) {
	// Disable echo using stty -echo
	disable := exec.Command("stty", "-echo")
	disable.Stdin = os.Stdin
	disable.Stdout = os.Stderr
	disable.Stderr = os.Stderr
	if err := disable.Run(); err != nil {
		return "", fmt.Errorf("stty -echo failed: %w", err)
	}

	// Ensure echo is restored on exit
	defer func() {
		enable := exec.Command("stty", "echo")
		enable.Stdin = os.Stdin
		enable.Stdout = os.Stderr
		enable.Stderr = os.Stderr
		enable.Run()
		fmt.Fprintln(os.Stderr)
	}()

	// Read password
	reader := bufio.NewReader(os.Stdin)
	password, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}

	return strings.TrimSuffix(password, "\n"), nil
}