package gitcfg

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfigReaderSupportsGitSyntax(t *testing.T) {
	data := `[user]
	name = Test User
	email = "test@example.com" # inline comment

[core]
	editor = vim
	filemode
	long = first\
second

[remote "origin"] ; section comment
	url = git@example.com:repo.git
	fetch = +refs/heads/*:refs/remotes/origin/*

[includeIf "gitdir:~/Git/abilia/"]
	path = ~/.gitconfig-abilia

[url "git@gitlab.com:"]
	insteadOf = https://gitlab.com/
`

	config := newConfig()
	parser := newParser(parserOptions{})
	err := parser.parseConfigReader(
		context.Background(),
		strings.NewReader(data),
		&parseFile{
			cfg:        config,
			source:     Source{Scope: ScopeGlobal, Path: "/tmp/gitconfig"},
			sourcePath: "/tmp/gitconfig",
			state:      newParseState(),
		},
	)
	if err != nil {
		t.Fatalf("parseConfigReader failed: %v", err)
	}

	assertValue(t, config, "user.email", "test@example.com")
	assertValue(t, config, "core.long", "firstsecond")
	assertValue(t, config, "remote.origin.url", "git@example.com:repo.git")
	assertValue(t, config, "includeIf.gitdir:~/Git/abilia/.path", "~/.gitconfig-abilia")
	assertValue(t, config, "url.git@gitlab.com:.insteadOf", "https://gitlab.com/")

	filemode, err := Get[bool](config, "core.filemode")
	if err != nil {
		t.Fatalf("Get core.filemode failed: %v", err)
	}
	if !filemode {
		t.Fatal("expected bare bool key to parse as true")
	}
}

func TestLoadFollowsIncludes(t *testing.T) {
	dir := t.TempDir()
	included := writeFile(t, dir, "included.gitconfig", `[user]
	name = Included User
`)
	main := writeFile(t, dir, "main.gitconfig", `[include]
	path = included.gitconfig
`)
	t.Setenv("GIT_CONFIG_GLOBAL", main)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", "")

	config, err := Load(context.Background(), WithScopes(ScopeGlobal))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	assertValue(t, config, "user.name", "Included User")

	sources := config.Sources()
	if len(sources) != 2 || sources[0].Path != main || sources[1].Path != included {
		t.Fatalf("unexpected sources: %#v", sources)
	}
}

func TestLoadIgnoresMissingInclude(t *testing.T) {
	dir := t.TempDir()
	main := writeFile(t, dir, "main.gitconfig", `[include]
	path = missing.gitconfig

[user]
	name = Present User
`)
	t.Setenv("GIT_CONFIG_GLOBAL", main)

	config, err := Load(context.Background(), WithScopes(ScopeGlobal))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	assertValue(t, config, "user.name", "Present User")
}

func TestLoadCanDisableIncludes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "included.gitconfig", `[user]
	name = Included User
`)
	main := writeFile(t, dir, "main.gitconfig", `[include]
	path = included.gitconfig
`)
	t.Setenv("GIT_CONFIG_GLOBAL", main)

	config, err := Load(context.Background(), WithScopes(ScopeGlobal), WithIncludes(false))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if config.Has("user.name") {
		t.Fatal("did not expect include to be followed")
	}
	assertValue(t, config, "include.path", "included.gitconfig")
}

func TestLoadFollowsGitdirConditionalInclude(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	writeFile(t, repo, ".git/HEAD", "ref: refs/heads/main\n")

	writeFile(t, dir, "conditional.gitconfig", `[user]
email = repo@example.com
`)
	main := writeFile(t, dir, "main.gitconfig", `[includeIf "gitdir:`+filepath.ToSlash(repo)+`/"]
	path = conditional.gitconfig
`)
	t.Setenv("GIT_CONFIG_GLOBAL", main)

	config, err := Load(context.Background(), WithScopes(ScopeGlobal), WithRepoPath(repo))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	assertValue(t, config, "user.email", "repo@example.com")
}

func TestGitdirConditionalIncludeSupportsBracketGlob(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "Work")
	writeFile(t, repo, ".git/HEAD", "ref: refs/heads/main\n")

	writeFile(t, dir, "conditional.gitconfig", `[user]
email = work@example.com
`)
	main := writeFile(
		t,
		dir,
		"main.gitconfig",
		`[includeIf "gitdir:`+filepath.ToSlash(filepath.Join(dir, "[wW]ork"))+`/"]
	path = conditional.gitconfig
`,
	)
	t.Setenv("GIT_CONFIG_GLOBAL", main)

	config, err := Load(context.Background(), WithScopes(ScopeGlobal), WithRepoPath(repo))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	assertValue(t, config, "user.email", "work@example.com")
}

func TestLoadFollowsOnBranchConditionalInclude(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	writeFile(t, repo, ".git/HEAD", "ref: refs/heads/main\n")

	conditional := writeFile(t, dir, "branch.gitconfig", `[core]
	editor = nvim
`)
	main := writeFile(t, dir, "main.gitconfig", `[includeIf "onbranch:main"]
	path = `+conditional+`
`)
	t.Setenv("GIT_CONFIG_GLOBAL", main)

	config, err := Load(context.Background(), WithScopes(ScopeGlobal), WithRepoPath(repo))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	assertValue(t, config, "core.editor", "nvim")
}

func TestLoadWithGitCommand(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git command not available")
	}

	dir := t.TempDir()
	global := writeFile(t, dir, "global.gitconfig", `[user]
	name = Git Command User
`)
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	t.Setenv("HOME", dir)

	config, err := Load(context.Background(), WithScopes(ScopeGlobal), WithGitCommand())
	if err != nil {
		t.Fatalf("Load with git command failed: %v", err)
	}
	assertValue(t, config, "user.name", "Git Command User")
}

func TestReloadPreservesGitCommandLoader(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git command not available")
	}

	dir := t.TempDir()
	writeFile(t, dir, "global.gitconfig", `[user]
	name = Before Reload
`)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "global.gitconfig"))
	t.Setenv("HOME", dir)

	config, err := Load(context.Background(), WithScopes(ScopeGlobal), WithGitCommand())
	if err != nil {
		t.Fatalf("Load with git command failed: %v", err)
	}
	if !config.snapshot().useGit {
		t.Fatal("expected config to preserve git-command loader mode")
	}

	writeFile(t, dir, "global.gitconfig", `[user]
	name = After Reload
`)
	if err := config.Reload(context.Background()); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	if !config.snapshot().useGit {
		t.Fatal("Reload should preserve git-command loader mode")
	}
	assertValue(t, config, "user.name", "After Reload")
}

func TestIncludeCycleReturnsError(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.gitconfig", `[include]
	path = b.gitconfig
`)
	writeFile(t, dir, "b.gitconfig", `[include]
	path = a.gitconfig
`)
	t.Setenv("GIT_CONFIG_GLOBAL", a)

	_, err := Load(context.Background(), WithScopes(ScopeGlobal))
	if !errors.Is(err, ErrIncludeCycle) {
		t.Fatalf("expected ErrIncludeCycle, got %v", err)
	}
}

func TestParseMalformedConfig(t *testing.T) {
	parser := newParser(parserOptions{})
	err := parser.parseConfigReader(
		context.Background(),
		strings.NewReader("[core\neditor = vim\n"),
		&parseFile{
			cfg:        newConfig(),
			source:     Source{Scope: ScopeGlobal, Path: "/tmp/gitconfig"},
			sourcePath: "/tmp/gitconfig",
			state:      newParseState(),
		},
	)
	if !errors.Is(err, ErrInvalidKeyFormat) {
		t.Fatalf("expected ErrInvalidKeyFormat, got %v", err)
	}
}

func assertValue(t *testing.T, config *Config, key, want string) {
	t.Helper()
	got, err := config.Value(key)
	if err != nil {
		t.Fatalf("Value(%q) failed: %v", key, err)
	}
	if got != want {
		t.Fatalf("Value(%q) = %q, want %q", key, got, want)
	}
}
