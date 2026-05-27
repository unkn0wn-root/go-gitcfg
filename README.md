# go-gitcfg

`go-gitcfg` is a small Go library for reading Git configuration files.

It uses a pure Go parser by default, preserves repeated keys, follows Git configuration precedence and supports `include.path` plus common `includeIf` conditions.

## Install

```sh
go get github.com/unkn0wn-root/go-gitcfg
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"

	"github.com/unkn0wn-root/go-gitcfg"
)

func main() {
	cfg, err := gitcfg.LoadGlobal(context.Background())
	if err != nil {
		panic(err)
	}

	user, err := cfg.User()
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s <%s>\n", user.Name, user.Email)
}
```

## Loading

```go
ctx := context.Background()

// Global configuration only. This is also the default for Load.
cfg, err := gitcfg.LoadGlobal(ctx)

// Local repository configuration only.
cfg, err = gitcfg.LoadLocal(ctx, "/path/to/repo")

// System, global, local, and worktree scopes in Git precedence order.
cfg, err = gitcfg.LoadAll(ctx, "/path/to/repo")

// Explicit scopes and options.
cfg, err = gitcfg.Load(
	ctx,
	gitcfg.WithScopes(gitcfg.ScopeGlobal, gitcfg.ScopeLocal),
	gitcfg.WithRepoPath("/path/to/repo"),
	gitcfg.WithIncludes(true),
)

// Optional: use `git config` instead of the pure-Go parser.
cfg, err = gitcfg.Load(ctx, gitcfg.WithGitCommand())
```

## Accessing Values

```go
name, err := gitcfg.Get[string](cfg, "user.name")
autocrlf, err := gitcfg.Get[bool](cfg, "core.autocrlf")
timeout := gitcfg.GetDefault[int](cfg, "http.timeout", 30)

editor, err := cfg.Value("core.editor")
editors := cfg.Values("core.editor") // all repeated values, in parse order
entries := cfg.Entries("remote.origin.fetch")

remoteURL, err := cfg.RemoteURL("origin")
user, err := cfg.User()
```

Simple getters return the last value. Use `Values` or `Entries` when you need every occurrence of a repeated key.

## Sections And Sources

```go
if cfg.Has("user.email") {
	fmt.Println("email configured")
}

for _, section := range cfg.Sections() {
	fmt.Println(section)
}

remote := cfg.Section("remote.origin")
all := cfg.All()
sources := cfg.Sources()
```

## Supported Scopes

- `ScopeSystem`: `/etc/gitconfig` and common system fallback paths
- `ScopeGlobal`: XDG global config and `~/.gitconfig`
- `ScopeLocal`: `.git/config`
- `ScopeWorktree`: `.git/config.worktree`

Environment overrides such as `GIT_CONFIG_GLOBAL`, `GIT_CONFIG_SYSTEM` and `GIT_CONFIG_NOSYSTEM` are respected.
