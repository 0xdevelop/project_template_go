// Package docs 以 embed 形式随包提供对外 API 文档；对外只有 gen_api_docs.sh 生成的
// api_methods.md。api_description.md 等内部契约不进 embed、不对外。
package docs

import "embed"

//go:embed api_methods.md
var APIDocFS embed.FS
