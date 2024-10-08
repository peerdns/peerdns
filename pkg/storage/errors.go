package storage

import "errors"

// ErrKeyNotFound is returned when a key is not found in the store.
var ErrKeyNotFound = errors.New("key not found")
