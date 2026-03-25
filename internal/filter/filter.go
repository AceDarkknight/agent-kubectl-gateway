package filter

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/AceDarkknight/agent-kubectl-gateway/internal/audit"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/config"
	"github.com/AceDarkknight/agent-kubectl-gateway/internal/model"

	"go.uber.org/zap"

	"go.yaml.in/yaml/v3"
)

// Filter filters the result based on rules.
type Filter struct {
	config *config.RulesConfig
}

// NewFilter creates a new Filter.
func NewFilter(cfg *config.RulesConfig) *Filter {
	return &Filter{
		config: cfg,
	}
}

// FilterResult filters the execution result.
func (f *Filter) FilterResult(req *model.ExecutionRequest, result *model.ExecutionResult) *model.ExecutionResult {
	outputFormat := ""
	if req.Options != nil {
		outputFormat = req.Options.Output
	}
	audit.Info("[Filter] 开始过滤结果",
		zap.String("resource", req.Resource),
		zap.String("namespace", req.Namespace),
		zap.String("output_format", outputFormat),
		zap.Int("original_size", len(result.Stdout)))

	// 如果命令执行失败或被拦截，不进行过滤
	if result.Status != "success" {
		audit.Warn("[Filter] 命令执行状态非 success，跳过过滤", zap.String("status", result.Status))
		return result
	}

	// 根据资源类型匹配脱敏规则
	filteredStdout := result.Stdout

	// 检查是否需要脱敏
	matchingRules := 0
	for _, rule := range f.config.Masking {
		if f.matchesRule(req.Resource, req.Namespace, &rule) {
			matchingRules++
			audit.Debug("[Filter] 匹配到脱敏规则",
				zap.String("resource", rule.Resource),
				zap.String("action", rule.Action))
			switch rule.Action {
			case "mask":
				filteredStdout = f.maskContent(filteredStdout, outputFormat)
			case "filter_fields":
				filteredStdout = f.filterFields(filteredStdout, rule.Fields, outputFormat)
			}
		}
	}
	audit.Debug("[Filter] 匹配到规则数量", zap.Int("count", matchingRules))

	result.Stdout = filteredStdout
	result.ResponseSize = len(filteredStdout) + len(result.Stderr)
	audit.Info("[Filter] 过滤完成", zap.Int("filtered_size", len(filteredStdout)))
	return result
}

// matchesRule checks if the rule matches the request.
func (f *Filter) matchesRule(resource, namespace string, rule *config.MaskingRule) bool {
	// 检查资源类型
	if rule.Resource != "*" && rule.Resource != resource {
		return false
	}

	// 检查命名空间
	for _, ns := range rule.Namespaces {
		if ns == "*" || ns == namespace {
			return true
		}
	}
	return false
}

// maskContent masks sensitive content based on output format.
// 对于 JSON/YAML 格式，掩码敏感字段（如 Secret 的 data 字段）
// 对于非结构化文本，使用正则表达式进行掩码
func (f *Filter) maskContent(content, outputFormat string) string {
	audit.Debug("[Filter.maskContent] 开始内容脱敏",
		zap.Int("original_size", len(content)),
		zap.String("output_format", outputFormat))

	// 如果输出格式不是 JSON/YAML，则使用正则表达式进行掩码
	if outputFormat != "json" && outputFormat != "yaml" {
		result := f.FilterWithRegex(content)
		audit.Debug("[Filter.maskContent] 正则表达式脱敏完成",
			zap.Int("processed_size", len(result)))
		return result
	}

	// 对于 JSON 格式，使用 sjson 进行处理
	if outputFormat == "json" {
		result := f.maskJSONContent(content)
		audit.Debug("[Filter.maskContent] JSON脱敏完成",
			zap.Int("processed_size", len(result)))
		return result
	}

	// 对于 YAML 格式，使用 yaml.v3 进行处理
	if outputFormat == "yaml" {
		result := f.maskYAMLContent(content)
		audit.Debug("[Filter.maskContent] YAML脱敏完成",
			zap.Int("processed_size", len(result)))
		return result
	}

	return content
}

// maskJsonContent masks sensitive content in JSON format.
// 主要用于掩码 Secret 的 data 字段
func (f *Filter) maskJSONContent(content string) string {
	// 解析 JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		// 如果解析失败，返回原始内容
		return content
	}

	// 检查是否是 List 结构，如果是则遍历 items 数组处理每个资源
	if kind, ok := data["kind"].(string); ok && kind == "List" {
		if items, ok := data["items"].([]interface{}); ok {
			for _, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					f.maskSingleResource(itemMap)
				}
			}
		}
	} else {
		// 单独资源处理
		f.maskSingleResource(data)
	}

	// 序列化回 JSON
	result, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return content
	}

	return string(result)
}

// maskSingleResource 对单个资源进行脱敏处理
func (f *Filter) maskSingleResource(data map[string]interface{}) {
	// 检查是否是 Secret 资源
	if kind, ok := data["kind"].(string); ok && kind == "Secret" {
		// 掩码 data 字段
		if dataMap, ok := data["data"].(map[string]interface{}); ok {
			for key := range dataMap {
				dataMap[key] = "*** MASKED BY PROXY ***"
			}
		}
		// 掩码 stringData 字段
		if stringDataMap, ok := data["stringData"].(map[string]interface{}); ok {
			for key := range stringDataMap {
				stringDataMap[key] = "*** MASKED BY PROXY ***"
			}
		}
	}
}

// maskYAMLContent masks sensitive content in YAML format.
// 主要用于掩码 Secret 的 data 字段
func (f *Filter) maskYAMLContent(content string) string {
	// 解析 YAML
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		// 如果解析失败，返回原始内容
		return content
	}

	// 检查是否是 List 结构，如果是则遍历 items 数组处理每个资源
	if kind, ok := data["kind"].(string); ok && kind == "List" {
		if items, ok := data["items"].([]interface{}); ok {
			for _, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					f.maskSingleResource(itemMap)
				}
			}
		}
	} else {
		// 单独资源处理
		f.maskSingleResource(data)
	}

	// 序列化回 YAML
	result, err := yaml.Marshal(data)
	if err != nil {
		return content
	}

	return string(result)
}

// filterFields filters out specified fields from JSON or YAML content.
// 支持 JSON 和 YAML 两种格式
func (f *Filter) filterFields(content string, fields []string, outputFormat string) string {
	audit.Debug("[Filter.filterFields] 开始字段过滤",
		zap.Int("original_size", len(content)),
		zap.Int("fields_count", len(fields)),
		zap.String("output_format", outputFormat))

	if outputFormat == "json" {
		result := f.filterJSONFields(content, fields)
		audit.Debug("[Filter.filterFields] JSON字段过滤完成",
			zap.Int("processed_size", len(result)))
		return result
	}

	if outputFormat == "yaml" {
		result := f.filterYAMLFields(content, fields)
		audit.Debug("[Filter.filterFields] YAML字段过滤完成",
			zap.Int("processed_size", len(result)))
		return result
	}

	audit.Debug("[Filter.filterFields] 非结构化格式，跳过字段过滤")
	return content
}

// filterJSONFields filters out specified fields from JSON content using sjson.
func (f *Filter) filterJSONFields(content string, fields []string) string {
	// 解析 JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		// 如果解析失败，返回原始内容
		return content
	}

	// 检查是否是 List 结构，如果是则遍历 items 数组处理每个资源
	if kind, ok := data["kind"].(string); ok && kind == "List" {
		if items, ok := data["items"].([]interface{}); ok {
			for _, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					f.filterSingleResourceFields(itemMap, fields)
				}
			}
		}
	} else {
		// 单独资源处理
		f.filterSingleResourceFields(data, fields)
	}

	// 序列化回 JSON
	result, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return content
	}

	return string(result)
}

// filterSingleResourceFields 对单个资源进行字段过滤
// 优化：使用原地修改，避免不必要的内存分配
func (f *Filter) filterSingleResourceFields(data map[string]interface{}, fields []string) {
	// 构建字段路径集合，用于快速查找
	fieldPaths := make(map[string]bool)
	for _, field := range fields {
		fieldPaths[field] = true
	}

	// 对每个要过滤的字段路径进行移除
	for field := range fieldPaths {
		f.removeJSONField(data, strings.Split(field, "."))
	}
}

// removeJSONField recursively removes a field from JSON data.
// 修复：添加深度限制防止栈溢出，并避免不必要的 slice 复制
func (f *Filter) removeJSONField(data map[string]interface{}, path []string) {
	if len(path) == 0 {
		return
	}

	// 递归深度保护：最大深度 20 层
	if len(path) > 20 {
		return
	}

	key := path[0]
	if len(path) == 1 {
		// 到达目标字段，直接删除
		delete(data, key)
		return
	}

	// 继续递归 - 不再复制 slice，直接使用子切片
	if nested, ok := data[key].(map[string]interface{}); ok {
		f.removeJSONField(nested, path[1:])
	}
}

// filterYAMLFields filters out specified fields from YAML content.
func (f *Filter) filterYAMLFields(content string, fields []string) string {
	// 解析 YAML
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &data); err != nil {
		// 如果解析失败，返回原始内容
		return content
	}

	// 检查是否是 List 结构，如果是则遍历 items 数组处理每个资源
	if kind, ok := data["kind"].(string); ok && kind == "List" {
		if items, ok := data["items"].([]interface{}); ok {
			for _, item := range items {
				if itemMap, ok := item.(map[string]interface{}); ok {
					f.filterSingleResourceYAMLFields(itemMap, fields)
				}
			}
		}
	} else {
		// 单独资源处理
		f.filterSingleResourceYAMLFields(data, fields)
	}

	// 序列化回 YAML
	result, err := yaml.Marshal(data)
	if err != nil {
		return content
	}

	return string(result)
}

// filterSingleResourceYAMLFields 对单个 YAML 资源进行字段过滤
// 优化：使用 map 去重，避免重复处理相同字段
func (f *Filter) filterSingleResourceYAMLFields(data map[string]interface{}, fields []string) {
	// 构建字段路径集合，用于快速查找
	fieldPaths := make(map[string]bool)
	for _, field := range fields {
		fieldPaths[field] = true
	}

	// 对每个要过滤的字段路径进行移除
	for field := range fieldPaths {
		f.removeYAMLField(data, strings.Split(field, "."))
	}
}

// removeYAMLField recursively removes a field from YAML data.
// 修复：添加深度限制防止栈溢出
func (f *Filter) removeYAMLField(data map[string]interface{}, path []string) {
	if len(path) == 0 {
		return
	}

	// 递归深度保护：最大深度 20 层
	if len(path) > 20 {
		return
	}

	key := path[0]
	if len(path) == 1 {
		// 到达目标字段，直接删除
		delete(data, key)
		return
	}

	// 继续递归 - 不再复制 slice，直接使用子切片
	if nested, ok := data[key].(map[string]interface{}); ok {
		f.removeYAMLField(nested, path[1:])
	}
}

// FilterWithRegex uses regex to mask sensitive data in plain text.
// 匹配并替换以下敏感信息：
// - base64 编码的 Secret 数据（40个以上字符）
// - Bearer Token
// - 密码字段
// - 私钥内容
func (f *Filter) FilterWithRegex(content string) string {
	// 定义脱敏模式
	patterns := []struct {
		regex       *regexp.Regexp
		replacement string
	}{
		{
			// 匹配 base64 编码的 Secret 数据（40个以上字符）
			regex:       regexp.MustCompile(`[A-Za-z0-9+/]{40,}={0,2}`),
			replacement: "*** MASKED BY PROXY ***",
		},
		{
			// 匹配 Bearer Token
			regex:       regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9\-._~+/]+=*`),
			replacement: "Bearer *** MASKED BY PROXY ***",
		},
		{
			// 匹配密码字段
			regex:       regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*\S+`),
			replacement: "$1: *** MASKED BY PROXY ***",
		},
		{
			// 匹配私钥内容
			regex:       regexp.MustCompile(`-----BEGIN [A-Z ]+ PRIVATE KEY-----[\s\S]*?-----END [A-Z ]+ PRIVATE KEY-----`),
			replacement: "*** MASKED BY PROXY (PRIVATE KEY) ***",
		},
	}

	result := content
	for _, pattern := range patterns {
		result = pattern.regex.ReplaceAllString(result, pattern.replacement)
	}

	return result
}
