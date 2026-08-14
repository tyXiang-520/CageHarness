// Package credential provides secure credential management.
// Phase 1: interface + mock + env + redaction.
// Phase 2: OS Keychain integration (macOS/Windows/Linux).
package credential

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// ErrKeychainUnavailable is returned when the OS keychain is not available.
var ErrKeychainUnavailable = errors.New("keychain: OS keychain not available on this system")

// keychainGetMarker is returned by Get on platforms where the keychain
// is write-only (e.g., Windows Credential Manager via cmdkey).
// The caller should use EnvStore to retrieve the actual value at runtime.
const keychainGetMarker = "[keychain: cannot retrieve plaintext — use env var]"

// KeychainStore implements CredentialStore using the OS-native keychain.
//
// Platform backends:
//   - Windows: Credential Manager (cmdkey.exe)
//   - macOS:   Keychain (security CLI)
//   - Linux:   Secret Service (secret-tool, part of libsecret)
//
// Security note: On Windows, the OS credential manager is write-only via
// cmdkey — passwords cannot be retrieved in plaintext (by design).
// Use EnvStore for runtime credential retrieval; the keychain serves
// as the secure backup and status verification layer.
//
// Fallback: if the OS keychain tool is not found, KeychainStore reports
// ErrKeychainUnavailable. The caller should fall back to EnvStore.
type KeychainStore struct {
	service   string
	available bool
}

// NewKeychainStore creates a new KeychainStore.
// It checks whether the OS keychain tool is available on this system.
func NewKeychainStore() *KeychainStore {
	ks := &KeychainStore{
		service: "CageHarness",
	}
	ks.available = ks.checkAvailability()
	return ks
}

// IsAvailable returns true if the OS keychain is available on this system.
func (k *KeychainStore) IsAvailable() bool {
	return k.available
}

// checkAvailability tests whether the OS keychain CLI tool is present.
func (k *KeychainStore) checkAvailability() bool {
	switch runtime.GOOS {
	case "windows":
		_, err := exec.LookPath("cmdkey.exe")
		return err == nil
	case "darwin":
		_, err := exec.LookPath("security")
		return err == nil
	case "linux":
		_, err := exec.LookPath("secret-tool")
		return err == nil
	default:
		return false
	}
}

// Set stores a credential in the OS keychain.
func (k *KeychainStore) Set(name, secret string) error {
	if !k.available {
		return ErrKeychainUnavailable
	}

	switch runtime.GOOS {
	case "windows":
		return k.setWindows(name, secret)
	case "darwin":
		return k.setDarwin(name, secret)
	case "linux":
		return k.setLinux(name, secret)
	default:
		return fmt.Errorf("keychain: unsupported OS %s", runtime.GOOS)
	}
}

// Get retrieves a credential from the OS keychain.
// On Windows, this returns a marker value because the OS credential
// manager does not expose plaintext passwords via cmdkey.
// Use EnvStore for runtime credential retrieval.
func (k *KeychainStore) Get(name string) (string, error) {
	if !k.available {
		return "", ErrKeychainUnavailable
	}

	switch runtime.GOOS {
	case "windows":
		return k.getWindows(name)
	case "darwin":
		return k.getDarwin(name)
	case "linux":
		return k.getLinux(name)
	default:
		return "", fmt.Errorf("keychain: unsupported OS %s", runtime.GOOS)
	}
}

// Delete removes a credential from the OS keychain.
func (k *KeychainStore) Delete(name string) error {
	if !k.available {
		return ErrKeychainUnavailable
	}

	switch runtime.GOOS {
	case "windows":
		return k.deleteWindows(name)
	case "darwin":
		return k.deleteDarwin(name)
	case "linux":
		return k.deleteLinux(name)
	default:
		return fmt.Errorf("keychain: unsupported OS %s", runtime.GOOS)
	}
}

// Exists checks if a credential exists in the OS keychain.
func (k *KeychainStore) Exists(name string) bool {
	if !k.available {
		return false
	}

	switch runtime.GOOS {
	case "windows":
		return k.existsWindows(name)
	case "darwin":
		return k.existsDarwin(name)
	case "linux":
		return k.existsLinux(name)
	default:
		return false
	}
}

// =============================================================================
// Windows — Credential Manager via cmdkey.exe
// =============================================================================

// keychainTarget builds the cmdkey target name for a credential.
func (k *KeychainStore) keychainTarget(name string) string {
	return fmt.Sprintf("%s:%s", k.service, name)
}

func (k *KeychainStore) setWindows(name, secret string) error {
	target := k.keychainTarget(name)
	cmd := exec.Command("cmdkey.exe",
		"/generic:"+target,
		"/user:CageHarness",
		"/pass:"+secret,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keychain (windows): cmdkey set failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (k *KeychainStore) getWindows(name string) (string, error) {
	// Windows Credential Manager via cmdkey does not expose the password.
	// Verify the credential exists, then return a marker.
	if !k.existsWindows(name) {
		return "", fmt.Errorf("%w: credential '%s' not found in Windows Credential Manager", ErrNotFound, name)
	}
	return keychainGetMarker, nil
}

func (k *KeychainStore) existsWindows(name string) bool {
	target := k.keychainTarget(name)
	cmd := exec.Command("cmdkey.exe", "/list:"+target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	outStr := string(output)
	// cmdkey /list:target shows "User: CageHarness" if the credential exists
	return strings.Contains(outStr, "CageHarness") && strings.Contains(outStr, "Generic")
}

func (k *KeychainStore) deleteWindows(name string) error {
	if !k.existsWindows(name) {
		return fmt.Errorf("%w: credential '%s' not found in keychain", ErrNotFound, name)
	}
	target := k.keychainTarget(name)
	cmd := exec.Command("cmdkey.exe", "/delete:"+target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keychain (windows): cmdkey delete failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// =============================================================================
// macOS — Keychain via security CLI
// =============================================================================

func (k *KeychainStore) setDarwin(name, secret string) error {
	cmd := exec.Command("security",
		"add-generic-password",
		"-a", "CageHarness",
		"-s", k.service,
		"-l", name,
		"-w", secret,
		"-U", // update if exists
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keychain (darwin): security add failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (k *KeychainStore) getDarwin(name string) (string, error) {
	cmd := exec.Command("security",
		"find-generic-password",
		"-a", "CageHarness",
		"-s", k.service,
		"-l", name,
		"-w", // output only the password
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(output)
		if strings.Contains(outStr, "could not be found") {
			return "", fmt.Errorf("%w: credential '%s' not found in keychain", ErrNotFound, name)
		}
		return "", fmt.Errorf("keychain (darwin): security find failed: %w\nOutput: %s", err, outStr)
	}
	return strings.TrimSpace(string(output)), nil
}

func (k *KeychainStore) existsDarwin(name string) bool {
	_, err := k.getDarwin(name)
	return err == nil
}

func (k *KeychainStore) deleteDarwin(name string) error {
	cmd := exec.Command("security",
		"delete-generic-password",
		"-a", "CageHarness",
		"-s", k.service,
		"-l", name,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(output)
		if strings.Contains(outStr, "could not be found") {
			return fmt.Errorf("%w: credential '%s' not found in keychain", ErrNotFound, name)
		}
		return fmt.Errorf("keychain (darwin): security delete failed: %w\nOutput: %s", err, outStr)
	}
	return nil
}

// =============================================================================
// Linux — Secret Service via secret-tool (libsecret)
// =============================================================================

func (k *KeychainStore) setLinux(name, secret string) error {
	cmd := exec.Command("secret-tool",
		"store",
		"--label="+fmt.Sprintf("CageHarness %s", name),
		"service", k.service,
		"account", name,
	)
	cmd.Stdin = strings.NewReader(secret)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keychain (linux): secret-tool store failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

func (k *KeychainStore) getLinux(name string) (string, error) {
	cmd := exec.Command("secret-tool",
		"lookup",
		"service", k.service,
		"account", name,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: credential '%s' not found in secret service", ErrNotFound, name)
	}
	return strings.TrimSpace(string(output)), nil
}

func (k *KeychainStore) existsLinux(name string) bool {
	_, err := k.getLinux(name)
	return err == nil
}

func (k *KeychainStore) deleteLinux(name string) error {
	cmd := exec.Command("secret-tool",
		"clear",
		"service", k.service,
		"account", name,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keychain (linux): secret-tool clear failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// compile-time interface check
var _ CredentialStore = (*KeychainStore)(nil)