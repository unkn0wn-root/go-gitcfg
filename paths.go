package gogitcfg

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func validateRepoPath(path string) error {
	if path == "" {
		return errors.New("empty path")
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}

	if !info.IsDir() {
		return errors.New("path is not a directory")
	}

	gitDir := filepath.Join(path, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return errors.New("not a Git repository")
	}

	return nil
}

func getSystemConfigPath() string {
	// Try to get from git config --system --list first
	if path := getSystemConfigPathFromGit(); path != "" {
		return path
	}

	return getSystemConfigPathFallback()
}

func getSystemConfigPathFromGit() string {
    cmd := exec.Command("git", "config", "--system", "--show-origin", "--list")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	return parseSystemConfigPath(string(output))
}

func parseSystemConfigPath(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "file:") {
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) > 0 {
				return strings.TrimPrefix(parts[0], "file:")
			}
		}
	}
	return ""
}

func getSystemConfigPathFallback() string {
	paths := []string{
		SystemConfigFile,
		"/usr/local/etc/gitconfig",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func getGlobalConfigPath() string {
	if path := getXDGConfigPath(); path != "" {
		return path
	}

	if path := getHomeConfigPath(); path != "" {
		return path
	}

	return ""
}

func getXDGConfigPath() string {
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig != "" {
		xdgPath := filepath.Join(xdgConfig, "git", "config")
		if _, err := os.Stat(xdgPath); err == nil {
			return xdgPath
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		xdgPath := filepath.Join(home, XDGConfigDir)
		if _, err := os.Stat(xdgPath); err == nil {
			return xdgPath
		}
	}

	return ""
}

func getHomeConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	globalPath := filepath.Join(home, GlobalConfigFile)
	if _, err := os.Stat(globalPath); err == nil {
		return globalPath
	}

	return ""
}

func getLocalConfigPath(repoPath string) string {
	if repoPath == "" {
		return ""
	}

	localPath := filepath.Join(repoPath, LocalConfigFile)
	if _, err := os.Stat(localPath); err == nil {
		return localPath
	}

	return ""
}

func getWorktreeConfigPath(repoPath string) string {
	if repoPath == "" {
		return ""
	}

	worktreePath := filepath.Join(repoPath, WorktreeConfigFile)
	if _, err := os.Stat(worktreePath); err == nil {
		return worktreePath
	}

	return ""
}

func getAllConfigPaths(opts *configOptions) []ConfigSource {
	var sources []ConfigSource

	if opts.includeSystem {
		if path := getSystemConfigPath(); path != "" {
			sources = append(sources, ConfigSource{
				Type: SourceTypeSystem,
				Path: path,
			})
		}
	}

	if opts.includeGlobal {
		if path := getGlobalConfigPath(); path != "" {
			sources = append(sources, ConfigSource{
				Type: SourceTypeGlobal,
				Path: path,
			})
		}
	}

	if opts.includeLocal && opts.repoPath != "" {
		if path := getLocalConfigPath(opts.repoPath); path != "" {
			sources = append(sources, ConfigSource{
				Type: SourceTypeLocal,
				Path: path,
			})
		}
	}

	if opts.includeWorktree && opts.repoPath != "" {
		if path := getWorktreeConfigPath(opts.repoPath); path != "" {
			sources = append(sources, ConfigSource{
				Type: SourceTypeWorktree,
				Path: path,
			})
		}
	}

	return sources
}
