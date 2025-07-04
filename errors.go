package gogitcfg

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrKeyNotFound      = errors.New("key not found")
	ErrSectionNotFound  = errors.New("section not found")
	ErrInvalidKeyFormat = errors.New("invalid key format")
	ErrInvalidValue     = errors.New("invalid value")
)

type ConfigError struct {
	Op      string
	Key     string
	Section string
	Source  string
	Err     error
}

func (e *ConfigError) Error() string {
	parts := []string{"gitconfig:", e.Op}

	if e.Section != "" && e.Key != "" {
		parts = append(parts, fmt.Sprintf("%s.%s", e.Section, e.Key))
	} else if e.Key != "" {
		parts = append(parts, e.Key)
	}

	if e.Source != "" {
		parts = append(parts, fmt.Sprintf("(source: %s)", e.Source))
	}

	parts = append(parts, e.Err.Error())
	return strings.Join(parts, " ")
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

func (e *ConfigError) Is(target error) bool {
	var targetErr *ConfigError
	if !errors.As(target, &targetErr) {
		return false
	}
	return e.Op == targetErr.Op && e.Key == targetErr.Key && e.Section == targetErr.Section
}

