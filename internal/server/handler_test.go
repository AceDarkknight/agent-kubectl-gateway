package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/config"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/model"
	"github.com/gin-gonic/gin"
)

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
