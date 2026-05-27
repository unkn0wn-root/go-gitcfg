package gitcfg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	includeSection  = "include"
	includeIfPrefix = "includeif."

	gitDirInsensitiveConditionPrefix = "gitdir/i:"
	gitDirConditionPrefix            = "gitdir:"
	onBranchConditionPrefix          = "onbranch:"
)

type parserOptions struct {
	includeFiles bool
	repoPath     string
}

type parser struct {
	opts parserOptions
}

type parseState struct {
	stack map[string]bool
	depth int
}

type parseFile struct {
	cfg        *Config
	source     Source
	sourcePath string
	state      *parseState
	section    string
}

func newParser(opts parserOptions) *parser {
	return &parser{opts: opts}
}

func newParseState() *parseState {
	return &parseState{stack: make(map[string]bool)}
}

func (p *parser) parseFromFiles(ctx context.Context, opts options) (*Config, error) {
	cfg := newConfig()
	s := cfg.mutableState()
	s.repoPath = opts.repoPath
	s.includes = opts.includeFiles
	s.useGit = false
	sources := getConfigSources(opts)
	cfg.setRoots(sources)

	ps := newParseState()
	for _, src := range sources {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := p.parseConfigFile(ctx, src.Path, src, cfg, ps); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

func (p *parser) parseFromGitCommand(ctx context.Context, opts options) (*Config, error) {
	cfg := newConfig()
	s := cfg.mutableState()
	s.repoPath = opts.repoPath
	s.includes = opts.includeFiles
	s.useGit = true

	sources := getConfigSources(opts)
	cfg.setRoots(sources)
	if len(sources) == 0 {
		return cfg, nil
	}

	seen := make(map[Scope]bool)
	for _, src := range sources {
		if seen[src.Scope] {
			continue
		}
		seen[src.Scope] = true

		out, err := p.runGitConfig(ctx, opts, src.Scope)
		if err != nil {
			return nil, err
		}

		for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
			if line == "" {
				continue
			}

			sourcePath, key, value := parseGitConfigOutputLine(line)
			if key == "" {
				continue
			}

			entrySrc := Source{Scope: src.Scope, Path: sourcePath}
			cfg.addSource(entrySrc)
			if err := cfg.addEntry(key, value, entrySrc, i+1); err != nil {
				return nil, &ConfigError{Op: "parse", Key: key, Source: sourcePath, Err: err}
			}
		}
	}

	return cfg, nil
}

func (p *parser) runGitConfig(ctx context.Context, opts options, scope Scope) (string, error) {
	args := []string{"config", "--list", "--show-origin", gitConfigScopeFlag(scope)}
	cmd := exec.CommandContext(ctx, "git", args...)
	if opts.repoPath != "" {
		cmd.Dir = opts.repoPath
	}

	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", &ConfigError{
				Op:  "load",
				Err: fmt.Errorf("git config failed: %s", string(ee.Stderr)),
			}
		}
		return "", &ConfigError{Op: "load", Err: fmt.Errorf("execute git config: %w", err)}
	}
	return string(out), nil
}

func gitConfigScopeFlag(scope Scope) string {
	switch scope {
	case ScopeSystem:
		return "--system"
	case ScopeGlobal:
		return "--global"
	case ScopeLocal:
		return "--local"
	case ScopeWorktree:
		return "--worktree"
	default:
		return "--global"
	}
}

func parseGitConfigOutputLine(line string) (sourcePath, key, value string) {
	if head, rest, ok := strings.Cut(line, "\t"); ok {
		if path, ok := strings.CutPrefix(head, "file:"); ok {
			sourcePath = path
		}
		line = rest
	}

	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return sourcePath, "", ""
	}
	return sourcePath, strings.TrimSpace(key), value
}

func (p *parser) parseConfigFile(
	ctx context.Context,
	path string,
	source Source,
	cfg *Config,
	ps *parseState,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return &ConfigError{Op: "parse", Source: path, Err: err}
	}
	source.Path = abs

	if ps.stack[abs] {
		return &ConfigError{Op: "parse", Source: abs, Err: ErrIncludeCycle}
	}
	if ps.depth > 100 {
		return &ConfigError{
			Op:     "parse",
			Source: abs,
			Err:    fmt.Errorf("%w: maximum include depth exceeded", ErrIncludeCycle),
		}
	}

	f, err := os.Open(abs)
	if err != nil {
		return &ConfigError{Op: "parse", Source: abs, Err: fmt.Errorf("open config file: %w", err)}
	}
	defer f.Close()

	cfg.addSource(source)
	ps.stack[abs] = true
	ps.depth++
	defer func() {
		ps.depth--
		delete(ps.stack, abs)
	}()

	pf := &parseFile{
		cfg:        cfg,
		source:     source,
		sourcePath: abs,
		state:      ps,
	}
	return p.parseConfigReader(ctx, f, pf)
}

func (p *parser) parseConfigReader(ctx context.Context, r io.Reader, pf *parseFile) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var pending string
	start := 0
	n := 0

	flush := func(line string, n int) error {
		return p.parseLogicalLine(ctx, pf, line, n)
	}

	for sc.Scan() {
		n++
		line := sc.Text()
		if pending == "" {
			start = n
			pending = line
		} else {
			pending += strings.TrimLeft(line, " \t")
		}

		if hasContinuation(pending) {
			pending = pending[:len(pending)-1]
			continue
		}

		if err := flush(pending, start); err != nil {
			return err
		}
		pending = ""
	}

	if pending != "" {
		if err := flush(pending, start); err != nil {
			return err
		}
	}

	if err := sc.Err(); err != nil {
		return &ConfigError{
			Op:     "parse",
			Source: pf.sourcePath,
			Err:    fmt.Errorf("scan config file: %w", err),
		}
	}
	return nil
}

func (p *parser) parseLogicalLine(ctx context.Context, pf *parseFile, line string, n int) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s := strings.TrimSpace(line)
	if s == "" || strings.HasPrefix(s, "#") || strings.HasPrefix(s, ";") {
		return nil
	}

	if strings.HasPrefix(s, "[") {
		next, err := parseSectionHeader(strings.TrimSpace(stripInlineComment(s)))
		if err != nil {
			return &ConfigError{
				Op:     "parse",
				Source: fmt.Sprintf("%s:%d", pf.sourcePath, n),
				Err:    err,
			}
		}
		pf.section = next
		return nil
	}

	key, value, err := parseAssignment(line)
	if err != nil {
		return &ConfigError{Op: "parse", Source: fmt.Sprintf("%s:%d", pf.sourcePath, n), Err: err}
	}

	key = buildFullKey(pf.section, key)
	if err := pf.cfg.addEntry(key, value, pf.source, n); err != nil {
		return &ConfigError{
			Op:     "parse",
			Key:    key,
			Source: fmt.Sprintf("%s:%d", pf.sourcePath, n),
			Err:    err,
		}
	}

	if p.opts.includeFiles {
		path, ok := p.includePath(key, value, pf.sourcePath)
		if ok {
			src := Source{Scope: pf.source.Scope, Path: path}
			if err := p.parseConfigFile(ctx, path, src, pf.cfg, pf.state); err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
		}
	}

	return nil
}

func parseSectionHeader(line string) (string, error) {
	if !strings.HasSuffix(line, "]") {
		return "", fmt.Errorf("%w: malformed section header", ErrInvalidKeyFormat)
	}
	s := strings.TrimSpace(line[1 : len(line)-1])
	if s == "" {
		return "", fmt.Errorf("%w: empty section", ErrInvalidKeyFormat)
	}

	base, rest, ok := strings.Cut(s, " ")
	if !ok {
		if !isValidSection(s) {
			return "", fmt.Errorf("%w: invalid section", ErrInvalidKeyFormat)
		}
		return s, nil
	}

	base = strings.TrimSpace(base)
	rest = strings.TrimSpace(rest)
	if base == "" || rest == "" {
		return "", fmt.Errorf("%w: malformed subsection", ErrInvalidKeyFormat)
	}
	if !strings.HasPrefix(rest, `"`) || !strings.HasSuffix(rest, `"`) {
		return "", fmt.Errorf("%w: subsection must be quoted", ErrInvalidKeyFormat)
	}
	sub, err := strconv.Unquote(rest)
	if err != nil {
		return "", fmt.Errorf("unquote subsection: %w", err)
	}
	return base + "." + sub, nil
}

func parseAssignment(line string) (key, value string, err error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", "", ErrInvalidKeyFormat
	}

	if k, v, ok := strings.Cut(line, "="); ok {
		key = strings.TrimSpace(k)
		value, err = parseValue(v)
		if key == "" {
			return "", "", ErrInvalidKeyFormat
		}
		return key, value, err
	}

	key = strings.TrimSpace(stripInlineComment(line))
	if key == "" || strings.ContainsAny(key, " \t") {
		return "", "", ErrInvalidKeyFormat
	}
	return key, "", nil
}

func parseValue(v string) (string, error) {
	v = strings.TrimSpace(stripInlineComment(v))
	if v == "" {
		return "", nil
	}
	if strings.HasPrefix(v, `"`) {
		if !strings.HasSuffix(v, `"`) {
			return "", fmt.Errorf("%w: unterminated quoted value", ErrInvalidValue)
		}
		s, err := strconv.Unquote(v)
		if err != nil {
			return "", err
		}
		return s, nil
	}
	return v, nil
}

func stripInlineComment(s string) string {
	quote := false
	esc := false
	for i, r := range s {
		if esc {
			esc = false
			continue
		}
		switch r {
		case '\\':
			if quote {
				esc = true
			}
		case '"':
			quote = !quote
		case '#', ';':
			if !quote && (i == 0 || isSpace(s[i-1])) {
				return s[:i]
			}
		}
	}
	return s
}

func buildFullKey(section, key string) string {
	if section == "" {
		return key
	}
	return section + "." + key
}

func hasContinuation(line string) bool {
	n := 0
	for i := len(line) - 1; i >= 0 && line[i] == '\\'; i-- {
		n++
	}
	return n%2 == 1
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t'
}

func (p *parser) includePath(key, value, sourcePath string) (string, bool) {
	section, name, err := splitConfigKey(key)
	if err != nil || !strings.EqualFold(name, "path") {
		return "", false
	}

	switch {
	case strings.EqualFold(section, includeSection):
		return resolveIncludePath(value, sourcePath), true
	case hasIncludeIfPrefix(section):
		cond := section[len(includeIfPrefix):]
		if p.matchIncludeCondition(cond) {
			return resolveIncludePath(value, sourcePath), true
		}
	}
	return "", false
}

func hasIncludeIfPrefix(section string) bool {
	return len(section) > len(includeIfPrefix) &&
		strings.EqualFold(section[:len(includeIfPrefix)], includeIfPrefix)
}

func resolveIncludePath(path, sourcePath string) string {
	path = expandHome(path)
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(sourcePath), path))
}

func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
}

func (p *parser) matchIncludeCondition(cond string) bool {
	s := strings.ToLower(cond)
	switch {
	case strings.HasPrefix(s, gitDirInsensitiveConditionPrefix):
		return p.matchGitDir(cond[len(gitDirInsensitiveConditionPrefix):], true)
	case strings.HasPrefix(s, gitDirConditionPrefix):
		return p.matchGitDir(cond[len(gitDirConditionPrefix):], false)
	case strings.HasPrefix(s, onBranchConditionPrefix):
		return p.matchBranch(cond[len(onBranchConditionPrefix):])
	default:
		return false
	}
}

func (p *parser) matchGitDir(pattern string, insensitive bool) bool {
	if p.opts.repoPath == "" {
		return false
	}

	repo, err := filepath.Abs(p.opts.repoPath)
	if err != nil {
		return false
	}
	gitDir, err := findGitDir(repo)
	if err != nil {
		return false
	}

	pattern = normalizeSlash(expandHome(pattern))
	if !filepath.IsAbs(filepath.FromSlash(pattern)) && !strings.ContainsAny(pattern, "*?[") {
		if abs, err := filepath.Abs(filepath.FromSlash(pattern)); err == nil {
			pattern = normalizeSlash(abs)
		}
	}

	candidates := []string{
		ensureTrailingSlash(normalizeSlash(repo)),
		ensureTrailingSlash(normalizeSlash(gitDir)),
	}

	if insensitive {
		pattern = strings.ToLower(pattern)
	}
	for _, c := range candidates {
		if insensitive {
			c = strings.ToLower(c)
		}
		if matchPathPattern(pattern, c) {
			return true
		}
	}
	return false
}

func (p *parser) matchBranch(pattern string) bool {
	if p.opts.repoPath == "" {
		return false
	}

	branch, err := currentBranch(p.opts.repoPath)
	if err != nil {
		return false
	}
	return matchPathPattern(normalizeSlash(pattern), normalizeSlash(branch))
}

func currentBranch(repoPath string) (string, error) {
	gitDir, err := findGitDir(repoPath)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(b))
	ref, ok := strings.CutPrefix(s, "ref: refs/heads/")
	if !ok {
		return "", errors.New("detached HEAD")
	}
	return ref, nil
}

func matchPathPattern(pattern, value string) bool {
	pattern = normalizeSlash(pattern)
	value = normalizeSlash(value)

	if !strings.ContainsAny(pattern, "*?[") {
		if strings.HasSuffix(pattern, "/") {
			return strings.HasPrefix(value, pattern)
		}
		return strings.TrimSuffix(value, "/") == strings.TrimSuffix(pattern, "/")
	}

	re, err := regexp.Compile(globToRegexp(pattern))
	if err != nil {
		return false
	}
	return re.MatchString(value)
}

func globToRegexp(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '[':
			end := globClassEnd(pattern, i+1)
			if end == -1 {
				b.WriteString(`\[`)
				continue
			}
			b.WriteString(globClassToRegexp(pattern[i+1 : end]))
			i = end
		default:
			b.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
		}
	}
	b.WriteString("$")
	return b.String()
}

func globClassEnd(pattern string, start int) int {
	for i := start; i < len(pattern); i++ {
		if pattern[i] == ']' && i > start {
			return i
		}
	}
	return -1
}

func globClassToRegexp(class string) string {
	var b strings.Builder
	b.WriteByte('[')
	if strings.HasPrefix(class, "!") {
		b.WriteByte('^')
		class = class[1:]
	} else if strings.HasPrefix(class, "^") {
		b.WriteString(`\^`)
		class = class[1:]
	}

	for i := 0; i < len(class); i++ {
		switch class[i] {
		case '\\':
			b.WriteString(`\\`)
		case '[':
			b.WriteString(`\[`)
		default:
			b.WriteByte(class[i])
		}
	}
	b.WriteByte(']')
	return b.String()
}

func normalizeSlash(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func ensureTrailingSlash(path string) string {
	if strings.HasSuffix(path, "/") {
		return path
	}
	return path + "/"
}
