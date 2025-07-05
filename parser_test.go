package gitcfg

import (
	"context"
	"strings"
	"testing"
)

func TestNewParser(t *testing.T) {
	parser := newParser()
	if parser == nil {
		t.Fatal("Parser is nil")
	}
	if parser.sectionRegex == nil {
		t.Error("Section regex is nil")
	}
	if parser.keyValueRegex == nil {
		t.Error("Key-value regex is nil")
	}
}

func TestParseConfigReader(t *testing.T) {
	parser := newParser()
	config := &Config{
		sections: make(map[string]map[string]string),
	}

	configData := `[user]
    name = Test User
    email = test@example.com

[core]
    editor = vim
    autocrlf = true
`

	reader := strings.NewReader(configData)
	err := parser.parseConfigReader(reader, config, "test")
	if err != nil {
		t.Fatalf("parseConfigReader failed: %v", err)
	}

	if config.sections["user"]["name"] != "Test User" {
		t.Errorf("Expected 'Test User', got '%s'", config.sections["user"]["name"])
	}
	if config.sections["user"]["email"] != "test@example.com" {
		t.Errorf("Expected 'test@example.com', got '%s'", config.sections["user"]["email"])
	}
	if config.sections["core"]["editor"] != "vim" {
		t.Errorf("Expected 'vim', got '%s'", config.sections["core"]["editor"])
	}
}

func TestParseGitConfigLine(t *testing.T) {
	parser := newParser()

	tests := []struct {
		line           string
		expectedKey    string
		expectedValue  string
		expectedSource string
	}{
		{"file:/home/user/.gitconfig\tuser.name=Test User", "user.name", "Test User", "/home/user/.gitconfig"},
		{"file:/etc/gitconfig\tcore.editor=vim", "core.editor", "vim", "/etc/gitconfig"},
		{"user.email=test@example.com", "user.email", "test@example.com", ""},
	}

	for _, test := range tests {
		key, value, source := parser.parseGitConfigLine(test.line)
		if key != test.expectedKey {
			t.Errorf("Expected key '%s', got '%s'", test.expectedKey, key)
		}
		if value != test.expectedValue {
			t.Errorf("Expected value '%s', got '%s'", test.expectedValue, value)
		}
		if source != test.expectedSource {
			t.Errorf("Expected source '%s', got '%s'", test.expectedSource, source)
		}
	}
}

func TestProcessQuotedValue(t *testing.T) {
	parser := newParser()

	tests := []struct {
		input    string
		expected string
	}{
		{`"quoted value"`, "quoted value"},
		{`unquoted value`, "unquoted value"},
		{`"value with spaces"`, "value with spaces"},
		{`"value with \"quotes\""`, `value with "quotes"`},
	}

	for _, test := range tests {
		result, err := parser.processQuotedValue(test.input)
		if err != nil {
			t.Errorf("processQuotedValue failed for '%s': %v", test.input, err)
		}
		if result != test.expected {
			t.Errorf("Expected '%s', got '%s'", test.expected, result)
		}
	}
}

func TestBuildFullKey(t *testing.T) {
	parser := newParser()

	tests := []struct {
		section  string
		key      string
		expected string
	}{
		{"user", "name", "user.name"},
		{"core", "editor", "core.editor"},
		{"", "key", "key"},
		{"remote \"origin\"", "url", "remote.origin.url"},
		{"branch \"main\"", "remote", "branch.main.remote"},
		{"submodule \"path/to/sub\"", "url", "submodule.path/to/sub.url"},
	}

	for _, test := range tests {
		result := parser.buildFullKey(test.section, test.key)
		if result != test.expected {
			t.Errorf("Expected '%s', got '%s'", test.expected, result)
		}
	}
}

func TestIsValidConfigKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"user.name", true},
		{"core.editor", true},
		{"remote.origin.url", true},
		{"invalid", false},
		{"", false},
		{"user.name.extra", true},
	}

	for _, test := range tests {
		result := isValidConfigKey(test.key)
		if result != test.expected {
			t.Errorf("Expected %v for key '%s', got %v", test.expected, test.key, result)
		}
	}
}

func TestIsValidSectionName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"user", true},
		{"core", true},
		{"remote", true},
		{"", false},
		{"user-section", true},
		{"user_section", true},
		{"123invalid", true},
		{"remote \"origin\"", true},
	}

	for _, test := range tests {
		result := isValidSectionName(test.name)
		if result != test.expected {
			t.Errorf("Expected %v for section '%s', got %v", test.expected, test.name, result)
		}
	}
}

func TestIsValidKeyName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"name", true},
		{"editor", true},
		{"url", true},
		{"", false},
		{"key-name", true},
		{"key_name", true},
		{"123key", true},
	}

	for _, test := range tests {
		result := isValidKeyName(test.name)
		if result != test.expected {
			t.Errorf("Expected %v for key '%s', got %v", test.expected, test.name, result)
		}
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
		hasError bool
	}{
		{"true", true, false},
		{"false", false, false},
		{"yes", true, false},
		{"no", false, false},
		{"on", true, false},
		{"off", false, false},
		{"1", true, false},
		{"0", false, false},
		{"", true, false}, // Git treats empty as true
		{"invalid", false, true},
	}

	for _, test := range tests {
		result, err := parseBool(test.value)
		if test.hasError {
			if err == nil {
				t.Errorf("Expected error for value '%s'", test.value)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for value '%s': %v", test.value, err)
			}
			if result != test.expected {
				t.Errorf("Expected %v for value '%s', got %v", test.expected, test.value, result)
			}
		}
	}
}

func TestConvertValue(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		result, err := convertValue[string]("test")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if result != "test" {
			t.Errorf("Expected 'test', got '%s'", result)
		}
	})

	t.Run("int", func(t *testing.T) {
		result, err := convertValue[int]("42")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if result != 42 {
			t.Errorf("Expected 42, got %d", result)
		}
	})

	t.Run("bool", func(t *testing.T) {
		result, err := convertValue[bool]("true")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if result != true {
			t.Errorf("Expected true, got %v", result)
		}
	})

	t.Run("float64", func(t *testing.T) {
		result, err := convertValue[float64]("3.14")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if result != 3.14 {
			t.Errorf("Expected 3.14, got %f", result)
		}
	})
}

func TestParseFromGitCommand(t *testing.T) {
	parser := newParser()
	opts := &configOptions{
		includeGlobal: true,
		timeout:       DefaultTimeout,
	}

	ctx := context.Background()
	config, err := parser.parseFromGitCommand(ctx, opts)

	// This test may fail if git is not available or configured
	if err != nil {
		t.Logf("parseFromGitCommand failed (expected if git not available): %v", err)
		return
	}

	if config == nil {
		t.Error("Config is nil")
	}
}

func TestSubsectionParsing(t *testing.T) {
	parser := newParser()
	config := &Config{
		sections: make(map[string]map[string]string),
	}

	// Test config with subsections
	configData := `[user]
    name = Test User
    email = test@example.com

[core]
    editor = vim
    autocrlf = true

[remote "origin"]
    url = https://github.com/example/repo.git
    fetch = +refs/heads/*:refs/remotes/origin/*

[remote "upstream"]
    url = https://github.com/upstream/repo.git
    fetch = +refs/heads/*:refs/remotes/upstream/*

[branch "main"]
    remote = origin
    merge = refs/heads/main
`

	reader := strings.NewReader(configData)
	err := parser.parseConfigReader(reader, config, "test")
	if err != nil {
		t.Fatalf("parseConfigReader failed: %v", err)
	}

	// Test that subsections are parsed correctly
	expectedSections := []string{"user", "core", "remote.origin", "remote.upstream", "branch.main"}
	for _, expectedSection := range expectedSections {
		if _, exists := config.sections[expectedSection]; !exists {
			t.Errorf("Expected section '%s' not found", expectedSection)
		}
	}

	// Test accessing subsection values
	if config.sections["remote.origin"]["url"] != "https://github.com/example/repo.git" {
		t.Errorf("Expected origin URL, got '%s'", config.sections["remote.origin"]["url"])
	}

	if config.sections["remote.upstream"]["url"] != "https://github.com/upstream/repo.git" {
		t.Errorf("Expected upstream URL, got '%s'", config.sections["remote.upstream"]["url"])
	}

	if config.sections["branch.main"]["remote"] != "origin" {
		t.Errorf("Expected branch main remote 'origin', got '%s'", config.sections["branch.main"]["remote"])
	}
}
