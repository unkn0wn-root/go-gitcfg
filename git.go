package gogitcfg

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type ConfigSource struct {
	Type ConfigSourceType
	Path string
}

type ConfigSourceType int

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

type Constraint interface {
	~string | ~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64 | ~bool
}

type User struct {
	Name  string
	Email string
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

func (c *Config) GetString(key string) (string, error) {
	return Get[string](c, key)
}

func (c *Config) GetInt(key string) (int, error) {
	return Get[int](c, key)
}

func (c *Config) GetBool(key string) (bool, error) {
	return Get[bool](c, key)
}

func (c *Config) GetFloat64(key string) (float64, error) {
	return Get[float64](c, key)
}

func (c *Config) GetMultiValue(key string) ([]string, error) {
	// For now, return single value in slice
    // @todo david: this should parse multi-value configurations
	value, err := c.GetString(key)
	if err != nil {
		return nil, err
	}

	return []string{value}, nil
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

func (c *Config) GetSectionSize(section string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sectionMap, exists := c.sections[section]
	if !exists {
		return 0
	}

	return len(sectionMap)
}

func (c *Config) GetKeys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var keys []string
	for section, sectionMap := range c.sections {
		for key := range sectionMap {
			keys = append(keys, fmt.Sprintf("%s.%s", section, key))
		}
	}

	return keys
}

func (c *Config) GetKeysInSection(section string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sectionMap, exists := c.sections[section]
	if !exists {
		return nil
	}

	keys := make([]string, 0, len(sectionMap))
	for key := range sectionMap {
		keys = append(keys, key)
	}

	return keys
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
		// No sources recorded, reload global config as fallback
		newConfig, err := LoadGlobalWithContext(ctx)
		if err != nil {
			return fmt.Errorf("failed to reload configuration: %w", err)
		}

		c.mu.Lock()
		c.sections = newConfig.sections
		c.sources = newConfig.sources
		c.mu.Unlock()

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

func (c *Config) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for _, sectionMap := range c.sections {
		count += len(sectionMap)
	}

	return count
}

func (c *Config) IsEmpty() bool {
	return c.Size() == 0
}

func (c *Config) GetUser() (*User, error) {
	name, err := c.GetString("user.name")
	if err != nil {
		return nil, fmt.Errorf("failed to get user.name: %w", err)
	}

	email, err := c.GetString("user.email")
	if err != nil {
		return nil, fmt.Errorf("failed to get user.email: %w", err)
	}

	return &User{
		Name:  name,
		Email: email,
	}, nil
}

func (c *Config) GetRemote(name string) (*Remote, error) {
	if name == "" {
		name = "origin"
	}

	sectionName := fmt.Sprintf("remote.%s", name)
	section := c.GetSection(sectionName)
	if len(section) == 0 {
		return nil, &ConfigError{
			Op:      "get",
			Section: sectionName,
			Err:     ErrSectionNotFound,
		}
	}

	remote := &Remote{
		Name: name,
	}

	if url, exists := section["url"]; exists {
		remote.URL = url
	}
	if fetchURL, exists := section["fetchurl"]; exists {
		remote.FetchURL = fetchURL
	}
	if pushURL, exists := section["pushurl"]; exists {
		remote.PushURL = pushURL
	}

	// Handle multiple fetch/push specifications
	// ffor now, handle single values
    // @todo: should maybe be extended?
	if fetch, exists := section["fetch"]; exists {
		remote.Fetch = []string{fetch}
	}
	if push, exists := section["push"]; exists {
		remote.Push = []string{push}
	}

	return remote, nil
}

func (c *Config) GetRemoteURL(remote string) (string, error) {
	if remote == "" {
		remote = "origin"
	}
	return c.GetString(fmt.Sprintf("remote.%s.url", remote))
}

func (c *Config) GetBranchConfig(name string) (*Branch, error) {
	sectionName := fmt.Sprintf("branch.%s", name)
	section := c.GetSection(sectionName)
	if len(section) == 0 {
		return nil, &ConfigError{
			Op:      "get",
			Section: sectionName,
			Err:     ErrSectionNotFound,
		}
	}

	branch := &Branch{
		Name: name,
	}

	if remote, exists := section["remote"]; exists {
		branch.Remote = remote
	}
	if merge, exists := section["merge"]; exists {
		branch.Merge = merge
	}
	if rebase, exists := section["rebase"]; exists {
		branch.Rebase = rebase
	}

	return branch, nil
}

func (c *Config) GetCoreConfig() (*CoreConfig, error) {
	core := &CoreConfig{}

	if editor, err := c.GetString(CoreEditor); err == nil {
		core.Editor = editor
	}
	if autocrlf, err := c.GetString(CoreAutoCRLF); err == nil {
		core.AutoCRLF = autocrlf
	}
	if safecrlf, err := c.GetString(CoreSafeCRLF); err == nil {
		core.SafeCRLF = safecrlf
	}
	if filemode, err := c.GetBool(CoreFileMode); err == nil {
		core.FileMode = filemode
	}
	if symlinks, err := c.GetBool(CoreSymlinks); err == nil {
		core.Symlinks = symlinks
	}
	if ignorecase, err := c.GetBool(CoreIgnoreCase); err == nil {
		core.IgnoreCase = ignorecase
	}
	if quotepath, err := c.GetBool(CoreQuotePath); err == nil {
		core.QuotePath = quotepath
	}
	if eol, err := c.GetString(CoreEOL); err == nil {
		core.EOL = eol
	}
	if pager, err := c.GetString(CorePager); err == nil {
		core.Pager = pager
	}
	if bare, err := c.GetBool(CoreBare); err == nil {
		core.Bare = bare
	}
	if logallrefupdates, err := c.GetBool(CoreLogAllRefUpdates); err == nil {
		core.LogAllRefUpdates = logallrefupdates
	}
	if repoformat, err := c.GetInt(CoreRepositoryFormatVersion); err == nil {
		core.RepositoryFormatVersion = repoformat
	}

	return core, nil
}

func (c *Config) GetHTTPConfig() (*HTTPConfig, error) {
	http := &HTTPConfig{}

	if postbuffer, err := c.GetInt(HTTPPostBuffer); err == nil {
		http.PostBuffer = postbuffer
	}
	if proxy, err := c.GetString(HTTPSProxy); err == nil {
		http.Proxy = proxy
	}
	if sslverify, err := c.GetBool(HTTPSLLVerify); err == nil {
		http.SSLVerify = sslverify
	}
	if timeout, err := c.GetInt(HTTPTimeout); err == nil {
		http.Timeout = time.Duration(timeout) * time.Second
	}
	if lowspeedlimit, err := c.GetInt(HTTPLowSpeedLimit); err == nil {
		http.LowSpeedLimit = lowspeedlimit
	}
	if lowspeedtime, err := c.GetInt(HTTPLowSpeedTime); err == nil {
		http.LowSpeedTime = time.Duration(lowspeedtime) * time.Second
	}

	return http, nil
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
