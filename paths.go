package gitcfg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// SystemConfigFile is the default system configuration path.
	SystemConfigFile = "/etc/gitconfig"
	// GlobalConfigFile is the default home relative global configuration file.
	GlobalConfigFile = ".gitconfig"
	// LocalConfigFile is the repository gitdir configuration file.
	LocalConfigFile = "config"
	// WorktreeConfigFile is the repository gitdir worktree configuration file.
	WorktreeConfigFile = "config.worktree"
	// XDGConfigFile is the home-relative XDG global configuration file.
	XDGConfigFile = ".config/git/config"
)

func getConfigSources(opts options) []Source {
	var srcs []Source
	for _, s := range opts.scopes {
		switch s {
		case ScopeSystem:
			for _, p := range getSystemConfigPaths() {
				srcs = append(srcs, Source{Scope: ScopeSystem, Path: p})
			}
		case ScopeGlobal:
			for _, p := range getGlobalConfigPaths() {
				srcs = append(srcs, Source{Scope: ScopeGlobal, Path: p})
			}
		case ScopeLocal:
			if p := getLocalConfigPath(opts.repoPath); p != "" {
				srcs = append(srcs, Source{Scope: ScopeLocal, Path: p})
			}
		case ScopeWorktree:
			if p := getWorktreeConfigPath(opts.repoPath); p != "" {
				srcs = append(srcs, Source{Scope: ScopeWorktree, Path: p})
			}
		}
	}
	return srcs
}

func getSystemConfigPaths() []string {
	if os.Getenv("GIT_CONFIG_NOSYSTEM") != "" {
		return nil
	}
	if p := os.Getenv("GIT_CONFIG_SYSTEM"); p != "" {
		if fileExists(p) {
			return []string{p}
		}
		return nil
	}

	var ps []string
	for _, p := range []string{SystemConfigFile, "/usr/local/etc/gitconfig"} {
		if fileExists(p) {
			ps = append(ps, p)
		}
	}
	return ps
}

func getGlobalConfigPaths() []string {
	if p := os.Getenv("GIT_CONFIG_GLOBAL"); p != "" {
		if fileExists(p) {
			return []string{p}
		}
		return nil
	}

	var ps []string
	if p := getXDGConfigPath(); p != "" {
		ps = append(ps, p)
	}
	if p := getHomeConfigPath(); p != "" {
		ps = append(ps, p)
	}
	return ps
}

func getXDGConfigPath() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		p := filepath.Join(dir, "git", "config")
		if fileExists(p) {
			return p
		}
		return ""
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	p := filepath.Join(home, XDGConfigFile)
	if fileExists(p) {
		return p
	}
	return ""
}

func getHomeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	p := filepath.Join(home, GlobalConfigFile)
	if fileExists(p) {
		return p
	}
	return ""
}

func getLocalConfigPath(repoPath string) string {
	dir, err := findGitDir(repoPath)
	if err != nil {
		return ""
	}
	p := filepath.Join(dir, LocalConfigFile)
	if fileExists(p) {
		return p
	}
	return ""
}

func getWorktreeConfigPath(repoPath string) string {
	dir, err := findGitDir(repoPath)
	if err != nil {
		return ""
	}
	p := filepath.Join(dir, WorktreeConfigFile)
	if fileExists(p) {
		return p
	}
	return ""
}

func findGitDir(repoPath string) (string, error) {
	if strings.TrimSpace(repoPath) == "" {
		return "", errors.New("empty path")
	}

	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return "", err
	}

	fi, err := os.Stat(repoPath)
	if err != nil {
		return "", fmt.Errorf("path does not exist: %w", err)
	}
	if !fi.IsDir() {
		return "", errors.New("path is not a directory")
	}

	gitPath := filepath.Join(repoPath, ".git")
	fi, err = os.Stat(gitPath)
	if err != nil {
		return "", errors.New("not a Git repository")
	}
	if fi.IsDir() {
		return gitPath, nil
	}

	b, err := os.ReadFile(gitPath)
	if err != nil {
		return "", fmt.Errorf("read .git file: %w", err)
	}

	s := strings.TrimSpace(string(b))
	dir, ok := strings.CutPrefix(s, "gitdir:")
	if !ok {
		return "", errors.New("invalid .git file")
	}

	dir = strings.TrimSpace(dir)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(repoPath, dir)
	}

	dir = filepath.Clean(dir)
	fi, err = os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("gitdir does not exist: %w", err)
	}
	if !fi.IsDir() {
		return "", errors.New("gitdir is not a directory")
	}
	return dir, nil
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}
