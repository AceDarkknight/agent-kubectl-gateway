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
			name: "Logs with options",
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
			expected: []string{"logs", "my-pod", "--tail", "100", "-c", "my-container", "-f"},
		},
		{
			name: "Logs without resource (pod name only)",
			req: &model.ExecutionRequest{
				Verb: "logs",
				Name: "my-pod",
				Options: &model.Options{
					TailLines: 50,
				},
			},
			expected: []string{"logs", "my-pod", "--tail", "50"},
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
