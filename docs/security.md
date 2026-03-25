# 安全说明文档

本文档详细阐述 agent-kubectl-gateway 网关实现的五大核心安全防线机制，确保 AI Agent 对 Kubernetes 集群的操作安全可控。

## 目录

- [安全架构概述](#安全架构概述)
- [一、动词白名单与黑名单机制](#一动词白名单与黑名单机制)
- [二、敏感数据脱敏保护 (Masking)](#二敏感数据脱敏保护-masking)
- [三、冗余字段过滤瘦身 (Filtering)](#三冗余字段过滤瘦身-filtering)
- [四、超大报文自动截断防御 (Max Output Length)](#四超大报文自动截断防御-max-output-length)
- [五、令牌桶限流防刷机制 (Rate Limiting)](#五令牌桶限流防刷机制-rate-limiting)
- [安全配置最佳实践](#安全配置最佳实践)

---

## 安全架构概述

agent-kubectl-gateway 采用多层纵深防御策略，在请求处理流程的各个阶段实施安全控制：

```
┌─────────────────────────────────────────────────────────────────────┐
│                        AI Agent 请求                                 │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  第一道防线：令牌桶限流 (Rate Limiting)                               │
│  • 防止请求洪泛攻击                                                   │
│  • 保护后端资源不被耗尽                                               │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  认证层：Bearer Token 验证                                           │
│  • 验证请求合法性                                                     │
│  • 识别 Agent 身份                                                   │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  第二道防线：动词白名单/黑名单 (Verb Allowlist/Blocklist)             │
│  • 控制可执行的操作类型                                               │
│  • 阻止危险操作                                                       │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  命令执行与输出捕获                                                   │
│  • 结构化参数构建 kubectl 命令                                        │
│  • 执行命令并捕获输出                                                 │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  第三道防线：超大报文截断 (Max Output Length)                         │
│  • 限制输出大小                                                       │
│  • 防止内存溢出                                                       │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  第四道防线：敏感数据脱敏 (Masking)                                   │
│  • 掩码 Secret 数据                                                  │
│  • 隐藏敏感信息                                                       │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  第五道防线：冗余字段过滤 (Filtering)                                 │
│  • 移除不必要的元数据                                                 │
│  • 减小返回体积                                                       │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        安全响应返回                                   │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 一、动词白名单与黑名单机制

### 1.1 机制说明

动词白名单与黑名单是控制 AI Agent 可执行操作类型的第一道业务防线。通过配置允许或禁止的 kubectl 操作动词，精确控制操作权限。

### 1.2 工作原理

```
┌──────────────────────────────────────────────────────────────┐
│                    动词校验流程                               │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│   请求动词 ──► 是否配置白名单？                               │
│                    │                                         │
│         ┌─────────┴─────────┐                               │
│         │ Yes               │ No                            │
│         ▼                   ▼                               │
│   动词在白名单中？      是否配置黑名单？                       │
│         │                   │                               │
│    ┌────┴────┐         ┌────┴────┐                         │
│    │ Yes     │ No      │ Yes     │ No                      │
│    ▼         ▼         ▼         ▼                         │
│   放行     拦截    动词在黑名单中？   放行                     │
│                      │                                       │
│                 ┌────┴────┐                                  │
│                 │ Yes     │ No                              │
│                 ▼         ▼                                  │
│                拦截      放行                                 │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 1.3 配置说明

在 [`configs/config.yaml`](../configs/config.yaml) 中配置：

```yaml
rules:
  # 动词白名单：允许执行的 kubectl 操作动词
  # 注意：白名单优先级高于黑名单，如果配置了白名单，则只允许白名单中的动词
  verb_allowlist:
    - "get"
    - "describe"
    - "logs"
    - "apply"
    - "create"
    - "patch"
    - "rollout"
    - "scale"
    - "top"

  # 动词黑名单：禁止执行的 kubectl 操作动词
  # 注意：黑名单仅在未配置白名单时生效
  verb_blocklist:
    - "delete"
    - "exec"
    - "port-forward"
```

### 1.4 优先级规则

| 场景 | 白名单 | 黑名单 | 结果 |
|------|--------|--------|------|
| 配置了白名单，动词在白名单中 | ✓ | - | **放行** |
| 配置了白名单，动词不在白名单中 | ✓ | - | **拦截** |
| 未配置白名单，动词在黑名单中 | ✗ | ✓ | **拦截** |
| 未配置白名单，动词不在黑名单中 | ✗ | ✓ | **放行** |
| 两者都未配置 | ✗ | ✗ | **放行** |

### 1.5 实现代码

核心验证逻辑位于 [`internal/executor/executor.go`](../internal/executor/executor.go)：

```go
// validateVerb validates the verb against allowlist/blocklist.
func (e *Executor) validateVerb(verb string) error {
    // 如果配置了白名单，只允许白名单中的动词
    if len(e.config.Rules.VerbAllowlist) > 0 {
        for _, allowed := range e.config.Rules.VerbAllowlist {
            if verb == allowed {
                return nil
            }
        }
        return fmt.Errorf("verb '%s' is not in the allowlist", verb)
    }

    // 如果配置了黑名单，禁止黑名单中的动词
    if len(e.config.Rules.VerbBlocklist) > 0 {
        for _, blocked := range e.config.Rules.VerbBlocklist {
            if verb == blocked {
                return fmt.Errorf("verb '%s' is blocked by policy", verb)
            }
        }
    }

    return nil
}
```

### 1.6 被拦截时的响应

```json
{
  "request_id": "req-12345678",
  "status": "blocked",
  "exit_code": -1,
  "blocked_reason": "verb 'delete' is blocked by policy",
  "duration_ms": 0,
  "response_size_bytes": 0
}
```

---

## 二、敏感数据脱敏保护 (Masking)

### 2.1 机制说明

敏感数据脱敏机制用于保护 Kubernetes 集群中的敏感信息，防止 AI Agent 意外获取或泄露密码、密钥、Token 等机密数据。

### 2.2 脱敏范围

| 数据类型 | 脱敏方式 | 触发条件 |
|----------|----------|----------|
| Secret 的 data 字段 | 替换为 `*** MASKED BY PROXY ***` | 资源类型为 `secrets` |
| Secret 的 stringData 字段 | 替换为 `*** MASKED BY PROXY ***` | 资源类型为 `secrets` |
| Base64 编码数据（40+字符） | 替换为 `*** MASKED BY PROXY ***` | 非结构化文本输出 |
| Bearer Token | 替换为 `Bearer *** MASKED BY PROXY ***` | 非结构化文本输出 |
| 密码字段 | 替换为 `password: *** MASKED BY PROXY ***` | 非结构化文本输出 |
| 私钥内容 | 替换为 `*** MASKED BY PROXY (PRIVATE KEY) ***` | 非结构化文本输出 |

### 2.3 工作原理

#### JSON/YAML 格式脱敏

```
┌──────────────────────────────────────────────────────────────┐
│                  JSON/YAML 脱敏流程                          │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│   原始输出 ──► 解析 JSON/YAML                                 │
│                    │                                         │
│                    ▼                                         │
│              检查资源类型                                     │
│                    │                                         │
│         ┌─────────┴─────────┐                               │
│         │ kind=Secret       │ 其他资源                       │
│         ▼                   ▼                               │
│   掩码 data/stringData   保持原样                            │
│         │                   │                               │
│         └─────────┬─────────┘                               │
│                   ▼                                          │
│              序列化返回                                       │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

#### 非结构化文本脱敏

对于非 JSON/YAML 格式的输出（如默认输出格式），使用正则表达式进行模式匹配和替换：

| 正则模式 | 匹配内容 |
|----------|----------|
| `[A-Za-z0-9+/]{40,}={0,2}` | Base64 编码数据（40+字符） |
| `(?i)bearer\s+[A-Za-z0-9\-._~+/]+=*` | Bearer Token |
| `(?i)(password\|passwd\|pwd)\s*[:=]\s*\S+` | 密码字段 |
| `-----BEGIN [A-Z ]+ PRIVATE KEY-----[\s\S]*?-----END [A-Z ]+ PRIVATE KEY-----` | 私钥内容 |

### 2.4 配置说明

```yaml
rules:
  masking:
    # 对所有命名空间的 secrets 进行脱敏
    - resource: "secrets"
      namespaces: ["*"]
      action: "mask"
```

### 2.5 实现代码

核心脱敏逻辑位于 [`internal/filter/filter.go`](../internal/filter/filter.go)：

```go
// maskSingleResource 对单个资源进行脱敏处理
func (f *Filter) maskSingleResource(data map[string]interface{}) {
    // 检查是否是 Secret 资源
    if kind, ok := data["kind"].(string); ok && kind == "Secret" {
        // 掩码 data 字段
        if dataMap, ok := data["data"].(map[string]interface{}); ok {
            for key := range dataMap {
                dataMap[key] = "*** MASKED BY PROXY ***"
            }
        }
        // 掩码 stringData 字段
        if stringDataMap, ok := data["stringData"].(map[string]interface{}); ok {
            for key := range stringDataMap {
                stringDataMap[key] = "*** MASKED BY PROXY ***"
            }
        }
    }
}
```

### 2.6 脱敏前后对比

**脱敏前：**

```json
{
  "kind": "Secret",
  "data": {
    "username": "YWRtaW4=",
    "password": "c3VwZXItc2VjcmV0LXBhc3N3b3Jk"
  }
}
```

**脱敏后：**

```json
{
  "kind": "Secret",
  "data": {
    "username": "*** MASKED BY PROXY ***",
    "password": "*** MASKED BY PROXY ***"
  }
}
```

---

## 三、冗余字段过滤瘦身 (Filtering)

### 3.1 机制说明

Kubernetes 资源对象包含大量元数据字段，其中许多字段对 AI Agent 来说是冗余的或不必要的。字段过滤机制自动移除这些字段，减小返回数据体积，提升传输效率。

### 3.2 默认过滤字段

| 字段路径 | 说明 | 过滤原因 |
|----------|------|----------|
| `metadata.annotations.kubectl.kubernetes.io/last-applied-configuration` | kubectl apply 的配置快照 | 通常包含完整的 YAML，体积大 |
| `metadata.managedFields` | 字段管理历史 | 元数据冗余，体积大 |
| `metadata.creationTimestamp` | 创建时间戳 | 通常不需要 |
| `status` | 资源状态 | 部分场景不需要，且体积较大 |

### 3.3 工作原理

```
┌──────────────────────────────────────────────────────────────┐
│                  字段过滤流程                                 │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│   原始输出 ──► 解析 JSON/YAML                                 │
│                    │                                         │
│                    ▼                                         │
│              匹配过滤规则                                     │
│                    │                                         │
│         ┌─────────┴─────────┐                               │
│         │ 匹配规则           │ 不匹配                        │
│         ▼                   ▼                               │
│   递归删除指定字段       保持原样                             │
│         │                   │                               │
│         └─────────┬─────────┘                               │
│                   ▼                                          │
│              序列化返回                                       │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 3.4 配置说明

```yaml
rules:
  masking:
    # 字段过滤规则：移除特定的 JSONPath/YAML 字段
    - resource: "*"
      namespaces: ["*"]
      action: "filter_fields"
      fields:
        - "metadata.annotations.kubectl.kubernetes.io/last-applied-configuration"
        - "metadata.managedFields"
        - "metadata.creationTimestamp"
        - "status"
      description: "过滤掉冗余字段，减小返回体积"
```

### 3.5 实现代码

核心过滤逻辑位于 [`internal/filter/filter.go`](../internal/filter/filter.go)：

```go
// removeJSONField recursively removes a field from JSON data.
// 添加深度限制防止栈溢出
func (f *Filter) removeJSONField(data map[string]interface{}, path []string) {
    if len(path) == 0 {
        return
    }

    // 递归深度保护：最大深度 20 层
    if len(path) > 20 {
        return
    }

    key := path[0]
    if len(path) == 1 {
        // 到达目标字段，直接删除
        delete(data, key)
        return
    }

    // 继续递归
    if nested, ok := data[key].(map[string]interface{}); ok {
        f.removeJSONField(nested, path[1:])
    }
}
```

### 3.6 过滤前后对比

**过滤前（约 5KB）：**

```json
{
  "kind": "Deployment",
  "metadata": {
    "name": "my-app",
    "creationTimestamp": "2024-01-15T10:30:00Z",
    "managedFields": [
      { "manager": "kubectl", "operation": "Update", "time": "2024-01-15T10:30:00Z" }
    ],
    "annotations": {
      "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",...}"
    }
  },
  "status": {
    "conditions": [...],
    "replicas": 3
  }
}
```

**过滤后（约 1KB）：**

```json
{
  "kind": "Deployment",
  "metadata": {
    "name": "my-app"
  }
}
```

---

## 四、超大报文自动截断防御 (Max Output Length)

### 4.1 机制说明

超大报文截断机制防止 kubectl 命令输出过大导致内存溢出、网络传输超时或 AI Agent 上下文溢出。

### 4.2 工作原理

```
┌──────────────────────────────────────────────────────────────┐
│                  输出截断流程                                 │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│   kubectl 命令执行 ──► 输出写入 limitedWriter                 │
│                              │                               │
│                              ▼                               │
│                    检查写入后大小                             │
│                              │                               │
│               ┌──────────────┴──────────────┐               │
│               │ 未超限                       │ 超限          │
│               ▼                              ▼               │
│           继续写入                      标记 truncated       │
│               │                          丢弃后续数据         │
│               │                              │               │
│               └──────────────┬──────────────┘               │
│                              ▼                               │
│                    返回结果 + 截断标记                        │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 4.3 配置说明

```yaml
execution:
  max_output_length: 10000  # 最大输出长度（字节）
  timeout_seconds: 30       # 命令执行超时（秒）
```

### 4.4 实现代码

核心截断逻辑位于 [`internal/executor/executor.go`](../internal/executor/executor.go)：

```go
// limitedWriter 是一个带截断功能的 Writer，用于安全地捕获命令输出
type limitedWriter struct {
    buf       strings.Builder
    maxLen    int
    truncated bool
}

// Write 实现 io.Writer 接口，在写入时检查是否超过限制
func (w *limitedWriter) Write(p []byte) (n int, err error) {
    if w.truncated {
        return len(p), nil // 已截断，丢弃后续数据
    }

    // 检查写入后是否会超过限制
    if w.buf.Len()+len(p) > w.maxLen {
        w.truncated = true
        // 只写入剩余空间
        remaining := w.maxLen - w.buf.Len()
        if remaining > 0 {
            w.buf.Write(p[:remaining])
        }
        return len(p), nil
    }

    return w.buf.Write(p)
}
```

### 4.5 截断提示

当输出被截断时，响应中会包含：

```json
{
  "truncated": true,
  "stdout": "...原始输出内容...\n... [Output truncated. The log is too long. Please use flags like --tail=100 or --since=1h to limit the output.]"
}
```

### 4.6 最佳实践

为避免输出被截断，建议在请求中使用限制参数：

```json
{
  "verb": "logs",
  "resource": "pods",
  "name": "my-pod",
  "mode": "structured",
  "options": {
    "tailLines": 100,
    "since": "1h"
  }
}
```

---

## 五、令牌桶限流防刷机制 (Rate Limiting)

### 5.1 机制说明

令牌桶限流机制防止 AI Agent 在短时间内发送大量请求，保护网关和后端 Kubernetes 集群免受请求洪泛攻击。

### 5.2 令牌桶算法原理

```
┌──────────────────────────────────────────────────────────────┐
│                    令牌桶算法                                 │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│              ┌─────────────────────┐                        │
│              │    令牌生成器        │                        │
│              │  (10 令牌/秒)        │                        │
│              └──────────┬──────────┘                        │
│                         │                                    │
│                         ▼                                    │
│              ┌─────────────────────┐                        │
│              │      令牌桶          │                        │
│              │   容量: 20 令牌      │                        │
│              │   [●●●●●●●●●●○○○○]   │                        │
│              └──────────┬──────────┘                        │
│                         │                                    │
│                         ▼                                    │
│              ┌─────────────────────┐                        │
│              │    请求处理判断      │                        │
│              └──────────┬──────────┘                        │
│                         │                                    │
│         ┌───────────────┴───────────────┐                   │
│         │ 有令牌                         │ 无令牌           │
│         ▼                                ▼                   │
│   取出1个令牌                      返回 429 错误             │
│   处理请求                         "Rate limit exceeded"     │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 5.3 默认配置

| 参数 | 默认值 | 说明 |
|------|--------|------|
| 速率 (Rate) | 10 请求/秒 | 每秒生成的令牌数量 |
| 突发上限 (Burst) | 20 请求 | 令牌桶最大容量 |

### 5.4 实现代码

限流中间件位于 [`internal/server/middleware.go`](../internal/server/middleware.go)：

```go
// RateLimitMiddleware returns a gin.HandlerFunc that does rate limiting using token bucket.
// Default rate: 10 requests per second, burst: 20.
func RateLimitMiddleware() gin.HandlerFunc {
    // Create a rate limiter: 10 events per second, burst of 20
    limiter := rate.NewLimiter(10, 20)

    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded"})
            c.Abort()
            return
        }
        c.Next()
    }
}
```

### 5.5 限流响应

当请求被限流时，返回：

```json
{
  "error": "Rate limit exceeded"
}
```

HTTP 状态码：`429 Too Many Requests`

### 5.6 客户端处理建议

当收到 429 响应时，客户端应：

1. **等待重试**：等待至少 100ms 后重试
2. **指数退避**：多次失败时采用指数退避策略
3. **请求合并**：合并多个小请求为单个批量请求

```python
import time
import requests

def execute_with_retry(url, headers, data, max_retries=3):
    for attempt in range(max_retries):
        response = requests.post(url, headers=headers, json=data)
        
        if response.status_code == 429:
            wait_time = 0.1 * (2 ** attempt)  # 指数退避
            time.sleep(wait_time)
            continue
            
        return response
    
    raise Exception("Max retries exceeded")
```

---

## 安全配置最佳实践

### 1. 生产环境推荐配置

```yaml
rules:
  # 严格限制可执行操作
  verb_allowlist:
    - "get"
    - "describe"
    - "logs"
    - "top"
  
  # 不配置黑名单（白名单已足够）

  masking:
    # 对所有敏感资源进行脱敏
    - resource: "secrets"
      namespaces: ["*"]
      action: "mask"
    
    # 过滤冗余字段
    - resource: "*"
      namespaces: ["*"]
      action: "filter_fields"
      fields:
        - "metadata.annotations.kubectl.kubernetes.io/last-applied-configuration"
        - "metadata.managedFields"
        - "metadata.creationTimestamp"
        - "status"

execution:
  max_output_length: 10000
  timeout_seconds: 30
  max_concurrent: 10
```

### 2. 开发环境推荐配置

```yaml
rules:
  # 开发环境可适当放宽
  verb_allowlist:
    - "get"
    - "describe"
    - "logs"
    - "apply"
    - "create"
    - "patch"
    - "rollout"
    - "scale"
    - "top"

  masking:
    - resource: "secrets"
      namespaces: ["*"]
      action: "mask"
```

### 3. 安全检查清单

- [ ] 已配置动词白名单，仅允许必要的操作
- [ ] 已配置敏感数据脱敏规则
- [ ] 已配置冗余字段过滤规则
- [ ] 已设置合理的输出大小限制
- [ ] 已设置合理的命令执行超时
- [ ] 已配置强认证 Token
- [ ] 已启用审计日志记录

---

## 相关文档

- [API 接口文档](./api.md)
- [配置说明](../configs/config.yaml)
- [部署指南](../deploy/k8s/)
