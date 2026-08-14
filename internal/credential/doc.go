// Package credential provides secure credential management.
//
// # Architecture
//
// Three implementations of CredentialStore:
//
//   - MockStore: in-memory store for testing (never persists)
//   - EnvStore: environment variables with .env file support
//   - KeychainStore: OS-native keychain via CLI tools
//     (Windows: cmdkey, macOS: security, Linux: secret-tool)
//
// # Security Model
//
// Credentials are NEVER:
//   - Hardcoded in source code
//   - Committed to Git (including history)
//   - Written to logs or terminal history
//   - Stored in plaintext configuration files
//
// The KeychainStore provides OS-level secure storage:
//   - Windows: Credential Manager (encrypted at rest via DPAPI)
//   - macOS: Keychain (encrypted at rest via the user's login keychain)
//   - Linux: Secret Service (via libsecret / gnome-keyring)
//
// On Windows, the OS credential manager is write-only via cmdkey —
// passwords cannot be retrieved in plaintext by design. The application
// reads credentials from environment variables at runtime; the keychain
// serves as the secure backup and status verification layer.
//
// EnvStore is a compatibility source. Its plaintext risk is documented:
// .env files are plaintext, and process environments are visible to
// the process and its children. For production use, prefer KeychainStore.
//
// # Interactive Setup
//
// The ReadPassword function provides cross-platform hidden password input:
//   - Windows: Console API (kernel32.dll GetConsoleMode/SetConsoleMode)
//   - Unix: stty -echo (available on all Unix-like systems)
//   - Fallback: plain input with a warning
//
// The harness key setup/status/clear CLI commands provide guided
// credential management. Status never shows plaintext values.
//
// # Audit Trail Redaction
//
// RedactSensitiveFields recursively redacts sensitive field values
// (api_key, token, secret, password, credential, authorization, auth)
// from audit log entries before serialization. Matching is case-insensitive
// and based on substring containment.
package credential