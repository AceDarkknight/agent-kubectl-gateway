package filter

import (
	"strings"
	"testing"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/config"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/model"
)

// 模拟真实 kubectl get deployments -o json 的输出
func realDeploymentListJSON() string {
	return `{
  "apiVersion": "apps/v1",
  "items": [
    {
      "apiVersion": "apps/v1",
      "kind": "Deployment",
      "metadata": {
        "annotations": {"deployment.kubernetes.io/revision": "1"},
        "creationTimestamp": "2026-03-12T12:30:00Z",
        "generateName": "k8sagent-774b4c56f7-",
        "labels": {"app": "k8sagent"},
        "managedFields": [{"manager": "kube-controller-manager"}],
        "name": "k8sagent",
        "namespace": "default",
        "resourceVersion": "260",
        "uid": "abc-def-123"
      },
      "spec": {
        "replicas": 2,
        "selector": {"matchLabels": {"app": "k8sagent"}},
        "template": {
          "metadata": {"labels": {"app": "k8sagent"}},
          "spec": {
            "containers": [
              {
                "image": "registry.example.com/k8sagent:latest",
                "imagePullPolicy": "Always",
                "name": "k8sagent",
                "ports": [{"containerPort": 8080}],
                "resources": {"limits": {"cpu": "500m", "memory": "512Mi"}}
              }
            ],
            "dnsPolicy": "ClusterFirst",
            "restartPolicy": "Always",
            "schedulerName": "default-scheduler",
            "securityContext": {},
            "terminationGracePeriodSeconds": 30
          }
        }
      },
      "status": {"availableReplicas": 2, "readyReplicas": 2, "replicas": 2}
    }
  ],
  "kind": "DeploymentList",
  "metadata": {"resourceVersion": "260"}
}`
}

func TestFullPipeline_DeploymentList(t *testing.T) {
	cfg := &config.RulesConfig{
		Masking: []config.MaskingRule{
			{
				Resource:   "*",
				Namespaces: []string{"*"},
				Action:     "filter_fields",
				Fields: []string{
					"metadata.annotations.kubectl.kubernetes.io/last-applied-configuration",
					"metadata.managedFields",
					"metadata.creationTimestamp",
					"metadata.generateName",
					"metadata.ownerReferences",
					"metadata.resourceVersion",
					"metadata.uid",
					"status",
				},
			},
			{
				Resource:   "deployments",
				Namespaces: []string{"*"},
				Action:     "strip_defaults",
			},
		},
	}

	filter := NewFilter(cfg)
	req := &model.ExecutionRequest{
		Verb:      "get",
		Resource:  "deployments",
		Namespace: "default",
		Output:    "json",
	}
	result := &model.ExecutionResult{
		Status: "success",
		Stdout: realDeploymentListJSON(),
	}

	filtered := filter.FilterResult(req, result)
	stdout := filtered.Stdout

	t.Logf("Filtered stdout:\n%s", stdout)

	// 检查 filter_fields 是否生效
	if strings.Contains(stdout, "managedFields") {
		t.Error("filter_fields failed: managedFields still present")
	}
	if strings.Contains(stdout, "creationTimestamp") {
		t.Error("filter_fields failed: creationTimestamp still present")
	}

	// 检查 strip_defaults 是否生效
	if strings.Contains(stdout, "dnsPolicy") {
		t.Error("strip_defaults failed: dnsPolicy still present")
	}
	if strings.Contains(stdout, "restartPolicy") {
		t.Error("strip_defaults failed: restartPolicy still present")
	}
	if strings.Contains(stdout, "schedulerName") {
		t.Error("strip_defaults failed: schedulerName still present")
	}
}

func TestFullPipeline_PodListYAML(t *testing.T) {
	yamlContent := `apiVersion: v1
items:
- apiVersion: v1
  kind: Pod
  metadata:
    annotations:
      cni.projectcalico.org/containerID: abc123
    creationTimestamp: "2026-03-12T12:30:00Z"
    generateName: k8sagent-774b4c56f7-
    labels:
      app: k8sagent
    managedFields:
    - manager: kube-controller-manager
    name: k8sagent-774b4c56f7-l6ph6
    namespace: default
    resourceVersion: "260"
    uid: 5d44e3da-0736-4986-8149-47bb9bc38e42
  spec:
    containers:
    - image: registry.example.com/k8sagent:latest
      imagePullPolicy: Always
      name: k8sagent
      ports:
      - containerPort: 8080
    dnsPolicy: ClusterFirst
    restartPolicy: Always
    schedulerName: default-scheduler
  status:
    phase: Running
kind: PodList
metadata:
  resourceVersion: ""
`

	cfg := &config.RulesConfig{
		Masking: []config.MaskingRule{
			{
				Resource:   "*",
				Namespaces: []string{"*"},
				Action:     "filter_fields",
				Fields: []string{
					"metadata.managedFields",
					"metadata.creationTimestamp",
					"metadata.generateName",
					"metadata.ownerReferences",
					"metadata.resourceVersion",
					"metadata.uid",
					"status",
				},
			},
			{
				Resource:   "pods",
				Namespaces: []string{"*"},
				Action:     "strip_defaults",
			},
		},
	}

	filter := NewFilter(cfg)
	req := &model.ExecutionRequest{
		Verb:      "get",
		Resource:  "pods",
		Namespace: "default",
		Output:    "yaml",
	}
	result := &model.ExecutionResult{
		Status: "success",
		Stdout: yamlContent,
	}

	filtered := filter.FilterResult(req, result)
	stdout := filtered.Stdout

	t.Logf("Filtered YAML:\n%s", stdout)

	if strings.Contains(stdout, "creationTimestamp:") {
		t.Error("filter_fields failed: creationTimestamp still in YAML")
	}
	if strings.Contains(stdout, "managedFields:") {
		t.Error("filter_fields failed: managedFields still in YAML")
	}
	if strings.Contains(stdout, "resourceVersion:") {
		t.Error("filter_fields failed: resourceVersion still in YAML")
	}
	if strings.Contains(stdout, "dnsPolicy:") {
		t.Error("strip_defaults failed: dnsPolicy still in YAML")
	}
	if strings.Contains(stdout, "restartPolicy:") {
		t.Error("strip_defaults failed: restartPolicy still in YAML")
	}
	if strings.Contains(stdout, "schedulerName:") {
		t.Error("strip_defaults failed: schedulerName still in YAML")
	}
}
