package storage

// Different variants of storage: SQLite, Redis, PostgreSQL
// One interface - different implementation

import "errors"

var (
	ErrURLNotFound = errors.New("url not found")
	ErrURLExists   = errors.New("url exists")
)
