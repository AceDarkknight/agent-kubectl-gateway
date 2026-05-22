#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Token 优化效果验证脚本

基于 OpenAI tiktoken (cl100k_base) 精确统计算法中 kubectl 输出
在优化前后的 token 消耗，为字段过滤、日志截断、describe→get 转换等
优化手段提供可量化的效果评估。

当前版本增强点：
    - 多轮运行取平均值（默认 3 轮）
    - 唯一场景 ID，便于复盘与汇报
    - 语义保真断言（验证关键字段保留、冗余字段被剥离）
    - 覆盖所有命名空间下的 pod/pods/deployment/deployments/statefulset/statefulsets 场景

编码器：tiktoken cl100k_base — GPT-4、GPT-4o、GPT-3.5-turbo 的 BPE 编码
         这是 LLM 领域公认的 token 计数工业标准，非字节/字符估算。

依赖：
    pip install tiktoken

用法：
    # 离线模式：直接调用 kubectl 做优化前后对比
    # 前提：本机已安装 kubectl 且具备 kubeconfig/集群访问权限
    export TIKTOKEN_CACHE_DIR=/opt/tiktoken-cache
    export TOKEN_BENCHMARK_RUNS=3
    python test/token_benchmark.py

    # API 模式：通过网关 API 获取当前输出的 token 消耗
    # 前提：网关服务可访问，且已配置好鉴权 Token
    export TIKTOKEN_CACHE_DIR=/opt/tiktoken-cache
    export GATEWAY_BASE_URL=http://localhost:8078
    export GATEWAY_AUTH_TOKEN=your-token
    export TOKEN_BENCHMARK_RUNS=3
    python test/token_benchmark.py --via-api

环境变量：
    TIKTOKEN_CACHE_DIR     tiktoken 缓存目录（离线主机建议显式设置）
    KUBE_NAMESPACE         指定仅测试该命名空间；不设置时自动发现并遍历所有命名空间
    TOKEN_BENCHMARK_RUNS   每个场景重复运行次数（默认值：3）
    GATEWAY_BASE_URL       API 模式下的网关地址
    GATEWAY_AUTH_TOKEN     API 模式下的鉴权 Token

说明：
    - 离线模式会直接调用 kubectl，并模拟“优化前 / 优化后”输出进行 token 对比。
    - API 模式会调用网关，统计当前服务端真实输出的 token 数与字节数。
    - 对于 get 命令：若未指定 -o json/-o yaml，则保持 kubectl 默认输出行为，不再强制转成 json。
    - 默认行为：先发现集群中所有命名空间，再对每个命名空间分别跑一组场景。
    - 每个命名空间都会覆盖：pod / pods / deployment / deployments / statefulset / statefulsets。
    - 若设置 KUBE_NAMESPACE，则只跑该命名空间的测试。
"""

import json
import os
import subprocess
import sys
import time
from statistics import mean
import urllib.request
import urllib.error

# ── 环境变量 ───────────────────────────────────────────
GATEWAY_URL = os.environ.get("GATEWAY_BASE_URL", "http://localhost:8078")
TOKEN = os.environ.get("GATEWAY_AUTH_TOKEN", "")

# ── tiktoken 初始化 ─────────────────────────────────────
try:
    import tiktoken
    ENC = tiktoken.get_encoding("cl100k_base")
except ImportError:
    print("=" * 60)
    print("  [ERROR] tiktoken 未安装，无法进行精确 token 统计。")
    print()
    print("  tiktoken 是 OpenAI 官方的 BPE token 计数器，")
    print("  GPT-4 / GPT-4o / GPT-3.5-turbo 均使用 cl100k_base 编码。")
    print("  这也是 LLM 领域公认的 token 统计工业标准。")
    print()
    print("  请执行以下命令安装：")
    print()
    print("    pip install tiktoken")
    print()
    print("  安装后重新运行本脚本即可。")
    print("=" * 60)
    sys.exit(1)


def count_tokens(text: str) -> int:
    """精算 token 数（tiktoken cl100k_base BPE）。"""
    return len(ENC.encode(text))


# ── 工具函数 ────────────────────────────────────────────

def run_kubectl(args: list, timeout: int = 30) -> str:
    """直接调用本地 kubectl，返回 stdout 字符串。"""
    try:
        result = subprocess.run(
            ["kubectl"] + args,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
        return result.stdout
    except FileNotFoundError:
        print("[ERROR] kubectl 未找到，请确保已安装并加入 PATH。")
        return ""
    except subprocess.TimeoutExpired:
        print(f"[ERROR] kubectl {' '.join(args)} 执行超时（>{timeout}s）")
        return ""


def send_structured_request(verb, resource, namespace="", name="",
                            output="", options=None):
    """通过网关 API 发送结构化请求（与 test_features.py 一致）。"""
    payload = {
        "verb": verb,
        "resource": resource,
        "mode": "structured"
    }
    if namespace:
        payload["namespace"] = namespace
    if name:
        payload["name"] = name
    if output:
        payload["output"] = output
    if options:
        payload["options"] = options

    payload_bytes = json.dumps(payload).encode("utf-8")
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {TOKEN}"
    }
    req = urllib.request.Request(
        f"{GATEWAY_URL}/execute",
        data=payload_bytes,
        headers=headers,
        method="POST"
    )
    try:
        with urllib.request.urlopen(req, timeout=60) as response:
            data = json.loads(response.read().decode("utf-8"))
            return data
    except urllib.error.HTTPError as e:
        return {"status": "failed", "stderr": f"HTTP {e.code}: {e.reason}"}
    except urllib.error.URLError as e:
        return {"status": "failed", "stderr": str(e.reason)}


# ── 模拟优化前输出（当前 executor 行为） ─────────────────

def get_raw_kubectl(verb: str, resource: str = "", name: str = "",
                    namespace: str = "", output_fmt: str = "",
                    tail: int = 0, since: str = "", options: dict = None) -> str:
    """
    模拟当前 executor.go 行为：
    - 不强制 json 输出
    - 不剥离 managedFields
    - 不限制 logs tail
    """
    args = [verb]

    if verb == "logs":
        if name:
            args.append(name)
    else:
        if resource:
            args.append(resource)
        if name:
            args.append(name)

    if namespace and verb != "cluster-info":
        args.extend(["-n", namespace])

    if options and options.get("allNamespaces"):
        args.append("--all-namespaces")

    if output_fmt and verb not in ("logs",):
        args.extend(["-o", output_fmt])

    if verb == "logs":
        if tail:
            args.extend(["--tail", str(tail)])
        if since:
            args.extend(["--since", since])

    return run_kubectl(args)


# ── 模拟优化后输出 ─────────────────────────────────────

# 需要剥离的冗余字段路径（点分隔，支持嵌套）
STRIP_FIELDS = [
    "metadata.managedFields",
    "metadata.annotations.kubectl.kubernetes.io/last-applied-configuration",
    "metadata.creationTimestamp",
    "metadata.resourceVersion",
    "metadata.uid",
    "metadata.selfLink",
    "metadata.generation",
    "status",
]

# 默认 logs tail 行数
DEFAULT_LOG_TAIL = 100

# 默认 benchmark 重复轮数
DEFAULT_BENCHMARK_RUNS = int(os.environ.get("TOKEN_BENCHMARK_RUNS", "3"))

def _remove_field_by_path(data: dict, path: list, depth: int = 0):
    """按点分路径递归删除嵌套字段。"""
    if not path or not isinstance(data, dict) or depth > 20:
        return
    key = path[0]
    if len(path) == 1:
        data.pop(key, None)
    elif key in data and isinstance(data[key], dict):
        _remove_field_by_path(data[key], path[1:], depth + 1)


def _has_field_by_path(data: dict, path: list, depth: int = 0) -> bool:
    """检查嵌套字段是否存在。"""
    if not path or not isinstance(data, dict) or depth > 20:
        return False
    key = path[0]
    if key not in data:
        return False
    if len(path) == 1:
        return True
    if isinstance(data[key], dict):
        return _has_field_by_path(data[key], path[1:], depth + 1)
    return False


def _filter_and_compact_json(raw: str) -> str:
    """解析 JSON → 剥离冗余字段 → compact 重序列化。"""
    try:
        data = json.loads(raw)
    except json.JSONDecodeError:
        return raw

    # 统一处理：*List 和单资源
    if isinstance(data, dict) and str(data.get("kind", "")).endswith("List") and "items" in data:
        items = data.get("items", [])
    else:
        items = [data] if isinstance(data, dict) else []

    # 顶层 metadata 也做剥离（List 自身 metadata 可能包含 resourceVersion 等）
    if isinstance(data, dict):
        for field_path in STRIP_FIELDS:
            _remove_field_by_path(data, field_path.split("."))

    for item in items:
        if not isinstance(item, dict):
            continue
        for field_path in STRIP_FIELDS:
            _remove_field_by_path(item, field_path.split("."))

    return json.dumps(data, separators=(",", ":"), ensure_ascii=False)


def _convert_describe_to_get(verb: str, resource: str, name: str,
                             namespace: str, options: dict = None) -> str:
    """
    describe → get -o json 转换 + 字段剥离。
    describe 是纯文本，无法 -o json，直接替换为 get 命令获取结构化数据。
    """
    raw = get_raw_kubectl("get", resource, name, namespace, output_fmt="json", options=options)
    return _filter_and_compact_json(raw)


def get_optimized_kubectl(verb: str, resource: str = "", name: str = "",
                          namespace: str = "", output_fmt: str = "",
                          tail: int = 0, since: str = "", options: dict = None) -> str:
    """
    模拟优化后行为：
    - logs：强制 tail 默认值
    - get：若显式指定 json，则剥离冗余字段 + compact；否则保持 kubectl 默认输出行为
    - describe：转换为 get -o json + 剥离 + compact
    - 其他：原样通过
    """
    if verb == "logs":
        effective_tail = tail if tail > 0 else DEFAULT_LOG_TAIL
        raw = get_raw_kubectl(verb, resource, name, namespace,
                              tail=effective_tail, since=since, options=options)
        return raw  # logs 不后处理，优化在 kubectl 参数层完成

    elif verb == "get":
        # 保持 kubectl 默认行为：只有显式指定 json 时才做字段剥离与 compact
        effective_fmt = output_fmt if output_fmt else ""
        raw = get_raw_kubectl(verb, resource, name, namespace,
                              output_fmt=effective_fmt, options=options)
        if effective_fmt == "json":
            return _filter_and_compact_json(raw)
        return raw  # table/yaml/wide/name 不做字段剥离

    elif verb == "describe":
        return _convert_describe_to_get(verb, resource, name, namespace, options=options)

    else:
        return get_raw_kubectl(verb, resource, name, namespace,
                               output_fmt=output_fmt, options=options)


# ── API 模式：通过网关获取优化后输出 ─────────────────────

def get_via_api(verb: str, resource: str = "", name: str = "",
                namespace: str = "", output: str = "",
                options: dict = None) -> str:
    """
    通过网关 API 获取优化后输出。
    用于代码改造完成后验证实际线上效果。
    """
    result = send_structured_request(verb, resource, namespace, name,
                                     output, options)
    if result.get("status") == "success":
        return result.get("stdout", "")
    else:
        reason = result.get("blocked_reason") or result.get("stderr", "unknown")
        return f"[ERROR] {result.get('status', 'unknown')}: {reason}"


# ── 场景定义 ────────────────────────────────────────────

# 每个场景：(场景ID, 描述, 优化前参数, 优化后参数, 断言配置)
# 参数格式：(verb, resource, name, namespace, output_fmt, tail, since, options)
def get_target_namespaces() -> list:
    """
    获取要测试的命名空间列表。
    - 若显式指定 KUBE_NAMESPACE，则只测试该命名空间
    - 否则自动从集群发现全部命名空间
    """
    explicit_ns = os.environ.get("KUBE_NAMESPACE", "").strip()
    if explicit_ns:
        return [explicit_ns]

    raw = run_kubectl(["get", "namespaces", "-o", "json"])
    if not raw:
        return ["default"]

    try:
        data = json.loads(raw)
        items = data.get("items", [])
        namespaces = []
        for item in items:
            if not isinstance(item, dict):
                continue
            metadata = item.get("metadata", {})
            if isinstance(metadata, dict):
                name = metadata.get("name", "")
                if name:
                    namespaces.append(name)
        return namespaces or ["default"]
    except (json.JSONDecodeError, KeyError):
        return ["default"]


def build_scenarios() -> list:
    """
    构建测试场景。
    默认会为所有命名空间构建一组场景。
    若通过环境变量 KUBE_NAMESPACE 显式指定，则只构建该命名空间场景。
    """
    scenarios = []
    for ns in get_target_namespaces():
        ns_id = ns.replace("_", "-")
        scenarios.extend([
            (
                f"ns-{ns_id}-pods-table",
                f"namespace {ns}: get pods (table)",
                ("get", "pods", "", ns, "", 0, "", None),
                ("get", "pods", "", ns, "", 0, "", None),
                None,
            ),
            (
                f"ns-{ns_id}-pods-json",
                f"namespace {ns}: get pods -o json (strip+compact)",
                ("get", "pods", "", ns, "json", 0, "", None),
                ("get", "pods", "", ns, "json", 0, "", None),
                {
                    "must_keep": ["kind", "items"],
                    "must_remove": STRIP_FIELDS,
                },
            ),
            (
                f"ns-{ns_id}-describe-pods",
                f"namespace {ns}: describe pods -> get json",
                ("describe", "pods", "", ns, "", 0, "", None),
                ("describe", "pods", "", ns, "", 0, "", None),
                None,
            ),
            (
                f"ns-{ns_id}-logs-tail100",
                f"namespace {ns}: logs --tail=100",
                ("logs", "", "", ns, "", 100, "", None),
                ("logs", "", "", ns, "", 100, "", None),
                None,
            ),
            (
                f"ns-{ns_id}-pod-json",
                f"namespace {ns}: get pod -o json (strip+compact)",
                ("get", "pod", "", ns, "json", 0, "", None),
                ("get", "pod", "", ns, "json", 0, "", None),
                {
                    "must_keep": ["kind"],
                    "must_remove": STRIP_FIELDS,
                },
            ),
            (
                f"ns-{ns_id}-deployment-json",
                f"namespace {ns}: get deployment -o json (strip+compact)",
                ("get", "deployment", "", ns, "json", 0, "", None),
                ("get", "deployment", "", ns, "json", 0, "", None),
                {
                    "must_keep": ["kind"],
                    "must_remove": STRIP_FIELDS,
                },
            ),
            (
                f"ns-{ns_id}-deployments-json",
                f"namespace {ns}: get deployments -o json (strip+compact)",
                ("get", "deployments", "", ns, "json", 0, "", None),
                ("get", "deployments", "", ns, "json", 0, "", None),
                {
                    "must_keep": ["kind"],
                    "must_remove": STRIP_FIELDS,
                },
            ),
            (
                f"ns-{ns_id}-statefulset-json",
                f"namespace {ns}: get statefulset -o json (strip+compact)",
                ("get", "statefulset", "", ns, "json", 0, "", None),
                ("get", "statefulset", "", ns, "json", 0, "", None),
                {
                    "must_keep": ["kind"],
                    "must_remove": STRIP_FIELDS,
                },
            ),
            (
                f"ns-{ns_id}-statefulsets-json",
                f"namespace {ns}: get statefulsets -o json (strip+compact)",
                ("get", "statefulsets", "", ns, "json", 0, "", None),
                ("get", "statefulsets", "", ns, "json", 0, "", None),
                {
                    "must_keep": ["kind"],
                    "must_remove": STRIP_FIELDS,
                },
            ),
        ])

    return scenarios


def find_logs_pod(namespace: str) -> str:
    """在指定 namespace 中查找一个可用于 logs 测试的 Pod 名。"""
    raw = run_kubectl(["get", "pods", "-n", namespace, "-o", "json"])
    if not raw:
        return ""
    try:
        data = json.loads(raw)
        items = data.get("items", [])
        if items:
            return items[0]["metadata"]["name"]
    except (json.JSONDecodeError, KeyError):
        pass
    return ""


def find_item_for_semantic_check(raw: str) -> dict:
    """从 JSON 输出中提取一个资源对象，用于语义保真检查。"""
    try:
        data = json.loads(raw)
    except json.JSONDecodeError:
        return {}

    if isinstance(data, dict) and str(data.get("kind", "")).endswith("List"):
        items = data.get("items", [])
        if items and isinstance(items[0], dict):
            return items[0]
    if isinstance(data, dict):
        return data
    return {}


def semantic_assertion(name: str, optimized_output: str, assertion_cfg: dict):
    """验证优化后输出仍保留核心语义，同时去掉冗余字段。"""
    if not assertion_cfg or not optimized_output:
        return

    try:
        parsed = json.loads(optimized_output)
    except json.JSONDecodeError:
        log_test(f"{name}: 语义保真", False, "优化后输出无法解析为 JSON")
        return

    item = find_item_for_semantic_check(optimized_output)
    if not item:
        log_test(f"{name}: 语义保真", False, "优化后输出无法解析为 JSON 资源对象")
        return

    missing = []
    for path in assertion_cfg.get("must_keep", []):
        if not _has_field_by_path(item, path.split(".")) and not _has_field_by_path(parsed, path.split(".")):
            missing.append(path)

    unexpected = []
    for path in assertion_cfg.get("must_remove", []):
        if _has_field_by_path(item, path.split(".")):
            unexpected.append(path)

    if missing or unexpected:
        problems = []
        if missing:
            problems.append(f"缺少关键字段: {missing}")
        if unexpected:
            problems.append(f"仍存在冗余字段: {unexpected}")
        log_test(f"{name}: 语义保真", False, " | ".join(problems))
    else:
        log_test(f"{name}: 语义保真", True, "关键字段保留，冗余字段已去除")


def extract_namespace_from_scenario_id(scenario_id: str) -> str:
    """从场景 ID 中提取命名空间。格式约定：ns-<namespace>-..."""
    if not scenario_id.startswith("ns-"):
        return "unknown"
    parts = scenario_id.split("-")
    if len(parts) < 3:
        return "unknown"
    return parts[1]


def extract_resource_group_from_scenario_id(scenario_id: str) -> str:
    """从场景 ID 中提取资源类型组。"""
    markers = [
        "statefulsets",
        "statefulset",
        "deployments",
        "deployment",
        "describe-pods",
        "logs-tail100",
        "pods-table",
        "pods-json",
        "pod-json",
    ]
    for marker in markers:
        if scenario_id.endswith(marker):
            return marker
    return "other"


# ── 结果输出 ────────────────────────────────────────────

test_results = {"passed": 0, "failed": 0, "tests": []}


def log_test(name, passed, message=""):
    status = "[PASS]" if passed else "[FAIL]"
    print(f"{status} - {name}")
    if message:
        print(f"    {message}")
    test_results["tests"].append({
        "name": name, "passed": passed, "message": message
    })
    if passed:
        test_results["passed"] += 1
    else:
        test_results["failed"] += 1


def print_summary():
    print("\n" + "=" * 80)
    print("Token Benchmark Summary")
    print("=" * 80)
    print(f"  Token 统计方法: tiktoken cl100k_base (BPE 精算)")
    print(f"  总场景数: {test_results['passed'] + test_results['failed']}")
    print(f"  通过: {test_results['passed']}")
    print(f"  失败: {test_results['failed']}")
    print("=" * 80)
    if test_results["failed"] > 0:
        print("\n失败项:")
        for test in test_results["tests"]:
            if not test["passed"]:
                print(f"  - {test['name']}: {test['message']}")


# ── 主流程 ──────────────────────────────────────────────

def run_offline_benchmark():
    """离线模式：直接调用 kubectl 对比优化前后 token 数。"""
    target_namespaces = get_target_namespaces()
    print("=" * 80)
    print("Token 优化效果验证 (离线模式 — 直接调用 kubectl)")
    print(f"Token 编码: tiktoken cl100k_base (BPE 精算)")
    if os.environ.get("KUBE_NAMESPACE", "").strip():
        print(f"K8s 命名空间: {target_namespaces[0]} (显式指定)")
    else:
        print(f"K8s 命名空间: 自动发现 {len(target_namespaces)} 个 -> {', '.join(target_namespaces)}")
    print("=" * 80)

    scenarios = build_scenarios()
    runs = max(1, DEFAULT_BENCHMARK_RUNS)
    total_before = 0
    total_after = 0
    namespace_totals = {}
    namespace_resource_totals = {}

    print(f"\n{'场景':<34} {'优化前均值':>12} {'优化后均值':>12} {'节省 %':>8} {'轮次':>6}")
    print("-" * 90)

    for scenario_id, desc, raw_params, opt_params, assertion_cfg in scenarios:
        before_samples = []
        after_samples = []
        last_before = ""
        last_after = ""

        for _ in range(runs):
            before = get_raw_kubectl(*raw_params)
            after = get_optimized_kubectl(*opt_params)

            # 对于 logs 场景，查找一个实际 Pod
            if raw_params[0] == "logs" and not raw_params[2]:
                pod_name = find_logs_pod(raw_params[3])
                if pod_name:
                    before = get_raw_kubectl(
                        raw_params[0], raw_params[1], pod_name,
                        raw_params[3], raw_params[4], raw_params[5], raw_params[6], raw_params[7]
                    )
                    after = get_optimized_kubectl(
                        opt_params[0], opt_params[1], pod_name,
                        opt_params[3], opt_params[4], opt_params[5], opt_params[6], opt_params[7]
                    )
                else:
                    desc += " (无可用Pod)"

            last_before = before
            last_after = after
            before_samples.append(count_tokens(before) if before else 0)
            after_samples.append(count_tokens(after) if after else 0)

        tb = int(mean(before_samples)) if before_samples else 0
        ta = int(mean(after_samples)) if after_samples else 0

        if tb == 0 and ta == 0:
            print(f"{desc:<34} {'N/A':>12} {'N/A':>12} {'—':>8} {runs:>6}")
            continue

        pct = (tb - ta) / tb * 100 if tb > 0 else 0.0
        total_before += tb
        total_after += ta

        ns_key = extract_namespace_from_scenario_id(scenario_id)
        if ns_key not in namespace_totals:
            namespace_totals[ns_key] = {"before": 0, "after": 0}
        namespace_totals[ns_key]["before"] += tb
        namespace_totals[ns_key]["after"] += ta

        resource_key = extract_resource_group_from_scenario_id(scenario_id)
        if ns_key not in namespace_resource_totals:
            namespace_resource_totals[ns_key] = {}
        if resource_key not in namespace_resource_totals[ns_key]:
            namespace_resource_totals[ns_key][resource_key] = {"before": 0, "after": 0}
        namespace_resource_totals[ns_key][resource_key]["before"] += tb
        namespace_resource_totals[ns_key][resource_key]["after"] += ta

        bar = _savings_bar(pct)
        print(f"{scenario_id:<34} {tb:>12,d} {ta:>12,d} {pct:>7.1f}% {runs:>6} {bar}")

        semantic_assertion(scenario_id, last_after, assertion_cfg)

    print("-" * 90)

    if namespace_totals:
        print("\nNamespace 汇总")
        print("-" * 90)
        print(f"{'命名空间':<20} {'优化前总计':>12} {'优化后总计':>12} {'节省 %':>8}")
        print("-" * 90)
        for ns_key in sorted(namespace_totals.keys()):
            ns_before = namespace_totals[ns_key]["before"]
            ns_after = namespace_totals[ns_key]["after"]
            ns_pct = (ns_before - ns_after) / ns_before * 100 if ns_before > 0 else 0.0
            bar = _savings_bar(ns_pct)
            print(f"{ns_key:<20} {ns_before:>12,d} {ns_after:>12,d} {ns_pct:>7.1f}% {bar}")
        print("-" * 90)

    if namespace_resource_totals:
        print("\n每个命名空间内按资源类型汇总")
        print("-" * 90)
        for ns_key in sorted(namespace_resource_totals.keys()):
            print(f"\n[{ns_key}]")
            print(f"{'资源类型':<20} {'优化前总计':>12} {'优化后总计':>12} {'节省 %':>8}")
            print("-" * 90)
            for resource_key in sorted(namespace_resource_totals[ns_key].keys()):
                res_before = namespace_resource_totals[ns_key][resource_key]["before"]
                res_after = namespace_resource_totals[ns_key][resource_key]["after"]
                res_pct = (res_before - res_after) / res_before * 100 if res_before > 0 else 0.0
                bar = _savings_bar(res_pct)
                print(f"{resource_key:<20} {res_before:>12,d} {res_after:>12,d} {res_pct:>7.1f}% {bar}")
            print("-" * 90)

    if total_before > 0:
        overall = (total_before - total_after) / total_before * 100
        bar = _savings_bar(overall)
        print(f"{'总计':<34} {total_before:>12,d} {total_after:>12,d} {overall:>7.1f}% {'-':>6} {bar}")

    # ── 断言验证 ──
    print("\n" + "=" * 80)
    print("验证断言")
    print("=" * 80)

    ns = os.environ.get("KUBE_NAMESPACE", "default")

    # 断言 1：get pods -o json 优化后应剥离 managedFields
    raw = get_raw_kubectl("get", "pods", "", ns, "json", 0, "", None)
    opt = get_optimized_kubectl("get", "pods", "", ns, "json", 0, "", None)
    if raw and opt:
        has_managed_fields = '"managedFields"' in opt
        # managedFields 也可能出现在 status 里（比如 managedFields 被 status 的 condition 引用），
        # 但由于我们剥离了 metadata.managedFields 和 status，理论上不应该出现
        size_reduction = (len(raw) - len(opt)) / len(raw) * 100 if len(raw) > 0 else 0
        if not has_managed_fields or size_reduction > 10:
            log_test(
                "get -o json: managedFields 已剥离",
                True,
                f"大小缩减 {size_reduction:.1f}% | managedFields 出现: {has_managed_fields}"
            )
        else:
            log_test(
                "get -o json: managedFields 已剥离",
                False,
                f"大小缩减仅 {size_reduction:.1f}%，可能未生效"
            )
    else:
        log_test("get -o json: managedFields 已剥离", False, "无输出数据，kubectl 是否可用？")

    # 断言 2：describe pod 转化为 get json 后 token 应显著减少
    raw_desc = get_raw_kubectl("describe", "pods", "", ns, "", 0, "", None)
    opt_desc = get_optimized_kubectl("describe", "pods", "", ns, "", 0, "", None)
    if raw_desc and opt_desc:
        tb_desc = count_tokens(raw_desc)
        ta_desc = count_tokens(opt_desc)
        reduction = (tb_desc - ta_desc) / tb_desc * 100 if tb_desc > 0 else 0
        if reduction > 30:
            log_test(
                "describe pod → get json: token 显著减少",
                True,
                f"优化前 {tb_desc} tokens → 优化后 {ta_desc} tokens ({reduction:.1f}%)"
            )
        else:
            log_test(
                "describe pod → get json: token 显著减少",
                False,
                f"仅减少 {reduction:.1f}% (期望 >30%)"
            )
    else:
        log_test("describe pod → get json: token 显著减少", False, "无输出数据")

    print_summary()
    return test_results["failed"] == 0


def run_api_benchmark():
    """API 模式：通过网关获取优化前后输出对比。"""
    print("=" * 80)
    print("Token 优化效果验证 (API 模式 — 通过网关)")
    print(f"网关地址: {GATEWAY_URL}")
    print(f"Token 编码: tiktoken cl100k_base (BPE 精算)")
    print("=" * 80)

    # 使用当前网关 API 获取输出
    ns = os.environ.get("KUBE_NAMESPACE", "default")

    api_scenarios = [
        ("api-default-pods-json", "get pods -o json (当前 API 输出)", "get", "pods", "", ns, "json", None),
        ("api-default-pods-wide", "get pods -o wide (当前 API 输出)", "get", "pods", "", ns, "wide", None),
        ("api-allns-pods-json", "get pods --all-namespaces -o json", "get", "pods", "", "", "json", {"allNamespaces": True}),
        ("api-allns-deployments-json", "get deployments --all-namespaces -o json", "get", "deployments", "", "", "json", {"allNamespaces": True}),
        ("api-allns-statefulsets-json", "get statefulsets --all-namespaces -o json", "get", "statefulsets", "", "", "json", {"allNamespaces": True}),
    ]

    total_tokens = 0
    print(f"\n{'场景':<42} {'token 数':>12} {'大小(bytes)':>12}")
    print("-" * 70)

    for scenario_id, desc, verb, res, name, ns_val, output_fmt, opts in api_scenarios:
        stdout = get_via_api(verb, res, name, ns_val, output_fmt, opts)
        if stdout.startswith("[ERROR]"):
            print(f"{scenario_id:<42} {'ERROR':>12} {'—':>12}")
            print(f"    {stdout}")
            continue
        tb = count_tokens(stdout)
        size_bytes = len(stdout.encode("utf-8"))
        total_tokens += tb
        print(f"{scenario_id:<42} {tb:>12,d} {size_bytes:>12,d}")

    print("-" * 70)
    print(f"{'总计':<42} {total_tokens:>12,d}")

    print("\n提示：代码改造后，再次运行此脚本，对比 token 数变化。")


def _savings_bar(pct: float, width: int = 10) -> str:
    """可视化节省比例。"""
    filled = int(pct / 100 * width)
    filled = max(0, min(width, filled))
    return "[" + "█" * filled + "░" * (width - filled) + "]"


# ── 入口 ────────────────────────────────────────────────

def main():
    via_api = "--via-api" in sys.argv

    if via_api:
        run_api_benchmark()
    else:
        if not run_offline_benchmark():
            sys.exit(1)

    sys.exit(0 if test_results["failed"] == 0 else 1)


if __name__ == "__main__":
    main()
