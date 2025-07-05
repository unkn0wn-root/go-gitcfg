package gitcfg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRepoPath(t *testing.T) {
	err := validateRepoPath("")
	if err == nil {
		t.Error("Expected error for empty path")
	}

	err = validateRepoPath("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for non-existent path")
	}

	tempDir, err := os.MkdirTemp("", "test-repo")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	err = validateRepoPath(tempDir)
	if err == nil {
		t.Error("Expected error for non-git directory")
	}

	gitDir := filepath.Join(tempDir, ".git")
	err = os.Mkdir(gitDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	err = validateRepoPath(tempDir)
	if err != nil {
		t.Errorf("Unexpected error for valid git repo: %v", err)
	}
}

func TestGetSystemConfigPath(t *testing.T) {
	path := getSystemConfigPath()
	t.Logf("System config path: %s", path)
}

func TestGetGlobalConfigPath(t *testing.T) {
	path := getGlobalConfigPath()
	t.Logf("Global config path: %s", path)
}

func TestGetLocalConfigPath(t *testing.T) {
	path := getLocalConfigPath("")
	if path != "" {
		t.Errorf("Expected empty path, got '%s'", path)
	}

	path = getLocalConfigPath("/nonexistent/path")
	if path != "" {
		t.Errorf("Expected empty path, got '%s'", path)
	}

	tempDir, err := os.MkdirTemp("", "test-repo")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	gitDir := filepath.Join(tempDir, ".git")
	err = os.Mkdir(gitDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	configPath := filepath.Join(gitDir, "config")
	err = os.WriteFile(configPath, []byte("[test]\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	path = getLocalConfigPath(tempDir)
	if path != configPath {
		t.Errorf("Expected '%s', got '%s'", configPath, path)
	}
}

func TestGetWorktreeConfigPath(t *testing.T) {
	path := getWorktreeConfigPath("")
	if path != "" {
		t.Errorf("Expected empty path, got '%s'", path)
	}

	path = getWorktreeConfigPath("/nonexistent/path")
	if path != "" {
		t.Errorf("Expected empty path, got '%s'", path)
	}

	tempDir, err := os.MkdirTemp("", "test-repo")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	gitDir := filepath.Join(tempDir, ".git")
	err = os.Mkdir(gitDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	configPath := filepath.Join(gitDir, "config.worktree")
	err = os.WriteFile(configPath, []byte("[test]\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	path = getWorktreeConfigPath(tempDir)
	if path != configPath {
		t.Errorf("Expected '%s', got '%s'", configPath, path)
	}
}

func TestGetAllConfigPaths(t *testing.T) {
	opts := &configOptions{
		includeGlobal: true,
	}

	sources := getAllConfigPaths(opts)
	t.Logf("Found %d config sources", len(sources))

	for _, source := range sources {
		t.Logf("Source: %s - %s", source.Type, source.Path)
	}
}

func TestGetXDGConfigPath(t *testing.T) {
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if originalXDG != "" {
			os.Setenv("XDG_CONFIG_HOME", originalXDG)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	tempDir, err := os.MkdirTemp("", "test-xdg")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	xdgDir := filepath.Join(tempDir, "git")
	err = os.MkdirAll(xdgDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create XDG git dir: %v", err)
	}

	configPath := filepath.Join(xdgDir, "config")
	err = os.WriteFile(configPath, []byte("[test]\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	os.Setenv("XDG_CONFIG_HOME", tempDir)

	path := getXDGConfigPath()
	if path != configPath {
		t.Errorf("Expected '%s', got '%s'", configPath, path)
	}
}

func TestGetHomeConfigPath(t *testing.T) {
	path := getHomeConfigPath()
	t.Logf("Home config path: %s", path)
}

func TestParseSystemConfigPath(t *testing.T) {
	tests := []struct {
		output   string
		expected string
	}{
		{"file:/etc/gitconfig\tcore.editor=vim", "/etc/gitconfig"},
		{"file:/usr/local/etc/gitconfig\tuser.name=test", "/usr/local/etc/gitconfig"},
		{"no file prefix", ""},
		{"", ""},
	}

	for _, test := range tests {
		result := parseSystemConfigPath(test.output)
		if result != test.expected {
			t.Errorf("Expected '%s', got '%s'", test.expected, result)
		}
	}
}

func TestGetSystemConfigPathFallback(t *testing.T) {
	path := getSystemConfigPathFallback()
	t.Logf("System config fallback path: %s", path)
}
