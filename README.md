# gogitcfg

A Go library for reading and working with Git configuration files. Supports both global and repository-specific configurations.

## Quick Start

### Load Global Configuration

```go
package main

import (
    "fmt"
    "github.com/unkn0wn-root/gogitcfg"
)

func main() {
    // Load global Git configuration
    config, err := gogitcfg.LoadGlobal()
    if err != nil {
        panic(err)
    }

    // Get user information
    user, err := config.GetUser()
    if err != nil {
        panic(err)
    }

    fmt.Printf("User: %s <%s>\n", user.Name, user.Email)
}
```

### Type-Safe Value Access

```go
// String values
name, err := config.GetString("user.name")
editor, err := config.GetString("core.editor")

// Boolean values
autocrlf, err := config.GetBool("core.autocrlf")
filemode, err := config.GetBool("core.filemode")

// Integer values
timeout, err := config.GetInt("http.timeout")

// Generic type-safe access
editor, err := gogitcfg.Get[string](config, "core.editor")
timeout, err := gogitcfg.Get[int](config, "http.timeout")

// With default values
timeout := gogitcfg.GetWithDefault[int](config, "http.timeout", 30)
```

### Load Different Configuration Sources

```go
// Load only global configuration
config, err := gogitcfg.LoadGlobal()

// Load only local repository configuration
config, err := gogitcfg.LoadLocal("/path/to/repo")

// Load all configuration sources in precedence order
config, err := gogitcfg.LoadAll("/path/to/repo")

// Load with specific options
config, err := gogitcfg.Load(
    gogitcfg.WithGlobal(),
    gogitcfg.WithLocal(),
    gogitcfg.WithRepoPath("/path/to/repo"),
)
```

### Structured Config Access

```go
// User configuration
user, err := config.GetUser()
fmt.Printf("Name: %s, Email: %s\n", user.Name, user.Email)

// Remote configuration
remote, err := config.GetRemote("origin")
fmt.Printf("URL: %s\n", remote.URL)

// Branch configuration
branch, err := config.GetBranchConfig("main")
fmt.Printf("Remote: %s, Merge: %s\n", branch.Remote, branch.Merge)

// Core configuration
core, err := config.GetCoreConfig()
fmt.Printf("Editor: %s, AutoCRLF: %s\n", core.Editor, core.AutoCRLF)
```

### Working with Sections and Keys

```go
// Check if key exists
if config.Has("user.name") {
    fmt.Println("User name is configured")
}

// Get all sections
sections := config.GetSections()
fmt.Printf("Found sections: %v\n", sections)

// Get all keys in a section
userKeys := config.GetKeysInSection("user")
fmt.Printf("User section keys: %v\n", userKeys)

// Get entire section as map
userSection := config.GetSection("user")
for key, value := range userSection {
    fmt.Printf("%s = %s\n", key, value)
}
```

### With context

```go
import "context"

ctx := context.WithTimeout(context.Background(), 30*time.Second)

// Load with context
config, err := gogitcfg.LoadWithContext(ctx, gogitcfg.WithGlobal())

// Reload with context
err = config.ReloadWithContext(ctx)
```

## Configuration Sources

The library supports all standard Git configuration sources:

- **System**: `/etc/gitconfig` (system-wide)
- **Global**: `~/.gitconfig` (user-specific)
- **Local**: `.git/config` (repository-specific)
- **Worktree**: `.git/config.worktree` (worktree-specific)
