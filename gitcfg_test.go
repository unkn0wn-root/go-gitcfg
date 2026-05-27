package gitcfg

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
)

type customInt int

func TestConfigAccessors(t *testing.T) {
	config := newConfig()
	source := Source{Scope: ScopeGlobal, Path: "/tmp/gitconfig"}

	mustAddEntry(t, config, "user.name", "Test User", source)
	mustAddEntry(t, config, "user.email", "test@example.com", source)
	mustAddEntry(t, config, "core.editor", "vim", source)
	mustAddEntry(t, config, "core.editor", "nvim", source)
	mustAddEntry(t, config, "remote.origin.url", "git@example.com:repo.git", source)

	editor, err := Get[string](config, "core.editor")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if editor != "nvim" {
		t.Fatalf("expected last editor value, got %q", editor)
	}

	values := config.Values("core.editor")
	if len(values) != 2 || values[0] != "vim" || values[1] != "nvim" {
		t.Fatalf("unexpected repeated values: %#v", values)
	}

	if got := GetDefault[string](config, "core.missing", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}

	mustAddEntry(t, config, "http.timeout", "42", source)
	timeout, err := Get[customInt](config, "http.timeout")
	if err != nil {
		t.Fatalf("Get named scalar failed: %v", err)
	}
	if timeout != 42 {
		t.Fatalf("unexpected named scalar value: %v", timeout)
	}

	if !config.Has("user.email") {
		t.Fatal("expected user.email to exist")
	}
	if config.Has("user.missing") {
		t.Fatal("did not expect user.missing to exist")
	}

	user, err := config.User()
	if err != nil {
		t.Fatalf("User failed: %v", err)
	}
	if user.Name != "Test User" || user.Email != "test@example.com" {
		t.Fatalf("unexpected user: %#v", user)
	}

	remote, err := config.RemoteURL("")
	if err != nil {
		t.Fatalf("RemoteURL failed: %v", err)
	}
	if remote != "git@example.com:repo.git" {
		t.Fatalf("unexpected remote URL: %q", remote)
	}
}

func TestConfigSectionsSourcesCloneAndString(t *testing.T) {
	config := newConfig()
	source := Source{Scope: ScopeGlobal, Path: "/tmp/gitconfig"}
	config.addSource(source)
	mustAddEntry(t, config, "user.name", "Test User", source)
	mustAddEntry(t, config, "core.editor", "nvim", source)

	sections := config.Sections()
	if len(sections) != 2 || sections[0] != "core" || sections[1] != "user" {
		t.Fatalf("unexpected sections: %#v", sections)
	}

	user := config.Section("USER")
	if user["name"] != "Test User" {
		t.Fatalf("unexpected user section: %#v", user)
	}
	user["name"] = "mutated"
	if config.Section("user")["name"] != "Test User" {
		t.Fatal("Section returned mutable internal state")
	}

	clone := config.Clone()
	mustAddEntry(t, config, "user.name", "Changed", source)
	value, err := clone.Value("user.name")
	if err != nil {
		t.Fatalf("clone Value failed: %v", err)
	}
	if value != "Test User" {
		t.Fatalf("clone changed with original: %q", value)
	}

	rendered := config.String()
	if !strings.Contains(rendered, "# global: /tmp/gitconfig") ||
		!strings.Contains(rendered, "[core]\n  editor = nvim") ||
		!strings.Contains(rendered, "[user]\n  name = Changed") {
		t.Fatalf("unexpected String output:\n%s", rendered)
	}
}

func TestTypedGetErrors(t *testing.T) {
	config := newConfig()
	source := Source{Scope: ScopeGlobal, Path: "/tmp/gitconfig"}
	mustAddEntry(t, config, "core.autocrlf", "maybe", source)

	_, err := Get[bool](config, "core.autocrlf")
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected ErrInvalidValue, got %v", err)
	}

	_, err = Get[string](config, "invalid")
	if !errors.Is(err, ErrInvalidKeyFormat) {
		t.Fatalf("expected ErrInvalidKeyFormat, got %v", err)
	}

	_, err = Get[string](config, "core.missing")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("expected ErrKeyNotFound, got %v", err)
	}

	_, err = Get[string](config, "user.name")
	if !errors.Is(err, ErrSectionNotFound) {
		t.Fatalf("expected ErrSectionNotFound, got %v", err)
	}
}

func TestConfigErrorPrefix(t *testing.T) {
	err := &ConfigError{Op: "get", Key: "user.name", Err: ErrKeyNotFound}
	if got := err.Error(); !strings.HasPrefix(got, "gitcfg:") {
		t.Fatalf("unexpected error prefix: %q", got)
	}
}

func TestLoadGlobalUsesFixtureEnvironment(t *testing.T) {
	home := t.TempDir()
	global := writeFile(t, home, ".gitconfig", `[user]
	name = Fixture User
`)
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("GIT_CONFIG_GLOBAL", global)

	config, err := LoadGlobal(context.Background())
	if err != nil {
		t.Fatalf("LoadGlobal failed: %v", err)
	}
	name, err := Get[string](config, "user.name")
	if err != nil {
		t.Fatalf("Get user.name failed: %v", err)
	}
	if name != "Fixture User" {
		t.Fatalf("unexpected user.name: %q", name)
	}
}

func TestLoadUsesScopePrecedence(t *testing.T) {
	dir := t.TempDir()
	global := writeFile(t, dir, "global.gitconfig", `[core]
	editor = vim
`)
	repo := dir + "/repo"
	writeFile(t, repo, ".git/config", `[core]
	editor = nvim
`)
	t.Setenv("GIT_CONFIG_GLOBAL", global)

	config, err := Load(
		context.Background(),
		WithScopes(ScopeGlobal, ScopeLocal),
		WithRepoPath(repo),
	)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	assertValue(t, config, "core.editor", "nvim")

	values := config.Values("core.editor")
	if len(values) != 2 || values[0] != "vim" || values[1] != "nvim" {
		t.Fatalf("unexpected precedence values: %#v", values)
	}
}

func TestLoadLocalDoesNotIncludeGlobal(t *testing.T) {
	dir := t.TempDir()
	global := writeFile(t, dir, "global.gitconfig", `[user]
	name = Global User
`)
	repo := dir + "/repo"
	writeFile(t, repo, ".git/config", `[core]
	editor = nvim
`)
	t.Setenv("GIT_CONFIG_GLOBAL", global)

	config, err := LoadLocal(context.Background(), repo)
	if err != nil {
		t.Fatalf("LoadLocal failed: %v", err)
	}
	if config.Has("user.name") {
		t.Fatal("LoadLocal should not include global config")
	}
	assertValue(t, config, "core.editor", "nvim")
}

func TestReloadRefreshesOriginalSources(t *testing.T) {
	dir := t.TempDir()
	global := writeFile(t, dir, "global.gitconfig", `[core]
	editor = vim
`)
	t.Setenv("GIT_CONFIG_GLOBAL", global)

	config, err := LoadGlobal(context.Background())
	if err != nil {
		t.Fatalf("LoadGlobal failed: %v", err)
	}
	assertValue(t, config, "core.editor", "vim")

	if err := os.WriteFile(global, []byte("[core]\neditor = nvim\n"), 0o644); err != nil {
		t.Fatalf("rewrite global config: %v", err)
	}

	if err := config.Reload(context.Background()); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	assertValue(t, config, "core.editor", "nvim")
}

func TestConfigConcurrentReadReload(t *testing.T) {
	dir := t.TempDir()
	global := writeFile(t, dir, "global.gitconfig", `[core]
	editor = vim
`)
	t.Setenv("GIT_CONFIG_GLOBAL", global)

	config, err := LoadGlobal(context.Background())
	if err != nil {
		t.Fatalf("LoadGlobal failed: %v", err)
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					_, _ = config.Value("core.editor")
					_ = config.Values("core.editor")
					_ = config.Sections()
				}
			}
		}()
	}

	for i := range 20 {
		editor := "vim"
		if i%2 == 1 {
			editor = "nvim"
		}
		if err := os.WriteFile(global, []byte("[core]\neditor = "+editor+"\n"), 0o644); err != nil {
			t.Fatalf("rewrite global config: %v", err)
		}
		if err := config.Reload(context.Background()); err != nil {
			t.Fatalf("Reload failed: %v", err)
		}
	}

	close(done)
	wg.Wait()
}

func TestScopeString(t *testing.T) {
	tests := map[Scope]string{
		ScopeSystem:   "system",
		ScopeGlobal:   "global",
		ScopeLocal:    "local",
		ScopeWorktree: "worktree",
		Scope(99):     "unknown",
	}

	for scope, want := range tests {
		if got := scope.String(); got != want {
			t.Fatalf("Scope(%d).String() = %q, want %q", scope, got, want)
		}
	}
}

func mustAddEntry(t *testing.T, config *Config, key, value string, source Source) {
	t.Helper()
	if err := config.addEntry(key, value, source, 1); err != nil {
		t.Fatalf("addEntry(%q) failed: %v", key, err)
	}
}
