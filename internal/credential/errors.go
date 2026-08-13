package credential

import "errors"

// ErrNotFound is returned when a credential key is not found.
var ErrNotFound = errors.New("credential: not found")