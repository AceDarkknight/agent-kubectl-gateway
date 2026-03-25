package audit

import (
	"os"
	"path/filepath"
	"time"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/config"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/model"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

// generalLogger 通用日志记录器，只写入 log_file
var generalLogger *zap.Logger

// auditLogger 审计日志记录器，只写入 audit_file
var auditLogger *zap.Logger

// Init 初始化审计日志记录器
// 设置全局日志记录器，如果配置为空则不初始化
func Init(cfg config.AuditConfig) error {
	// 解析日志级别
	level := zapcore.InfoLevel
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	// 创建编码器配置
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 确定编码器格式
	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 创建通用的日志写入器（LogFile）
	var generalSyncer zapcore.WriteSyncer
	if cfg.LogFile != "" {
		// 确保日志文件目录存在
		if err := os.MkdirAll(filepath.Dir(cfg.LogFile), 0755); err != nil {
			return err
		}
		generalSyncer = zapcore.AddSync(&lumberjack.Logger{
			Filename:   cfg.LogFile,
			MaxSize:    cfg.MaxSizeMB, // MB
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays, // days
			Compress:   cfg.Compress,
		})
	} else {
		generalSyncer = zapcore.AddSync(os.Stdout)
	}

	// 创建专用的审计日志写入器（AuditFile）
	var auditSyncer zapcore.WriteSyncer
	if cfg.AuditFile != "" {
		// 确保审计日志文件目录存在
		if err := os.MkdirAll(filepath.Dir(cfg.AuditFile), 0755); err != nil {
			return err
		}
		auditSyncer = zapcore.AddSync(&lumberjack.Logger{
			Filename:   cfg.AuditFile,
			MaxSize:    cfg.MaxSizeMB, // MB
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays, // days
			Compress:   cfg.Compress,
		})
	} else {
		auditSyncer = zapcore.AddSync(os.Stdout)
	}

	// 创建通用日志记录器，只写入 log_file
	generalCore := zapcore.NewCore(encoder, generalSyncer, level)
	generalLogger = zap.New(generalCore, zap.AddCaller(), zap.AddCallerSkip(1))

	// 创建审计日志记录器，只写入 audit_file
	auditCore := zapcore.NewCore(encoder, auditSyncer, level)
	auditLogger = zap.New(auditCore, zap.AddCaller(), zap.AddCallerSkip(1))

	return nil
}

// Info 记录通用日志 - Info 级别
func Info(msg string, fields ...zap.Field) {
	if generalLogger != nil {
		generalLogger.Info(msg, fields...)
	}
}

// Error 记录通用日志 - Error 级别
func Error(msg string, fields ...zap.Field) {
	if generalLogger != nil {
		generalLogger.Error(msg, fields...)
	}
}

// Warn 记录通用日志 - Warn 级别
func Warn(msg string, fields ...zap.Field) {
	if generalLogger != nil {
		generalLogger.Warn(msg, fields...)
	}
}

// Debug 记录通用日志 - Debug 级别
func Debug(msg string, fields ...zap.Field) {
	if generalLogger != nil {
		generalLogger.Debug(msg, fields...)
	}
}

// GetLogger 获取通用日志记录器，供需要更多控制的高级用法
func GetLogger() *zap.Logger {
	return generalLogger
}

// Log 记录审计日志条目
func Log(entry model.AuditLogEntry) error {
	// 如果审计日志记录器未初始化，则静默忽略
	if auditLogger == nil {
		return nil
	}
	// 使用审计日志记录器记录结构化日志
	auditLogger.Info("audit",
		zap.Time("timestamp", entry.Timestamp),
		zap.String("source_ip", entry.SourceIP),
		zap.String("agent_id", entry.AgentID),
		zap.String("command", entry.Command),
		zap.String("status", entry.Status),
		zap.Int64("duration_ms", entry.DurationMs),
		zap.Int("response_size", entry.ResponseSize),
		zap.String("error_message", entry.ErrorMessage),
	)

	return nil
}

// LogRequest 记录请求审计日志
func LogRequest(req *model.ExecutionRequest, agentID, sourceIP, status string, durationMs int64, responseSize int, errorMessage string) error {
	entry := model.AuditLogEntry{
		Timestamp:    time.Now().UTC(),
		SourceIP:     sourceIP,
		AgentID:      agentID,
		Command:      "kubectl " + req.Verb + " " + req.Resource + " " + req.Name,
		Status:       status,
		DurationMs:   durationMs,
		ResponseSize: responseSize,
		ErrorMessage: errorMessage,
	}

	return Log(entry)
}

// Close 关闭日志记录器
func Close() error {
	var errs []error

	if generalLogger != nil {
		if err := generalLogger.Sync(); err != nil {
			errs = append(errs, err)
		}
	}

	if auditLogger != nil {
		if err := auditLogger.Sync(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}
