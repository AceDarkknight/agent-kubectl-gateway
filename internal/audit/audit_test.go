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
