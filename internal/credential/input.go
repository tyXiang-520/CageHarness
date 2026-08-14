// Package credential provides secure credential management.
package credential

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ReadPassword reads a password from stdin with echo disabled.
// It prints the prompt to stderr, reads the password without echoing,
// and returns the trimmed input.
//
// On platforms where terminal echo control is not available, it falls
// back to reading with echo enabled and prints a warning to stderr.
//
// The returned password is NOT logged or persisted by this function.
// Callers are responsible for clearing the password from memory when done.
func ReadPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)

	password, err := readPasswordSecure()
	if err != nil {
		// Fallback: read with echo enabled
		fmt.Fprintln(os.Stderr, "(note: input will be visible on this terminal)")
		fmt.Fprint(os.Stderr, "> ")
		reader := bufio.NewReader(os.Stdin)
		password, err = reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
	}

	fmt.Fprintln(os.Stderr)
	return strings.TrimSpace(password), nil
}

// ConfirmPassword prompts the user to enter a password twice and checks they match.
func ConfirmPassword(prompt string) (string, error) {
	first, err := ReadPassword(prompt)
	if err != nil {
		return "", err
	}

	second, err := ReadPassword("Confirm: ")
	if err != nil {
		return "", err
	}

	if first != second {
		return "", fmt.Errorf("passwords do not match")
	}

	return first, nil
}

// MaskKey returns a masked version of a key suitable for display.
// For keys shorter than 12 characters, it returns all asterisks.
// For longer keys, it shows the first 4 and last 4 characters.
// If the key is the keychainGetMarker, it returns a descriptive message.
func MaskKey(key string) string {
	if key == "" {
		return "(not set)"
	}
	if key == keychainGetMarker {
		return "**** (stored in OS keychain)"
	}
	if len(key) <= 12 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}