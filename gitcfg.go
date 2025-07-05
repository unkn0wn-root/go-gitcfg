package gitcfg

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

const (
	// System-wide Git configuration (/etc/gitconfig).
	SourceTypeSystem ConfigSourceType = iota
	// User-specific Git configuration (~/.gitconfig).
	SourceTypeGlobal
	// Repository-specific Git configuration (.git/config).
	SourceTypeLocal
	// Worktree-specific Git configuration (.git/config.worktree).
	SourceTypeWorktree
)

type Constraint interface {
	~string | ~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | ~bool
}

type User struct {
	Name  string
	Email string
}

type ConfigSource struct {
	Type ConfigSourceType
	Path string
}

type ConfigSourceType int

func (t ConfigSourceType) String() string {
	switch t {
	case SourceTypeSystem:
		return "system"
	case SourceTypeGlobal:
		return "global"
	case SourceTypeLocal:
		return "local"
	case SourceTypeWorktree:
		return "worktree"
	default:
		return "unknown"
	}
}

type Config struct {
	mu       sync.RWMutex
	sections map[string]map[string]string
	sources  []ConfigSource
}

func (c *Config) String() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var sb strings.Builder

	if len(c.sources) > 0 {
		sb.WriteString("# Configuration sources:\n")
		for _, source := range c.sources {
			sb.WriteString(fmt.Sprintf("# %s: %s\n", source.Type, source.Path))
		}
		sb.WriteString("\n")
	}

	for section, sectionMap := range c.sections {
		sb.WriteString(fmt.Sprintf("[%s]\n", section))
		for key, value := range sectionMap {
			// Quote values that contain spaces or special characters
			if strings.ContainsAny(value, " \t\n\r\"\\") {
				sb.WriteString(fmt.Sprintf("  %s = %q\n", key, value))
			} else {
				sb.WriteString(fmt.Sprintf("  %s = %s\n", key, value))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (c *Config) Has(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return false
	}

	section, subkey := parts[0], parts[1]

	sectionMap, exists := c.sections[section]
	if !exists {
		return false
	}

	_, exists = sectionMap[subkey]
	return exists
}

func (c *Config) GetSection(section string) map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sectionMap, exists := c.sections[section]
	if !exists {
		return make(map[string]string)
	}

	result := make(map[string]string, len(sectionMap))
	for k, v := range sectionMap {
		result[k] = v
	}
	return result
}

func (c *Config) GetSections() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sections := make([]string, 0, len(c.sections))
	for section := range c.sections {
		sections = append(sections, section)
	}
	return sections
}

func (c *Config) HasSection(section string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, exists := c.sections[section]
	return exists
}


func (c *Config) GetAll() map[string]map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]map[string]string, len(c.sections))
	for section, sectionMap := range c.sections {
		result[section] = make(map[string]string, len(sectionMap))
		for k, v := range sectionMap {
			result[section][k] = v
		}
	}
	return result
}

func (c *Config) GetSources() []ConfigSource {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sources := make([]ConfigSource, len(c.sources))
	copy(sources, c.sources)
	return sources
}

func (c *Config) Reload() error {
	return c.ReloadWithContext(context.Background())
}

func (c *Config) ReloadWithContext(ctx context.Context) error {
	c.mu.Lock()
	sources := make([]ConfigSource, len(c.sources))
	copy(sources, c.sources)
	c.mu.Unlock()

	if len(sources) == 0 {
		return nil
	}

	newConfig := &Config{
		sections: make(map[string]map[string]string),
		sources:  make([]ConfigSource, 0, len(sources)),
	}

	parser := newParser()
	for _, source := range sources {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := parser.parseConfigFile(source.Path, newConfig); err != nil {
			return fmt.Errorf("failed to reload from %s: %w", source.Path, err)
		}
		newConfig.sources = append(newConfig.sources, source)
	}

	c.mu.Lock()
	c.sections = newConfig.sections
	c.sources = newConfig.sources
	c.mu.Unlock()

	return nil
}

func (c *Config) Clone() *Config {
	c.mu.RLock()
	defer c.mu.RUnlock()

	clone := &Config{
		sections: make(map[string]map[string]string, len(c.sections)),
		sources:  make([]ConfigSource, len(c.sources)),
	}

	// deep copy
	for section, sectionMap := range c.sections {
		clone.sections[section] = make(map[string]string, len(sectionMap))
		for k, v := range sectionMap {
			clone.sections[section][k] = v
		}
	}

	copy(clone.sources, c.sources)

	return clone
}


func (c *Config) GetUser() (*User, error) {
	name, err := Get[string](c, "user.name")
	if err != nil {
		return nil, fmt.Errorf("failed to get user.name: %w", err)
	}

	email, err := Get[string](c, "user.email")
	if err != nil {
		return nil, fmt.Errorf("failed to get user.email: %w", err)
	}

	return &User{
		Name:  name,
		Email: email,
	}, nil
}


func (c *Config) GetRemoteURL(remote string) (string, error) {
	if remote == "" {
		remote = "origin"
	}
	return Get[string](c, fmt.Sprintf("remote.%s.url", remote))
}

func (c *Config) setRawValue(key, value string) error {
	if !isValidConfigKey(key) {
		return fmt.Errorf("%w: %s", ErrInvalidKeyFormat, key)
	}

	section, remaining, err := parseConfigKey(key)
	if err != nil {
		return fmt.Errorf("%w: %s", err, key)
	}

	if !isValidSectionName(section) && !isValidSubsectionName(section) {
		return fmt.Errorf("%w: invalid section name %s", ErrInvalidKeyFormat, section)
	}
	if !isValidKeyName(remaining) {
		return fmt.Errorf("%w: invalid key name %s", ErrInvalidKeyFormat, remaining)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sections[section] == nil {
		c.sections[section] = make(map[string]string)
	}

	c.sections[section][remaining] = value
	return nil
}

// Retrieve a configuration value with type conversion.
func Get[T Constraint](c *Config, key string) (T, error) {
	var zero T

	c.mu.RLock()
	defer c.mu.RUnlock()

	section, subkey, err := parseConfigKey(key)
	if err != nil {
		return zero, &ConfigError{
			Op:  "get",
			Key: key,
			Err: err,
		}
	}

	sectionMap, exists := c.sections[section]
	if !exists {
		return zero, &ConfigError{
			Op:      "get",
			Key:     subkey,
			Section: section,
			Err:     ErrSectionNotFound,
		}
	}

	value, exists := sectionMap[subkey]
	if !exists {
		return zero, &ConfigError{
			Op:      "get",
			Key:     subkey,
			Section: section,
			Err:     ErrKeyNotFound,
		}
	}

	converted, err := convertValue[T](value)
	if err != nil {
		return zero, &ConfigError{
			Op:      "get",
			Key:     subkey,
			Section: section,
			Err:     fmt.Errorf("type conversion failed: %w", err),
		}
	}

	return converted, nil
}

// Retrieve a configuration value with a default
func GetWithDefault[T Constraint](c *Config, key string, defaultValue T) T {
	value, err := Get[T](c, key)
	if err != nil {
		return defaultValue
	}
	return value
}
