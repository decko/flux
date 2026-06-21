// Package adapter defines common errors and types shared across
// adapter implementations (ticket sources, SCM systems, etc.).
package adapter

import "errors"

// ErrNotImplemented is a sentinel error returned by adapter methods
// whose implementation has not yet been provided.
var ErrNotImplemented = errors.New("not implemented")
