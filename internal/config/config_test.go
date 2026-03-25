package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// 创建一个临时配置文件
	content := `
server:
  port: 8081
  host: "localhost"

auth:
  token: "test-token"

audit:
  log_file: "./test.log"
  level: "debug"
  format: "json"

execution:
  max_output_length: 5000
  timeout_seconds: 60
  max_concurrent: 5

rules:
  verb_allowlist:
    - "get"
    - "describe"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// 加载配置
	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// 验证配置
	if cfg.Server.Port != 8081 {
		t.Errorf("Expected port 8081, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "localhost" {
		t.Errorf("Expected host localhost, got %s", cfg.Server.Host)
	}
	if cfg.Auth.Token != "test-token" {
		t.Errorf("Expected token 'test-token', got '%s'", cfg.Auth.Token)
	}
	if cfg.Execution.MaxOutputLength != 5000 {
		t.Errorf("Expected max_output_length 5000, got %d", cfg.Execution.MaxOutputLength)
	}
	if len(cfg.Rules.VerbAllowlist) != 2 {
		t.Errorf("Expected 2 verbs in allowlist, got %d", len(cfg.Rules.VerbAllowlist))
	}
}

func TestGetDefaultConfig(t *testing.T) {
	cfg := GetDefaultConfig()
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Execution.MaxOutputLength != 10000 {
		t.Errorf("Expected default max_output_length 10000, got %d", cfg.Execution.MaxOutputLength)
	}
}
