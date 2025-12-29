// Package validator provides input validation for the application
package validator

import (
	"errors"
	"fmt"
	"unicode"
)

var (
	// ErrInvalidLetter is returned when the letter parameter is invalid
	ErrInvalidLetter = errors.New("invalid letter: must be one or more alphabetic characters")
	// ErrEmptyString is returned when a string parameter is empty
	ErrEmptyString = errors.New("string cannot be empty")
)

// ValidateLetter validates that a letter or string of letters contains only alphabetic characters
func ValidateLetter(letters string) error {
	if letters == "" {
		return ErrEmptyString
	}

	for _, r := range letters {
		if !unicode.IsLetter(r) {
			return fmt.Errorf("%w: got '%c'", ErrInvalidLetter, r)
		}
	}

	return nil
}

// ValidateNonEmpty validates that a string is not empty
func ValidateNonEmpty(s string) error {
	if s == "" {
		return ErrEmptyString
	}
	return nil
}

// ValidateID validates that an ID is positive
func ValidateID(id int64) error {
	if id <= 0 {
		return fmt.Errorf("invalid id: %d (must be positive)", id)
	}
	return nil
}
