package executor

import (
	"fmt"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/model"
)

// Builder builds kubectl arguments from ExecutionRequest.
type Builder struct{}

// NewBuilder creates a new Builder.
func NewBuilder() *Builder {
	return &Builder{}
}

// BuildArgs builds the arguments for kubectl command.
func (b *Builder) BuildArgs(req *model.ExecutionRequest) []string {
	args := []string{req.Verb}

	// kubectl logs 的特殊处理：不需要添加资源类型，直接用 pod 名称
	// 正确格式：kubectl logs <pod-name> 或 kubectl logs <resource>/<name>
	// 错误格式：kubectl logs pods <pod-name> （会把 pods 当作 pod 名）
	if req.Verb == "logs" {
		if req.Name != "" {
			args = append(args, req.Name)
		}
	} else {
		// 添加资源类型
		if req.Resource != "" {
			args = append(args, req.Resource)
		}

		// 添加资源名称
		if req.Name != "" {
			args = append(args, req.Name)
		}
	}

	// 添加子资源
	if req.Subresource != "" {
		args = append(args, req.Subresource)
	}

	// 添加命名空间
	if req.Namespace != "" {
		args = append(args, "-n", req.Namespace)
	}

	// 添加选项
	if req.Options != nil {
		// 标签选择器
		if req.Options.LabelSelector != "" {
			args = append(args, "-l", req.Options.LabelSelector)
		}

		// 字段选择器
		if req.Options.FieldSelector != "" {
			args = append(args, "--field-selector", req.Options.FieldSelector)
		}

	}

	// 输出格式
	if req.Output != "" {
		args = append(args, "-o", req.Output)
	}

	if req.Options != nil {
		// 日志相关参数
		if req.Verb == "logs" {
			if req.Options.TailLines > 0 {
				args = append(args, "--tail", fmt.Sprintf("%d", req.Options.TailLines))
			}
			if req.Options.Since != "" {
				args = append(args, "--since", req.Options.Since)
			}
			if req.Options.Container != "" {
				args = append(args, "-c", req.Options.Container)
			}
			if req.Options.Follow {
				args = append(args, "-f")
			}
			if req.Options.Previous {
				args = append(args, "-p")
			}
		}

		// 限制数量
		if req.Options.Limit > 0 {
			args = append(args, "--limit", fmt.Sprintf("%d", req.Options.Limit))
		}

		// 所有命名空间
		if req.Options.AllNamespaces {
			args = append(args, "--all-namespaces")
		}
	}

	return args
}
