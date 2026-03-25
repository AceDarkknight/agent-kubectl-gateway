# AI Agent kubectl 代理 CLI 工具实现计划 (agent-kubectl-gateway)

## 1. 项目概述

本项目旨在开发一个安全的 kubectl 代理工具，允许 AI Agent 通过 HTTP API 执行 kubectl 命令，同时提供认证、审计、命令拦截和结果脱敏等安全机制。

## 2. 预期目录结构

```
agent-kubectl-gateway/
├── cmd/
│   └── agent-kubectl-gateway/
│       └── main.go                 # 程序入口，初始化配置、启动 HTTP Server
├── internal/
│   ├── server/
│   │   ├── server.go               # HTTP Server 初始化与路由注册
│   │   ├── handler.go              # 请求处理函数（/execute 端点）
│   │   └── middleware.go           # 中间件（认证、审计、限流等）
│   ├── auth/
│   │   └── auth.go                 # 请求验证
│   ├── audit/
│   │   ├── audit.go                # 审计日志记录核心逻辑
│   │   └── logger.go               # 日志轮转与持久化（基于 lumberjack）
│   ├── filter/
│   │   ├── filter.go               # 命令前置拦截与结果后置过滤
│   │   ├── rules.go                # 拦截规则与脱敏规则解析
│   │   └── masker.go               # 敏感数据脱敏实现
│   ├── executor/
│   │   ├── executor.go             # 命令执行核心逻辑（调用 kubectl）
│   │   ├── builder.go              # 从结构化输入组装参数数组
│   │   └── result.go               # 执行结果标准化结构
│   ├── config/
│   │   └── config.go               # 配置文件加载与热更新（基于 Viper）
│   └── model/
│       └── model.go                # 共享数据结构定义（请求、响应、错误码等）
├── configs/
│   └── config.yaml                 # 默认配置文件模板
├── scripts/
│   ├── build.sh                    # 构建脚本
│   └── install.sh                  # 安装脚本
├── deploy/
│   ├── Dockerfile                  # Docker 镜像构建文件
│   └── k8s/
│       ├── deployment.yaml         # Kubernetes Deployment 配置
│       └── service.yaml            # Kubernetes Service 配置
├── docs/
│   ├── api.md                      # API 接口文档
│   └── security.md                 # 安全机制说明文档
├── go.mod                          # Go 模块依赖定义
├── go.sum                          # 依赖版本锁定
├── Makefile                        # 构建、测试、部署命令集合
└── README.md                       # 项目说明文档
```

## 3. 实现阶段划分

### 第一阶段：核心基础设施（P0）

**目标**：搭建项目骨架，实现核心执行流程

#### 3.1 项目初始化

**步骤**：
1. 初始化 Go 模块，创建 `go.mod` 文件
2. 创建项目目录结构
3. 配置依赖管理（Gin、Viper、Zap、Lumberjack 等）
4. 创建 `Makefile`，定义构建、测试、运行命令

**预期效果**：
- 项目结构清晰，符合 Go 语言标准项目布局
- 依赖管理完善，可通过 `go mod tidy` 安装所有依赖
- 支持 `make build`、`make test`、`make run` 等常用命令

#### 3.2 数据模型定义

**步骤**：
1. 创建 `internal/model/model.go`，定义共享数据结构
2. 定义请求结构体（`ExecutionRequest`），包含 verb、resource、namespace、name、subresource、options、output、mode 等字段
3. 定义响应结构体（`ExecutionResult`），包含 request_id、status、exit_code、stdout、stderr、truncated、duration_ms、response_size_bytes、blocked_reason 等字段
4. 定义错误码枚举（`ERR_INVALID_COMMAND`、`ERR_BLOCKED_VERB`、`ERR_KUBECTL_NOT_FOUND`、`ERR_EXEC_TIMEOUT`、`ERR_NON_ZERO_EXIT`、`ERR_OUTPUT_TOO_LARGE`）

**预期效果**：
- 数据结构清晰，字段命名规范
- 支持 JSON 序列化/反序列化
- 错误码体系完整，便于上层模块识别和处理

#### 3.3 配置管理模块

**步骤**：
1. 创建 `internal/config/config.go`，实现配置加载逻辑
2. 使用 Viper 库加载 YAML 配置文件
3. 定义配置结构体，包含 server、auth、audit、execution、rules 等配置段
4. 实现配置热更新机制，使用 fsnotify 监控配置文件变化
5. 创建 `configs/config.yaml` 默认配置文件模板

**预期效果**：
- 支持从 YAML 文件加载配置
- 支持配置热更新，修改配置文件后无需重启服务
- 配置结构清晰，各模块配置独立

#### 3.4 HTTP Server 搭建

**步骤**：
1. 创建 `internal/server/server.go`，初始化 Gin HTTP Server
2. 注册路由，定义 `/execute` 端点（POST 方法）
3. 创建 `internal/server/handler.go`，实现请求处理函数
4. 实现请求参数绑定，将 JSON 请求体绑定到 `ExecutionRequest` 结构体
5. 实现响应组装，将 `ExecutionResult` 序列化为 JSON 返回

**预期效果**：
- HTTP Server 可正常启动，监听配置的端口
- `/execute` 端点可接收 POST 请求
- 请求参数绑定正确，响应格式规范

#### 3.5 认证模块

**步骤**：
1. 创建 `internal/auth/auth.go`，实现请求验证逻辑
2. 验证请求合法性

**预期效果**：
- 请求验证逻辑正确
- 错误信息清晰，便于排查问题

#### 3.6 执行模块

**步骤**：
1. 创建 `internal/executor/executor.go`，实现命令执行核心逻辑
2. 创建 `internal/executor/builder.go`，实现从结构化输入组装参数数组
3. 创建 `internal/executor/result.go`，定义执行结果标准化结构
4. 使用 `os/exec` 标准库调用本地 kubectl 二进制
5. 实现超时控制，使用 `context.WithTimeout`
6. 实现流式读取 stdout，使用 `io.LimitReader` + `bufio.Scanner`
7. 实现输出截断机制，超过 `max_output_length` 时截断并追加提示

**预期效果**：
- 命令执行正确，可成功调用 kubectl
- 参数组装正确，从结构化输入构建参数数组
- 超时控制有效，防止命令无限等待
- 输出截断机制正常，大输出不会导致内存溢出

### 第二阶段：安全增强（P1）

**目标**：增强安全性和可观测性

#### 3.7 鉴权/过滤模块

**步骤**：
1. 创建 `internal/filter/filter.go`，实现命令前置拦截与结果后置过滤
2. 创建 `internal/filter/rules.go`，实现拦截规则与脱敏规则解析
3. 创建 `internal/filter/masker.go`，实现敏感数据脱敏
4. 实现动词白名单/黑名单校验，根据配置拦截不允许执行的命令
5. 实现基于正则表达式的脱敏，对非结构化文本进行敏感信息替换
6. 实现结构化数据的字段过滤，对 JSON/YAML 输出移除指定字段

**预期效果**：
- 命令拦截正确，黑名单动词（如 delete、exec）被拦截
- 白名单优先级高于黑名单
- 敏感数据脱敏有效，Secret 内容被替换为 `*** MASKED BY PROXY ***`
- 字段过滤正确，指定字段被移除

#### 3.8 审计/日志模块

**步骤**：
1. 创建 `internal/audit/audit.go`，实现审计日志记录核心逻辑
2. 创建 `internal/audit/logger.go`，实现日志轮转与持久化
3. 使用 Zap 库实现高性能结构化日志
4. 使用 Lumberjack 库实现日志轮转，支持按大小、时间轮转
5. 记录请求全生命周期信息，包括 timestamp、source_ip、command、status、duration_ms、response_size_bytes、error_message 等字段
6. 日志格式强制为 JSON，便于接入 ELK、Loki 等日志分析系统

**预期效果**：
- 审计日志记录完整，包含所有关键字段
- 日志格式为 JSON，便于后续分析
- 日志轮转正常，防止日志文件撑爆磁盘
- 日志性能良好，不影响主流程性能

#### 3.9 中间件编排

**步骤**：
1. 创建 `internal/server/middleware.go`，实现中间件编排
2. 实现认证中间件，验证请求合法性
3. 实现审计中间件，记录请求和响应日志
4. 实现限流中间件，使用 `golang.org/x/time/rate` 实现令牌桶限流
5. 实现错误处理中间件，统一处理 panic 和错误

**预期效果**：
- 中间件编排正确，请求处理流程清晰
- 认证中间件可正确验证请求合法性
- 审计中间件可记录完整的请求生命周期
- 限流中间件可防止恶意请求

### 第三阶段：部署与文档（P2）

**目标**：完善部署配置和文档

#### 3.10 Docker 镜像构建

**步骤**：
1. 创建 `deploy/Dockerfile`，定义多阶段构建
2. 第一阶段：使用 golang 镜像编译 Go 程序
3. 第二阶段：使用 alpine 镜像运行程序，减小镜像体积
4. 配置 kubectl 工具，确保容器内可执行 kubectl

**预期效果**：
- Docker 镜像构建成功，体积合理
- 容器内可正常执行 kubectl 命令
- 支持通过环境变量或配置文件注入配置

#### 3.11 Kubernetes 部署配置

**步骤**：
1. 创建 `deploy/k8s/deployment.yaml`，定义 Kubernetes Deployment
2. 创建 `deploy/k8s/service.yaml`，定义 Kubernetes Service
3. 配置资源限制（CPU、内存）
4. 配置健康检查（livenessProbe、readinessProbe）
5. 配置 ConfigMap 挂载配置文件
6. 配置 Secret 挂载 TLS 证书

**预期效果**：
- Deployment 配置正确，可正常部署到 Kubernetes 集群
- Service 配置正确，可对外暴露服务
- 资源限制合理，防止资源耗尽
- 健康检查有效，可自动重启异常 Pod

#### 3.12 文档编写

**步骤**：
1. 创建 `README.md`，编写项目说明文档
2. 创建 `docs/api.md`，编写 API 接口文档
3. 创建 `docs/security.md`，编写安全机制说明文档
4. 编写使用示例，展示如何调用 API

**预期效果**：
- README.md 清晰说明项目用途、安装方式、使用方法
- API 文档完整，包含接口说明、请求参数、响应参数、错误码说明
- 安全文档详细，说明认证、授权、审计、脱敏等安全机制
- 使用示例丰富，便于用户快速上手

## 4. 实现步骤详细清单

### 4.1 第一阶段：核心基础设施（P0）

| 步骤 | 任务 | 优先级 | 预期产出 |
|------|------|--------|----------|
| 1.1 | 初始化 Go 模块，创建 go.mod | P0 | go.mod 文件 |
| 1.2 | 创建项目目录结构 | P0 | 完整的目录结构 |
| 1.3 | 配置依赖管理 | P0 | go.sum 文件 |
| 1.4 | 创建 Makefile | P0 | Makefile 文件 |
| 1.5 | 定义数据模型 | P0 | internal/model/model.go |
| 1.6 | 实现配置管理模块 | P0 | internal/config/config.go |
| 1.7 | 创建默认配置文件 | P0 | configs/config.yaml |
| 1.8 | 搭建 HTTP Server | P0 | internal/server/server.go |
| 1.9 | 实现请求处理函数 | P0 | internal/server/handler.go |
| 1.10 | 实现认证模块 | P0 | internal/auth/auth.go |
| 1.11 | 实现执行模块 | P0 | internal/executor/executor.go, builder.go, result.go |
| 1.12 | 创建程序入口 | P0 | cmd/agent-kubectl-gateway/main.go |

### 4.2 第二阶段：安全增强（P1）

| 步骤 | 任务 | 优先级 | 预期产出 |
|------|------|--------|----------|
| 2.1 | 实现鉴权/过滤模块 | P1 | internal/filter/filter.go, rules.go, masker.go |
| 2.2 | 实现审计/日志模块 | P1 | internal/audit/audit.go, logger.go |
| 2.3 | 实现中间件编排 | P1 | internal/server/middleware.go |
| 2.4 | 编写单元测试 | P1 | 各模块的 _test.go 文件 |

### 4.3 第三阶段：部署与文档（P2）

| 步骤 | 任务 | 优先级 | 预期产出 |
|------|------|--------|----------|
| 3.1 | 创建 Dockerfile | P2 | deploy/Dockerfile |
| 3.2 | 创建 Kubernetes 部署配置 | P2 | deploy/k8s/deployment.yaml, service.yaml |
| 3.3 | 编写 README.md | P2 | README.md |
| 3.4 | 编写 API 文档 | P2 | docs/api.md |
| 3.5 | 编写安全文档 | P2 | docs/security.md |

## 5. 预期效果

### 5.1 功能效果

1. **命令执行**：代理工具接收结构化输入，组装参数数组，安全调用 kubectl 二进制
2. **命令拦截**：根据配置的动词白名单/黑名单，拦截不允许执行的命令（如 delete、exec）
3. **结果脱敏**：对 kubectl 返回的结果进行脱敏处理，如将 Secret 内容替换为 `*** MASKED BY PROXY ***`
4. **输出截断**：当命令输出超过配置的最大长度时，自动截断并追加提示信息
5. **审计日志**：记录所有请求的详细信息，包括时间戳、来源 IP、命令、状态、耗时等

### 5.2 安全效果

1. **防命令注入**：采用结构化输入，从结构化字段直接组装参数数组，从根本上避免 Shell 注入风险
2. **传输安全**：支持 HTTPS (TLS) 加密通信，防止数据在网络中被窃听
3. **最小权限**：运行代理工具的系统用户仅具有执行 kubectl 的权限
4. **限流防刷**：在 API 层添加 Rate Limit，防止恶意 Agent 发起大量请求
5. **审计追踪**：完整记录请求生命周期，支持事后审计和问题排查

### 5.3 性能效果

1. **高性能日志**：使用 Zap 库实现高性能结构化日志，性能远超标准库 log
2. **流式读取**：使用 `io.LimitReader` + `bufio.Scanner` 流式读取输出，防止大输出导致内存溢出
3. **并发控制**：使用信号量限制并发执行数量，防止资源耗尽
4. **超时保护**：使用 `context.WithTimeout` 实现命令执行超时保护

### 5.4 可维护性效果

1. **模块化设计**：各模块职责单一，边界清晰，便于维护和扩展
2. **配置驱动**：通过 YAML 配置文件管理所有配置，支持热更新
3. **完善文档**：提供 README、API 文档、安全文档，便于用户理解和使用
4. **单元测试**：各模块编写单元测试，确保代码质量

## 6. 技术栈总结

| 类别 | 技术选型 | 用途 |
|------|----------|------|
| 编程语言 | Go 1.21+ | 主要开发语言 |
| Web 框架 | Gin | HTTP API 服务 |
| 配置管理 | Viper | YAML 配置加载与热更新 |
| 日志库 | Zap + Lumberjack | 高性能结构化日志与日志轮转 |
| 限流器 | golang.org/x/time/rate | API 限流 |
| 文件监控 | fsnotify | 配置文件变化监控 |
| JSON 处理 | tidwall/gjson + tidwall/sjson | 高效 JSON 字段操作 |
| YAML 处理 | gopkg.in/yaml.v3 | YAML 解析与序列化 |

## 7. 风险与应对

| 风险 | 影响 | 应对措施 |
|------|------|----------|
| kubectl 未安装或路径错误 | 命令执行失败 | 在启动时检查 kubectl 是否可用，提供清晰的错误提示 |
| 配置文件格式错误 | 服务启动失败 | 使用 Viper 的严格模式，提供详细的配置验证错误信息 |
| 大输出导致内存溢出 | 服务崩溃 | 使用流式读取 + 输出截断机制，限制最大输出长度 |
| 并发请求过多 | 资源耗尽 | 使用信号量限制并发数量，超出限制时排队等待或拒绝 |
| 日志文件撑爆磁盘 | 磁盘空间不足 | 使用 Lumberjack 实现日志轮转，配置最大保留天数和文件大小 |

## 8. 验收标准

### 8.1 功能验收

- [ ] 代理工具可正常启动，监听配置的端口
- [ ] `/execute` 端点可接收 POST 请求，返回 JSON 响应
- [ ] 命令执行正确，可成功调用 kubectl
- [ ] 命令拦截正确，黑名单动词被拦截
- [ ] 结果脱敏有效，敏感数据被替换
- [ ] 输出截断正常，大输出被截断并追加提示
- [ ] 审计日志记录完整，包含所有关键字段

### 8.2 安全验收

- [ ] 结构化输入防注入，无法通过输入执行任意命令
- [ ] HTTPS 通信加密，Token 不会被窃听
- [ ] 限流机制有效，可防止恶意请求
- [ ] 审计日志完整，支持事后审计

### 8.3 性能验收

- [ ] 单次请求处理时间 < 500ms（不含 kubectl 执行时间）
- [ ] 支持 100 并发请求，无性能下降
- [ ] 大输出（10MB）处理时间 < 2s
- [ ] 日志记录不影响主流程性能

### 8.4 可维护性验收

- [ ] 代码结构清晰，模块职责单一
- [ ] 单元测试覆盖率 > 80%
- [ ] 文档完整，包含 README、API 文档、安全文档
- [ ] 配置文件清晰，支持热更新

## 9. 后续优化方向

1. **Prometheus 监控**：接入 Prometheus 监控系统，暴露请求量、响应时间、错误率等指标
2. **多集群支持**：支持配置多个 Kubernetes 集群，根据请求参数选择目标集群
3. **命令缓存**：对频繁查询的命令结果进行缓存，减少 kubectl 调用次数
4. **WebSocket 支持**：支持 WebSocket 长连接，实现实时日志跟踪
5. **Web UI**：提供 Web 管理界面，便于查看审计日志和管理配置
