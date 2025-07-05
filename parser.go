package gogitcfg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type parser struct {
	sectionRegex      *regexp.Regexp
	keyValueRegex     *regexp.Regexp
	commentRegex      *regexp.Regexp
	continuationRegex *regexp.Regexp
}

func newParser() *parser {
	return &parser{
		sectionRegex:      regexp.MustCompile(`^\s*\[([^\]]+)\]\s*$`),
		keyValueRegex:     regexp.MustCompile(`^\s*([^=\s]+)\s*=\s*(.*)$`),
		commentRegex:      regexp.MustCompile(`^\s*[#;]`),
		continuationRegex: regexp.MustCompile(`^\s+(.*)$`),
	}
}

func (p *parser) parseFromGitCommand(ctx context.Context, opts *configOptions) (*Config, error) {
	config := &Config{
		sections: make(map[string]map[string]string),
		sources:  make([]ConfigSource, 0),
	}

	if opts.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.timeout)
		defer cancel()
	}

	args := []string{"config", "--list", "--null", "--show-origin"}

	sourceFlags := p.buildSourceFlags(opts)
	if len(sourceFlags) > 0 {
		args = append(args, sourceFlags...)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	if opts.repoPath != "" {
		cmd.Dir = opts.repoPath
	}

	output, err := cmd.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return nil, &ConfigError{
				Op:  "load",
				Err: fmt.Errorf("git config failed: %s", string(exitError.Stderr)),
			}
		}
		return nil, &ConfigError{
			Op:  "load",
			Err: fmt.Errorf("failed to execute git config: %w", err),
		}
	}

	return p.parseGitConfigOutput(string(output), config)
}

func (p *parser) parseFromFiles(ctx context.Context, opts *configOptions) (*Config, error) {
	config := &Config{
		sections: make(map[string]map[string]string),
		sources:  make([]ConfigSource, 0),
	}

	for _, source := range getAllConfigPaths(opts) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if err := p.parseConfigFile(source.Path, config); err != nil {
			return nil, err
		}
		config.sources = append(config.sources, source)
	}

	return config, nil
}

func (p *parser) buildSourceFlags(opts *configOptions) []string {
	var sourceFlags []string

	if opts.includeSystem {
		sourceFlags = append(sourceFlags, "--system")
	}
	if opts.includeGlobal {
		sourceFlags = append(sourceFlags, "--global")
	}
	if opts.includeLocal {
		sourceFlags = append(sourceFlags, "--local")
	}

	return sourceFlags
}

func (p *parser) parseGitConfigOutput(output string, config *Config) (*Config, error) {
	lines := strings.Split(strings.TrimRight(output, "\x00"), "\x00")
	for _, line := range lines {
		if line == "" {
			continue
		}

		key, value, source := p.parseGitConfigLine(line)
		if key != "" {
			if err := config.setRawValue(key, value); err != nil {
				return nil, &ConfigError{
					Op:     "parse",
					Key:    key,
					Source: source,
					Err:    err,
				}
			}
		}
	}

	return config, nil
}

func (p *parser) parseGitConfigLine(line string) (key, value, source string) {
	// Parse show-origin format: "file:path\tkey=value"
	if strings.HasPrefix(line, "file:") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			source = strings.TrimPrefix(parts[0], "file:")
			kvParts := strings.SplitN(parts[1], "=", 2)
			if len(kvParts) == 2 {
				key = strings.TrimSpace(kvParts[0])
				value = kvParts[1]
			}
		}
		return key, value, source
	}

	// Fallback for older git versions
	parts := strings.SplitN(line, "=", 2)
	if len(parts) == 2 {
		key = strings.TrimSpace(parts[0])
		value = parts[1]
	}
	return key, value, source
}

func (p *parser) parseConfigFile(path string, config *Config) error {
	file, err := os.Open(path)
	if err != nil {
		return &ConfigError{
			Op:     "parse",
			Source: path,
			Err:    fmt.Errorf("failed to open config file: %w", err),
		}
	}
	defer file.Close()

	return p.parseConfigReader(file, config, path)
}

func (p *parser) parseConfigReader(reader io.Reader, config *Config, source string) error {
	scanner := bufio.NewScanner(reader)
	var currentSection string
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		if line == "" || p.commentRegex.MatchString(line) {
			continue
		}

		if matches := p.sectionRegex.FindStringSubmatch(line); matches != nil {
			currentSection = strings.TrimSpace(matches[1])
			continue
		}

		if matches := p.keyValueRegex.FindStringSubmatch(line); matches != nil {
			key := strings.TrimSpace(matches[1])
			value := strings.TrimSpace(matches[2])

			if processedValue, err := p.processQuotedValue(value); err != nil {
				return &ConfigError{
					Op:     "parse",
					Key:    key,
					Source: fmt.Sprintf("%s:%d", source, lineNumber),
					Err:    fmt.Errorf("invalid quoted value: %w", err),
				}
			} else {
				value = processedValue
			}

			fullKey := p.buildFullKey(currentSection, key)
			if err := config.setRawValue(fullKey, value); err != nil {
				return &ConfigError{
					Op:     "parse",
					Key:    fullKey,
					Source: fmt.Sprintf("%s:%d", source, lineNumber),
					Err:    err,
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return &ConfigError{
			Op:     "parse",
			Source: source,
			Err:    fmt.Errorf("scanner error: %w", err),
		}
	}

	return nil
}

func (p *parser) processQuotedValue(value string) (string, error) {
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return strconv.Unquote(value)
	}
	return value, nil
}

func (p *parser) buildFullKey(section, key string) string {
	if section == "" {
		return key
	}

	// Handle subsections like remote "origin" -> remote.origin
	if strings.Contains(section, " ") {
		parts := strings.SplitN(section, " ", 2)
		if len(parts) == 2 {
			subsection := strings.TrimSpace(parts[1])
			if len(subsection) >= 2 && subsection[0] == '"' && subsection[len(subsection)-1] == '"' {
				return parts[0] + "." + subsection[1:len(subsection)-1] + "." + key
			}
		}
	}

	return section + "." + key
}

func isValidConfigKey(key string) bool {
	if key == "" || !strings.Contains(key, ".") {
		return false
	}
	for _, r := range key {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '.' && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

func isValidSectionName(name string) bool {
	if name == "" {
		return false
	}

	// Handle subsections like remote "origin"
	if strings.Contains(name, " ") {
		parts := strings.SplitN(name, " ", 2)
		if len(parts) != 2 || !isValidKeyName(parts[0]) {
			return false
		}
		subsection := strings.TrimSpace(parts[1])
		return len(subsection) >= 2 && subsection[0] == '"' && subsection[len(subsection)-1] == '"'
	}

	return isValidKeyName(name)
}

func isValidSubsectionName(name string) bool {
	if name == "" || !strings.Contains(name, ".") {
		return false
	}
	for _, part := range strings.Split(name, ".") {
		if !isValidKeyName(part) {
			return false
		}
	}
	return true
}

func isValidKeyName(name string) bool {
	if name == "" {
		return false
	}

	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return false
		}
	}

	return true
}

func parseBool(value string) (bool, error) {
	if value == "" {
		return true, nil // Git treats empty values as true
	}

	lower := strings.ToLower(strings.TrimSpace(value))
	switch lower {
	case "true", "yes", "on", "1":
		return true, nil
	case "false", "no", "off", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value: %s", value)
	}
}

func convertValue[T Constraint](value string) (T, error) {
	var result any
	var err error

	switch any(*new(T)).(type) {
	case string:
		result = value
	case int:
		result, err = strconv.Atoi(value)
	case int8:
		var v int64
		v, err = strconv.ParseInt(value, 10, 8)
		if err == nil {
			result = int8(v)
		}
	case int16:
		var v int64
		v, err = strconv.ParseInt(value, 10, 16)
		if err == nil {
			result = int16(v)
		}
	case int32:
		var v int64
		v, err = strconv.ParseInt(value, 10, 32)
		if err == nil {
			result = int32(v)
		}
	case int64:
		result, err = strconv.ParseInt(value, 10, 64)
	case uint:
		var v uint64
		v, err = strconv.ParseUint(value, 10, 0)
		if err == nil {
			result = uint(v)
		}
	case uint8:
		var v uint64
		v, err = strconv.ParseUint(value, 10, 8)
		if err == nil {
			result = uint8(v)
		}
	case uint16:
		var v uint64
		v, err = strconv.ParseUint(value, 10, 16)
		if err == nil {
			result = uint16(v)
		}
	case uint32:
		var v uint64
		v, err = strconv.ParseUint(value, 10, 32)
		if err == nil {
			result = uint32(v)
		}
	case uint64:
		result, err = strconv.ParseUint(value, 10, 64)
	case float32:
		var v float64
		v, err = strconv.ParseFloat(value, 32)
		if err == nil {
			result = float32(v)
		}
	case float64:
		result, err = strconv.ParseFloat(value, 64)
	case bool:
		result, err = parseBool(value)
	default:
		err = fmt.Errorf("unsupported type")
	}

	if err != nil {
		var zero T
		return zero, fmt.Errorf("%w: %v", ErrInvalidValue, err)
	}

	return result.(T), nil
}
