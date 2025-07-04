package gogitcfg

import (
	"context"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	config, err := Load(WithGlobal())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if config == nil {
		t.Fatal("Config is nil")
	}
}

func TestLoadWithContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	config, err := LoadWithContext(ctx, WithGlobal())
	if err != nil {
		t.Fatalf("LoadWithContext failed: %v", err)
	}
	if config == nil {
		t.Fatal("Config is nil")
	}
}

func TestLoadGlobal(t *testing.T) {
	config, err := LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal failed: %v", err)
	}
	if config == nil {
		t.Fatal("Config is nil")
	}
}

func TestConfigGet(t *testing.T) {
	config := &Config{
		sections: map[string]map[string]string{
			"test": {"key": "value"},
		},
	}

	value, err := Get[string](config, "test.key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if value != "value" {
		t.Errorf("Expected 'value', got '%s'", value)
	}
}

func TestConfigGetWithDefault(t *testing.T) {
	config := &Config{
		sections: map[string]map[string]string{},
	}

	value := GetWithDefault[string](config, "nonexistent.key", "default")
	if value != "default" {
		t.Errorf("Expected 'default', got '%s'", value)
	}
}

func TestConfigHas(t *testing.T) {
	config := &Config{
		sections: map[string]map[string]string{
			"test": {"key": "value"},
		},
	}

	if !config.Has("test.key") {
		t.Error("Expected key to exist")
	}
	if config.Has("nonexistent.key") {
		t.Error("Expected key to not exist")
	}
}

func TestConfigGetSection(t *testing.T) {
	config := &Config{
		sections: map[string]map[string]string{
			"test": {"key1": "value1", "key2": "value2"},
		},
	}

	section := config.GetSection("test")
	if len(section) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(section))
	}
	if section["key1"] != "value1" {
		t.Errorf("Expected 'value1', got '%s'", section["key1"])
	}
}

func TestConfigGetSections(t *testing.T) {
	config := &Config{
		sections: map[string]map[string]string{
			"test1": {"key": "value"},
			"test2": {"key": "value"},
		},
	}

	sections := config.GetSections()
	if len(sections) != 2 {
		t.Errorf("Expected 2 sections, got %d", len(sections))
	}
}

func TestConfigString(t *testing.T) {
	config := &Config{
		sections: map[string]map[string]string{
			"test": {"key": "value"},
		},
	}

	str := config.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestConfigClone(t *testing.T) {
	config := &Config{
		sections: map[string]map[string]string{
			"test": {"key": "value"},
		},
	}

	clone := config.Clone()
	if clone == nil {
		t.Fatal("Clone is nil")
	}

	// Modify original
	config.sections["test"]["key"] = "modified"

	// Clone should be unchanged
	if clone.sections["test"]["key"] != "value" {
		t.Error("Clone was modified when original changed")
	}
}


func TestConfigGetUser(t *testing.T) {
	config := &Config{
		sections: map[string]map[string]string{
			"user": {"name": "Test User", "email": "test@example.com"},
		},
	}

	user, err := config.GetUser()
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if user.Name != "Test User" {
		t.Errorf("Expected 'Test User', got '%s'", user.Name)
	}
	if user.Email != "test@example.com" {
		t.Errorf("Expected 'test@example.com', got '%s'", user.Email)
	}
}

func TestConfigError(t *testing.T) {
	err := &ConfigError{
		Op:      "test",
		Key:     "test.key",
		Section: "test",
		Source:  "test.config",
		Err:     ErrKeyNotFound,
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("Expected non-empty error string")
	}
}

func TestConfigSourceType(t *testing.T) {
	tests := []struct {
		sourceType ConfigSourceType
		expected   string
	}{
		{SourceTypeSystem, "system"},
		{SourceTypeGlobal, "global"},
		{SourceTypeLocal, "local"},
		{SourceTypeWorktree, "worktree"},
	}

	for _, test := range tests {
		if test.sourceType.String() != test.expected {
			t.Errorf("Expected '%s', got '%s'", test.expected, test.sourceType.String())
		}
	}
}
