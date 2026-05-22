# kubectl 输出 Token 优化实施计划（修订版）

> ⚠️ **本计划已全部实施完毕（阶段 A/B/C）。** 保留仅作历史记录。文档中"当前现状"描述为实施前的代码状态，与当前代码库不符，请勿作为参考依据。

## 1. 背景与目标

### 1.1 问题

当前 `agent-kubectl-gateway` 会将 `kubectl` 输出在过滤后直接返回给 LLM。现状存在三类 token 浪费：

1. **固定冗余字段未充分剥离**：如 `managedFields`、`resourceVersion`、`uid`。
2. **`logs` 默认返回范围过大**：未显式传参时可能返回大量历史日志，被输出截断后仍浪费 token。
3. **结构化输出存在可继续压缩空间**：JSON 当前为 pretty-print；部分 Kubernetes 默认值字段可进一步裁剪。

### 1.2 目标

在**不改变现有安全边界**、**不破坏 JSON/YAML 双格式过滤能力**前提下，分阶段降低返回给 LLM 的 token 消耗，并把实现边界、验证方式、回滚方式写清楚，确保可直接按计划实施。

### 1.3 本次修订的强约束

本修订版按以下约束执行：

1. `logs` 场景下，`TailLines <= 0` 统一视为“未提供有效值”，触发默认 `--tail 100`；**不支持通过 `tailLines: 0` 获取全量日志**。
2. 计划中引用但仓库中不存在的函数/文件，统一按“**明确新增实现**”处理，不再假设已存在。
3. 所有结构化过滤改造必须同时覆盖 **JSON 与 YAML**。
4. 所有阶段必须写明**代码验证、测试验证、兼容性验证**。
5. 本计划**不评估工时**，只评估实现影响范围与落地顺序。

---

## 2. 当前代码现状（按仓库核对）

以下内容已按当前仓库代码核对：

### 2.1 结果过滤现状

- 文件：`internal/filter/filter.go`
- 当前入口：`FilterResult(req, result)`
- 当前仅支持两个动作：
  - `mask`
  - `filter_fields`
- 当前过滤在 `result.Status != "success"` 时直接跳过。
- 当前 `maskContent` 已按输出格式区分：
  - `json`
  - `yaml`
  - 非结构化文本（正则）
- 当前 JSON 处理会重新序列化，且使用 **`json.MarshalIndent`**，不是 compact JSON。

### 2.2 logs 参数构建现状

- 文件：`internal/executor/builder.go`
- 当前 `BuildArgs(req)`：
  - `logs` 场景不追加 `resource`，直接使用 `req.Name`
  - 仅当 `req.Options.TailLines > 0` 时才追加 `--tail`
  - 仅当 `req.Options.Since != ""` 时才追加 `--since`
- `internal/model/model.go` 中 `Options.TailLines` 类型为 `int`，**没有“未设置 / 显式 0”区分能力**。

### 2.3 配置现状

- 文件：`configs/config.yaml`
- 当前 `filter_fields` 已配置字段：
  - `metadata.annotations.kubectl.kubernetes.io/last-applied-configuration`
  - `metadata.managedFields`
  - `metadata.creationTimestamp`
  - `status`

### 2.4 测试现状

- `internal/executor/builder_test.go`：已有基础 `logs`、`get`、`all namespaces` 测试。
- `internal/filter/filter_test.go`：已有 JSON/YAML 脱敏、JSON 字段过滤、List 脱敏等测试。
- `test/test_features.py`：已有集成测试框架，但当前计划相关断言仍需扩展。

### 2.5 依赖与构建现状

- `go.mod`：`go 1.25.1`
- `deploy/Dockerfile`：构建镜像仍为 `golang:1.21-alpine`

这意味着若引入 `k8s.io/api` / `k8s.io/client-go`，**必须把依赖兼容性与容器构建验证写入计划**，不能只写代码改动。

---

## 3. 设计原则

1. **先做低风险高收益项，再做侵入性重构项。**
2. **不破坏现有 JSON/YAML 过滤能力。**
3. **新增行为必须可测试、可回滚、可解释。**
4. **默认值裁剪只删除“可稳定判定为默认值”的字段。**
5. **Admission 注入字段与 scheme 默认值分开处理，不混为一谈。**

---

## 4. 实施阶段

## 4.1 阶段 A：扩展固定字段过滤（必做）

### 4.1.1 目标

在不改动过滤主流程的前提下，直接扩大 `filter_fields` 配置收益。

### 4.1.2 变更内容

修改 `configs/config.yaml` 中现有 `filter_fields` 规则，新增以下字段：

```yaml
fields:
  - "metadata.annotations.kubectl.kubernetes.io/last-applied-configuration"
  - "metadata.managedFields"
  - "metadata.creationTimestamp"
  - "metadata.generateName"
  - "metadata.ownerReferences"
  - "metadata.resourceVersion"
  - "metadata.uid"
  - "status"
```

### 4.1.3 影响范围

- `configs/config.yaml`
- `internal/filter/filter_test.go`
- `test/test_features.py`

### 4.1.4 实现边界

- **不新增动作**。
- **不修改 `FilterResult` 主流程**。
- 仅扩展配置与测试。

### 4.1.5 风险与说明

- `ownerReferences` 在部分排障场景有价值，但对 LLM 常规分析收益较低。
- 当前策略仍是“对返回给 LLM 的结果做过滤”；若未来要区分审计输出与对外响应，可再拆分策略。

### 4.1.6 验证要求

1. 单元测试新增覆盖：
   - `metadata.generateName`
   - `metadata.ownerReferences`
   - `metadata.resourceVersion`
   - `metadata.uid`
2. 集成测试验证上述字段不会出现在响应中。
3. YAML 输出场景也要验证字段过滤生效。

---

## 4.2 阶段 B：为 logs 注入安全默认值（必做）

### 4.2.1 目标

避免 `kubectl logs` 在未指定范围时返回大体量日志，减少 token 浪费并降低被截断后丢失有效尾部上下文的概率。

### 4.2.2 变更内容

在 `internal/executor/builder.go` 的 `BuildArgs(req)` 中，为 `req.Verb == "logs"` 增加默认值注入逻辑：

- 当 `req.Options == nil` 或 `req.Options.TailLines <= 0` 时，追加 `--tail 100`
- 当 `req.Options == nil` 或 `req.Options.Since == ""` 时，追加 `--since 1h`

建议按以下语义实现：

```go
if req.Verb == "logs" {
    if req.Options == nil || req.Options.TailLines <= 0 {
        args = append(args, "--tail", "100")
    }
    if req.Options == nil || req.Options.Since == "" {
        args = append(args, "--since", "1h")
    }
}
```

### 4.2.3 明确语义

- `tailLines <= 0`：都视为无效输入，统一回退默认值。
- **不允许查询全量日志**。
- 若调用方需要更大窗口，只能显式传入 `tailLines > 0` 或更长 `since`。

### 4.2.4 影响范围

- `internal/executor/builder.go`
- `internal/executor/builder_test.go`
- API 文档（如有 logs 参数说明）
- 可能影响调用方对 logs 默认行为的预期

### 4.2.5 实现边界

- **不修改 `model.Options` 字段结构**。
- **不引入“显式 full logs”语义**。
- **不改变现有 `logs` 使用 `req.Name` 而非 `req.Resource` 的构建规则**。

### 4.2.6 必增测试

至少新增以下测试：

- `TestBuildArgs_LogsDefaultTailWhenOptionsNil`
- `TestBuildArgs_LogsDefaultSinceWhenOptionsNil`
- `TestBuildArgs_LogsDefaultTailWhenTailNonPositive`
- `TestBuildArgs_LogsExplicitTail`
- `TestBuildArgs_LogsExplicitSince`
- `TestBuildArgs_LogsDefaultTailAndSinceTogether`

并确认以下场景不冲突：

- `container`
- `follow`
- `previous`
- `namespace`

---

## 4.3 阶段 C：新增 strip_defaults 动作并保留 JSON/YAML 双支持（增量增强）

### 4.3.1 目标

在现有 `mask` / `filter_fields` 之外，新增 `strip_defaults` 动作，用于删除 Kubernetes 结构化输出中**可稳定判定为默认值**的字段，同时保持 JSON 与 YAML 都可处理。

### 4.3.2 必须先明确的事实

1. 当前仓库**没有** `strip_defaults` 动作。
2. 当前仓库**没有**以下计划草案中提到的函数，若采用该方向，必须显式新增：
   - `defaultsForResource(...)`
   - `stripDefaultsInPlace(...)`
   - `marshalCompact(...)`
   - 与统一解析相关的辅助函数
3. Kubernetes `scheme.Default()` 可以提供一部分默认值，但**默认 tolerations（如 not-ready / unreachable 300s）来自 admission controller**，不能直接当作纯 scheme 默认值统一删除。

### 4.3.3 新增文件与新增函数（明确新增，不再假设存在）

建议新增：

- `internal/filter/defaults.go`
  - 构建资源默认值描述
  - 资源到默认值裁剪器的映射
- `internal/filter/defaults_test.go`
  - 默认值裁剪专项测试

建议新增函数（命名可微调，但职责必须落到代码）：

- `func (f *Filter) stripDefaults(content, outputFormat, resource string) string`
- `func (f *Filter) stripDefaultsJSON(content, resource string) string`
- `func (f *Filter) stripDefaultsYAML(content, resource string) string`
- `func (f *Filter) stripDefaultsSingleResource(data map[string]any, resource string)`
- `func (f *Filter) compactJSON(content string) string`

> 说明：本修订版**不强制**你必须把 `filter_fields` 与 `strip_defaults` 合并成单次解析管线；但如果实施时决定合并，也必须保持 JSON/YAML 双支持，不得回退为仅 JSON。

### 4.3.4 推荐落地策略

优先采用**保守可验证版本**：

#### 第一步：先把 `strip_defaults` 做成独立动作

- 在 `FilterResult` 的规则动作分派中新增：
  - `case "strip_defaults": ...`
- 初版允许 `strip_defaults` 独立完成 JSON/YAML 解析、原地修改、再序列化。
- 这样风险更低，也更容易验证不会破坏当前 `filter_fields`。

#### 第二步：确认功能正确后，再考虑合并解析管线

- 若后续确实需要降低一次序列化成本，再把：
  - `filter_fields`
  - `strip_defaults`
  - `compact json`
  合并为统一结构化处理管线。
- 合并前必须先有测试兜底，且 YAML 不能丢。

### 4.3.5 strip_defaults 初始覆盖范围

初始版本覆盖以下工作负载资源及其 List 结果中的确定性默认值：

- **Pod / PodList**
- **Deployment / DeploymentList**
- **StatefulSet / StatefulSetList**

其中：

- Pod 直接处理资源根对象的 `spec`
- Deployment 处理 `spec.template.spec`
- StatefulSet 处理 `spec.template.spec`

且仅删除下列两类字段：

#### A. 可由 scheme/defaulting 稳定推导的默认值

例如：

- `spec.dnsPolicy`
- `spec.restartPolicy`
- `spec.schedulerName`
- `spec.enableServiceLinks`
- `spec.securityContext`（仅当为空结构）

> 注意：是否纳入 `terminationGracePeriodSeconds`、`serviceAccountName`、`serviceAccount`，必须以当前 K8s 版本实际 defaulting 结果与测试结果为准，不能只靠文档猜测。Deployment / StatefulSet 不单独维护另一套默认值规则，优先复用 PodTemplateSpec 中可稳定判定的默认值集合。

#### B. 明确排除项

以下内容**第一版不纳入统一 strip_defaults 自动删除**：

- admission controller 注入的默认 tolerations
- 需要结合集群行为判定的字段
- 不同 K8s 版本差异过大的字段

如需处理 tolerations，必须作为**单独子项**，并先证明不会误删用户自定义值。

### 4.3.6 JSON 与 YAML 处理要求

#### JSON

- `strip_defaults` 处理后输出为 **compact JSON**（即不再使用 `json.MarshalIndent`）
- 仅结构化输出为 `json` 时启用 compact JSON

#### YAML

- 继续支持 YAML 解析、字段删除、回写
- YAML 输出保持 YAML 格式，不强制转 JSON
- 不要求对 YAML 做 compact 化，但必须保证字段裁剪结果一致

### 4.3.7 配置变更

在 `configs/config.yaml` 中新增动作规则：

```yaml
- resource: "pods"
  namespaces: ["*"]
  action: "strip_defaults"

- resource: "pod"
  namespaces: ["*"]
  action: "strip_defaults"

- resource: "deployments"
  namespaces: ["*"]
  action: "strip_defaults"

- resource: "deployment"
  namespaces: ["*"]
  action: "strip_defaults"

- resource: "statefulsets"
  namespaces: ["*"]
  action: "strip_defaults"

- resource: "statefulset"
  namespaces: ["*"]
  action: "strip_defaults"
```

> 是否需要同时匹配单数/复数资源名，应按当前请求中 `req.Resource` 的实际取值决定；实施时必须以代码调用现状为准。若代码中已统一资源命名，可收敛为实际使用的那一组，避免冗余配置。

### 4.3.8 依赖变更

若采用 scheme/defaulting 路线，新增依赖：

```go
require (
    k8s.io/api v0.31.0
    k8s.io/client-go v0.31.0
)
```

### 4.3.9 必做兼容性验证

因为当前仓库存在：

- `go.mod`：`go 1.25.1`
- Docker 构建：`golang:1.21-alpine`

所以必须在计划实施中加入以下验证：

1. 本地 `go test ./...` 可通过。
2. 容器构建可通过。
3. `deploy/Dockerfile` 的 builder 镜像统一升级为 **Go 1.25**（与 `go.mod` 保持一致），不再保留 1.21。
4. 在引入 `k8s.io/*` 依赖后，必须重新验证 `go test ./...`、二进制构建、Docker 构建均通过。
5. 若升级 builder 镜像后出现镜像构建差异，必须在同一实施阶段内修正，不能把版本统一留作后续事项。

---

## 5. 测试与验证计划

## 5.1 单元测试

### A. builder

文件：`internal/executor/builder_test.go`

新增覆盖：

- logs 默认 `--tail 100`
- logs 默认 `--since 1h`
- `TailLines <= 0` 时强制回退默认值
- 显式 `tailLines > 0` 不被覆盖
- 显式 `since` 不被覆盖
- `container` / `follow` / `previous` 与默认值并存

### B. filter_fields

文件：`internal/filter/filter_test.go`

新增覆盖：

- JSON 单资源字段过滤
- JSON List 字段过滤
- YAML 单资源字段过滤
- YAML List 字段过滤
- 新增 4 个 metadata 字段过滤

### C. strip_defaults

文件：`internal/filter/defaults_test.go`

新增覆盖：

- Pod 默认值被删除
- 用户显式设置的非默认值保留
- YAML Pod 默认值被删除
- PodList / List 中每个 item 都能被处理
- Deployment `spec.template.spec` 默认值被删除
- StatefulSet `spec.template.spec` 默认值被删除
- DeploymentList / StatefulSetList 中每个 item 都能被处理
- compact JSON 生效（无缩进）
- 非 success 结果不进入 strip_defaults
- 非目标资源不触发 strip_defaults

### D. 回归测试

必须确认以下现有能力未被破坏：

- Secret JSON 脱敏
- Secret YAML 脱敏
- List 结构脱敏
- 原有 `filter_fields` 规则继续生效

## 5.2 集成测试

文件：`test/test_features.py`

需要新增或扩展：

1. `get ... -o json` 返回中以下字段不存在：
   - `managedFields`
   - `generateName`
   - `ownerReferences`
   - `resourceVersion`
   - `uid`
   - `status`
2. `get ... -o yaml` 返回中同样验证上述字段被裁剪。
3. `logs` 未传参数时，实际命令效果受 `--tail 100 --since 1h` 约束。
4. `logs` 显式传 `tailLines` / `since` 时，默认值不覆盖显式值。

## 5.3 Token 效果验证

文件：`test/token_benchmark.py`

目标不是只跑一次脚本，而是分别验证：

1. 固定字段过滤前后 token 对比
2. logs 默认值注入前后 token 对比
3. strip_defaults + compact JSON 前后 token 对比

建议输出三组对比结果，避免收益归因混淆。

## 6. 实施顺序

按以下顺序实施：

1. **阶段 A：扩展固定字段过滤**
2. **阶段 B：logs 默认值注入**
3. **阶段 C-1：新增 strip_defaults 独立动作（JSON + YAML）**
4. **阶段 C-2：将 JSON 输出切为 compact JSON**
5. **阶段 C-3：仅在确有必要时，合并结构化解析管线**

这样可以保证：

- 先拿到低风险收益
- 再做行为变更清晰的 logs 优化
- 最后做侵入性最大的结构化裁剪

---

## 7. 验收标准

满足以下条件才算完成：

1. `config.yaml` 已新增 4 个 metadata 过滤字段。
2. `logs` 未传有效 `tailLines` / `since` 时，默认追加 `--tail 100 --since 1h`。
3. `tailLines <= 0` 不能绕过默认值获取全量日志。
4. `strip_defaults` 已作为独立动作落地，且同时支持 JSON / YAML。
5. `strip_defaults` 同时覆盖 Pod、Deployment、StatefulSet 及其 List 结果中的目标默认字段。
6. JSON 输出在结构化过滤后为 compact JSON。
7. Admission 注入的默认 tolerations 未被误删，或尚未纳入第一版实现。
8. 单元测试、集成测试、构建验证全部通过。

---

## 8. 回滚方案

### 8.1 配置级回滚

可直接回滚 `configs/config.yaml` 中：

- 新增的 `filter_fields` 字段
- 新增的 `strip_defaults` 动作规则

利用现有配置热更新能力恢复旧策略。

### 8.2 代码级回滚

以下改动需通过代码回退：

- `builder.go` 中 logs 默认值注入
- `filter.go` / `defaults.go` 中 `strip_defaults` 与 compact JSON 实现
- `Dockerfile` 的 Go 版本升级（若发生）

### 8.3 回滚优先级

若出现问题，优先按以下顺序回退：

1. 回退 `strip_defaults`
2. 回退 compact JSON
3. 回退 logs 默认值注入
4. 最后才回退固定字段过滤配置

---

## 9. 本修订版相对原计划的关键修正

1. 明确废除“`tailLines: 0` 可获取全量日志”的说法。
2. 不再假设若干 `strip_defaults` 相关函数已经存在，统一改为明确新增。
3. 明确要求 JSON 与 YAML 双支持，禁止因重构丢失 YAML。
4. 明确把依赖、构建、Docker 兼容性验证写进计划。
5. 不再把 admission 默认 tolerations 与 scheme 默认值混在一起处理。
6. 不再按工时评估任务，只按影响范围和实施顺序组织。
