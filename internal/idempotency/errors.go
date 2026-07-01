package idempotency

import "errors"

// ErrKeyInProgress is returned when a duplicate request arrives while the original is still executing.
var ErrKeyInProgress = errors.New("idempotency: key reserved but response not yet available")
