package gitcfg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRepoPath(t *testing.T) {
	if _, err := findGitDir(""); err == nil {
		t.Fatal("expected empty path to fail")
	}
	if _, err := findGitDir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected missing path to fail")
	}

	dir := t.TempDir()
	if _, err := findGitDir(dir); err == nil {
		t.Fatal("expected non-repository to fail")
	}

	writeFile(t, dir, ".git/HEAD", "ref: refs/heads/main\n")
	if _, err := findGitDir(dir); err != nil {
		t.Fatalf("expected .git directory repository to pass: %v", err)
	}
}

func TestValidateRepoPathSupportsGitFile(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	gitDir := filepath.Join(dir, "actual.git")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("mkdir git dir: %v", err)
	}
	writeFile(t, repo, ".git", "gitdir: ../actual.git\n")

	got, err := findGitDir(repo)
	if err != nil {
		t.Fatalf("findGitDir failed: %v", err)
	}
	if got != gitDir {
		t.Fatalf("findGitDir = %q, want %q", got, gitDir)
	}
}

func TestGlobalConfigPathsIncludeXDGAndHome(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	xdgPath := writeFile(t, xdg, "git/config", "[core]\n")
	homePath := writeFile(t, home, ".gitconfig", "[user]\n")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("GIT_CONFIG_GLOBAL", "")

	paths := getGlobalConfigPaths()
	if len(paths) != 2 || paths[0] != xdgPath || paths[1] != homePath {
		t.Fatalf("unexpected global paths: %#v", paths)
	}
}

func TestGlobalConfigEnvOverride(t *testing.T) {
	dir := t.TempDir()
	override := writeFile(t, dir, "override.gitconfig", "[user]\n")
	t.Setenv("GIT_CONFIG_GLOBAL", override)

	paths := getGlobalConfigPaths()
	if len(paths) != 1 || paths[0] != override {
		t.Fatalf("unexpected override paths: %#v", paths)
	}
}

func TestConfigSourcesRespectExplicitScopes(t *testing.T) {
	dir := t.TempDir()
	global := writeFile(t, dir, "global.gitconfig", "[user]\n")
	repo := filepath.Join(dir, "repo")
	local := writeFile(t, repo, ".git/config", "[core]\n")
	t.Setenv("GIT_CONFIG_GLOBAL", global)

	sources := getConfigSources(options{
		scopes:   []Scope{ScopeLocal},
		repoPath: repo,
	})
	if len(sources) != 1 {
		t.Fatalf("expected only local source, got %#v", sources)
	}
	if sources[0].Scope != ScopeLocal || sources[0].Path != local {
		t.Fatalf("unexpected source: %#v", sources[0])
	}
}

func TestSystemConfigEnv(t *testing.T) {
	dir := t.TempDir()
	system := writeFile(t, dir, "system.gitconfig", "[core]\n")
	t.Setenv("GIT_CONFIG_SYSTEM", system)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "")

	paths := getSystemConfigPaths()
	if len(paths) != 1 || paths[0] != system {
		t.Fatalf("unexpected system paths: %#v", paths)
	}

	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	if paths := getSystemConfigPaths(); len(paths) != 0 {
		t.Fatalf("expected no system paths, got %#v", paths)
	}
}

func writeFile(t *testing.T, dir, rel, content string) string {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
