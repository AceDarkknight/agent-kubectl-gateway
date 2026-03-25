package server

import (
	"net/http"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/auth"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// AuthMiddleware returns a gin.HandlerFunc that validates the token from Authorization header.
func AuthMiddleware(authenticator *auth.Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Authenticate the request using the authenticator
		agentID, err := authenticator.Authenticate(c.Request)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Set the agentID in the context for later use
		c.Set("agentID", agentID)
		c.Next()
	}
}

// RateLimitMiddleware returns a gin.HandlerFunc that does rate limiting using token bucket.
// 参数 r 表示每秒允许的请求数，burst 表示突发请求的最大值。
// 如果 r <= 0 或 burst <= 0，将使用默认值 10 QPS 和 20 Burst。
func RateLimitMiddleware(requestsPerSecond float64, burst int) gin.HandlerFunc {
	// 使用配置的参数，如果无效则使用默认值
	if requestsPerSecond <= 0 {
		requestsPerSecond = 10
	}

	if burst <= 0 {
		burst = 20
	}

	// Create a rate limiter: r events per second, burst of burst
	limiter := rate.NewLimiter(rate.Limit(requestsPerSecond), burst)

	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded"})
			c.Abort()
			return
		}
		c.Next()
	}
}
