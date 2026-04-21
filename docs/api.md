# API 接口文档

本文档详细描述了 agent-kubectl-gateway 网关的核心 API 接口，供上层 AI Agent 或其他客户端集成使用。

## 目录

- [概述](#概述)
- [认证方式](#认证方式)
- [基础信息](#基础信息)
- [核心接口](#核心接口)
  - [POST /execute](#post-execute)
- [数据模型](#数据模型)
  - [请求体结构](#请求体结构)
  - [响应体结构](#响应体结构)
- [错误码说明](#错误码说明)
- [使用示例](#使用示例)

---

## 概述

agent-kubectl-gateway 是一个安全的 kubectl 命令代理网关，为 AI Agent 提供结构化的 Kubernetes 资源操作能力。通过该网关，AI Agent 可以安全地执行 kubectl 命令，同时享受多层安全防护机制。

## 认证方式

所有 API 请求均需要通过 **Bearer Token** 认证。

### 认证头格式

```http
Authorization: Bearer <your-token>
```

### 认证说明

| 项目 | 说明 |
|------|------|
| 认证方式 | Bearer Token |
| 请求头 | `Authorization` |
| Token 格式 | `Bearer <token>` |
| Token 来源 | 由网关管理员预先配置 |

### 认证失败响应

当 Token 无效或缺失时，网关将返回：

```json
{
  "error": "Invalid token"
}
```

HTTP 状态码：`401 Unauthorized`

---

## 基础信息

| 项目 | 值 |
|------|-----|
| 基础 URL | `http://<host>:8078` |
| 内容类型 | `application/json` |
| 字符编码 | `UTF-8` |
| 默认端口 | `8078` |

---

## 核心接口

### POST /execute

执行 kubectl 命令的核心接口。通过结构化参数构建并执行 kubectl 命令，返回执行结果。

#### 请求信息

| 项目 | 值 |
|------|-----|
| 路径 | `/execute` |
| 方法 | `POST` |
| Content-Type | `application/json` |

#### 请求体 JSON 格式

```json
{
  "verb": "<操作动词>",
  "resource": "<资源类型>",
  "namespace": "<命名空间>",
  "name": "<资源名称>",
  "subresource": "<子资源类型>",
  "output": "<输出格式>",
  "mode": "structured",
  "options": {
    "labelSelector": "<标签选择器>",
    "fieldSelector": "<字段选择器>",
    "limit": <数量限制>,
    "container": "<容器名称>",
    "tailLines": <日志行数>,
    "since": "<时间范围>",
    "follow": <是否跟踪>,
    "previous": <是否获取前一个容器日志>,
    "allNamespaces": <是否查询所有命名空间>
  }
}
```

#### 请求参数说明

##### 必填参数

| 参数 | 类型 | 说明 | 示例值 |
|------|------|------|--------|
| `verb` | string | kubectl 操作动词 | `get`, `describe`, `logs`, `apply`, `create`, `patch`, `rollout`, `scale`, `top` |
| `resource` | string | Kubernetes 资源类型 | `pods`, `deployments`, `services`, `configmaps`, `secrets` |
| `mode` | string | 输入模式，固定为 `structured` | `structured` |

##### 可选参数

| 参数 | 类型 | 说明 | 示例值 |
|------|------|------|--------|
| `namespace` | string | 命名空间，集群级别资源可为空 | `default`, `kube-system` |
| `name` | string | 资源名称，查询所有资源时可为空 | `my-app-pod` |
| `subresource` | string | 子资源类型 | `log`, `status`, `scale` |
| `output` | string | 输出格式 | `json`, `yaml`, `wide`, `name` |

##### Options 参数说明

| 参数 | 类型 | 说明 | 示例值 |
|------|------|------|--------|
| `labelSelector` | string | 标签选择器 | `app=nginx,tier=frontend` |
| `fieldSelector` | string | 字段选择器 | `status.phase=Running` |
| `limit` | int | 返回结果数量限制 | `100` |
| `container` | string | 容器名称（用于 logs 命令） | `main-container` |
| `tailLines` | int | 日志尾部行数 | `100` |
| `since` | string | 时间范围 | `1h`, `30m`, `1d` |
| `follow` | bool | 是否持续跟踪日志输出 | `false`（注意：网关不支持流式响应，建议使用 `false`） |
| `previous` | bool | 是否获取前一个容器的日志 | `false` |
| `allNamespaces` | bool | 是否查询所有命名空间 | `true` |

---

#### 响应体结构

```json
{
  "request_id": "<请求唯一标识>",
  "status": "<执行状态>",
  "exit_code": <退出码>,
  "stdout": "<标准输出>",
  "stderr": "<标准错误输出>",
  "truncated": <是否被截断>,
  "duration_ms": <执行耗时毫秒>,
  "response_size_bytes": <响应大小字节>,
  "blocked_reason": "<被拦截原因>"
}
```

#### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `request_id` | string | 请求唯一标识，用于追踪和日志关联 |
| `status` | string | 执行状态：`success`（成功）、`failed`（失败）、`blocked`（被拦截） |
| `exit_code` | int | kubectl 进程退出码，`0` 表示成功 |
| `stdout` | string | kubectl 命令的标准输出内容 |
| `stderr` | string | kubectl 命令的标准错误输出内容 |
| `truncated` | bool | 输出是否因超出限制而被截断 |
| `duration_ms` | int64 | 命令执行耗时（毫秒） |
| `response_size_bytes` | int | 最终返回数据大小（字节） |
| `blocked_reason` | string | 被拦截原因，仅在 `status` 为 `blocked` 或 `failed` 时存在 |

---

## 错误码说明

网关定义了以下错误码，用于标识不同类型的错误：

| 错误码 | 名称 | 说明 | HTTP 状态码 |
|--------|------|------|-------------|
| `ERR_INVALID_COMMAND` | 参数非法 | 请求参数格式不正确或缺少必填参数 | 400 |
| `ERR_BLOCKED_VERB` | 动词被拦截 | 动词不在白名单中或在黑名单中 | 200 |
| `ERR_KUBECTL_NOT_FOUND` | kubectl 不存在 | 系统中未安装 kubectl 命令 | 200 |
| `ERR_EXEC_TIMEOUT` | 执行超时 | 命令执行超过配置的超时时间 | 200 |
| `ERR_NON_ZERO_EXIT` | 非零退出码 | kubectl 命令执行失败 | 200 |
| `ERR_OUTPUT_TOO_LARGE` | 输出过大 | 输出内容超过最大限制并被截断 | 200 |

### 错误响应示例

#### 认证失败

```json
{
  "error": "Invalid token"
}
```

#### 请求参数错误

```json
{
  "status": "failed",
  "blocked_reason": "Invalid request body: invalid character 'a' looking for beginning of value"
}
```

#### 动词被拦截

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

#### 执行超时

```json
{
  "request_id": "req-12345678",
  "status": "failed",
  "exit_code": -1,
  "blocked_reason": "ERR_EXEC_TIMEOUT",
  "duration_ms": 30000,
  "response_size_bytes": 0
}
```

---

## 使用示例

### 示例 1：获取 Pod 列表

**请求：**

```bash
curl -X POST http://localhost:8078/execute \
  -H "Authorization: Bearer your-token-here" \
  -H "Content-Type: application/json" \
  -d '{
    "verb": "get",
    "resource": "pods",
    "namespace": "default",
    "output": "json",
    "mode": "structured"
  }'
```

**响应：**

```json
{
  "request_id": "req-12345678",
  "status": "success",
  "exit_code": 0,
  "stdout": "{\n  \"apiVersion\": \"v1\",\n  \"items\": [...],\n  \"kind\": \"List\"\n}",
  "stderr": "",
  "truncated": false,
  "duration_ms": 150,
  "response_size_bytes": 2048
}
```

### 示例 2：获取特定 Pod 的日志

**请求：**

```bash
curl -X POST http://localhost:8078/execute \
  -H "Authorization: Bearer your-token-here" \
  -H "Content-Type: application/json" \
  -d '{
    "verb": "logs",
    "resource": "pods",
    "namespace": "default",
    "name": "my-app-pod",
    "mode": "structured",
    "options": {
      "container": "main-container",
      "tailLines": 100,
      "since": "1h"
    }
  }'
```

**响应：**

```json
{
  "request_id": "req-12345678",
  "status": "success",
  "exit_code": 0,
  "stdout": "2026-03-16T10:00:00Z INFO Server started\n2026-03-16T10:00:01Z INFO Processing request...",
  "stderr": "",
  "truncated": false,
  "duration_ms": 80,
  "response_size_bytes": 512
}
```

### 示例 3：使用标签选择器查询 Deployment

**请求：**

```bash
curl -X POST http://localhost:8078/execute \
  -H "Authorization: Bearer your-token-here" \
  -H "Content-Type: application/json" \
  -d '{
    "verb": "get",
    "resource": "deployments",
    "namespace": "production",
    "output": "yaml",
    "mode": "structured",
    "options": {
      "labelSelector": "app=nginx,tier=frontend"
    }
  }'
```

**响应：**

```json
{
  "request_id": "req-12345678",
  "status": "success",
  "exit_code": 0,
  "stdout": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: nginx-deployment\n...",
  "stderr": "",
  "truncated": false,
  "duration_ms": 120,
  "response_size_bytes": 1536
}
```

### 示例 4：描述 Service 详情

**请求：**

```bash
curl -X POST http://localhost:8078/execute \
  -H "Authorization: Bearer your-token-here" \
  -H "Content-Type: application/json" \
  -d '{
    "verb": "describe",
    "resource": "services",
    "namespace": "default",
    "name": "my-service",
    "mode": "structured"
  }'
```

**响应：**

```json
{
  "request_id": "req-12345678",
  "status": "success",
  "exit_code": 0,
  "stdout": "Name:              my-service\nNamespace:         default\nSelector:          app=my-app\nType:              ClusterIP\n...",
  "stderr": "",
  "truncated": false,
  "duration_ms": 95,
  "response_size_bytes": 1024
}
```

### 示例 5：查询所有命名空间的 Pod

**请求：**

```bash
curl -X POST http://localhost:8078/execute \
  -H "Authorization: Bearer your-token-here" \
  -H "Content-Type: application/json" \
  -d '{
    "verb": "get",
    "resource": "pods",
    "output": "wide",
    "mode": "structured",
    "options": {
      "allNamespaces": true
    }
  }'
```

**响应：**

```json
{
  "request_id": "req-12345678",
  "status": "success",
  "exit_code": 0,
  "stdout": "NAMESPACE     NAME                     READY   STATUS    RESTARTS   AGE\ndefault       nginx-pod                1/1     Running   0          2d\nkube-system   calico-node-xxx          1/1     Running   0          39d",
  "stderr": "",
  "truncated": false,
  "duration_ms": 110,
  "response_size_bytes": 896
}
```

### 示例 6：获取 Secret（敏感数据将被脱敏）

**请求：**

```bash
curl -X POST http://localhost:8078/execute \
  -H "Authorization: Bearer your-token-here" \
  -H "Content-Type: application/json" \
  -d '{
    "verb": "get",
    "resource": "secrets",
    "namespace": "default",
    "name": "my-secret",
    "output": "json",
    "mode": "structured"
  }'
```

**响应（data 字段已被脱敏）：**

```json
{
  "request_id": "req-12345678",
  "status": "success",
  "exit_code": 0,
  "stdout": "{\n  \"kind\": \"Secret\",\n  \"data\": {\n    \"password\": \"*** MASKED BY PROXY ***\"\n  }\n}",
  "stderr": "",
  "truncated": false,
  "duration_ms": 50,
  "response_size_bytes": 256
}
```

---

## 健康检查接口

### GET /health

用于检查网关服务是否正常运行。

**请求：**

```bash
curl http://localhost:8078/health
```

**响应：**

```json
{
  "status": "ok"
}
```

---

## 注意事项

1. **认证必须**：所有 `/execute` 请求必须携带有效的 Bearer Token
2. **动词限制**：仅允许执行白名单中的动词，黑名单中的动词将被拦截
3. **输出截断**：当输出超过配置的最大长度（默认 10000 字节）时，输出将被截断
4. **敏感数据脱敏**：Secret 资源的敏感数据将自动被脱敏处理
5. **字段过滤**：部分冗余字段（如 `managedFields`、`last-applied-configuration`）将被自动过滤
6. **限流保护**：网关实现了令牌桶限流机制，默认 10 请求/秒，突发上限 20 请求

---

## 相关文档

- [安全说明文档](./security.md)
- [配置说明](../configs/config.yaml)
- [部署指南](../deploy/k8s/)
