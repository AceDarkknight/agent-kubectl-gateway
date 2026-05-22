package executor

import (
	"reflect"
	"testing"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/model"
)

func TestBuilder_BuildArgs(t *testing.T) {
	builder := NewBuilder()

	tests := []struct {
		name     string
		req      *model.ExecutionRequest
		expected []string
	}{
		{
			name: "Basic get pods",
			req: &model.ExecutionRequest{
				Verb:     "get",
				Resource: "pods",
			},
			expected: []string{"get", "pods"},
		},
		{
			name: "Get pod with name and namespace",
			req: &model.ExecutionRequest{
				Verb:      "get",
				Resource:  "pods",
				Name:      "my-pod",
				Namespace: "default",
			},
			expected: []string{"get", "pods", "my-pod", "-n", "default"},
		},
		{
			name: "Logs with explicit tail and container/follow, since defaults",
			req: &model.ExecutionRequest{
				Verb:     "logs",
				Resource: "pods",
				Name:     "my-pod",
				Options: &model.Options{
					TailLines: 100,
					Container: "my-container",
					Follow:    true,
				},
			},
			expected: []string{"logs", "my-pod", "--tail", "100", "--since", "1h", "-c", "my-container", "-f"},
		},
		{
			name: "Logs without resource (pod name only), since defaults",
			req: &model.ExecutionRequest{
				Verb: "logs",
				Name: "my-pod",
				Options: &model.Options{
					TailLines: 50,
				},
			},
			expected: []string{"logs", "my-pod", "--tail", "50", "--since", "1h"},
		},
		{
			name: "Logs with nil options, all defaults injected",
			req: &model.ExecutionRequest{
				Verb: "logs",
				Name: "my-pod",
			},
			expected: []string{"logs", "my-pod", "--tail", "100", "--since", "1h"},
		},
		{
			name: "Logs with tail zero, defaults to --tail 100",
			req: &model.ExecutionRequest{
				Verb: "logs",
				Name: "my-pod",
				Options: &model.Options{
					TailLines: 0,
				},
			},
			expected: []string{"logs", "my-pod", "--tail", "100", "--since", "1h"},
		},
		{
			name: "Logs with explicit tail and since, no defaults",
			req: &model.ExecutionRequest{
				Verb: "logs",
				Name: "my-pod",
				Options: &model.Options{
					TailLines: 50,
					Since:     "30m",
				},
			},
			expected: []string{"logs", "my-pod", "--tail", "50", "--since", "30m"},
		},
		{
			name: "Logs with explicit since only, tail defaults to 100",
			req: &model.ExecutionRequest{
				Verb: "logs",
				Name: "my-pod",
				Options: &model.Options{
					TailLines: 0,
					Since:     "30m",
				},
			},
			expected: []string{"logs", "my-pod", "--tail", "100", "--since", "30m"},
		},
		{
			name: "Logs with namespace and nil options, all defaults injected",
			req: &model.ExecutionRequest{
				Verb:      "logs",
				Name:      "my-pod",
				Namespace: "default",
			},
			expected: []string{"logs", "my-pod", "-n", "default", "--tail", "100", "--since", "1h"},
		},
		{
			name: "Logs with previous flag and defaults",
			req: &model.ExecutionRequest{
				Verb: "logs",
				Name: "my-pod",
				Options: &model.Options{
					Previous: true,
				},
			},
			expected: []string{"logs", "my-pod", "--tail", "100", "--since", "1h", "-p"},
		},
		{
			name: "Get with label selector and output json",
			req: &model.ExecutionRequest{
				Verb:     "get",
				Resource: "deployments",
				Output:   "json",
				Options: &model.Options{
					LabelSelector: "app=backend",
					Limit:         10,
				},
			},
			expected: []string{"get", "deployments", "-l", "app=backend", "-o", "json", "--limit", "10"},
		},
		{
			name: "Get with all namespaces",
			req: &model.ExecutionRequest{
				Verb:     "get",
				Resource: "services",
				Options: &model.Options{
					AllNamespaces: true,
				},
			},
			expected: []string{"get", "services", "--all-namespaces"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := builder.BuildArgs(tt.req)
			if !reflect.DeepEqual(args, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, args)
			}
		})
	}
}
