package gitcfg

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
)

// Scope identifies a Git configuration scope.
type Scope int

const (
	// ScopeSystem is the system-wide Git configuration.
	ScopeSystem Scope = iota
	// ScopeGlobal is the user-level Git configuration.
	ScopeGlobal
	// ScopeLocal is the repository-local Git configuration.
	ScopeLocal
	// ScopeWorktree is the worktree-specific Git configuration.
	ScopeWorktree
)

func (s Scope) String() string {
	switch s {
	case ScopeSystem:
		return "system"
	case ScopeGlobal:
		return "global"
	case ScopeLocal:
		return "local"
	case ScopeWorktree:
		return "worktree"
	default:
		return "unknown"
	}
}

// Source describes a configuration file that contributed entries.
type Source struct {
	Scope Scope
	Path  string
}

// Entry is one raw Git configuration key value entry.
type Entry struct {
	Key     string
	Section string
	Name    string
	Value   string
	Source  Source
	Line    int
}

// User is the structured user.name and user.email.
type User struct {
	Name  string
	Email string
}

type configState struct {
	entries  map[string][]Entry
	sources  []Source
	roots    []Source
	repoPath string
	includes bool
	useGit   bool
}

// Config stores parsed Git configuration entries.
// Repeated keys are preserved. Simple accessors return last value.
// Reload atomically replaces the underlying snapshot.
type Config struct {
	state atomic.Pointer[configState]
}

func newConfig() *Config {
	config := &Config{}
	config.state.Store(&configState{entries: make(map[string][]Entry)})
	return config
}

func (c *Config) snapshot() *configState {
	if c == nil {
		return &configState{}
	}
	state := c.state.Load()
	if state == nil {
		return &configState{}
	}
	return state
}

func (c *Config) mutableState() *configState {
	state := c.state.Load()
	if state == nil {
		state = &configState{entries: make(map[string][]Entry)}
		c.state.Store(state)
	}
	if state.entries == nil {
		state.entries = make(map[string][]Entry)
	}
	return state
}

func (c *Config) Has(key string) bool {
	_, ok := lastEntry(c.snapshot(), key)
	return ok
}

// Value returns the effective value for key.
// If the key appears more than once, Value returns the last value in Git
// precedence/order.
func (c *Config) Value(key string) (string, error) {
	state := c.snapshot()
	entry, ok := lastEntry(state, key)
	if !ok {
		section, name, _ := splitConfigKey(key)
		if section == "" {
			return "", &ConfigError{Op: "get", Key: key, Err: ErrInvalidKeyFormat}
		}
		if !hasSection(state, section) {
			return "", &ConfigError{Op: "get", Section: section, Key: name, Err: ErrSectionNotFound}
		}
		return "", &ConfigError{Op: "get", Section: section, Key: name, Err: ErrKeyNotFound}
	}
	return entry.Value, nil
}

// Values returns all values for key in parse order.
func (c *Config) Values(key string) []string {
	normalized, err := normalizeKey(key)
	if err != nil {
		return nil
	}

	entries := c.snapshot().entries[normalized]
	values := make([]string, len(entries))
	for i, entry := range entries {
		values[i] = entry.Value
	}
	return values
}

// Entries returns all raw entries for key in parse order.
func (c *Config) Entries(key string) []Entry {
	normalized, err := normalizeKey(key)
	if err != nil {
		return nil
	}

	entries := c.snapshot().entries[normalized]
	result := make([]Entry, len(entries))
	copy(result, entries)
	return result
}

// Section returns the effective key-value map for section.
func (c *Config) Section(section string) map[string]string {
	section = strings.ToLower(strings.TrimSpace(section))
	result := make(map[string]string)

	for _, entries := range c.snapshot().entries {
		if len(entries) == 0 {
			continue
		}
		entry := entries[len(entries)-1]
		if entry.Section == section {
			result[entry.Name] = entry.Value
		}
	}
	return result
}

// HasSection reports whether section has any effective keys.
func (c *Config) HasSection(section string) bool {
	return len(c.Section(section)) > 0
}

// Sections returns all sections in deterministic order.
func (c *Config) Sections() []string {
	seen := make(map[string]struct{})
	for _, entries := range c.snapshot().entries {
		if len(entries) == 0 {
			continue
		}
		seen[entries[len(entries)-1].Section] = struct{}{}
	}

	sections := make([]string, 0, len(seen))
	for section := range seen {
		sections = append(sections, section)
	}
	sort.Strings(sections)
	return sections
}

// All returns the effective configuration grouped by section.
func (c *Config) All() map[string]map[string]string {
	return allValues(c.snapshot())
}

// Sources returns the configuration sources that were read.
func (c *Config) Sources() []Source {
	return cloneSources(c.snapshot().sources)
}

// Reload reloads the original root sources into c.
func (c *Config) Reload(ctx context.Context) error {
	state := c.snapshot()
	roots := state.roots

	if len(roots) == 0 {
		return nil
	}

	parser := newParser(parserOptions{includeFiles: state.includes, repoPath: state.repoPath})
	var next *Config
	if state.useGit {
		var err error
		next, err = parser.parseFromGitCommand(ctx, options{
			scopes:       scopesFromSources(roots),
			repoPath:     state.repoPath,
			includeFiles: state.includes,
			useGit:       true,
		})
		if err != nil {
			return err
		}
	} else {
		next = newConfig()
		nextState := next.mutableState()
		nextState.roots = append(nextState.roots, roots...)
		nextState.repoPath = state.repoPath
		nextState.includes = state.includes
		parseState := newParseState()
		for _, source := range roots {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := parser.parseConfigFile(
				ctx,
				source.Path,
				source,
				next,
				parseState,
			); err != nil {
				return err
			}
		}
	}

	c.state.Store(next.snapshot())
	return nil
}

// Clone returns a deep copy.
func (c *Config) Clone() *Config {
	clone := &Config{}
	clone.state.Store(cloneState(c.snapshot()))
	return clone
}

// User returns user.name and user.email as a User.
func (c *Config) User() (User, error) {
	name, err := Get[string](c, "user.name")
	if err != nil {
		return User{}, fmt.Errorf("get user.name: %w", err)
	}

	email, err := Get[string](c, "user.email")
	if err != nil {
		return User{}, fmt.Errorf("get user.email: %w", err)
	}

	return User{Name: name, Email: email}, nil
}

// RemoteURL returns the URL for remote, defaulting to origin.
func (c *Config) RemoteURL(remote string) (string, error) {
	if remote == "" {
		remote = "origin"
	}
	return Get[string](c, "remote."+remote+".url")
}

// String returns a deterministic Git-config-like representation.
func (c *Config) String() string {
	state := c.snapshot()
	all := allValues(state)
	sections := make([]string, 0, len(all))
	for section := range all {
		sections = append(sections, section)
	}
	sort.Strings(sections)

	var b strings.Builder
	if len(state.sources) > 0 {
		b.WriteString("# Configuration sources:\n")
		for _, source := range state.sources {
			b.WriteString("# ")
			b.WriteString(source.Scope.String())
			b.WriteString(": ")
			b.WriteString(source.Path)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	for _, section := range sections {
		b.WriteString("[")
		b.WriteString(section)
		b.WriteString("]\n")

		keys := make([]string, 0, len(all[section]))
		for key := range all[section] {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			b.WriteString("  ")
			b.WriteString(key)
			b.WriteString(" = ")
			b.WriteString(formatConfigValue(all[section][key]))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

// Get returns key converted to T.
func Get[T Scalar](c *Config, key string) (T, error) {
	var zero T
	if c == nil {
		return zero, &ConfigError{Op: "get", Key: key, Err: ErrKeyNotFound}
	}

	value, err := c.Value(key)
	if err != nil {
		return zero, err
	}

	converted, err := convertValue[T](value)
	if err != nil {
		section, name, _ := splitConfigKey(key)
		return zero, &ConfigError{Op: "get", Section: section, Key: name, Err: err}
	}
	return converted, nil
}

// GetDefault returns key converted to T or fallback if key cannot be read.
func GetDefault[T Scalar](c *Config, key string, fallback T) T {
	value, err := Get[T](c, key)
	if err != nil {
		return fallback
	}
	return value
}

func (c *Config) addEntry(key, value string, source Source, line int) error {
	section, name, err := splitConfigKey(key)
	if err != nil {
		return err
	}

	section = strings.ToLower(section)
	name = strings.ToLower(name)
	key = section + "." + name
	entry := Entry{
		Key:     key,
		Section: section,
		Name:    name,
		Value:   value,
		Source:  source,
		Line:    line,
	}

	s := c.mutableState()
	s.entries[key] = append(s.entries[key], entry)
	return nil
}

func (c *Config) addSource(source Source) {
	s := c.mutableState()
	if slices.Contains(s.sources, source) {
		return
	}
	s.sources = append(s.sources, source)
}

func (c *Config) setRoots(roots []Source) {
	s := c.mutableState()
	s.roots = append(s.roots[:0], roots...)
}

func scopesFromSources(sources []Source) []Scope {
	scopes := make([]Scope, 0, len(sources))
	seen := make(map[Scope]bool)
	for _, src := range sources {
		if seen[src.Scope] {
			continue
		}
		seen[src.Scope] = true
		scopes = append(scopes, src.Scope)
	}
	return scopes
}

func lastEntry(state *configState, key string) (Entry, bool) {
	key, err := normalizeKey(key)
	if err != nil {
		return Entry{}, false
	}

	entries := state.entries[key]
	if len(entries) == 0 {
		return Entry{}, false
	}
	return entries[len(entries)-1], true
}

func hasSection(state *configState, section string) bool {
	section = strings.ToLower(strings.TrimSpace(section))
	for _, entries := range state.entries {
		if len(entries) == 0 {
			continue
		}
		if entries[len(entries)-1].Section == section {
			return true
		}
	}
	return false
}

func allValues(state *configState) map[string]map[string]string {
	result := make(map[string]map[string]string)
	for _, entries := range state.entries {
		if len(entries) == 0 {
			continue
		}
		entry := entries[len(entries)-1]
		if result[entry.Section] == nil {
			result[entry.Section] = make(map[string]string)
		}
		result[entry.Section][entry.Name] = entry.Value
	}
	return result
}

func cloneState(state *configState) *configState {
	clone := &configState{
		entries:  make(map[string][]Entry, len(state.entries)),
		sources:  cloneSources(state.sources),
		roots:    cloneSources(state.roots),
		repoPath: state.repoPath,
		includes: state.includes,
		useGit:   state.useGit,
	}
	for key, entries := range state.entries {
		clone.entries[key] = append([]Entry(nil), entries...)
	}
	return clone
}

func cloneSources(sources []Source) []Source {
	result := make([]Source, len(sources))
	copy(result, sources)
	return result
}

func formatConfigValue(value string) string {
	if value == "" {
		return ""
	}
	if strings.ContainsAny(value, " \t\n\r\"\\#;") {
		return strconv.Quote(value)
	}
	return value
}
