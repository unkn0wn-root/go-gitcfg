package gitcfg

import (
	"context"
	"fmt"
	"time"
)

// DefaultTimeout is the default timeout used by Load.
const DefaultTimeout = 30 * time.Second

type options struct {
	scopes       []Scope
	repoPath     string
	includeFiles bool
	useGit       bool
	timeout      time.Duration
}

// Option configures Load.
type Option func(*options)

// WithScopes selects the Git configuration scopes to load.
func WithScopes(scopes ...Scope) Option {
	return func(o *options) {
		o.scopes = append(o.scopes[:0], scopes...)
	}
}

// WithRepoPath sets the repository path used for local/worktree scopes and includeIf evaluation.
func WithRepoPath(path string) Option {
	return func(o *options) {
		o.repoPath = path
	}
}

// WithIncludes enables or disables include.path and includeIf handling.
func WithIncludes(enabled bool) Option {
	return func(o *options) {
		o.includeFiles = enabled
	}
}

// WithTimeout sets the maximum time spent loading configuration.
func WithTimeout(timeout time.Duration) Option {
	return func(o *options) {
		o.timeout = timeout
	}
}

// WithGitCommand loads configuration by invoking git config instead of the pure-Go parser.
func WithGitCommand() Option {
	return func(o *options) {
		o.useGit = true
	}
}

// Load reads Git configuration according to opts.
func Load(ctx context.Context, opts ...Option) (*Config, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}

	if o.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.timeout)
		defer cancel()
	}

	if needsRepo(o.scopes) {
		if _, err := findGitDir(o.repoPath); err != nil {
			return nil, &ConfigError{
				Op:  "load",
				Err: fmt.Errorf("invalid repository path: %w", err),
			}
		}
	}

	p := newParser(parserOptions{
		includeFiles: o.includeFiles,
		repoPath:     o.repoPath,
	})

	if o.useGit {
		return p.parseFromGitCommand(ctx, o)
	}
	return p.parseFromFiles(ctx, o)
}

// LoadGlobal loads user-level Git configuration.
func LoadGlobal(ctx context.Context) (*Config, error) {
	return Load(ctx, WithScopes(ScopeGlobal))
}

// LoadLocal loads repository-local Git configuration.
func LoadLocal(ctx context.Context, repoPath string) (*Config, error) {
	return Load(ctx, WithScopes(ScopeLocal), WithRepoPath(repoPath))
}

// LoadAll loads system, global, local, and worktree configuration in precedence order.
func LoadAll(ctx context.Context, repoPath string) (*Config, error) {
	return Load(
		ctx,
		WithScopes(ScopeSystem, ScopeGlobal, ScopeLocal, ScopeWorktree),
		WithRepoPath(repoPath),
	)
}

func defaultOptions() options {
	return options{
		scopes:       []Scope{ScopeGlobal},
		includeFiles: true,
		timeout:      DefaultTimeout,
	}
}

func needsRepo(scopes []Scope) bool {
	for _, s := range scopes {
		if s == ScopeLocal || s == ScopeWorktree {
			return true
		}
	}
	return false
}
