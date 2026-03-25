package filter

import (
	"strings"
	"testing"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/config"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/model"
)

func TestFilterResult(t *testing.T) {
	cfg := &config.RulesConfig{
		Masking: []config.MaskingRule{
			{
				Resource:   "secrets",
				Namespaces: []string{"*"},
				Action:     "mask",
			},
			{
				Resource:   "*",
				Namespaces: []string{"*"},
				Action:     "filter_fields",
				Fields:     []string{"metadata.managedFields", "status"},
			},
		},
	}

	filter := NewFilter(cfg)

	tests := []struct {
		name     string
		req      *model.ExecutionRequest
		result   *model.ExecutionResult
		expected string
	}{
		{
			name: "mask secret in JSON format",
			req: &model.ExecutionRequest{
				Resource:  "secrets",
				Namespace: "default",
				Options: &model.Options{
					Output: "json",
				},
			},
			result: &model.ExecutionResult{
				Status: "success",
				Stdout: `{"kind":"Secret","data":{"password":"cGFzc3dvcmQ=","username":"dXNlcm5hbWU="}}`,
			},
			expected: "*** MASKED BY PROXY ***",
		},
		{
			name: "mask secret in YAML format",
			req: &model.ExecutionRequest{
				Resource:  "secrets",
				Namespace: "default",
				Options: &model.Options{
					Output: "yaml",
				},
			},
			result: &model.ExecutionResult{
				Status: "success",
				Stdout: `kind: Secret
data:
  password: cGFzc3dvcmQ=
  username: dXNlcm5hbMkeitJrbGViS3BsYyc="}`,
			},
			expected: "*** MASKED BY PROXY ***",
		},
		{
			name: "filter fields in JSON format",
			req: &model.ExecutionRequest{
				Resource:  "pods",
				Namespace: "default",
				Options: &model.Options{
					Output: "json",
				},
			},
			result: &model.ExecutionResult{
				Status: "success",
				Stdout: `{"metadata":{"name":"test-pod","managedFields":[{"manager":"kubectl"}]},"status":{"phase":"Running"}}`,
			},
			expected: "managedFields",
		},
		{
			name: "failed result should not be filtered",
			req: &model.ExecutionRequest{
				Resource:  "secrets",
				Namespace: "default",
				Options: &model.Options{
					Output: "json",
				},
			},
			result: &model.ExecutionResult{
				Status: "failed",
				Stdout: `{"kind":"Secret","data":{"password":"cGFzc3dvcmQ="}}`,
			},
			expected: "cGFzc3dvcmQ=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := filter.FilterResult(tt.req, tt.result)
			if tt.name == "failed result should not be filtered" {
				if filtered.Stdout != tt.result.Stdout {
					t.Errorf("expected stdout to be unchanged, got %s", filtered.Stdout)
				}
			} else if tt.name == "filter fields in JSON format" {
				if strings.Contains(filtered.Stdout, tt.expected) {
					t.Errorf("expected field %s to be removed, got %s", tt.expected, filtered.Stdout)
				}
			} else {
				if !strings.Contains(filtered.Stdout, tt.expected) {
					t.Errorf("expected stdout to contain %s, got %s", tt.expected, filtered.Stdout)
				}
			}
		})
	}
}

func TestMaskContent(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	tests := []struct {
		name         string
		content      string
		outputFormat string
		expected     string
	}{
		{
			name:         "mask JSON secret",
			content:      `{"kind":"Secret","data":{"password":"cGFzc3dvcmQ="}}`,
			outputFormat: "json",
			expected:     "*** MASKED BY PROXY ***",
		},
		{
			name:         "mask YAML secret",
			content:      "kind: Secret\ndata:\n  password: cGFzc3dvcmQ=",
			outputFormat: "yaml",
			expected:     "*** MASKED BY PROXY ***",
		},
		{
			name:         "mask plain text with base64",
			content:      "Token: dGhpcyBpcyBhIHZlcnkgbG9uZyBiYXNlNjQgZW5jb2RlZCBzdHJpbmc=",
			outputFormat: "",
			expected:     "*** MASKED BY PROXY ***",
		},
		{
			name:         "mask plain text with bearer token",
			content:      "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			outputFormat: "",
			expected:     "Bearer *** MASKED BY PROXY ***",
		},
		{
			name:         "mask plain text with password",
			content:      "password: mySecretPassword123",
			outputFormat: "",
			expected:     "*** MASKED BY PROXY ***",
		},
		{
			name:         "non-secret JSON should not be masked",
			content:      `{"kind":"Pod","metadata":{"name":"test-pod"}}`,
			outputFormat: "json",
			expected:     "test-pod",
		},
		// 新增：List 结构的测试用例
		{
			name:         "mask JSON List with secrets",
			content:      `{"kind":"List","items":[{"kind":"Secret","data":{"password":"cGFzc3dvcmQ="}},{"kind":"Secret","data":{"token":"dG9rZW4="}}]}`,
			outputFormat: "json",
			expected:     "*** MASKED BY PROXY ***",
		},
		{
			name:         "mask YAML List with secrets",
			content:      "kind: List\nitems:\n  - kind: Secret\n    data:\n      password: cGFzc3dvcmQ=\n  - kind: Secret\n    data:\n      token: dG9rZW4=",
			outputFormat: "yaml",
			expected:     "*** MASKED BY PROXY ***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.maskContent(tt.content, tt.outputFormat)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("expected result to contain %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestFilterFields(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	tests := []struct {
		name         string
		content      string
		fields       []string
		outputFormat string
		shouldRemove bool
	}{
		{
			name:         "filter JSON fields",
			content:      `{"metadata":{"name":"test","managedFields":[{"manager":"kubectl"}]},"status":{"phase":"Running"}}`,
			fields:       []string{"metadata.managedFields", "status"},
			outputFormat: "json",
			shouldRemove: true,
		},
		{
			name:         "filter YAML fields",
			content:      "metadata:\n  name: test\n  managedFields:\n    - manager: kubectl\nstatus:\n  phase: Running",
			fields:       []string{"metadata.managedFields", "status"},
			outputFormat: "yaml",
			shouldRemove: true,
		},
		{
			name:         "non-JSON/YAML format should return original",
			content:      "plain text content",
			fields:       []string{"field1"},
			outputFormat: "",
			shouldRemove: false,
		},
		// 新增：List 结构的字段过滤测试用例
		{
			name:         "filter JSON List fields",
			content:      `{"kind":"List","items":[{"metadata":{"name":"test1","managedFields":[{"manager":"kubectl"}]},"status":{"phase":"Running"}},{"metadata":{"name":"test2","managedFields":[{"manager":"kubectl"}]},"status":{"phase":"Running"}}]}`,
			fields:       []string{"metadata.managedFields", "status"},
			outputFormat: "json",
			shouldRemove: true,
		},
		{
			name:         "filter YAML List fields",
			content:      "kind: List\nitems:\n  - metadata:\n      name: test1\n      managedFields:\n        - manager: kubectl\n    status:\n      phase: Running\n  - metadata:\n      name: test2\n      managedFields:\n        - manager: kubectl\n    status:\n      phase: Running",
			fields:       []string{"metadata.managedFields", "status"},
			outputFormat: "yaml",
			shouldRemove: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.filterFields(tt.content, tt.fields, tt.outputFormat)
			if tt.shouldRemove {
				for _, field := range tt.fields {
					parts := strings.Split(field, ".")
					lastPart := parts[len(parts)-1]
					if strings.Contains(result, lastPart) {
						t.Errorf("expected field %s to be removed, but it still exists in %s", field, result)
					}
				}
			} else {
				if result != tt.content {
					t.Errorf("expected content to be unchanged, got %s", result)
				}
			}
		})
	}
}

func TestFilterWithRegex(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "mask base64 encoded string",
			content:  "Token: dGhpcyBpcyBhIHZlcnkgbG9uZyBiYXNlNjQgZW5jb2RlZCBzdHJpbmc=",
			expected: "*** MASKED BY PROXY ***",
		},
		{
			name:     "mask bearer token",
			content:  "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "Bearer *** MASKED BY PROXY ***",
		},
		{
			name:     "mask password field",
			content:  "password: mySecretPassword123",
			expected: "*** MASKED BY PROXY ***",
		},
		{
			name:     "mask private key",
			content:  "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----",
			expected: "*** MASKED BY PROXY (PRIVATE KEY) ***",
		},
		{
			name:     "mask passwd field",
			content:  "passwd: secret123",
			expected: "*** MASKED BY PROXY ***",
		},
		{
			name:     "mask pwd field",
			content:  "pwd: secret123",
			expected: "*** MASKED BY PROXY ***",
		},
		{
			name:     "non-sensitive content should not be masked",
			content:  "This is a normal text without sensitive data",
			expected: "This is a normal text without sensitive data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.FilterWithRegex(tt.content)
			if !strings.Contains(result, tt.expected) {
				t.Errorf("expected result to contain %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestMatchesRule(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	tests := []struct {
		name      string
		resource  string
		namespace string
		rule      *config.MaskingRule
		expected  bool
	}{
		{
			name:      "match all resources and namespaces",
			resource:  "pods",
			namespace: "default",
			rule: &config.MaskingRule{
				Resource:   "*",
				Namespaces: []string{"*"},
			},
			expected: true,
		},
		{
			name:      "match specific resource",
			resource:  "secrets",
			namespace: "default",
			rule: &config.MaskingRule{
				Resource:   "secrets",
				Namespaces: []string{"*"},
			},
			expected: true,
		},
		{
			name:      "match specific namespace",
			resource:  "pods",
			namespace: "kube-system",
			rule: &config.MaskingRule{
				Resource:   "*",
				Namespaces: []string{"kube-system"},
			},
			expected: true,
		},
		{
			name:      "no match different resource",
			resource:  "pods",
			namespace: "default",
			rule: &config.MaskingRule{
				Resource:   "secrets",
				Namespaces: []string{"*"},
			},
			expected: false,
		},
		{
			name:      "no match different namespace",
			resource:  "pods",
			namespace: "default",
			rule: &config.MaskingRule{
				Resource:   "*",
				Namespaces: []string{"kube-system"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.matchesRule(tt.resource, tt.namespace, tt.rule)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
