package gitcfg

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrKeyNotFound means the requested key is not present.
	ErrKeyNotFound = errors.New("key not found")
	// ErrSectionNotFound means the requested section is not present.
	ErrSectionNotFound = errors.New("section not found")
	// ErrInvalidKeyFormat means a key cannot be parsed as Git config syntax.
	ErrInvalidKeyFormat = errors.New("invalid key format")
	// ErrInvalidValue means a value cannot be converted or parsed.
	ErrInvalidValue = errors.New("invalid value")
	// ErrIncludeCycle means config includes recursively reference each other.
	ErrIncludeCycle = errors.New("include cycle")
)

// ConfigError adds operation, key, section and source context to errors.
type ConfigError struct {
	Op      string
	Key     string
	Section string
	Source  string
	Err     error
}

func (e *ConfigError) Error() string {
	parts := []string{"gitcfg:"}
	if e.Op != "" {
		parts = append(parts, e.Op)
	}

	if e.Section != "" && e.Key != "" {
		parts = append(parts, fmt.Sprintf("%s.%s", e.Section, e.Key))
	} else if e.Key != "" {
		parts = append(parts, e.Key)
	}

	if e.Source != "" {
		parts = append(parts, fmt.Sprintf("(source: %s)", e.Source))
	}

	if e.Err != nil {
		parts = append(parts, e.Err.Error())
	}
	return strings.Join(parts, " ")
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}
