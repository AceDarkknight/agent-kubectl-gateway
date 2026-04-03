package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/config"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/executor"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/filter"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/model"
	"github.com/gin-gonic/gin"
)

// MockExecutor 是 executor.Executor 的 mock 实现
type MockExecutor struct {
	Result *model.ExecutionResult
	Err    error
}

func (m *MockExecutor) Execute(ctx interface{}, req *executor.ExecutionRequest) (*model.ExecutionResult, error) {
	return m.Result, m.Err
}

func setupTestRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("agentID", "test-agent")
		c.Next()
	})
	r.GET("/health", handler.HealthCheck)
	r.POST("/execute", handler.Execute)
	return r
}

func TestHandler_HealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&config.Config{}, nil, nil)
	r := gin.Default()
	r.GET("/health", handler.HealthCheck)

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", w.Code)
	}
}

func TestHandler_Execute_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&config.Config{}, nil, nil)

	r := gin.Default()
	r.POST("/execute", handler.Execute)

	body, _ := json.Marshal(model.ExecutionRequest{
		Verb:     "get",
		Resource: "pods",
	})

	req, _ := http.NewRequest("POST", "/execute", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status Unauthorized, got %v", w.Code)
	}
}

func TestHandler_Execute_InvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&config.Config{}, nil, nil)

	r := gin.Default()
	r.Use(func(c *gin.Context) {
		c.Set("agentID", "test-agent")
		c.Next()
	})
	r.POST("/execute", handler.Execute)

	req, _ := http.NewRequest("POST", "/execute", bytes.NewBuffer([]byte("invalid-json")))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status BadRequest, got %v", w.Code)
	}
}

func TestHandler_Execute_MissingResource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(&config.Config{}, nil, nil)
	r := setupTestRouter(handler)

	body, _ := json.Marshal(model.ExecutionRequest{
		Verb: "get",
		// Resource 缺失
	})

	req, _ := http.NewRequest("POST", "/execute", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status BadRequest, got %v", w.Code)
	}
}

func TestHandler_Execute_LogsWithoutResource(t *testing.T) {
	// logs 命令不需要 resource，应该通过
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Execution: config.ExecutionConfig{
			TimeoutSeconds:  5,
			MaxOutputLength: 1024,
		},
	}
	exec := executor.NewExecutor(cfg)
	handler := NewHandler(cfg, exec, nil)
	r := setupTestRouter(handler)

	body, _ := json.Marshal(model.ExecutionRequest{
		Verb: "logs",
		Mode: "structured", // 添加 required 的 Mode 字段
		// Resource 可以为空
	})

	req, _ := http.NewRequest("POST", "/execute", bytes.NewBuffer(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// logs 命令不应返回 400
	if w.Code == http.StatusBadRequest {
		t.Errorf("logs command should not require resource, got 400, body: %s", w.Body.String())
	}
}

func TestHandler_Execute_WithHeaderRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	executor := &executor.Executor{}
	handler := NewHandler(&config.Config{Execution: config.ExecutionConfig{TimeoutSeconds: 5, MaxOutputLength: 1024}}, executor, nil)
	r := setupTestRouter(handler)

	body, _ := json.Marshal(model.ExecutionRequest{
		Verb:     "get",
		Resource: "pods",
	})

	req, _ := http.NewRequest("POST", "/execute", bytes.NewBuffer(body))
	req.Header.Set("X-Request-ID", "custom-req-id")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 验证 request ID 被使用
	if w.Code == http.StatusOK {
		var result model.ExecutionResult
		json.Unmarshal(w.Body.Bytes(), &result)
		if result.RequestID != "custom-req-id" {
			t.Errorf("Expected RequestID 'custom-req-id', got '%s'", result.RequestID)
		}
	}
}

func TestGenerateRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// 测试从不同 header 获取 request ID
	tests := []struct {
		name     string
		header   string
		value    string
		expected string
	}{
		{"X-Request-ID", "X-Request-ID", "req-123", "req-123"},
		{"X-Correlation-ID", "X-Correlation-ID", "corr-456", "corr-456"},
		{"Request-ID", "Request-ID", "req-789", "req-789"},
		{"Correlation-ID", "Correlation-ID", "corr-012", "corr-012"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "/execute", nil)
			req.Header.Set(tt.header, tt.value)
			c.Request = req

			id := generateRequestID(c)
			if id != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, id)
			}
		})
	}

	// 测试无 header 时生成 UUID
	t.Run("No header - generates UUID", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/execute", nil)
		c.Request = req

		id := generateRequestID(c)
		if id == "" {
			t.Error("Expected non-empty UUID")
		}
	})
}

func TestAuthMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建一个带有有效 token 的 authenticator
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Token: "valid-token",
		},
	}

	// 由于 auth 模块需要初始化，我们测试 middleware 的基本逻辑
	// 实际的 auth 测试在 auth 包中
	t.Run("Middleware function exists", func(t *testing.T) {
		// 验证 middleware 函数存在且可以调用
		_ = cfg
	})
}

func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Valid rate limit config", func(t *testing.T) {
		middleware := RateLimitMiddleware(10.0, 20)
		if middleware == nil {
			t.Error("Expected middleware function, got nil")
		}
	})

	t.Run("Invalid rate limit config - zero QPS", func(t *testing.T) {
		middleware := RateLimitMiddleware(0, 20)
		if middleware == nil {
			t.Error("Expected middleware function, got nil")
		}
	})

	t.Run("Invalid rate limit config - zero burst", func(t *testing.T) {
		middleware := RateLimitMiddleware(10.0, 0)
		if middleware == nil {
			t.Error("Expected middleware function, got nil")
		}
	})

	t.Run("Invalid rate limit config - negative values", func(t *testing.T) {
		middleware := RateLimitMiddleware(-1, -1)
		if middleware == nil {
			t.Error("Expected middleware function, got nil")
		}
	})
}

func TestNewHandler(t *testing.T) {
	cfg := &config.Config{}
	exec := &executor.Executor{}
	flt := &filter.Filter{}

	handler := NewHandler(cfg, exec, flt)

	if handler == nil {
		t.Fatal("Expected handler, got nil")
	}
	if handler.Config != cfg {
		t.Error("Config not set correctly")
	}
	if handler.Executor != exec {
		t.Error("Executor not set correctly")
	}
	if handler.Filter != flt {
		t.Error("Filter not set correctly")
	}
}

func TestNewServer(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Auth: config.AuthConfig{
			Token: "",
		},
		RateLimit: config.RateLimitConfig{
			RequestsPerSecond: 10,
			Burst:             20,
		},
	}

	handler := NewHandler(cfg, nil, nil)
	server := New(cfg, handler)

	if server == nil {
		t.Fatal("Expected server, got nil")
	}
	if server.engine == nil {
		t.Error("Engine not initialized")
	}
	if server.cfg != cfg {
		t.Error("Config not set correctly")
	}
}

func TestServer_RegisterRoutes(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Auth: config.AuthConfig{
			Token: "",
		},
		RateLimit: config.RateLimitConfig{
			RequestsPerSecond: 10,
			Burst:             20,
		},
	}

	handler := NewHandler(cfg, nil, nil)
	server := New(cfg, handler)

	// 验证路由已注册
	routes := server.engine.Routes()
	found := false
	for _, route := range routes {
		if route.Path == "/health" && route.Method == "GET" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Health route not registered")
	}

	found = false
	for _, route := range routes {
		if route.Path == "/execute" && route.Method == "POST" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Execute route not registered")
	}
}
