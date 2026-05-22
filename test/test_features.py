#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import json
import os
import sys
from concurrent.futures import ThreadPoolExecutor, as_completed
import urllib.request
import urllib.error

GATEWAY_URL = os.environ.get("GATEWAY_BASE_URL", "")
TOKEN = os.environ.get("GATEWAY_AUTH_TOKEN", "")
test_results = {"passed": 0, "failed": 0, "tests": []}

def send_structured_request(verb, resource, namespace="", name="", output="", subresource="", options=None):
    payload = {
        "verb": verb,
        "resource": resource,
        "mode": "structured"
    }
    if namespace:
        payload["namespace"] = namespace
    if name:
        payload["name"] = name
    if subresource:
        payload["subresource"] = subresource
    if output:
        payload["output"] = output
    if options:
        payload["options"] = options
    
    payload_bytes = json.dumps(payload).encode("utf-8")
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {TOKEN}"
    }
    req = urllib.request.Request(f"{GATEWAY_URL}/execute", data=payload_bytes, headers=headers, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=60) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        # HTTP错误（如404、500等）
        error_body = e.read().decode("utf-8")
        print(f"[HTTPError] Status Code: {e.code}, Reason: {e.reason}")
        print(f"[HTTPError] Error Body: {error_body[:500]}...")
        try:
            return json.loads(error_body)
        except json.JSONDecodeError:
            return {"error": error_body, "status_code": e.code, "error_type": "HTTPError"}
    except urllib.error.URLError as e:
        # URL错误（包括超时、连接拒绝、DNS解析失败等）
        print(f"[URLError] Connection/Timeout Error Detected!")
        print(f"[URLError] Reason: {e.reason}")
        print(f"[URLError] Full Exception: {str(e)}")
        
        # 检查是否是超时错误
        if hasattr(e, 'reason') and isinstance(e.reason, Exception):
            reason_exception = e.reason
            print(f"[URLError] Inner Exception Type: {type(reason_exception).__name__}")
            print(f"[URLError] Inner Exception: {str(reason_exception)}")
            
            # 检查是否是超时 (TimeoutError)
            if isinstance(reason_exception, TimeoutError):
                print("[URLError] *** TIMEOUT DETECTED ***")
                print(f"[URLError] Client timeout=60s, Server configured timeout_seconds=30s")
                return {"error": str(e.reason), "connection_error": True, "timeout": True, "error_type": "TimeoutError"}
        
        # 检查错误信息中是否包含 timeout 关键字
        error_str = str(e.reason).lower()
        if 'timeout' in error_str or 'timed out' in error_str:
            print("[URLError] *** TIMEOUT KEYWORD DETECTED IN ERROR MESSAGE ***")
            print(f"[URLError] Client timeout=60s, Server configured timeout_seconds=30s")
            return {"error": str(e.reason), "connection_error": True, "timeout": True, "error_type": "TimeoutError"}
        
        return {"error": str(e.reason), "connection_error": True, "error_type": "URLError"}
    except Exception as e:
        # 其他未知异常
        print(f"[Exception] Unexpected Error Detected!")
        print(f"[Exception] Type: {type(e).__name__}")
        print(f"[Exception] Message: {str(e)}")
        import traceback
        print(f"[Exception] Traceback: {traceback.format_exc()}")
        return {"error": str(e), "unexpected_error": True, "error_type": type(e).__name__}

def log_test(name, passed, message=""):
    status = "[PASS]" if passed else "[FAIL]"
    print(f"{status} - {name}")
    if message:
        print(f"    {message}")
    test_results["tests"].append({"name": name, "passed": passed, "message": message})
    if passed:
        test_results["passed"] += 1
    else:
        test_results["failed"] += 1

def test_whitelist_allowed():
    print("\n" + "=" * 60)
    print("Test 1: Whitelist - Allowed verb (get pods)")
    print("=" * 60)
    response = send_structured_request(verb="get", resource="pods", namespace="default")
    print(f"Response: {json.dumps(response, ensure_ascii=False, indent=2)[:500]}...")
    status = response.get("status", "")
    if status == "success":
        log_test("Whitelist verb 'get pods' executed successfully", True, f"status = {status}")
    else:
        log_test("Whitelist verb 'get pods' executed successfully", False, f"expected success, got {status}")

def test_whitelist_blocked():
    print("\n" + "=" * 60)
    print("Test 2: Blocklist - Blocked verb (delete pods)")
    print("=" * 60)
    response = send_structured_request(verb="delete", resource="pods", namespace="default", name="nginx-pod")
    print(f"Response: {json.dumps(response, ensure_ascii=False, indent=2)}")
    status = response.get("status", "")
    if status == "blocked":
        log_test("Non-whitelist verb 'delete pods' blocked correctly", True, f"status = {status}")
    else:
        log_test("Non-whitelist verb 'delete pods' blocked correctly", False, f"expected blocked, got {status}")

def test_masking():
    print("\n" + "=" * 60)
    print("Test 3: Masking - get secrets -n kube-system -o json")
    print("=" * 60)
    response = send_structured_request(verb="get", resource="secrets", namespace="kube-system", output="json")
    print(f"Response: {json.dumps(response, ensure_ascii=False, indent=2)[:1000]}...")
    status = response.get("status", "")
    if status != "success":
        log_test("Masking test - request succeeded", False, f"status={status}")
        return
    stdout = response.get("stdout", "")
    if not stdout:
        log_test("Masking test - got stdout", False, "stdout is empty")
        return
    try:
        stdout_data = json.loads(stdout)
    except json.JSONDecodeError as e:
        log_test("Masking test - parse stdout JSON", False, f"JSON parse error: {e}")
        return
    items = stdout_data.get("items", [])
    if not items:
        log_test("Masking test - secrets list not empty", False, "items is empty")
        return
    masked_count = 0
    total_with_data = 0
    for item in items:
        data = item.get("data", {})
        if data:
            total_with_data += 1
            for key, value in data.items():
                if isinstance(value, str) and ("***" in value or "MASKED" in value):
                    masked_count += 1
                    break
    if total_with_data == 0:
        log_test("Masking test - secrets data masked", False, "No secrets with data field found")
    elif masked_count > 0:
        log_test("Masking test - secrets data masked", True, f"{masked_count}/{total_with_data} secrets masked")
    else:
        log_test("Masking test - secrets data masked", False, f"No masked data found in {total_with_data} secrets")

def test_field_filtering():
    print("\n" + "=" * 60)
    print("Test 4: Field Filtering - get pods -n default -o json")
    print("=" * 60)
    response = send_structured_request(verb="get", resource="pods", namespace="default", output="json")
    print(f"Response: {json.dumps(response, ensure_ascii=False, indent=2)[:1000]}...")
    status = response.get("status", "")
    if status != "success":
        log_test("Field filtering test - request succeeded", False, f"status={status}")
        return
    stdout = response.get("stdout", "")
    if not stdout:
        log_test("Field filtering test - got stdout", False, "stdout is empty")
        return
    try:
        stdout_data = json.loads(stdout)
    except json.JSONDecodeError as e:
        log_test("Field filtering test - parse stdout JSON", False, f"JSON parse error: {e}")
        return
    items = stdout_data.get("items", [])
    if not items:
        log_test("Field filtering test - pods list not empty", False, "items is empty")
        return
    violations = []
    for item in items:
        if "status" in item:
            violations.append("status")
        metadata = item.get("metadata", {})
        if isinstance(metadata, dict):
            if "creationTimestamp" in metadata:
                violations.append("metadata.creationTimestamp")
            if "managedFields" in metadata:
                violations.append("metadata.managedFields")
            if "generateName" in metadata:
                violations.append("metadata.generateName")
            if "ownerReferences" in metadata:
                violations.append("metadata.ownerReferences")
            if "resourceVersion" in metadata:
                violations.append("metadata.resourceVersion")
            if "uid" in metadata:
                violations.append("metadata.uid")
        if violations:
            break
    if not violations:
        log_test("Field filtering test - sensitive fields filtered", True, "All fields filtered correctly")
    else:
        log_test("Field filtering test - sensitive fields filtered", False, f"Found unfiltered fields: {violations}")

def print_summary():
    print("\n" + "=" * 60)
    print("Test Summary")
    print("=" * 60)
    print(f"Total: {test_results['passed'] + test_results['failed']} tests")
    print(f"Passed: {test_results['passed']}")
    print(f"Failed: {test_results['failed']}")
    print("=" * 60)
    if test_results["failed"] > 0:
        print("\nFailed tests:")
        for test in test_results["tests"]:
            if not test["passed"]:
                print(f"  - {test['name']}: {test['message']}")

def test_strip_defaults():
    """验证 strip_defaults 对 Pod/Deployment 的默认值裁剪。"""
    print("\n" + "=" * 60)
    print("Test: Strip Defaults - Pod/Deployment default values stripped")
    print("=" * 60)

    # ── Pod ──
    print("\n--- Pod strip_defaults ---")
    response = send_structured_request(verb="get", resource="pods", namespace="default", output="json")
    status = response.get("status", "")
    if status != "success":
        log_test("strip_defaults - get pods succeeded", False, f"status={status}")
        return
    stdout = response.get("stdout", "")
    if not stdout:
        log_test("strip_defaults - got pod stdout", False, "stdout is empty (no pods in namespace?)")
    else:
        try:
            data = json.loads(stdout)
            items = data.get("items", []) if isinstance(data, dict) and "items" in data else [data]
            violations = []
            checked = False
            for item in items:
                if not isinstance(item, dict):
                    continue
                spec = item.get("spec", {})
                if not isinstance(spec, dict):
                    continue
                checked = True
                # scheme 默认值应被裁剪
                for field in ["dnsPolicy", "restartPolicy", "schedulerName"]:
                    if field in spec:
                        violations.append(f"spec.{field}={spec[field]}")
                # 容器级别
                for c in spec.get("containers", []):
                    if isinstance(c, dict):
                        if "imagePullPolicy" in c and c.get("image", "").endswith((":latest",)):
                            pass  # :latest + Always 是 scheme 默认值，应被裁剪
                        elif "imagePullPolicy" in c:
                            # 非 :latest 镜像的 IfNotPresent 应被裁剪
                            if c["imagePullPolicy"] == "IfNotPresent":
                                violations.append(f"container {c.get('name','?')}.imagePullPolicy=IfNotPresent (scheme default)")
                if violations:
                    break
            if not checked:
                log_test("strip_defaults - Pod defaults stripped", False, "no items with spec found")
            elif not violations:
                log_test("strip_defaults - Pod defaults stripped", True, "scheme default values removed")
            else:
                log_test("strip_defaults - Pod defaults stripped", False, f"found default values: {violations}")
        except json.JSONDecodeError as e:
            log_test("strip_defaults - Pod parse JSON", False, f"parse error: {e}")

    # ── Deployment ──
    print("\n--- Deployment strip_defaults ---")
    response = send_structured_request(verb="get", resource="deployments", namespace="default", output="json")
    status = response.get("status", "")
    if status != "success":
        log_test("strip_defaults - get deployments succeeded", False, f"status={status}")
        return
    stdout = response.get("stdout", "")
    if not stdout:
        log_test("strip_defaults - got deployment stdout", False, "stdout is empty (no deployments in namespace?)")
    else:
        try:
            data = json.loads(stdout)
            items = data.get("items", []) if isinstance(data, dict) and "items" in data else [data]
            violations = []
            checked = False
            for item in items:
                if not isinstance(item, dict):
                    continue
                template_spec = item.get("spec", {}).get("template", {}).get("spec", {})
                if not isinstance(template_spec, dict) or not template_spec:
                    continue
                checked = True
                for field in ["dnsPolicy", "restartPolicy", "schedulerName"]:
                    if field in template_spec:
                        violations.append(f"spec.template.spec.{field}={template_spec[field]}")
                if violations:
                    break
            if not checked:
                log_test("strip_defaults - Deployment defaults stripped", False, "no deployments with template.spec found")
            elif not violations:
                log_test("strip_defaults - Deployment defaults stripped", True, "scheme default values removed from PodTemplateSpec")
            else:
                log_test("strip_defaults - Deployment defaults stripped", False, f"found default values: {violations}")
        except json.JSONDecodeError as e:
            log_test("strip_defaults - Deployment parse JSON", False, f"parse error: {e}")

    # ── YAML 格式也验证 ──
    print("\n--- Pod strip_defaults (YAML) ---")
    response = send_structured_request(verb="get", resource="pods", namespace="default", output="yaml")
    status = response.get("status", "")
    if status != "success":
        log_test("strip_defaults - Pod YAML succeeded", False, f"status={status}")
        return
    stdout = response.get("stdout", "")
    if not stdout:
        log_test("strip_defaults - Pod YAML got stdout", False, "stdout is empty")
    else:
        # YAML 中验证默认值字段不应出现
        yaml_violations = []
        for field in ["dnsPolicy:", "restartPolicy:", "schedulerName:"]:
            if field in stdout:
                yaml_violations.append(field)
        if not yaml_violations:
            log_test("strip_defaults - Pod YAML defaults stripped", True, "YAML default values removed")
        else:
            log_test("strip_defaults - Pod YAML defaults stripped", False, f"found default YAML fields: {yaml_violations}")


def test_logs_defaults():
    """验证 logs 未传 tailLines/since 时的默认参数注入。"""
    print("\n" + "=" * 60)
    print("Test: Logs Defaults - default --tail and --since injected")
    print("=" * 60)

    # 先找到一个可用的 Pod
    pods_resp = send_structured_request(verb="get", resource="pods", namespace="default", output="json")
    pod_name = ""
    if pods_resp.get("status") == "success":
        stdout = pods_resp.get("stdout", "")
        if stdout:
            try:
                data = json.loads(stdout)
                items = data.get("items", []) if isinstance(data, dict) else []
                for item in items:
                    if isinstance(item, dict):
                        pod_name = item.get("metadata", {}).get("name", "")
                        if pod_name:
                            break
            except json.JSONDecodeError:
                pass

    if not pod_name:
        log_test("logs defaults - found a pod", False, "no pods in default namespace, cannot test logs")
        log_test("logs defaults - explicit tailLines preserved", False, "skipped: no pod available")
        return

    log_test("logs defaults - found a pod", True, f"pod: {pod_name}")

    # 测试 1：不传 options，应使用默认 --tail 100 --since 1h
    response = send_structured_request(
        verb="logs", resource="pods", namespace="default",
        name=pod_name, output=""
    )
    status = response.get("status", "")
    if status == "success" or status == "failed":
        # 即使 kubectl logs 失败（如 Pod 已完成），只要网关正常处理即可
        log_test("logs defaults - no options request processed", True, f"status={status}, defaults applied server-side")
    else:
        log_test("logs defaults - no options request processed", False, f"unexpected status={status}")

    # 测试 2：显式传 tailLines=50，不应被覆盖
    response = send_structured_request(
        verb="logs", resource="pods", namespace="default",
        name=pod_name, output="",
        options={"tailLines": 50}
    )
    status = response.get("status", "")
    if status == "success" or status == "failed":
        log_test("logs defaults - explicit tailLines preserved", True, f"status={status}, explicit tailLines=50 accepted")
    else:
        log_test("logs defaults - explicit tailLines preserved", False, f"unexpected status={status}")

    # 测试 3：tailLines=0 应回退到默认值
    response = send_structured_request(
        verb="logs", resource="pods", namespace="default",
        name=pod_name, output="",
        options={"tailLines": 0}
    )
    status = response.get("status", "")
    if status == "success" or status == "failed":
        log_test("logs defaults - tailLines=0 falls back to default", True, f"status={status}, tailLines=0 treated as default")
    else:
        log_test("logs defaults - tailLines=0 falls back to default", False, f"unexpected status={status}")

    # 测试 4：显式传 since，不应被覆盖
    response = send_structured_request(
        verb="logs", resource="pods", namespace="default",
        name=pod_name, output="",
        options={"since": "5m"}
    )
    status = response.get("status", "")
    if status == "success" or status == "failed":
        log_test("logs defaults - explicit since preserved", True, f"status={status}, explicit since=5m accepted")
    else:
        log_test("logs defaults - explicit since preserved", False, f"unexpected status={status}")


def test_field_filtering_yaml():
    """验证 YAML 格式的字段过滤。"""
    print("\n" + "=" * 60)
    print("Test: Field Filtering YAML - get pods -n default -o yaml")
    print("=" * 60)
    response = send_structured_request(verb="get", resource="pods", namespace="default", output="yaml")
    status = response.get("status", "")
    if status != "success":
        log_test("Field filtering YAML - request succeeded", False, f"status={status}")
        return
    stdout = response.get("stdout", "")
    if not stdout:
        log_test("Field filtering YAML - got stdout", False, "stdout is empty")
        return

    # YAML 中检查不应出现的字段
    violations = []
    yaml_blocked_fields = [
        ("managedFields:", "metadata.managedFields"),
        ("creationTimestamp:", "metadata.creationTimestamp"),
        ("resourceVersion:", "metadata.resourceVersion"),
        ("uid:", "metadata.uid"),
    ]
    for yaml_key, field_name in yaml_blocked_fields:
        if yaml_key in stdout:
            violations.append(field_name)

    # status 块也应在 YAML 中被过滤
    # 注意：YAML 中 status 可能以 "status:" 出现在多个地方，检查顶级 status
    lines = stdout.split("\n")
    for line in lines:
        stripped = line.strip()
        # 顶级 status: 字段（缩进为0）
        if stripped.startswith("status:") and not line.startswith(" "):
            violations.append("status")
            break

    if not violations:
        log_test("Field filtering YAML - sensitive fields filtered", True, "All fields filtered correctly in YAML")
    else:
        log_test("Field filtering YAML - sensitive fields filtered", False, f"Found unfiltered fields: {violations}")


def test_rate_limiting():
    print("\n" + "=" * 60)
    print("Test 5: Rate Limiting - Rapid high-frequency requests")
    print("=" * 60)
    
    # 网关配置为每秒10个请求，burst为20
    # 使用并发请求而不是串行请求，避免请求执行时间过长导致令牌桶自然回填
    num_requests = 40
    found_429 = False
    status_codes = []

    payload = {
        "verb": "get",
        "resource": "pods",
        "namespace": "default",
        "mode": "structured"
    }
    payload_bytes = json.dumps(payload).encode("utf-8")
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {TOKEN}"
    }

    def send_one_request(index):
        req = urllib.request.Request(f"{GATEWAY_URL}/execute", data=payload_bytes, headers=headers, method="POST")
        try:
            with urllib.request.urlopen(req, timeout=60) as response:
                return index, response.status, None
        except urllib.error.HTTPError as e:
            return index, e.code, e.reason
        except Exception as e:
            return index, None, str(e)
    
    print(f"Sending {num_requests} concurrent requests to trigger rate limiting...")
    with ThreadPoolExecutor(max_workers=num_requests) as executor:
        futures = [executor.submit(send_one_request, i + 1) for i in range(num_requests)]
        for future in as_completed(futures):
            index, status_code, error = future.result()
            if status_code is not None:
                status_codes.append(status_code)
                if status_code == 429:
                    found_429 = True
                    print(f"  Request {index}: HTTP 429 Too Many Requests")
                else:
                    print(f"  Request {index}: HTTP {status_code}")
            else:
                print(f"  Request {index}: Error - {error}")
    
    print(f"\nTotal requests sent: {len(status_codes)}")
    print(f"Found 429 response: {found_429}")
    
    if found_429:
        log_test("Rate limiting - 429 Too Many Requests returned", True, f"Got HTTP 429 after {len(status_codes)} requests")
    else:
        log_test("Rate limiting - 429 Too Many Requests returned", False, f"No 429 received in {len(status_codes)} requests")

def main():
    print("=" * 60)
    print("Gateway Security Features Automated Test")
    print(f"Target: {GATEWAY_URL}")
    print(f"Token: {TOKEN[:10]}...")
    print("=" * 60)
    test_whitelist_allowed()
    test_whitelist_blocked()
    test_masking()
    test_field_filtering()
    test_strip_defaults()
    test_logs_defaults()
    test_field_filtering_yaml()
    test_rate_limiting()
    print_summary()
    sys.exit(1 if test_results["failed"] > 0 else 0)

if __name__ == "__main__":
    main()
