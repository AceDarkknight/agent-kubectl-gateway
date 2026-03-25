package server

import (
	"fmt"
	"net/http"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/audit"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/config"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/executor"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/filter"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"go.uber.org/zap"
)

var (
	requestIDheaders = []string{
		"X-Request-ID",
		"X-Correlation-ID",
		"Request-ID",
		"Correlation-ID",
	}
)

// Handler handles HTTP requests.
type Handler struct {
	Config   *config.Config
	Executor *executor.Executor
	Filter   *filter.Filter
}

// NewHandler creates a new Handler.
func NewHandler(cfg *config.Config, exec *executor.Executor, filter *filter.Filter) *Handler {
	return &Handler{
		Config:   cfg,
		Executor: exec,
		Filter:   filter,
	}
}

// Execute handles the /execute endpoint.
func (h *Handler) Execute(c *gin.Context) {
	// 1. 验证 Token 并获取 Agent ID (now handled by middleware)
	audit.Info("[Handler] 收到请求，开始处理")
	agentID, exists := c.Get("agentID")
	if !exists {
		audit.Error("[Handler] 鉴权失败", zap.Error(fmt.Errorf("agentID not found in context")))
		c.JSON(http.StatusUnauthorized, model.ExecutionResult{
			Status:        "failed",
			BlockedReason: "Unauthorized",
		})
		return
	}
	audit.Info("[Handler] 鉴权通过", zap.String("agent_id", agentID.(string)))

	// 2. 绑定请求参数
	var req model.ExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		audit.Error("[Handler] 请求参数解析失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, model.ExecutionResult{
			Status:        "failed",
			BlockedReason: "Invalid request body: " + err.Error(),
		})
		return
	}
	output := ""
	if req.Options != nil {
		output = req.Options.Output
	}
	audit.Info("[Handler] 请求参数解析完成",
		zap.String("verb", req.Verb),
		zap.String("resource", req.Resource),
		zap.String("namespace", req.Namespace),
		zap.String("output", output))

	// 3. 构建执行请求
	execReq := &executor.ExecutionRequest{
		ExecutionRequest: req,
		AgentID:          agentID.(string),
		RequestID:        generateRequestID(c),
	}
	audit.Info("[Handler] 构建执行请求完成", zap.String("request_id", execReq.RequestID))

	// 4. 执行命令
	audit.Info("[Handler] 开始执行命令", zap.String("request_id", execReq.RequestID))
	result, err := h.Executor.Execute(c.Request.Context(), execReq)
	if err != nil {
		audit.Error("[Handler] 执行命令出错",
			zap.String("request_id", execReq.RequestID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.ExecutionResult{
			RequestID:     execReq.RequestID,
			Status:        "failed",
			BlockedReason: "Internal error: " + err.Error(),
		})
		return
	}
	// result 可能为 nil，增加安全检查
	if result != nil {
		audit.Info("[Handler] 执行命令完成",
			zap.String("request_id", execReq.RequestID),
			zap.String("status", result.Status),
			zap.Int64("duration_ms", result.DurationMs),
			zap.Int("response_size", result.ResponseSize))
	} else {
		audit.Warn("[Handler] 执行命令完成, result is nil", zap.String("request_id", execReq.RequestID))
	}

	// 5. 过滤结果
	if h.Filter != nil && result != nil {
		stdoutLen := len(result.Stdout)
		audit.Info("[Handler] 开始过滤结果",
			zap.String("request_id", execReq.RequestID),
			zap.Int("original_size", stdoutLen))
		result = h.Filter.FilterResult(&req, result)
		filteredStdoutLen := len(result.Stdout)
		audit.Info("[Handler] 过滤完成",
			zap.String("request_id", execReq.RequestID),
			zap.Int("filtered_size", filteredStdoutLen))
	}

	// 6. 记录审计日志
	if result != nil {
		sourceIP := c.ClientIP()
		responseSize := result.ResponseSize
		audit.LogRequest(&req, agentID.(string), sourceIP, result.Status, result.DurationMs, responseSize, result.BlockedReason)
	}

	// 7. 返回结果
	audit.Info("[Handler] 返回响应", zap.String("request_id", execReq.RequestID))
	c.JSON(http.StatusOK, result)
}

// HealthCheck handles the /health endpoint.
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// generateRequestID 生成请求 ID。
// 优先从 HTTP 请求头获取 X-Request-ID、X-Correlation-ID 或 Request-Id；
// 如果不存在，则使用 UUID 生成一个新的唯一 ID。
func generateRequestID(c *gin.Context) string {
	// 尝试从 HTTP 请求头获取常用的 request ID 字段
	for _, header := range requestIDheaders {
		if id := c.GetHeader(header); id != "" {
			return id
		}
	}
	// 没有获取到，使用 UUID 生成新的唯一 ID
	return uuid.New().String()
}
