# PROJECT KNOWLEDGE BASE

**Generated:** 2026-05-12
**Commit:** 8c056dc
**Branch:** main

## OVERVIEW

AI Agent kubectl 安全代理网关。Go + Gin HTTP 服务，接收结构化 JSON 请求，组装为 kubectl 参数执行，返回结果。多层安全防护（认证→审计→动词过滤→结果脱敏→输出截断→限流）。

## STRUCTURE

```
.
├── cmd/agent-kubectl-gateway/main.go   # 入口，组件初始化与启动
├── internal/
│   ├── server/         # Gin HTTP 服务 + 路由 + 中间件 + handler
│   ├── auth/           # Bearer Token 认证
│   ├── audit/          # 双通道日志（general + audit），zap + lumberjack
│   ├── filter/         # 动词白/黑名单 + 结果脱敏(mask/filter_fields)
│   ├── executor/       # kubectl 命令构建(builder.go) + 执行(executor.go)
│   ├── config/         # Viper 配置加载 + fsnotify 热更新
│   └── model/          # 请求/响应/审计数据结构定义
├── configs/config.yaml # 运行配置
├── deploy/             # Dockerfile + K8s manifests
├── docs/               # API 文档 + 安全说明 + 设计方案
├── test/test_features.py # Python 集成测试
└── Makefile
```

## WHERE TO LOOK

| 任务 | 位置 | 说明 |
|------|------|------|
| 新增 API 端点 | `internal/server/server.go` + `handler.go` | 注册路由 + 实现 handler |
| 修改请求/响应结构 | `internal/model/model.go` | 所有数据模型集中定义 |
| 新增 kubectl 参数 | `internal/executor/builder.go` | `BuildArgs()` 方法 |
| 修改安全规则逻辑 | `internal/filter/filter.go` | 动词拦截 + 脱敏规则 |
| 修改认证方式 | `internal/auth/auth.go` + `server/middleware.go` | Authenticator 接口 |
| 调整配置项 | `internal/config/config.go` + `configs/config.yaml` | Config 结构体 + YAML |
| 修改审计日志 | `internal/audit/audit.go` | 双 logger：general + audit |
| 部署相关 | `deploy/` | Dockerfile + K8s deployment/rbac/service |

## CONVENTIONS

- **注释语言**: 代码注释全中文，exported 符号有英文 doc comment
- **包结构**: 标准 Go flat layout，`internal/` 按领域分包，每包 1-2 个主文件 + `_test.go`
- **日志**: 全局 `audit.Info/Error/Warn/Debug`，不传 logger 参数，`[模块名]` 前缀标识来源
- **错误处理**: handler 层返回 JSON `model.ExecutionResult`，含 `blocked_reason` 字段
- **配置映射**: struct tag 用 `mapstructure`，配置热更新用 `fsnotify` + `sync.RWMutex`
- **测试**: 每个包 `_test.go` 同目录，executor 使用 `CommandRunner` 接口 mock
- **命名**: Go 标准 camelCase，JSON 字段 snake_case（`request_id`, `exit_code`）

## ANTI-PATTERNS (THIS PROJECT)

- **禁止 Shell 注入**: 绝不使用 `sh -c` / `bash -c` 执行命令，只通过 `exec.CommandContext` + 参数数组
- **禁止裸字符串命令**: 必须通过 `model.ExecutionRequest` 结构体传入参数
- **禁止跳过审计**: 所有 handler 操作必须记录 audit 日志
- **禁止忽略截断**: 输出必须经过 `limitedWriter`，防止 OOM

## UNIQUE STYLES

- 审计包用全局函数（`audit.Info/Error/Warn/Debug`）而非实例方法，所有模块直接调用
- executor 抽象了 `CommandRunner` 接口用于测试注入
- filter 的 `filter_fields` 动作通过 `sjson`/`gjson` 或 YAML 反序列化操作 JSON/YAML 字段
- handler 中 `generateRequestID` 优先从请求头 `X-Request-ID` 等获取，回退到 UUID

## COMMANDS

```bash
# 构建
make build                  # 输出到 build/agent-kubectl-gateway

# 运行
make run                    # 等效 go run ./cmd/agent-kubectl-gateway
go run ./cmd/agent-kubectl-gateway -config configs/config.yaml

# 测试
make test                   # go test -v ./...

# 代码检查
make lint                   # 需要安装 golangci-lint

# Docker
make docker-build           # 构建镜像 agent-kubectl-gateway:latest

# 依赖
make tidy                   # go mod tidy
```

## NOTES

- 配置文件默认路径 `configs/config.yaml`，可通过 `-config` 参数覆盖
- 认证 Token 为空时所有请求直接通过（开发模式），**生产必须配置**
- `limitedWriter.Write()` 在截断后仍返回成功（`return len(p), nil`），这是防死锁设计
- `audit.go` 有双通道：`generalLogger` → `log_file`，`auditLogger` → `audit_file`
- 测试用 Python 写的集成测试在 `test/test_features.py`，需要运行中的服务
- Go 版本 `1.25.1`，Dockerfile 中构建阶段用 `golang:1.21-alpine`（版本不一致，待更新）
