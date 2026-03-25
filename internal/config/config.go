package config

import (
	"fmt"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Config represents the application configuration.
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Auth      AuthConfig      `mapstructure:"auth"`
	Audit     AuditConfig     `mapstructure:"audit"`
	Execution ExecutionConfig `mapstructure:"execution"`
	Rules     RulesConfig     `mapstructure:"rules"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
}

// ServerConfig holds the server configuration.
type ServerConfig struct {
	Port    int    `mapstructure:"port"`
	Host    string `mapstructure:"host"`
	TLSCert string `mapstructure:"tls_cert"`
	TLSKey  string `mapstructure:"tls_key"`
}

// AuthConfig holds the authentication configuration.
type AuthConfig struct {
	Token string `mapstructure:"token"`
}

// TokenConfig 已废弃，使用 AuthConfig.Token 代替
// TokenConfig holds the token and agent ID mapping.
// type TokenConfig struct {
// 	Token   string `mapstructure:"token"`
// 	AgentID string `mapstructure:"agent_id"`
// }

// AuditConfig holds the audit logging configuration.
type AuditConfig struct {
	LogFile    string `mapstructure:"log_file"`
	AuditFile  string `mapstructure:"audit_file"` // 专用的审计日志文件
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	Compress   bool   `mapstructure:"compress"`
}

// ExecutionConfig holds the execution configuration.
type ExecutionConfig struct {
	MaxOutputLength int `mapstructure:"max_output_length"`
	TimeoutSeconds  int `mapstructure:"timeout_seconds"`
	MaxConcurrent   int `mapstructure:"max_concurrent"`
}

// RulesConfig holds the rules configuration.
type RulesConfig struct {
	VerbAllowlist []string      `mapstructure:"verb_allowlist"`
	VerbBlocklist []string      `mapstructure:"verb_blocklist"`
	Masking       []MaskingRule `mapstructure:"masking"`
}

// RateLimitConfig holds the rate limiting configuration.
type RateLimitConfig struct {
	RequestsPerSecond float64 `mapstructure:"requests_per_second"`
	Burst             int     `mapstructure:"burst"`
}

// MaskingRule represents a masking rule.
type MaskingRule struct {
	Resource   string   `mapstructure:"resource"`
	Namespaces []string `mapstructure:"namespaces"`
	Action     string   `mapstructure:"action"` // mask, drop, filter_fields
	Fields     []string `mapstructure:"fields"` // for filter_fields action
}

// ConfigManager manages the application configuration.
type ConfigManager struct {
	viper *viper.Viper
	cfg   *Config
	mu    sync.RWMutex
}

// New creates a new ConfigManager.
func New(configPath string) (*ConfigManager, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// 启用环境变量支持，环境变量优先级高于配置文件
	v.AutomaticEnv()
	// 绑定特定的环境变量
	v.BindEnv("auth.token", "AGENT_KUBECTL_TOKEN")
	v.BindEnv("server.port", "AGENT_KUBECTL_PORT")
	v.BindEnv("server.host", "AGENT_KUBECTL_HOST")
	v.BindEnv("audit.log_file", "AGENT_KUBECTL_LOG_FILE")
	v.BindEnv("audit.audit_file", "AGENT_KUBECTL_AUDIT_FILE")
	v.BindEnv("audit.level", "AGENT_KUBECTL_AUDIT_LEVEL")

	// Set default values
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("audit.log_file", "./audit.log")
	v.SetDefault("audit.audit_file", "./audit_detail.log")
	v.SetDefault("audit.level", "info")
	v.SetDefault("audit.format", "json")
	v.SetDefault("audit.max_size_mb", 100)
	v.SetDefault("audit.max_backups", 30)
	v.SetDefault("audit.max_age_days", 30)
	v.SetDefault("audit.compress", true)
	v.SetDefault("execution.max_output_length", 10000)
	v.SetDefault("execution.timeout_seconds", 30)
	v.SetDefault("execution.max_concurrent", 10)
	v.SetDefault("rate_limit.requests_per_second", 10)
	v.SetDefault("rate_limit.burst", 20)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cm := &ConfigManager{
		viper: v,
		cfg:   &cfg,
	}

	return cm, nil
}

// Get returns the current configuration.
func (cm *ConfigManager) Get() *Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.cfg
}

// Watch watches for configuration changes.
func (cm *ConfigManager) Watch(onChange func(*Config)) error {
	cm.viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
		var cfg Config
		if err := cm.viper.Unmarshal(&cfg); err != nil {
			fmt.Printf("Error reloading config: %v\n", err)
			return
		}
		cm.mu.Lock()
		cm.cfg = &cfg
		cm.mu.Unlock()
		onChange(&cfg)
	})
	cm.viper.WatchConfig()
	return nil
}

// LoadConfig is a convenience function to load configuration from a file.
func LoadConfig(configPath string) (*Config, error) {
	cm, err := New(configPath)
	if err != nil {
		return nil, err
	}
	return cm.Get(), nil
}

// GetDefaultConfig returns a default configuration for reference.
func GetDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 8080,
			Host: "0.0.0.0",
		},
		Auth: AuthConfig{
			Token: "agent-token-12345",
		},
		Audit: AuditConfig{
			LogFile:    "./audit.log",
			AuditFile:  "./audit_detail.log",
			Level:      "info",
			Format:     "json",
			MaxSizeMB:  100,
			MaxBackups: 30,
			MaxAgeDays: 30,
			Compress:   true,
		},
		Execution: ExecutionConfig{
			MaxOutputLength: 10000,
			TimeoutSeconds:  30,
			MaxConcurrent:   10,
		},
		Rules: RulesConfig{
			VerbAllowlist: []string{"get", "describe", "logs", "apply", "create", "patch", "rollout", "scale", "top"},
			VerbBlocklist: []string{"delete", "exec", "port-forward"},
			Masking: []MaskingRule{
				{Resource: "secrets", Namespaces: []string{"*"}, Action: "mask"},
				{Resource: "*", Namespaces: []string{"*"}, Action: "filter_fields", Fields: []string{"metadata.annotations.kubectl.kubernetes.io/last-applied-configuration", "metadata.managedFields", "metadata.creationTimestamp", "status"}},
			},
		},
		RateLimit: RateLimitConfig{
			RequestsPerSecond: 10,
			Burst:             20,
		},
	}
}
