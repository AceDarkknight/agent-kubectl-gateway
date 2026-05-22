package filter

import (
	"strings"
	"testing"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/config"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/model"
)

// samplePodJSON 返回一个包含 scheme 默认值的 Pod JSON。
// 注意：image 使用 "nginx:1.25"（非 :latest），此时 scheme 默认 imagePullPolicy 为 IfNotPresent。
func samplePodJSON() string {
	return `{
  "kind": "Pod",
  "metadata": {"name": "test-pod"},
  "spec": {
    "dnsPolicy": "ClusterFirst",
    "restartPolicy": "Always",
    "schedulerName": "default-scheduler",
    "enableServiceLinks": true,
    "terminationGracePeriodSeconds": 30,
    "securityContext": {},
    "containers": [
      {
        "name": "main",
        "image": "nginx:1.25",
        "imagePullPolicy": "IfNotPresent",
        "terminationMessagePath": "/dev/termination-log",
        "terminationMessagePolicy": "File",
        "securityContext": {},
        "ports": [{"containerPort": 80}]
      }
    ]
  }
}`
}

// samplePodJSONWithCustomValues 返回一个所有值都非默认的 Pod JSON。
func samplePodJSONWithCustomValues() string {
	return `{
  "kind": "Pod",
  "metadata": {"name": "test-pod"},
  "spec": {
    "dnsPolicy": "Default",
    "restartPolicy": "OnFailure",
    "schedulerName": "my-scheduler",
    "enableServiceLinks": false,
    "terminationGracePeriodSeconds": 60,
    "securityContext": {"runAsNonRoot": true},
    "containers": [
      {
        "name": "main",
        "image": "nginx:1.25",
        "imagePullPolicy": "Always",
        "terminationMessagePath": "/custom/path",
        "terminationMessagePolicy": "FallbackToLogsOnError",
        "securityContext": {"runAsUser": 1000},
        "ports": [{"containerPort": 80}]
      }
    ]
  }
}`
}

// sampleDeploymentJSON 返回一个包含默认值的 Deployment JSON。
func sampleDeploymentJSON() string {
	return `{
  "kind": "Deployment",
  "metadata": {"name": "test-deploy"},
  "spec": {
    "replicas": 3,
    "template": {
      "spec": {
        "dnsPolicy": "ClusterFirst",
        "restartPolicy": "Always",
        "schedulerName": "default-scheduler",
        "enableServiceLinks": true,
        "terminationGracePeriodSeconds": 30,
        "securityContext": {},
        "containers": [
          {
            "name": "main",
            "image": "nginx:1.25",
            "imagePullPolicy": "IfNotPresent",
            "terminationMessagePath": "/dev/termination-log",
            "terminationMessagePolicy": "File",
            "securityContext": {}
          }
        ]
      }
    }
  }
}`
}

// sampleStatefulSetJSON 返回一个包含默认值的 StatefulSet JSON。
func sampleStatefulSetJSON() string {
	return `{
  "kind": "StatefulSet",
  "metadata": {"name": "test-sts"},
  "spec": {
    "replicas": 2,
    "template": {
      "spec": {
        "dnsPolicy": "ClusterFirst",
        "restartPolicy": "Always",
        "schedulerName": "default-scheduler",
        "terminationGracePeriodSeconds": 30,
        "containers": [
          {
            "name": "main",
            "image": "nginx:1.25",
            "imagePullPolicy": "IfNotPresent",
            "terminationMessagePath": "/dev/termination-log",
            "terminationMessagePolicy": "File"
          }
        ]
      }
    }
  }
}`
}

// samplePodListJSON 返回包含多个 Pod 的 List JSON。
func samplePodListJSON() string {
	return `{
  "kind": "PodList",
  "items": [
    {
      "metadata": {"name": "pod-1"},
      "spec": {
        "dnsPolicy": "ClusterFirst",
        "restartPolicy": "Always",
        "schedulerName": "default-scheduler",
        "containers": [{"name": "main", "image": "nginx:1.25", "imagePullPolicy": "IfNotPresent"}]
      }
    },
    {
      "metadata": {"name": "pod-2"},
      "spec": {
        "dnsPolicy": "ClusterFirst",
        "restartPolicy": "Always",
        "schedulerName": "default-scheduler",
        "containers": [{"name": "sidecar", "image": "envoy:1.25", "imagePullPolicy": "IfNotPresent"}]
      }
    }
  ]
}`
}

// sampleDeploymentListJSON 返回包含多个 Deployment 的 List JSON。
func sampleDeploymentListJSON() string {
	return `{
  "kind": "DeploymentList",
  "items": [
    {
      "metadata": {"name": "deploy-1"},
      "spec": {
        "template": {
          "spec": {
            "dnsPolicy": "ClusterFirst",
            "restartPolicy": "Always",
            "schedulerName": "default-scheduler",
            "containers": [{"name": "main", "image": "nginx:1.25", "imagePullPolicy": "IfNotPresent"}]
          }
        }
      }
    },
    {
      "metadata": {"name": "deploy-2"},
      "spec": {
        "template": {
          "spec": {
            "dnsPolicy": "ClusterFirst",
            "restartPolicy": "Always",
            "schedulerName": "default-scheduler",
            "containers": [{"name": "main", "image": "redis:7.0", "imagePullPolicy": "IfNotPresent"}]
          }
        }
      }
    }
  ]
}`
}

// samplePodYAML 返回一个包含默认值的 Pod YAML。
func samplePodYAML() string {
	return `kind: Pod
metadata:
  name: test-pod
spec:
  dnsPolicy: ClusterFirst
  restartPolicy: Always
  schedulerName: default-scheduler
  enableServiceLinks: true
  terminationGracePeriodSeconds: 30
  securityContext: {}
  containers:
    - name: main
      image: nginx:1.25
      imagePullPolicy: IfNotPresent
      terminationMessagePath: /dev/termination-log
      terminationMessagePolicy: File
      securityContext: {}
      ports:
        - containerPort: 80
`
}

func TestStripDefaults_PodDefaultsRemoved(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	result := filter.stripDefaults(samplePodJSON(), "json", "pods")

	// 验证 scheme 默认值被删除
	defaultFields := []string{
		"dnsPolicy",
		"restartPolicy",
		"schedulerName",
		"enableServiceLinks",
		"terminationGracePeriodSeconds",
		"imagePullPolicy",
		"terminationMessagePath",
		"terminationMessagePolicy",
	}
	for _, field := range defaultFields {
		if strings.Contains(result, `"`+field+`"`) {
			t.Errorf("默认字段 %q 应被删除，但仍存在于结果中: %s", field, result)
		}
	}

	// 验证空 securityContext 被删除
	if strings.Contains(result, `"securityContext"`) {
		t.Errorf("空 securityContext 应被删除，但仍存在于结果中: %s", result)
	}

	// 验证非默认值保留
	if !strings.Contains(result, `"name":"test-pod"`) {
		t.Errorf("非默认字段 name 应保留，但不存在于结果中: %s", result)
	}
	if !strings.Contains(result, `"image":"nginx:1.25"`) {
		t.Errorf("非默认字段 image 应保留，但不存在于结果中: %s", result)
	}
}

func TestStripDefaults_CustomValuesPreserved(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	result := filter.stripDefaults(samplePodJSONWithCustomValues(), "json", "pods")

	// 所有自定义值都不是默认值，应全部保留
	customFields := []string{
		`"dnsPolicy":"Default"`,
		`"restartPolicy":"OnFailure"`,
		`"schedulerName":"my-scheduler"`,
		`"enableServiceLinks":false`,
		`"terminationGracePeriodSeconds":60`,
		`"imagePullPolicy":"Always"`,
		`"terminationMessagePath":"/custom/path"`,
		`"terminationMessagePolicy":"FallbackToLogsOnError"`,
	}
	for _, field := range customFields {
		if !strings.Contains(result, field) {
			t.Errorf("自定义值 %q 应保留，但不存在于结果中: %s", field, result)
		}
	}

	// 非空 securityContext 应保留
	if !strings.Contains(result, `"securityContext"`) {
		t.Errorf("非空 securityContext 应保留，但不存在于结果中: %s", result)
	}
}

func TestStripDefaults_YAMLPodDefaultsRemoved(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	result := filter.stripDefaults(samplePodYAML(), "yaml", "pods")

	// 验证默认值被删除
	yamlDefaultFields := []string{
		"dnsPolicy:",
		"restartPolicy:",
		"schedulerName:",
		"enableServiceLinks:",
		"terminationGracePeriodSeconds:",
		"imagePullPolicy:",
		"terminationMessagePath:",
		"terminationMessagePolicy:",
	}
	for _, field := range yamlDefaultFields {
		if strings.Contains(result, field) {
			t.Errorf("默认字段 %q 应被删除，但仍存在于 YAML 结果中: %s", field, result)
		}
	}

	// 验证非默认值保留
	if !strings.Contains(result, "name: test-pod") {
		t.Errorf("非默认字段 name 应保留: %s", result)
	}
	if !strings.Contains(result, "image: nginx:1.25") {
		t.Errorf("非默认字段 image 应保留: %s", result)
	}
}

func TestStripDefaults_PodList(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	result := filter.stripDefaults(samplePodListJSON(), "json", "pods")

	if strings.Contains(result, `"dnsPolicy"`) {
		t.Errorf("PodList 中每个 Pod 的默认 dnsPolicy 应被删除: %s", result)
	}
	if strings.Contains(result, `"restartPolicy"`) {
		t.Errorf("PodList 中每个 Pod 的默认 restartPolicy 应被删除: %s", result)
	}
	if strings.Contains(result, `"imagePullPolicy"`) {
		t.Errorf("PodList 中每个容器的默认 imagePullPolicy 应被删除: %s", result)
	}

	if !strings.Contains(result, `"name":"pod-1"`) {
		t.Errorf("Pod 名称 pod-1 应保留: %s", result)
	}
	if !strings.Contains(result, `"name":"pod-2"`) {
		t.Errorf("Pod 名称 pod-2 应保留: %s", result)
	}
}

func TestStripDefaults_Deployment(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	result := filter.stripDefaults(sampleDeploymentJSON(), "json", "deployments")

	if strings.Contains(result, `"dnsPolicy"`) {
		t.Errorf("Deployment PodTemplateSpec 中的默认 dnsPolicy 应被删除: %s", result)
	}
	if strings.Contains(result, `"restartPolicy"`) {
		t.Errorf("Deployment PodTemplateSpec 中的默认 restartPolicy 应被删除: %s", result)
	}
	if strings.Contains(result, `"imagePullPolicy"`) {
		t.Errorf("Deployment 容器的默认 imagePullPolicy 应被删除: %s", result)
	}
	if strings.Contains(result, `"securityContext"`) {
		t.Errorf("Deployment 空 securityContext 应被删除: %s", result)
	}

	if !strings.Contains(result, `"replicas":3`) {
		t.Errorf("Deployment replicas 应保留: %s", result)
	}
}

func TestStripDefaults_StatefulSet(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	result := filter.stripDefaults(sampleStatefulSetJSON(), "json", "statefulsets")

	if strings.Contains(result, `"dnsPolicy"`) {
		t.Errorf("StatefulSet PodTemplateSpec 中的默认 dnsPolicy 应被删除: %s", result)
	}
	if strings.Contains(result, `"restartPolicy"`) {
		t.Errorf("StatefulSet PodTemplateSpec 中的默认 restartPolicy 应被删除: %s", result)
	}
	if strings.Contains(result, `"imagePullPolicy"`) {
		t.Errorf("StatefulSet 容器的默认 imagePullPolicy 应被删除: %s", result)
	}

	if !strings.Contains(result, `"replicas":2`) {
		t.Errorf("StatefulSet replicas 应保留: %s", result)
	}
}

func TestStripDefaults_DeploymentList(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	result := filter.stripDefaults(sampleDeploymentListJSON(), "json", "deployments")

	if strings.Contains(result, `"dnsPolicy"`) {
		t.Errorf("DeploymentList 中每个 Deployment 的默认 dnsPolicy 应被删除: %s", result)
	}
	if strings.Contains(result, `"imagePullPolicy"`) {
		t.Errorf("DeploymentList 中每个容器的默认 imagePullPolicy 应被删除: %s", result)
	}

	if !strings.Contains(result, `"name":"deploy-1"`) {
		t.Errorf("Deployment 名称 deploy-1 应保留: %s", result)
	}
	if !strings.Contains(result, `"name":"deploy-2"`) {
		t.Errorf("Deployment 名称 deploy-2 应保留: %s", result)
	}
}

func TestStripDefaults_CompactJSON(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	result := filter.stripDefaults(samplePodJSON(), "json", "pods")

	if strings.Contains(result, "\n  ") {
		t.Errorf("compact JSON 不应包含缩进换行: %s", result)
	}
	if strings.Contains(result, "\n") {
		t.Errorf("compact JSON 应为单行输出: %s", result)
	}
}

func TestStripDefaults_NonSuccessNotProcessed(t *testing.T) {
	cfg := &config.RulesConfig{
		Masking: []config.MaskingRule{
			{
				Resource:   "pods",
				Namespaces: []string{"*"},
				Action:     "strip_defaults",
			},
		},
	}
	filter := NewFilter(cfg)

	req := &model.ExecutionRequest{
		Resource:  "pods",
		Namespace: "default",
		Output:    "json",
	}
	result := &model.ExecutionResult{
		Status: "failed",
		Stdout: samplePodJSON(),
	}

	filtered := filter.FilterResult(req, result)

	if !strings.Contains(filtered.Stdout, `"dnsPolicy"`) {
		t.Errorf("非 success 状态不应触发 strip_defaults，默认值应保留: %s", filtered.Stdout)
	}
}

func TestStripDefaults_NonTargetResourceNotProcessed(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	serviceJSON := `{"kind":"Service","metadata":{"name":"test-svc"},"spec":{"type":"ClusterIP"}}`
	result := filter.stripDefaults(serviceJSON, "json", "services")

	if !strings.Contains(result, `"kind":"Service"`) {
		t.Errorf("非目标资源 Service 不应被裁剪: %s", result)
	}
	if !strings.Contains(result, `"type":"ClusterIP"`) {
		t.Errorf("Service 的字段应保留: %s", result)
	}
}

func TestStripDefaults_NonStructuredFormat(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	result := filter.stripDefaults("some plain text", "", "pods")
	if result != "some plain text" {
		t.Errorf("非结构化格式应返回原始内容，got: %s", result)
	}
}

func TestStripDefaults_InvalidJSON(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	result := filter.stripDefaults("not valid json", "json", "pods")
	if result != "not valid json" {
		t.Errorf("无效 JSON 应返回原始内容，got: %s", result)
	}
}

func TestStripDefaults_InvalidYAML(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	result := filter.stripDefaults("not: valid: yaml: {{{}", "yaml", "pods")
	_ = result // 主要确保不 panic
}

func TestStripDefaults_StatefulSetList(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	stsListJSON := `{
  "kind": "StatefulSetList",
  "items": [
    {
      "metadata": {"name": "sts-1"},
      "spec": {
        "template": {
          "spec": {
            "dnsPolicy": "ClusterFirst",
            "restartPolicy": "Always",
            "schedulerName": "default-scheduler",
            "containers": [{"name": "main", "image": "nginx:1.25", "imagePullPolicy": "IfNotPresent"}]
          }
        }
      }
    }
  ]
}`

	result := filter.stripDefaults(stsListJSON, "json", "statefulsets")

	if strings.Contains(result, `"dnsPolicy"`) {
		t.Errorf("StatefulSetList 中的默认 dnsPolicy 应被删除: %s", result)
	}
	if strings.Contains(result, `"imagePullPolicy"`) {
		t.Errorf("StatefulSetList 中容器的默认 imagePullPolicy 应被删除: %s", result)
	}
}

func TestStripDefaults_PodWithInitContainers(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	podJSON := `{
  "kind": "Pod",
  "metadata": {"name": "test-pod"},
  "spec": {
    "dnsPolicy": "ClusterFirst",
    "restartPolicy": "Always",
    "initContainers": [
      {
        "name": "init",
        "image": "busybox:1.36",
        "imagePullPolicy": "IfNotPresent",
        "terminationMessagePath": "/dev/termination-log",
        "terminationMessagePolicy": "File"
      }
    ],
    "containers": [
      {
        "name": "main",
        "image": "nginx:1.25",
        "imagePullPolicy": "IfNotPresent"
      }
    ]
  }
}`

	result := filter.stripDefaults(podJSON, "json", "pod")

	if strings.Contains(result, `"imagePullPolicy"`) {
		t.Errorf("initContainers 中的默认 imagePullPolicy 应被删除: %s", result)
	}
	if strings.Contains(result, `"terminationMessagePath"`) {
		t.Errorf("initContainers 中的默认 terminationMessagePath 应被删除: %s", result)
	}
}

// TestStripDefaults_LatestTagImagePolicy 验证 scheme 正确推导 :latest 镜像的 imagePullPolicy。
// 当 image 以 :latest 结尾时，scheme 默认 imagePullPolicy 为 Always，
// 此时用户显式设置的 IfNotPresent 不应被删除。
func TestStripDefaults_LatestTagImagePolicy(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	podJSON := `{
  "kind": "Pod",
  "metadata": {"name": "test-pod"},
  "spec": {
    "containers": [
      {
        "name": "main",
        "image": "nginx:latest",
        "imagePullPolicy": "IfNotPresent"
      }
    ]
  }
}`

	result := filter.stripDefaults(podJSON, "json", "pod")

	if !strings.Contains(result, `"imagePullPolicy":"IfNotPresent"`) {
		t.Errorf("image :latest + imagePullPolicy IfNotPresent 应保留（非 scheme 默认值）: %s", result)
	}
}

// TestStripDefaults_LatestTagImagePolicyDefault 验证 :latest + Always（scheme 默认）被删除。
func TestStripDefaults_LatestTagImagePolicyDefault(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	podJSON := `{
  "kind": "Pod",
  "metadata": {"name": "test-pod"},
  "spec": {
    "containers": [
      {
        "name": "main",
        "image": "nginx:latest",
        "imagePullPolicy": "Always"
      }
    ]
  }
}`

	result := filter.stripDefaults(podJSON, "json", "pod")

	if strings.Contains(result, `"imagePullPolicy"`) {
		t.Errorf("image :latest + imagePullPolicy Always 是 scheme 默认值，应被删除: %s", result)
	}
}

// TestStripDefaults_ContextDependentDNS 验证 hostNetwork 影响的 dnsPolicy 默认值推导。
func TestStripDefaults_ContextDependentDNS(t *testing.T) {
	cfg := &config.RulesConfig{}
	filter := NewFilter(cfg)

	podJSON := `{
  "kind": "Pod",
  "metadata": {"name": "test-pod"},
  "spec": {
    "hostNetwork": true,
    "dnsPolicy": "Default",
    "restartPolicy": "Always",
    "containers": [{"name": "main", "image": "nginx:1.25"}]
  }
}`

	result := filter.stripDefaults(podJSON, "json", "pod")

	// dnsPolicy "Default" 是 hostNetwork=true 时的 scheme 默认值 → 应被删除
	if strings.Contains(result, `"dnsPolicy"`) {
		t.Errorf("hostNetwork=true 时 dnsPolicy Default 是 scheme 默认值，应被删除: %s", result)
	}
	// hostNetwork 本身是用户显式设置，应保留
	if !strings.Contains(result, `"hostNetwork":true`) {
		t.Errorf("hostNetwork=true 是用户显式设置，应保留: %s", result)
	}
}


