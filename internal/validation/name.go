package validation

import (
	"errors"
	"strings"
)

// ValidateName validates profile name
func ValidateName(name string) error {
	trimmed := strings.TrimSpace(name)

	if trimmed == "" {
		return errors.New("name is required")
	}

	if len(trimmed) > 100 {
		return errors.New("name is too long (max 100 characters)")
	}

	return nil
}
