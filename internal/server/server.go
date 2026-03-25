package server

import (
	"fmt"
	"log"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/auth"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/config"

	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server.
type Server struct {
	engine *gin.Engine
	cfg    *config.Config
}

// New creates a new Server.
func New(cfg *config.Config, handler *Handler) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())

	// 获取限流配置，提供默认值兜底
	rateLimit := cfg.RateLimit
	// Register global middleware
	engine.Use(RateLimitMiddleware(rateLimit.RequestsPerSecond, rateLimit.Burst))
	engine.Use(AuthMiddleware(auth.NewAuthenticator(cfg.Auth.Token)))

	server := &Server{
		engine: engine,
		cfg:    cfg,
	}

	server.registerRoutes(handler)

	return server
}

// registerRoutes registers the routes.
func (s *Server) registerRoutes(handler *Handler) {
	s.engine.GET("/health", handler.HealthCheck)
	s.engine.POST("/execute", handler.Execute)
}

// Run starts the server.
func (s *Server) Run() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	log.Printf("Starting server on %s", addr)
	return s.engine.Run(addr)
}
