package gogitcfg

import (
    "fmt"
    "time"
    "context"
)
type configOptions struct {
	includeSystem   bool
	includeGlobal   bool
	includeLocal    bool
	includeWorktree bool
	repoPath        string
	useGitCommand   bool
	timeout         time.Duration
}

type ConfigOption func(*configOptions)

func WithSystem() ConfigOption {
	return func(opts *configOptions) {
		opts.includeSystem = true
	}
}

func WithGlobal() ConfigOption {
	return func(opts *configOptions) {
		opts.includeGlobal = true
	}
}

func WithLocal() ConfigOption {
	return func(opts *configOptions) {
		opts.includeLocal = true
	}
}

func WithWorktree() ConfigOption {
	return func(opts *configOptions) {
		opts.includeWorktree = true
	}
}

func WithRepoPath(path string) ConfigOption {
	return func(opts *configOptions) {
		opts.repoPath = path
	}
}

func WithGitCommand() ConfigOption {
	return func(opts *configOptions) {
		opts.useGitCommand = true
	}
}

func WithTimeout(timeout time.Duration) ConfigOption {
	return func(opts *configOptions) {
		opts.timeout = timeout
	}
}

func Load(opts ...ConfigOption) (*Config, error) {
	return LoadWithContext(context.Background(), opts...)
}

func LoadWithContext(ctx context.Context, opts ...ConfigOption) (*Config, error) {
	options := &configOptions{
		includeGlobal: true, // Default to global config
		timeout:       DefaultTimeout,
	}

	for _, opt := range opts {
		opt(options)
	}

	if (options.includeLocal || options.includeWorktree) && options.repoPath != "" {
		if err := validateRepoPath(options.repoPath); err != nil {
			return nil, &ConfigError{
				Op:  "load",
				Err: fmt.Errorf("invalid repository path: %w", err),
			}
		}
	}

	parser := newParser()
	if options.useGitCommand {
		return parser.parseFromGitCommand(ctx, options)
	}

	return parser.parseFromFiles(ctx, options)
}

func LoadGlobal() (*Config, error) {
	return Load(WithGlobal())
}

func LoadLocal(repoPath string) (*Config, error) {
	return Load(WithLocal(), WithRepoPath(repoPath))
}

func LoadAll(repoPath string) (*Config, error) {
	return Load(WithSystem(), WithGlobal(), WithLocal(), WithWorktree(), WithRepoPath(repoPath))
}

func LoadGlobalWithContext(ctx context.Context) (*Config, error) {
	return LoadWithContext(ctx, WithGlobal())
}

func LoadLocalWithContext(ctx context.Context, repoPath string) (*Config, error) {
	return LoadWithContext(ctx, WithLocal(), WithRepoPath(repoPath))
}

func LoadAllWithContext(ctx context.Context, repoPath string) (*Config, error) {
	return LoadWithContext(ctx, WithSystem(), WithGlobal(), WithLocal(), WithWorktree(), WithRepoPath(repoPath))
}
