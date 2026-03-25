package executor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/audit"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/config"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/model"

	"go.uber.org/zap"
)

// Executor executes kubectl commands.
type Executor struct {
	config    *config.Config
	builder   *Builder
	maxOutput int
	timeout   time.Duration
}

// ExecutionRequest represents the request for execution.
type ExecutionRequest struct {
	model.ExecutionRequest
	AgentID   string
	RequestID string
}

// limitedWriter 是一个带截断功能的 Writer，用于安全地捕获命令输出
type limitedWriter struct {
	buf       strings.Builder
	maxLen    int
	truncated bool
}

// Write 实现 io.Writer 接口，在写入时检查是否超过限制
func (w *limitedWriter) Write(p []byte) (n int, err error) {
	if w.truncated {
		return len(p), nil // 已截断，丢弃后续数据
	}

	// 检查写入后是否会超过限制
	if w.buf.Len()+len(p) > w.maxLen {
		w.truncated = true
		// 只写入剩余空间
		remaining := w.maxLen - w.buf.Len()
		if remaining > 0 {
			w.buf.Write(p[:remaining])
		}
		return len(p), nil // 返回完整长度，避免 io.Writer 合约错误
	}

	return w.buf.Write(p)
}

// String 返回已写入的字符串
func (w *limitedWriter) String() string {
	return w.buf.String()
}

// Truncated 返回是否发生了截断
func (w *limitedWriter) Truncated() bool {
	return w.truncated
}

// NewExecutor creates a new Executor.
func NewExecutor(cfg *config.Config) *Executor {
	return &Executor{
		config:    cfg,
		builder:   NewBuilder(),
		maxOutput: cfg.Execution.MaxOutputLength,
		timeout:   time.Duration(cfg.Execution.TimeoutSeconds) * time.Second,
	}
}

// Execute executes the kubectl command.
func (e *Executor) Execute(ctx context.Context, req *ExecutionRequest) (*model.ExecutionResult, error) {
	audit.Info("[Executor] 开始执行命令",
		zap.String("request_id", req.RequestID),
		zap.String("verb", req.Verb),
		zap.String("resource", req.Resource),
		zap.String("namespace", req.Namespace))

	// 1. 动词白名单/黑名单校验
	audit.Debug("[Executor] 开始动词校验", zap.String("verb", req.Verb))
	if err := e.validateVerb(req.Verb); err != nil {
		audit.Warn("[Executor] 动词校验未通过",
			zap.String("verb", req.Verb),
			zap.Error(err))
		return &model.ExecutionResult{
			RequestID:     req.RequestID,
			Status:        "blocked",
			BlockedReason: err.Error(),
			ExitCode:      -1,
			DurationMs:    0,
			ResponseSize:  0,
		}, nil
	}
	audit.Info("[Executor] 动词校验通过", zap.String("verb", req.Verb))

	// 2. 参数组装
	audit.Debug("[Executor] 开始组装参数")
	args := e.builder.BuildArgs(&req.ExecutionRequest)
	audit.Debug("[Executor] 参数组装完成", zap.Any("args", args))

	// 3. 超时控制
	timeoutCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	audit.Debug("[Executor] 超时控制设置完成", zap.Duration("timeout", e.timeout))

	// 4. 执行命令
	audit.Info("[Executor] 开始执行 kubectl 命令")
	cmd := exec.CommandContext(timeoutCtx, "kubectl", args...)

	// 使用 limitedWriter 替代 io.Pipe()，避免死锁
	// 标准库会自动并发排空 stdout/stderr 管道
	stdoutWriter := &limitedWriter{maxLen: e.maxOutput}
	cmd.Stdout = stdoutWriter

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	start := time.Now()

	// 5. 执行命令（标准库自动处理管道排空，绝无死锁）
	err := cmd.Run()
	duration := time.Since(start)

	stdoutStr := stdoutWriter.String()
	truncated := stdoutWriter.Truncated()
	stderrStr := stderrBuf.String()

	audit.Info("[Executor] 命令执行完成",
		zap.Duration("duration", duration),
		zap.Bool("truncated", truncated))

	audit.Debug("[Executor] 输出读取完成",
		zap.Int("stdout_size", len(stdoutStr)),
		zap.Int("stderr_size", len(stderrStr)))

	// 6. 结果组装
	audit.Debug("[Executor] 开始组装结果")
	result := &model.ExecutionResult{
		RequestID:    req.RequestID,
		DurationMs:   duration.Milliseconds(),
		ResponseSize: len(stdoutStr) + len(stderrStr),
		Truncated:    truncated,
		Stdout:       stdoutStr,
		Stderr:       stderrStr,
	}

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// 记录执行超时日志
			audit.Error("[Executor] 执行超时",
				zap.String("request_id", req.RequestID),
				zap.String("agent_id", req.AgentID),
				zap.String("verb", req.Verb),
				zap.String("resource", req.Resource),
				zap.Duration("timeout", e.timeout),
				zap.Duration("duration", duration))
			result.Status = "failed"
			result.ExitCode = -1
			result.BlockedReason = string(model.ErrExecTimeout)
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			audit.Warn("[Executor] 命令执行失败",
				zap.Int("exit_code", exitErr.ExitCode()),
				zap.String("stderr", stderrStr))
			result.Status = "failed"
			result.ExitCode = exitErr.ExitCode()
		} else {
			audit.Error("[Executor] kubectl未找到或其他错误", zap.Error(err))
			result.Status = "failed"
			result.ExitCode = -1
			result.BlockedReason = string(model.ErrKubectlNotFound)
		}
	} else {
		audit.Info("[Executor] 命令执行成功", zap.Int("exit_code", 0))
		result.Status = "success"
		result.ExitCode = 0
	}

	// 如果被截断，追加提示信息
	if truncated {
		result.Stdout += "\n... [Output truncated. The log is too long. Please use flags like --tail=100 or --since=1h to limit the output.]"
	}

	audit.Info("[Executor] 执行完成",
		zap.String("request_id", req.RequestID),
		zap.String("status", result.Status),
		zap.Int64("duration_ms", result.DurationMs),
		zap.Int("response_size", result.ResponseSize))
	return result, nil
}

// validateVerb validates the verb against allowlist/blocklist.
func (e *Executor) validateVerb(verb string) error {
	// 如果配置了白名单，只允许白名单中的动词
	if len(e.config.Rules.VerbAllowlist) > 0 {
		for _, allowed := range e.config.Rules.VerbAllowlist {
			if verb == allowed {
				return nil
			}
		}
		return fmt.Errorf("verb '%s' is not in the allowlist", verb)
	}

	// 如果配置了黑名单，禁止黑名单中的动词
	if len(e.config.Rules.VerbBlocklist) > 0 {
		for _, blocked := range e.config.Rules.VerbBlocklist {
			if verb == blocked {
				return fmt.Errorf("verb '%s' is blocked by policy", verb)
			}
		}
	}

	return nil
}
