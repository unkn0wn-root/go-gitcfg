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
// Generic type-safe access
name, err := gogitcfg.Get[string](config, "user.name")
editor, err := gogitcfg.Get[string](config, "core.editor")
autocrlf, err := gogitcfg.Get[bool](config, "core.autocrlf")
filemode, err := gogitcfg.Get[bool](config, "core.filemode")
timeout, err := gogitcfg.Get[int](config, "http.timeout")

// With default values
timeout := gogitcfg.GetWithDefault[int](config, "http.timeout", 30)
editor := gogitcfg.GetWithDefault[string](config, "core.editor", "vim")
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

// Remote URL (simplified access)
remoteURL, err := config.GetRemoteURL("origin")
fmt.Printf("URL: %s\n", remoteURL)

// Direct section access for complex configurations
remoteSection := config.GetSection("remote.origin")
for key, value := range remoteSection {
    fmt.Printf("%s = %s\n", key, value)
}
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

// Get entire section as map
userSection := config.GetSection("user")
for key, value := range userSection {
    fmt.Printf("%s = %s\n", key, value)
}

// Check if section exists
if config.HasSection("user") {
    fmt.Println("User section exists")
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
