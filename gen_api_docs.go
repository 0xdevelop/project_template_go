//go:build ignore

// gen_api_docs.go 是 build-tag 隔离的文档生成器（与 tools.go 同款隔离方式，不属于任何包、
// 不进任何构建），只由根目录 gen_api_docs.sh 经 `go run gen_api_docs.go [version]` 调用。
// 它从 api_supported_methods 方法注册表生成对外文档 docs/api_methods.md；生成物禁止手改。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/0xYeah/project_template_go/ability"
	"github.com/0xYeah/project_template_go/api/api_error_code"
	"github.com/0xYeah/project_template_go/api/api_supported_methods"
	"github.com/0xYeah/project_template_go/config"
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

func writeMethodsDoc(methods []api_supported_methods.SupportedMethod, version string) error {
	var doc strings.Builder
	doc.WriteString("# API 方法清单\n\n")
	fmt.Fprintf(
		&doc,
		"> 本文件由 `gen_api_docs.sh` 从方法注册表生成（%s，共 %d 个方法），禁止手改；新增方法后重新执行生成。\n",
		version, len(methods),
	)

	doc.WriteString(`
## 统一调用方式

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

### 业务错误码

| error_code | error_msg |
| ---------- | --------- |
| 0 | 成功 |
`)
	for _, definedError := range []error{
		api_error_code.ErrMethodNotFound,
		api_error_code.ErrMethodNotSupported,
		api_error_code.ErrInvalidArguments,
		api_error_code.ErrPermissionDenied,
	} {
		businessError, ok := api_error_code.As(definedError)
		if !ok {
			return fmt.Errorf("business error is not an api_error_code.Error: %v", definedError)
		}
		fmt.Fprintf(&doc, "| %d | %s |\n", businessError.Code, businessError.Message)
	}

	for _, method := range methods {
		fmt.Fprintf(&doc, "\n## %s\n\n", method.Name)
		if method.Description != "" {
			doc.WriteString(method.Description)
			doc.WriteString("\n")
		}
		if method.Async {
			doc.WriteString("\n异步方法：受理后返回 `task_id`，进度与结果经任务查询方法读取。\n")
		}
		if method.InputSchema != nil {
			schema, err := json.MarshalIndent(method.InputSchema, "", "  ")
			if err != nil {
				return fmt.Errorf("encode InputSchema of %s: %w", method.Name, err)
			}
			doc.WriteString("\n`arguments` JSON Schema：\n\n```json\n")
			doc.Write(schema)
			doc.WriteString("\n```\n")
		}
	}
	return os.WriteFile(methodsDocPath, []byte(doc.String()), 0644)
}
