package model

import (
	"encoding/json"
	"testing"
)

func TestExecutionRequestJSON(t *testing.T) {
	jsonStr := `{"verb": "get", "resource": "pods", "namespace": "default", "mode": "structured"}`
	var req ExecutionRequest
	err := json.Unmarshal([]byte(jsonStr), &req)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if req.Verb != "get" {
		t.Errorf("Expected verb 'get', got '%s'", req.Verb)
	}
	if req.Resource != "pods" {
		t.Errorf("Expected resource 'pods', got '%s'", req.Resource)
	}
	if req.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got '%s'", req.Namespace)
	}
	if req.Mode != "structured" {
		t.Errorf("Expected mode 'structured', got '%s'", req.Mode)
	}
}

func TestExecutionResultJSON(t *testing.T) {
	result := ExecutionResult{
		RequestID:     "req-123",
		Status:        "success",
		ExitCode:      0,
		Stdout:        "NAME   READY   STATUS\ntest   1/1     Running",
		Stderr:        "",
		Truncated:     false,
		DurationMs:    100,
		ResponseSize:  50,
		BlockedReason: "",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var req ExecutionResult
	err = json.Unmarshal(data, &req)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if req.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", req.Status)
	}
}
