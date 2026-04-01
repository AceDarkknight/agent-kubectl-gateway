package model

import "time"

// ExecutionRequest represents the request from AI Agent to execute kubectl command.
// 结构化输入，从根本上避免 Shell 注入。
type ExecutionRequest struct {
	Verb        string   `json:"verb" binding:"required"` // kubectl 操作动词，如 get, describe, logs, apply, create 等
	Resource    string   `json:"resource"`                // Kubernetes 资源类型，如 pods, deployments, services 等（logs 命令可选）
	Namespace   string   `json:"namespace"`               // 命名空间，如 default, kube-system。集群级别资源可为空
	Name        string   `json:"name"`                    // 资源名称，查询所有资源时可为空
	Subresource string   `json:"subresource"`             // 子资源类型，如 log, status, scale 等
	Options     *Options `json:"options"`                 // 命令选项参数
	Mode        string   `json:"mode" binding:"required"` // 输入模式，固定为 structured
}

// Options represents the command options for kubectl.
type Options struct {
	LabelSelector string `json:"labelSelector"` // 标签选择器
	FieldSelector string `json:"fieldSelector"` // 字段选择器
	Limit         int    `json:"limit"`         // 返回结果数量限制
	Container     string `json:"container"`     // 容器名称，用于 logs 或 exec 命令
	TailLines     int    `json:"tailLines"`     // 日志尾部行数
	Since         string `json:"since"`         // 时间范围，如 1h, 30m
	Follow        bool   `json:"follow"`        // 是否持续跟踪日志输出
	Previous      bool   `json:"previous"`      // 是否获取前一个容器的日志
	AllNamespaces bool   `json:"allNamespaces"` // 是否查询所有命名空间
	Output        string `json:"output"`        // 输出格式，如 json, yaml, wide, name
}

// ExecutionResult represents the result of kubectl command execution.
type ExecutionResult struct {
	RequestID     string `json:"request_id"`          // 请求唯一标识
	Status        string `json:"status"`              // 执行状态：success, failed, blocked
	ExitCode      int    `json:"exit_code"`           // kubectl 进程退出码
	Stdout        string `json:"stdout"`              // 标准输出内容
	Stderr        string `json:"stderr"`              // 标准错误输出内容
	Truncated     bool   `json:"truncated"`           // 输出是否被截断
	DurationMs    int64  `json:"duration_ms"`         // 命令执行耗时（毫秒）
	ResponseSize  int    `json:"response_size_bytes"` // 最终返回数据大小（字节）
	BlockedReason string `json:"blocked_reason"`      // 被拦截原因
}

// AuditLogEntry represents an audit log entry.
type AuditLogEntry struct {
	Timestamp    time.Time `json:"timestamp"`           // 请求发生的时间戳
	SourceIP     string    `json:"source_ip"`           // 发起请求的来源 IP 地址
	AgentID      string    `json:"agent_id"`            // 触发请求的 Agent 标识
	Command      string    `json:"command"`             // 实际请求执行的完整 kubectl 命令及参数
	Status       string    `json:"status"`              // 执行结果状态：success, failed, intercepted
	DurationMs   int64     `json:"duration_ms"`         // 命令执行或请求处理的耗时
	ResponseSize int       `json:"response_size_bytes"` // 最终返回给 Agent 的数据大小
	ErrorMessage string    `json:"error_message"`       // 错误原因
}

// ErrorCode represents error codes.
type ErrorCode string

// Define error codes.
const (
	ErrInvalidCommand  ErrorCode = "ERR_INVALID_COMMAND"   // 参数非法
	ErrBlockedVerb     ErrorCode = "ERR_BLOCKED_VERB"      // 动词被黑名单或不在白名单中
	ErrKubectlNotFound ErrorCode = "ERR_KUBECTL_NOT_FOUND" // kubectl 不存在
	ErrExecTimeout     ErrorCode = "ERR_EXEC_TIMEOUT"      // 执行超时
	ErrNonZeroExit     ErrorCode = "ERR_NON_ZERO_EXIT"     // 非零退出码
	ErrOutputTooLarge  ErrorCode = "ERR_OUTPUT_TOO_LARGE"  // 输出过大
)

// ErrorCodeToMessage maps error codes to human-readable messages.
var ErrorCodeToMessage = map[ErrorCode]string{
	ErrInvalidCommand:  "Invalid command parameters",
	ErrBlockedVerb:     "Command execution denied by proxy policy",
	ErrKubectlNotFound: "kubectl command not found",
	ErrExecTimeout:     "Command execution timeout",
	ErrNonZeroExit:     "Command execution failed with non-zero exit code",
	ErrOutputTooLarge:  "Output too large, truncated",
}
