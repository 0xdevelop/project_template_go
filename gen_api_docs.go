//go:build ignore

// gen_api_docs.go 是 build-tag 隔离的文档生成器（与 tools.go 同款隔离方式，不属于任何包、
// 不进任何构建），只由根目录 gen_api_docs.sh 经 `go run gen_api_docs.go [version]` 调用。
// 它从 api_supported_methods 方法注册表生成对外文档 docs/api_methods.md；生成物禁止手改。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/0xdevelop/project_template_go/ability"
	"github.com/0xdevelop/project_template_go/api/api_error_code"
	"github.com/0xdevelop/project_template_go/api/api_supported_methods"
	"github.com/0xdevelop/project_template_go/config"
)

const methodsDocPath = "docs/api_methods.md"

func main() {
	version := config.ProjectVersion
	if len(os.Args) > 1 && os.Args[1] != "" {
		version = os.Args[1]
	}

	ability.LoadAbilityAPIMethods()
	methods := api_supported_methods.Methods()
	if len(methods) == 0 {
		fail("method registry is empty")
	}

	if err := writeMethodsDoc(methods, version); err != nil {
		fail(err.Error())
	}
	fmt.Printf(
		"[gen_api_docs] wrote %s (%s, %d methods)\n",
		methodsDocPath, version, len(methods),
	)
}

func fail(message string) {
	fmt.Fprintf(os.Stderr, "[gen_api_docs] FAIL: %s\n", message)
	os.Exit(1)
}

// docSection 是生成文档的一个 H1 编号节：固定说明节、独立方法节（如 test）或功能域分组节。
type docSection struct {
	title        string
	fixedBody    string
	singleMethod *api_supported_methods.SupportedMethod
	methods      []api_supported_methods.SupportedMethod
}

func writeMethodsDoc(methods []api_supported_methods.SupportedMethod, version string) error {
	errorCodeTable, err := renderErrorCodeTable()
	if err != nil {
		return err
	}
	sections := []*docSection{
		{title: "统一调用方式", fixedBody: unifiedCallBody},
		{title: "业务错误码", fixedBody: errorCodeTable},
	}
	// 方法按名字前缀归组（auth.* → Auth）；无点方法（test）独立成节。分组顺序 = 注册表首次出现顺序。
	groupsByTitle := map[string]*docSection{}
	for _, method := range methods {
		prefix, _, hasGroup := strings.Cut(method.Name, ".")
		if !hasGroup {
			sections = append(sections, &docSection{title: method.Name, singleMethod: &method})
			continue
		}
		groupTitle := strings.ToUpper(prefix[:1]) + prefix[1:]
		group, exists := groupsByTitle[groupTitle]
		if !exists {
			group = &docSection{title: groupTitle}
			groupsByTitle[groupTitle] = group
			sections = append(sections, group)
		}
		group.methods = append(group.methods, method)
	}

	// 不生成 TOC 块：目录职责在渲染端（IDE 大纲 / docs_api 页左侧分类树），编号 H1/H2 承担结构。
	var doc strings.Builder
	fmt.Fprintf(
		&doc,
		"> 本文件由 `gen_api_docs.sh` 从方法注册表生成（%s，共 %d 个方法），禁止手改；新增方法后重新执行生成。\n",
		version, len(methods),
	)

	for sectionIndex, section := range sections {
		fmt.Fprintf(&doc, "\n# %d. %s\n", sectionIndex+1, section.title)
		doc.WriteString(section.fixedBody)
		if section.singleMethod != nil {
			if err = renderMethodBody(&doc, section.singleMethod); err != nil {
				return err
			}
		}
		for methodIndex, method := range section.methods {
			fmt.Fprintf(&doc, "\n## %d.%d. %s\n", sectionIndex+1, methodIndex+1, method.Name)
			if err = renderMethodBody(&doc, &method); err != nil {
				return err
			}
		}
	}
	return os.WriteFile(methodsDocPath, []byte(doc.String()), 0644)
}

func renderMethodBody(doc *strings.Builder, method *api_supported_methods.SupportedMethod) error {
	if method.Description != "" {
		doc.WriteString("\n")
		doc.WriteString(method.Description)
		doc.WriteString("\n")
	}
	if method.Async {
		doc.WriteString("\n异步方法：受理后返回 `task_id`，进度与结果经任务查询方法读取。\n")
	}
	if method.InputSchema != nil {
		if err := renderArgumentsExample(doc, method); err != nil {
			return err
		}
		schema, err := json.MarshalIndent(method.InputSchema, "", "  ")
		if err != nil {
			return fmt.Errorf("encode InputSchema of %s: %w", method.Name, err)
		}
		doc.WriteString("\n`arguments` JSON Schema（约束说明，非请求体；`required` 数组 = 必填字段清单）：\n\n```json\n")
		doc.Write(schema)
		doc.WriteString("\n```\n")
	}
	return nil
}

// renderArgumentsExample 从 InputSchema 生成 arguments 传参举例：只展开必填字段
// （enum 取首值、date-time 给样例、其余用 <字段名> 占位），可选字段单独列名。
func renderArgumentsExample(doc *strings.Builder, method *api_supported_methods.SupportedMethod) error {
	properties, ok := method.InputSchema["properties"].(map[string]interface{})
	if !ok || len(properties) == 0 {
		return nil
	}
	var required []string
	if rawRequired, exists := method.InputSchema["required"]; exists {
		typedRequired, typedOK := rawRequired.([]string)
		if !typedOK {
			return fmt.Errorf("malformed required list in InputSchema of %s", method.Name)
		}
		required = typedRequired
	}
	example := map[string]interface{}{}
	for _, name := range required {
		example[name] = examplePlaceholder(name, properties[name])
	}
	optionalNames := make([]string, 0)
	for name := range properties {
		if example[name] == nil {
			optionalNames = append(optionalNames, "`"+name+"`")
		}
	}
	sort.Strings(optionalNames)

	var exampleBuffer bytes.Buffer
	exampleEncoder := json.NewEncoder(&exampleBuffer)
	exampleEncoder.SetEscapeHTML(false)
	exampleEncoder.SetIndent("", "  ")
	if err := exampleEncoder.Encode(example); err != nil {
		return fmt.Errorf("encode arguments example of %s: %w", method.Name, err)
	}
	doc.WriteString("\n`arguments` 传参举例（仅含必填字段）：\n\n```json\n")
	doc.WriteString(strings.TrimRight(exampleBuffer.String(), "\n"))
	doc.WriteString("\n```\n")
	if len(optionalNames) > 0 {
		fmt.Fprintf(doc, "\n可选字段：%s。\n", strings.Join(optionalNames, "、"))
	}
	return nil
}

// examplePlaceholder 按字段 schema 造举例值：enum 取首值、date-time 给固定样例、其余 <字段名> 占位。
func examplePlaceholder(name string, rawPropertySchema interface{}) interface{} {
	propertySchema, ok := rawPropertySchema.(map[string]interface{})
	if !ok {
		return "<" + name + ">"
	}
	if enumValues, exists := propertySchema["enum"].([]interface{}); exists && len(enumValues) > 0 {
		return enumValues[0]
	}
	if enumValues, exists := propertySchema["enum"].([]string); exists && len(enumValues) > 0 {
		return enumValues[0]
	}
	if format, exists := propertySchema["format"].(string); exists && format == "date-time" {
		return "2026-01-02T15:04:05Z"
	}
	return "<" + name + ">"
}

func renderErrorCodeTable() (string, error) {
	var table strings.Builder
	table.WriteString(`
| error_code | error_msg |
| ---------- | --------- |
| 0 | 成功 |
`)
	for _, definedError := range []error{
		api_error_code.ErrMethodNotFound,
		api_error_code.ErrMethodNotSupported,
		api_error_code.ErrInvalidArguments,
		api_error_code.ErrPermissionDenied,
		api_error_code.ErrVerifyCodeDeliveryFailed,
	} {
		businessError, ok := api_error_code.As(definedError)
		if !ok {
			return "", fmt.Errorf("business error is not an api_error_code.Error: %v", definedError)
		}
		fmt.Fprintf(&table, "| %d | %s |\n", businessError.Code, businessError.Message)
	}
	return table.String(), nil
}

const unifiedCallBody = `
所有方法经同一入口调用，JSON-RPC over HTTP、MCP、WebSocket、gRPC 只是外壳不同，
运输同一份请求和同一份结果。业务方法由 ` + "`params.name`" + ` 选择，业务入参放在
` + "`params.arguments`" + `：

` + "```json" + `
{
  "jsonrpc": "2.0",
  "id": "request-id",
  "method": "tools/call",
  "params": {
    "name": "auth.login.email",
    "arguments": {
      "email": "user@example.com",
      "password": "password"
    }
  }
}
` + "```" + `

统一返回 MCP ` + "`CallToolResult`" + `：成功 ` + "`isError=false`" + `，` + "`content[0].text`" + `
直接承载业务 JSON；业务失败（含未注册方法、参数不合法、鉴权失败）HTTP 状态仍为
200，` + "`isError=true`" + `，` + "`content[0].text`" + ` 为
` + "`{\"error_code\":...,\"error_msg\":\"...\"}`" + `。

**方法节怎么读**：每个方法节给出方法语义、` + "`arguments`" + ` **传参举例**（必填字段的
实际请求形态，占位值按真实值替换）与 ` + "`arguments`" + ` **JSON Schema**（机器可校验的
约束说明——` + "`required`" + ` 数组表示「哪些字段必填」，` + "`properties`" + ` 内是各字段
类型与长度约束；**schema 本身不是请求体的一部分，不要照抄进请求**）。
`
