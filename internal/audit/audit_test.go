package audit

import (
	"os"
	"strings"
	"testing"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/config"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/model"
)

func TestAuditLog(t *testing.T) {
	tempLog, _ := os.CreateTemp("", "audit-*.log")
	defer os.Remove(tempLog.Name())

	cfg := config.AuditConfig{
		Level:     "debug",
		Format:    "json",
		AuditFile: tempLog.Name(),
	}

	err := Init(cfg)
	if err != nil {
		t.Fatalf("Failed to initialize audit: %v", err)
	}

	err = LogRequest(&model.ExecutionRequest{
		Verb:      "get",
		Resource:  "pods",
		Name:      "my-pod",
		Namespace: "default",
	}, "test-agent", "127.0.0.1", "success", 100, 1024, "")

	if err != nil {
		t.Fatalf("Failed to log request: %v", err)
	}

	Close()

	content, _ := os.ReadFile(tempLog.Name())
	if len(content) == 0 {
		t.Error("Audit log file is empty")
	}

	if !strings.Contains(string(content), "my-pod") {
		t.Error("Audit log should contain 'my-pod'")
	}
}

func TestInit_InvalidLevel(t *testing.T) {
	cfg := config.AuditConfig{
		Level: "invalid-level",
	}
	err := Init(cfg)
	if err != nil {
		t.Errorf("Init should not fail on invalid level, expected default, got: %v", err)
	}
}

func TestInit_LogFileDirectoryError(t *testing.T) {
	// 使用一个无法创建目录的路径
	cfg := config.AuditConfig{
		LogFile: "C:/Windows/System32/invalid_log_dir/test.log", // 通常无法在这里创建目录
	}
	err := Init(cfg)
	if err == nil {
		t.Error("Init should fail when LogFile directory cannot be created")
	}
}

func TestInit_AuditFileDirectoryError(t *testing.T) {
	// 使用一个无法创建目录的路径
	cfg := config.AuditConfig{
		AuditFile: "C:/Windows/System32/invalid_audit_dir/test.log",
	}
	err := Init(cfg)
	if err == nil {
		t.Error("Init should fail when AuditFile directory cannot be created")
	}
}

func TestLog_BeforeInit(t *testing.T) {
	// 重置全局变量
	generalLogger = nil
	auditLogger = nil

	err := Log(model.AuditLogEntry{
		Status: "success",
	})
	if err != nil {
		t.Errorf("Log should not fail if not initialized, got: %v", err)
	}
}

func TestLog_WithAuditLoggerInitialized(t *testing.T) {
	tempLog, _ := os.CreateTemp("", "audit-*.log")
	defer os.Remove(tempLog.Name())

	cfg := config.AuditConfig{
		AuditFile: tempLog.Name(),
	}
	Init(cfg)
	defer Close()

	err := Log(model.AuditLogEntry{
		Status: "success",
	})
	if err != nil {
		t.Errorf("Log should not fail, got: %v", err)
	}
}

func TestClose_NoLoggers(t *testing.T) {
	generalLogger = nil
	auditLogger = nil
	err := Close()
	if err != nil {
		t.Errorf("Close should not fail if loggers are nil, got: %v", err)
	}
}
