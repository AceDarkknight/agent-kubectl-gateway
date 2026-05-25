package filter

import (
	"encoding/json"
	"strings"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/audit"

	corev1 "k8s.io/api/core/v1"

	"go.uber.org/zap"
	"go.yaml.in/yaml/v3"
)

// stripDefaults 处理 strip_defaults 动作的入口。
// 使用 k8s.io/api typed struct + 自行实现的 Kubernetes scheme defaulting 逻辑。
// 流程：map → typed struct → 清空待检测字段 → 应用 defaults → 比较 → 从 map 中删除匹配项。
func (f *Filter) stripDefaults(content, outputFormat, resource string) string {
	audit.Debug("[Filter.stripDefaults] 开始默认值裁剪",
		zap.String("output_format", outputFormat),
		zap.String("resource", resource),
		zap.Int("content_size", len(content)))

	switch outputFormat {
	case "json":
		return f.stripDefaultsJSON(content, resource)
	case "yaml":
		return f.stripDefaultsYAML(content, resource)
	default:
		audit.Debug("[Filter.stripDefaults] 非结构化格式，跳过默认值裁剪")
		return content
	}
}

// stripDefaultsJSON 对 JSON 格式内容执行默认值裁剪，输出 compact JSON。
func (f *Filter) stripDefaultsJSON(content, resource string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		audit.Warn("[Filter.stripDefaultsJSON] JSON 解析失败，返回原始内容", zap.Error(err))
		return content
	}

	kind, _ := data["kind"].(string)

	if strings.HasSuffix(kind, "List") {
		if items, ok := data["items"].([]interface{}); ok {
			for _, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					f.stripDefaultsSingleResource(itemMap, resource)
				}
			}
		}
	} else {
		f.stripDefaultsSingleResource(data, resource)
	}

	// compact JSON 输出
	result, err := json.Marshal(data)
	if err != nil {
		audit.Warn("[Filter.stripDefaultsJSON] JSON 序列化失败，返回原始内容", zap.Error(err))
		return content
	}

	audit.Debug("[Filter.stripDefaultsJSON] 裁剪完成", zap.Int("result_size", len(result)))
	return string(result)
}

// stripDefaultsYAML 对 YAML 格式内容执行默认值裁剪，输出保持 YAML。
func (f *Filter) stripDefaultsYAML(content, resource string) string {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		audit.Warn("[Filter.stripDefaultsYAML] YAML 解析失败，返回原始内容", zap.Error(err))
		return content
	}

	kind, _ := data["kind"].(string)

	if strings.HasSuffix(kind, "List") {
		if items, ok := data["items"].([]interface{}); ok {
			for _, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					f.stripDefaultsSingleResource(itemMap, resource)
				}
			}
		}
	} else {
		f.stripDefaultsSingleResource(data, resource)
	}

	result, err := yaml.Marshal(data)
	if err != nil {
		audit.Warn("[Filter.stripDefaultsYAML] YAML 序列化失败，返回原始内容", zap.Error(err))
		return content
	}

	audit.Debug("[Filter.stripDefaultsYAML] 裁剪完成", zap.Int("result_size", len(result)))
	return string(result)
}

// stripDefaultsSingleResource 对单个资源 map 执行默认值裁剪。
// 根据 resource 类型定位到 PodSpec 所在位置。
func (f *Filter) stripDefaultsSingleResource(data map[string]interface{}, resource string) {
	spec, ok := data["spec"].(map[string]interface{})
	if !ok {
		return
	}

	switch {
	case isPodResource(resource):
		f.stripPodSpecFromMap(spec)
	case isDeploymentResource(resource), isStatefulSetResource(resource):
		template, ok := spec["template"].(map[string]interface{})
		if !ok {
			return
		}
		templateSpec, ok := template["spec"].(map[string]interface{})
		if !ok {
			return
		}
		f.stripPodSpecFromMap(templateSpec)
	}
}

// stripPodSpecFromMap 使用 typed struct + scheme defaulting 从 PodSpec map 中裁剪默认值。
func (f *Filter) stripPodSpecFromMap(specMap map[string]interface{}) {
	// map → typed PodSpec
	specJSON, err := json.Marshal(specMap)
	if err != nil {
		return
	}
	var spec corev1.PodSpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		return
	}

	// 创建清空了待检测字段的副本，然后应用 defaulting
	defaulted := f.getDefaultedPodSpec(&spec)

	// PodSpec 顶层字段比较与删除
	if spec.DNSPolicy == defaulted.DNSPolicy {
		delete(specMap, "dnsPolicy")
	}
	if spec.RestartPolicy == defaulted.RestartPolicy {
		delete(specMap, "restartPolicy")
	}
	if spec.SchedulerName == defaulted.SchedulerName {
		delete(specMap, "schedulerName")
	}
	if spec.TerminationGracePeriodSeconds != nil && defaulted.TerminationGracePeriodSeconds != nil &&
		*spec.TerminationGracePeriodSeconds == *defaulted.TerminationGracePeriodSeconds {
		delete(specMap, "terminationGracePeriodSeconds")
	}
	if spec.EnableServiceLinks != nil && defaulted.EnableServiceLinks != nil &&
		*spec.EnableServiceLinks == *defaulted.EnableServiceLinks {
		delete(specMap, "enableServiceLinks")
	}
	if isNilOrEmptyPodSecurityContext(spec.SecurityContext) {
		delete(specMap, "securityContext")
	}

	// preemptionPolicy: 仅删除默认值 "PreemptLowerPriority"
	if pp, ok := specMap["preemptionPolicy"].(string); ok && pp == "PreemptLowerPriority" {
		delete(specMap, "preemptionPolicy")
	}

	// tolerations: 仅精确删除两条 Kubernetes 默认注入的 toleration
	stripDefaultTolerations(specMap)

	// containers
	if containers, ok := specMap["containers"].([]interface{}); ok {
		for i, c := range containers {
			if cmap, ok := c.(map[string]interface{}); ok && i < len(spec.Containers) && i < len(defaulted.Containers) {
				f.stripContainerFromMap(cmap, &spec.Containers[i], &defaulted.Containers[i])
			}
		}
	}

	// initContainers
	if initContainers, ok := specMap["initContainers"].([]interface{}); ok {
		for i, c := range initContainers {
			if cmap, ok := c.(map[string]interface{}); ok && i < len(spec.InitContainers) && i < len(defaulted.InitContainers) {
				f.stripContainerFromMap(cmap, &spec.InitContainers[i], &defaulted.InitContainers[i])
			}
		}
	}
}

// getDefaultedPodSpec 通过清空待检测字段 → 重新应用 defaulting 获取真实默认值。
// defaulting 逻辑与 Kubernetes scheme.defaulting 保持一致：
//   - dnsPolicy: hostNetwork 时为 DNSDefault，否则为 DNSClusterFirst
//   - imagePullPolicy: 依赖 image tag（:latest / 无 tag → Always，其他 → IfNotPresent）
//   - 其他字段使用 Kubernetes 稳定的 scheme 默认值
func (f *Filter) getDefaultedPodSpec(original *corev1.PodSpec) *corev1.PodSpec {
	cp := original.DeepCopy()

	// 清空 PodSpec 顶层可被 default 的字段
	cp.DNSPolicy = ""
	cp.RestartPolicy = ""
	cp.SchedulerName = ""
	cp.TerminationGracePeriodSeconds = nil
	cp.EnableServiceLinks = nil
	cp.SecurityContext = nil

	// 清空容器级别可被 default 的字段（保留 Image 以支持 imagePullPolicy 的上下文推导）
	for i := range cp.Containers {
		cp.Containers[i].ImagePullPolicy = ""
		cp.Containers[i].TerminationMessagePath = ""
		cp.Containers[i].TerminationMessagePolicy = ""
		cp.Containers[i].SecurityContext = nil
	}
	for i := range cp.InitContainers {
		cp.InitContainers[i].ImagePullPolicy = ""
		cp.InitContainers[i].TerminationMessagePath = ""
		cp.InitContainers[i].TerminationMessagePolicy = ""
		cp.InitContainers[i].SecurityContext = nil
	}

	// 应用 Kubernetes scheme defaulting
	applyPodSpecDefaults(cp)

	return cp
}

// applyPodSpecDefaults 对 PodSpec 应用与 Kubernetes scheme.defaulting 一致的默认值。
// 仅设置已清空（零值）的字段，不影响已有值的字段。
func applyPodSpecDefaults(spec *corev1.PodSpec) {
	// dnsPolicy: 依赖 hostNetwork
	if spec.DNSPolicy == "" {
		if spec.HostNetwork {
			spec.DNSPolicy = corev1.DNSDefault
		} else {
			spec.DNSPolicy = corev1.DNSClusterFirst
		}
	}

	// restartPolicy
	if spec.RestartPolicy == "" {
		spec.RestartPolicy = corev1.RestartPolicyAlways
	}

	// schedulerName
	if spec.SchedulerName == "" {
		spec.SchedulerName = "default-scheduler"
	}

	// terminationGracePeriodSeconds
	if spec.TerminationGracePeriodSeconds == nil {
		spec.TerminationGracePeriodSeconds = int64Ptr(30)
	}

	// enableServiceLinks
	if spec.EnableServiceLinks == nil {
		spec.EnableServiceLinks = boolPtr(true)
	}

	// containers
	for i := range spec.Containers {
		applyContainerDefaults(&spec.Containers[i])
	}
	// initContainers
	for i := range spec.InitContainers {
		applyContainerDefaults(&spec.InitContainers[i])
	}
}

// applyContainerDefaults 对容器应用与 Kubernetes scheme.defaulting 一致的默认值。
func applyContainerDefaults(c *corev1.Container) {
	// imagePullPolicy: 依赖 image tag
	if c.ImagePullPolicy == "" {
		c.ImagePullPolicy = defaultImagePullPolicy(c.Image)
	}
	// terminationMessagePath
	if c.TerminationMessagePath == "" {
		c.TerminationMessagePath = corev1.TerminationMessagePathDefault
	}
	// terminationMessagePolicy
	if c.TerminationMessagePolicy == "" {
		c.TerminationMessagePolicy = corev1.TerminationMessageReadFile
	}
}

// defaultImagePullPolicy 与 Kubernetes 的 DefaultImagePullPolicy 一致：
// :latest 或无 tag → Always，其他 → IfNotPresent
func defaultImagePullPolicy(image string) corev1.PullPolicy {
	if strings.HasSuffix(image, ":latest") || !strings.Contains(image, ":") {
		return corev1.PullAlways
	}
	return corev1.PullIfNotPresent
}

// stripContainerFromMap 比较原始容器与 defaulted 容器，从 map 中删除匹配默认值的字段。
func (f *Filter) stripContainerFromMap(cmap map[string]interface{}, original, defaulted *corev1.Container) {
	if original.ImagePullPolicy == defaulted.ImagePullPolicy {
		delete(cmap, "imagePullPolicy")
	}
	if original.TerminationMessagePath != "" && defaulted.TerminationMessagePath != "" &&
		original.TerminationMessagePath == defaulted.TerminationMessagePath {
		delete(cmap, "terminationMessagePath")
	}
	if original.TerminationMessagePolicy == defaulted.TerminationMessagePolicy {
		delete(cmap, "terminationMessagePolicy")
	}
	if isNilOrEmptySecurityContext(original.SecurityContext) {
		delete(cmap, "securityContext")
	}
}

// --- 辅助函数 ---

func int64Ptr(v int64) *int64 { return &v }
func boolPtr(v bool) *bool    { return &v }

// isNilOrEmptyPodSecurityContext 判断 PodSecurityContext 是否为空（所有字段均为零值）。
func isNilOrEmptyPodSecurityContext(sc *corev1.PodSecurityContext) bool {
	if sc == nil {
		return false
	}
	return sc.RunAsNonRoot == nil &&
		sc.RunAsUser == nil &&
		sc.RunAsGroup == nil &&
		sc.FSGroup == nil &&
		sc.SeccompProfile == nil &&
		sc.WindowsOptions == nil &&
		sc.Sysctls == nil &&
		sc.SupplementalGroups == nil &&
		sc.FSGroupChangePolicy == nil
}

// isNilOrEmptySecurityContext 判断容器 SecurityContext 是否为空。
func isNilOrEmptySecurityContext(sc *corev1.SecurityContext) bool {
	if sc == nil {
		return false
	}
	return sc.RunAsNonRoot == nil &&
		sc.RunAsUser == nil &&
		sc.RunAsGroup == nil &&
		sc.Capabilities == nil &&
		sc.Privileged == nil &&
		sc.ReadOnlyRootFilesystem == nil &&
		sc.AllowPrivilegeEscalation == nil &&
		sc.SeccompProfile == nil &&
		sc.WindowsOptions == nil
}

// isPodResource 判断资源是否为 Pod 类型。
func isPodResource(resource string) bool {
	return resource == "pod" || resource == "pods"
}

// isDeploymentResource 判断资源是否为 Deployment 类型。
func isDeploymentResource(resource string) bool {
	return resource == "deployment" || resource == "deployments"
}

// isStatefulSetResource 判断资源是否为 StatefulSet 类型。
func isStatefulSetResource(resource string) bool {
	return resource == "statefulset" || resource == "statefulsets"
}

// stripDefaultTolerations 精确删除 Kubernetes 默认注入的两条 toleration。
// 这两条 toleration 由 kubelet/apiserver admission 自动注入，不是用户显式配置：
//   - node.kubernetes.io/not-ready:NoExecute:300
//   - node.kubernetes.io/unreachable:NoExecute:300
//
// 采用全字段精确匹配，只删除完全一致的条目，不会误删用户自定义 toleration。
func stripDefaultTolerations(specMap map[string]interface{}) {
	tolerations, ok := specMap["tolerations"].([]interface{})
	if !ok || len(tolerations) == 0 {
		return
	}

	filtered := make([]interface{}, 0, len(tolerations))
	for _, t := range tolerations {
		if isDefaultK8sToleration(t) {
			continue
		}
		filtered = append(filtered, t)
	}

	if len(filtered) == 0 {
		delete(specMap, "tolerations")
	} else {
		specMap["tolerations"] = filtered
	}
}

// isDefaultK8sToleration 判断 toleration 是否为 Kubernetes 默认注入的 toleration。
// 仅精确匹配以下两条：
//   - {"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}
//   - {"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}
func isDefaultK8sToleration(t interface{}) bool {
	m, ok := t.(map[string]interface{})
	if !ok {
		return false
	}

	effect, _ := m["effect"].(string)
	if effect != "NoExecute" {
		return false
	}
	operator, _ := m["operator"].(string)
	if operator != "Exists" {
		return false
	}
	key, _ := m["key"].(string)
	if key != "node.kubernetes.io/not-ready" && key != "node.kubernetes.io/unreachable" {
		return false
	}

	// tolerationSeconds 必须存在且为 300
	// JSON 反序列化为 float64，YAML 反序列化为 int
	switch ts := m["tolerationSeconds"].(type) {
	case float64:
		if ts != 300 {
			return false
		}
	case int:
		if ts != 300 {
			return false
		}
	default:
		return false
	}

	return true
}

