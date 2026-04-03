package executor

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/config"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/model"
)

// MockCommandRunner 是 CommandRunner 接口的 mock 实现
type MockCommandRunner struct {
	Stdout string
	Stderr string
	Err    error
}

func (m *MockCommandRunner) Run(ctx context.Context, stdout, stderr io.Writer, name string, args ...string) error {
	// 将模拟输出写入提供的 writer
	if m.Stdout != "" {
		stdout.Write([]byte(m.Stdout))
	}
	if m.Stderr != "" {
		stderr.Write([]byte(m.Stderr))
	}
	return m.Err
}

func newTestExecutor(cfg *config.Config) *Executor {
	return &Executor{
		config:    cfg,
		builder:   NewBuilder(),
		maxOutput: cfg.Execution.MaxOutputLength,
		timeout:   time.Duration(cfg.Execution.TimeoutSeconds) * time.Second,
	}
}

func TestExecutor_ExecuteSuccess(t *testing.T) {
	cfg := &config.Config{
		Execution: config.ExecutionConfig{
			MaxOutputLength: 1000,
			TimeoutSeconds:  30,
		},
	}

	mockRunner := &MockCommandRunner{
		Stdout: "nginx-1\nnginx-2",
		Stderr: "",
		Err:    nil,
	}

	exec := newTestExecutor(cfg)
	exec.runner = mockRunner

	req := &ExecutionRequest{
		ExecutionRequest: model.ExecutionRequest{
			Verb:     "get",
			Resource: "pods",
		},
		AgentID:   "test-agent",
		RequestID: "req-1",
	}

	result, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", result.Status)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != "nginx-1\nnginx-2" {
		t.Errorf("Expected stdout 'nginx-1\\nnginx-2', got '%s'", result.Stdout)
	}
}

func TestExecutor_ExecuteExitError(t *testing.T) {
	cfg := &config.Config{
		Execution: config.ExecutionConfig{
			MaxOutputLength: 1000,
			TimeoutSeconds:  30,
		},
	}

	// 模拟非零退出码
	exitErr := &exec.ExitError{Stderr: []byte("error: not found")}
	mockRunner := &MockCommandRunner{
		Stdout: "",
		Stderr: "error: not found",
		Err:    exitErr,
	}

	exec := newTestExecutor(cfg)
	exec.runner = mockRunner

	req := &ExecutionRequest{
		ExecutionRequest: model.ExecutionRequest{
			Verb:     "get",
			Resource: "pods",
		},
		AgentID:   "test-agent",
		RequestID: "req-2",
	}

	result, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", result.Status)
	}
	if result.ExitCode == 0 {
		t.Errorf("Expected non-zero exit code, got %d", result.ExitCode)
	}
}

func TestExecutor_ExecuteTimeout(t *testing.T) {
	cfg := &config.Config{
		Execution: config.ExecutionConfig{
			MaxOutputLength: 1000,
			TimeoutSeconds:  1,
		},
	}

	// 模拟超时错误
	mockRunner := &MockCommandRunner{
		Stdout: "",
		Stderr: "",
		Err:    context.DeadlineExceeded,
	}

	exec := newTestExecutor(cfg)
	exec.runner = mockRunner

	req := &ExecutionRequest{
		ExecutionRequest: model.ExecutionRequest{
			Verb:     "get",
			Resource: "pods",
		},
		AgentID:   "test-agent",
		RequestID: "req-3",
	}

	result, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", result.Status)
	}
	if result.ExitCode != -1 {
		t.Errorf("Expected exit code -1, got %d", result.ExitCode)
	}
	if result.BlockedReason != string(model.ErrExecTimeout) {
		t.Errorf("Expected BlockedReason '%s', got '%s'", model.ErrExecTimeout, result.BlockedReason)
	}
}

func TestExecutor_ExecuteKubectlNotFound(t *testing.T) {
	cfg := &config.Config{
		Execution: config.ExecutionConfig{
			MaxOutputLength: 1000,
			TimeoutSeconds:  30,
		},
	}

	// 模拟 kubectl 未找到
	mockRunner := &MockCommandRunner{
		Stdout: "",
		Stderr: "",
		Err:    errors.New("kubectl not found"),
	}

	exec := newTestExecutor(cfg)
	exec.runner = mockRunner

	req := &ExecutionRequest{
		ExecutionRequest: model.ExecutionRequest{
			Verb:     "get",
			Resource: "pods",
		},
		AgentID:   "test-agent",
		RequestID: "req-4",
	}

	result, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", result.Status)
	}
	if result.ExitCode != -1 {
		t.Errorf("Expected exit code -1, got %d", result.ExitCode)
	}
	if result.BlockedReason != string(model.ErrKubectlNotFound) {
		t.Errorf("Expected BlockedReason '%s', got '%s'", model.ErrKubectlNotFound, result.BlockedReason)
	}
}

func TestExecutor_ExecuteTruncated(t *testing.T) {
	cfg := &config.Config{
		Execution: config.ExecutionConfig{
			MaxOutputLength: 10, // 设置很小的最大输出长度
			TimeoutSeconds:  30,
		},
	}

	longOutput := strings.Repeat("a", 100) // 超出 MaxOutputLength 的输出
	mockRunner := &MockCommandRunner{
		Stdout: longOutput,
		Stderr: "",
		Err:    nil,
	}

	exec := newTestExecutor(cfg)
	exec.runner = mockRunner

	req := &ExecutionRequest{
		ExecutionRequest: model.ExecutionRequest{
			Verb:     "logs",
			Resource: "pod/my-pod",
		},
		AgentID:   "test-agent",
		RequestID: "req-5",
	}

	result, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Truncated {
		t.Errorf("Expected truncated to be true")
	}
	if result.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", result.Status)
	}
}

func TestExecutor_ExecuteWithAuditLog(t *testing.T) {
	// 初始化 audit 模块以测试日志记录路径
	cfg := &config.Config{
		Execution: config.ExecutionConfig{
			MaxOutputLength: 1000,
			TimeoutSeconds:  30,
		},
	}

	mockRunner := &MockCommandRunner{
		Stdout: "result",
		Stderr: "",
		Err:    nil,
	}

	exec := newTestExecutor(cfg)
	exec.runner = mockRunner

	req := &ExecutionRequest{
		ExecutionRequest: model.ExecutionRequest{
			Verb:      "get",
			Resource:  "pods",
			Namespace: "default",
			Name:      "my-pod",
		},
		AgentID:   "test-agent",
		RequestID: "req-6",
	}

	result, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", result.Status)
	}
	if result.RequestID != "req-6" {
		t.Errorf("Expected RequestID 'req-6', got '%s'", result.RequestID)
	}
}

func TestExecutor_ExecuteBlockedVerb(t *testing.T) {
	cfg := &config.Config{
		Execution: config.ExecutionConfig{
			MaxOutputLength: 1000,
			TimeoutSeconds:  30,
		},
		Rules: config.RulesConfig{
			VerbAllowlist: []string{"get", "describe"},
			VerbBlocklist: []string{"delete", "exec"},
		},
	}

	mockRunner := &MockCommandRunner{
		Stdout: "",
		Stderr: "",
		Err:    nil,
	}

	exec := newTestExecutor(cfg)
	exec.runner = mockRunner

	req := &ExecutionRequest{
		ExecutionRequest: model.ExecutionRequest{
			Verb:     "delete",
			Resource: "pod",
			Name:     "my-pod",
		},
		AgentID:   "test-agent",
		RequestID: "req-7",
	}

	result, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Status != "blocked" {
		t.Errorf("Expected status 'blocked', got '%s'", result.Status)
	}
	if result.ExitCode != -1 {
		t.Errorf("Expected exit code -1, got %d", result.ExitCode)
	}
}

func TestExecutor_ExecuteAllowlistBlocked(t *testing.T) {
	cfg := &config.Config{
		Execution: config.ExecutionConfig{
			MaxOutputLength: 1000,
			TimeoutSeconds:  30,
		},
		Rules: config.RulesConfig{
			VerbAllowlist: []string{"get", "describe"},
		},
	}

	mockRunner := &MockCommandRunner{
		Stdout: "",
		Stderr: "",
		Err:    nil,
	}

	exec := newTestExecutor(cfg)
	exec.runner = mockRunner

	req := &ExecutionRequest{
		ExecutionRequest: model.ExecutionRequest{
			Verb:     "patch",
			Resource: "pod",
			Name:     "my-pod",
		},
		AgentID:   "test-agent",
		RequestID: "req-8",
	}

	result, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Status != "blocked" {
		t.Errorf("Expected status 'blocked', got '%s'", result.Status)
	}
}

func TestExecutor_ExecuteInvalidCommand(t *testing.T) {
	cfg := &config.Config{
		Execution: config.ExecutionConfig{
			MaxOutputLength: 1000,
			TimeoutSeconds:  30,
		},
	}

	// 模拟 command not found
	mockRunner := &MockCommandRunner{
		Stdout: "",
		Stderr: "",
		Err:    errors.New("exec: \"kubectl\": executable file not found in $PATH"),
	}

	exec := newTestExecutor(cfg)
	exec.runner = mockRunner

	req := &ExecutionRequest{
		ExecutionRequest: model.ExecutionRequest{
			Verb:     "get",
			Resource: "pods",
		},
		AgentID:   "test-agent",
		RequestID: "req-9",
	}

	result, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", result.Status)
	}
	if result.ExitCode != -1 {
		t.Errorf("Expected exit code -1, got %d", result.ExitCode)
	}
}

func TestLimitedWriter_Write(t *testing.T) {
	w := &limitedWriter{maxLen: 10}
	data := []byte("hello world")
	n, err := w.Write(data)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected %d bytes written, got %d", len(data), n)
	}
	if !w.Truncated() {
		t.Errorf("Expected truncated to be true")
	}
	if w.String() != "hello worl" { // 只写了 10 个字符
		t.Errorf("Expected 'hello worl', got '%s'", w.String())
	}
}

func TestLimitedWriter_WriteAfterTruncate(t *testing.T) {
	w := &limitedWriter{maxLen: 5}
	w.Write([]byte("1234567890"))
	w.Write([]byte("more data")) // 写入后应该被丢弃

	if w.String() != "12345" {
		t.Errorf("Expected '12345', got '%s'", w.String())
	}
}

func TestLimitedWriter_WriteSmallChunk(t *testing.T) {
	w := &limitedWriter{maxLen: 10}
	w.Write([]byte("abc"))
	w.Write([]byte("def"))

	if w.Truncated() {
		t.Errorf("Expected not truncated")
	}
	if w.String() != "abcdef" {
		t.Errorf("Expected 'abcdef', got '%s'", w.String())
	}
}

func TestLimitedWriter_WriteExactLimit(t *testing.T) {
	w := &limitedWriter{maxLen: 5}
	w.Write([]byte("12345"))

	if w.Truncated() {
		t.Errorf("Expected not truncated")
	}
	if w.String() != "12345" {
		t.Errorf("Expected '12345', got '%s'", w.String())
	}
}

func TestValidateVerb(t *testing.T) {
	tests := []struct {
		name        string
		allowlist   []string
		blocklist   []string
		verb        string
		shouldError bool
	}{
		{
			name:        "allowlist - allowed verb",
			allowlist:   []string{"get", "describe"},
			verb:        "get",
			shouldError: false,
		},
		{
			name:        "allowlist - blocked verb",
			allowlist:   []string{"get", "describe"},
			verb:        "delete",
			shouldError: true,
		},
		{
			name:        "blocklist - blocked verb",
			blocklist:   []string{"delete", "exec"},
			verb:        "delete",
			shouldError: true,
		},
		{
			name:        "blocklist - allowed verb",
			blocklist:   []string{"delete", "exec"},
			verb:        "get",
			shouldError: false,
		},
		{
			name:        "no lists - any verb allowed",
			verb:        "anything",
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Rules: config.RulesConfig{
					VerbAllowlist: tt.allowlist,
					VerbBlocklist: tt.blocklist,
				},
			}
			exec := newTestExecutor(cfg)
			err := exec.validateVerb(tt.verb)
			if tt.shouldError && err == nil {
				t.Errorf("Expected error but got nil")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got %v", err)
			}
		})
	}
}
