package model

import (
	"errors"
	"fmt"
	"time"
)

// User represents an authenticated user of the flux control plane.
// PasswordHash is never serialized to JSON (json:"-").
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // never serialize
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// Validate checks that the user has all required fields populated.
// Returns all validation errors joined together.
func (u User) Validate() error {
	var errs []error
	if u.Email == "" {
		errs = append(errs, fmt.Errorf("user email is required"))
	}
	if u.PasswordHash == "" {
		errs = append(errs, fmt.Errorf("user password hash is required"))
	}
	if u.Role == "" {
		errs = append(errs, fmt.Errorf("user role is required"))
	}
	return errors.Join(errs...)
}
