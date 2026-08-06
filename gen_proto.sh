#!/usr/bin/env bash
# gen_proto.sh — 生成 project_template_go 的 protobuf Go 代码。
# 用法：./gen_proto.sh
# 依赖：protoc（在 PATH 中）；protoc-gen-go / protoc-gen-go-grpc 由本脚本
#       自动按 go.mod 里 tools.go 锁定的版本安装到 $GOBIN（即 $GOPATH/bin）。
#       注意：$GOBIN 需要在 PATH 中，protoc 才能找到 plugin binary。
#
# 布局约定：
#   api/api_grpc/proto/     ← .proto 源
#   api/api_grpc/protobuf/  ← 生成的 .pb.go
#
# 输出策略：用 protoc --go_opt=module=<module> 让输出按 go_package 减去
# module 前缀自动放置。
set -euo pipefail

MODULE="github.com/0xYeah/project_template_go"

# ─── 工具链同步：从本仓 go.mod 锁定版本装到 $GOBIN ────────────────────────
echo ">>> 同步工具链版本 (从 go.mod)"
go install google.golang.org/protobuf/cmd/protoc-gen-go
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc

# gen_domain 跑 protoc 为一个 domain 内的所有 .proto 生成 go + grpc 代码。
# 每个 domain 使用独立 -I，内部 .proto 使用扁平 import "xxx.proto"。
#   $1  label
#   $2  domain proto dir
#   $3+ proto 文件名
gen_domain() {
  local label="$1"
  local proto_dir="$2"
  shift 2
  local proto_files=("$@")

  if [ ${#proto_files[@]} -eq 0 ]; then
    echo "!! [${label}] gen_domain 需要至少 1 个 .proto 文件名"
    exit 1
  fi

  local proto_paths=()
  for p in "${proto_files[@]}"; do
    if [ ! -f "${proto_dir}/${p}" ]; then
      echo "!! [${label}] proto 源不存在: ${proto_dir}/${p}"
      exit 1
    fi
    proto_paths+=("${proto_dir}/${p}")
  done

  echo ">>> 生成 ${label} (${#proto_files[@]} files)"
  protoc \
    --go_out=. --go_opt=module="${MODULE}" \
    --go-grpc_out=. --go-grpc_opt=module="${MODULE}" \
    -I "${proto_dir}" \
    "${proto_paths[@]}"
  echo ">>> ${label} 完成"
}

# sync_external_proto 从邻接仓复制 .proto 到本仓并重写 go_package。
#   $1  external src
#   $2  local dst
#   $3  go_package
sync_external_proto() {
  local src="$1"
  local dst="$2"
  local pkg="$3"

  if [ ! -f "$src" ]; then
    return 1
  fi

  mkdir -p "$(dirname "$dst")"
  cp "$src" "$dst"
  awk -v pkg="$pkg" '/^option go_package/ {
    print "option go_package = \"" pkg "\";"
    next
  } { print }' "$dst" > "${dst}.tmp" && mv "${dst}.tmp" "$dst"
}

# ─── 显式注册本仓 gRPC 契约 ────────────────────────────────────────────────

gen_domain api_grpc \
  "api/api_grpc/proto" \
  api.proto

echo ">>> 全部完成。当前输出："
find api/api_grpc/protobuf -name "*.pb.go" -type f | sort
