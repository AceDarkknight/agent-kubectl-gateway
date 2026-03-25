package auth

import (
	"errors"
	"net/http"
	"strings"
)

// 默认的 Agent ID，当 Token 验证通过后返回
const DefaultAgentID = "default"

// Authenticator handles token authentication.
type Authenticator struct {
	token string // 配置的 Token
}

// NewAuthenticator creates a new Authenticator with the given token.
func NewAuthenticator(token string) *Authenticator {
	return &Authenticator{token: token}
}

// Authenticate validates the request and returns the Agent ID.
// 提取 Token 并与配置的 Token 进行比较验证。
func (a *Authenticator) Authenticate(r *http.Request) (string, error) {
	// 从 Header 中提取 Token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("missing authorization header")
	}

	// 解析 Bearer Token
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", errors.New("invalid authorization header format")
	}

	// 验证 Token 是否与配置的 Token 匹配
	if a.token != "" && parts[1] != a.token {
		return "", errors.New("invalid token")
	}

	// Token 验证通过，返回空的 Agent ID
	return "", nil
}
