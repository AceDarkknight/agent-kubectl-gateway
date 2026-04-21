#!/usr/bin/env python3
# -*- coding: utf-8 -*-
import json
import os
import sys
import urllib.request
import urllib.error

GATEWAY_URL = os.environ.get("GATEWAY_BASE_URL", "")
TOKEN = os.environ.get("GATEWAY_AUTH_TOKEN", "")
test_results = {"passed": 0, "failed": 0, "tests": []}

def send_structured_request(verb, resource, namespace="", name="", output="", subresource=""):
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

def test_rate_limiting():
    print("\n" + "=" * 60)
    print("Test 5: Rate Limiting - Rapid high-frequency requests")
    print("=" * 60)
    
    # 网关配置为每秒10个请求，burst为20
    # 连续发送25个请求以触发限流
    num_requests = 25
    found_429 = False
    status_codes = []
    
    print(f"Sending {num_requests} rapid requests to trigger rate limiting...")
    for i in range(num_requests):
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
        req = urllib.request.Request(f"{GATEWAY_URL}/execute", data=payload_bytes, headers=headers, method="POST")
        try:
            with urllib.request.urlopen(req, timeout=60) as response:
                status_code = response.status
                status_codes.append(status_code)
                if status_code == 429:
                    found_429 = True
                    print(f"  Request {i+1}: HTTP 429 Too Many Requests")
                    break
                else:
                    print(f"  Request {i+1}: HTTP {status_code}")
        except urllib.error.HTTPError as e:
            status_codes.append(e.code)
            if e.code == 429:
                found_429 = True
                print(f"  Request {i+1}: HTTP 429 Too Many Requests")
                break
            else:
                print(f"  Request {i+1}: HTTP {e.code} {e.reason}")
        except Exception as e:
            print(f"  Request {i+1}: Error - {str(e)}")
    
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
    test_rate_limiting()
    print_summary()
    sys.exit(1 if test_results["failed"] > 0 else 0)

if __name__ == "__main__":
    main()
