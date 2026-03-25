package executor

import (
	"context"
	"testing"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/config"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/model"
)

func TestExecutor_ValidateVerb(t *testing.T) {
	cfg := &config.Config{
		Execution: config.ExecutionConfig{
			TimeoutSeconds:  5,
			MaxOutputLength: 1024,
		},
		Rules: config.RulesConfig{
			VerbAllowlist: []string{"get", "list"},
			VerbBlocklist: []string{"delete", "exec"},
		},
	}

	exec := NewExecutor(cfg)

	err := exec.validateVerb("get")
	if err != nil {
		t.Errorf("Expected 'get' to be allowed, got error: %v", err)
	}

	err = exec.validateVerb("delete")
	if err == nil {
		t.Errorf("Expected 'delete' to be blocked")
	}
}

func TestExecutor_ExecuteBlocked(t *testing.T) {
	cfg := &config.Config{
		Execution: config.ExecutionConfig{
			TimeoutSeconds:  1,
			MaxOutputLength: 1024,
		},
		Rules: config.RulesConfig{
			VerbBlocklist: []string{"blocked-verb"},
		},
	}
	exec := NewExecutor(cfg)

	req := &ExecutionRequest{
		ExecutionRequest: model.ExecutionRequest{
			Verb: "blocked-verb",
		},
		AgentID:   "agent-1",
		RequestID: "req-1",
	}

	res, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if res.Status != "blocked" {
		t.Errorf("Expected status 'blocked', got %s", res.Status)
	}
}
