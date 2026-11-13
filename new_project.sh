#!/usr/bin/env bash
# Usage:
#   bash new_project.sh [module_path] <api_key_prefix>
#   wget -qO- https://raw.githubusercontent.com/0xdevelop/project_template_go/main/new_project.sh | bash -s -- [module_path] <api_key_prefix>
#
# Run from inside your project directory (already cloned or freshly created).
# api_key_prefix 必填，只含小写字母数字与下划线且以 _ 结尾（如 xjbk_），只替换实例凭证前缀。
#
# Examples:
#   bash -s -- github.com/myorg/my_service svc_
#   bash -s -- mycompany.com/backend svc_
#   bash -s -- gitlab.com/team/api svc_

set -euo pipefail

TEMPLATE_REPO="https://github.com/0xdevelop/project_template_go.git"
TEMPLATE_MODULE="github.com/0xdevelop/project_template_go"
TEMPLATE_NAME="project_template_go"

usage() {
    echo "Usage: bash new_project.sh [module_path] <api_key_prefix>"
    echo ""
    echo "  module_path      Go module path (optional if go.mod already exists)"
    echo "                   e.g. my_project"
    echo "                        github.com/myorg/my_service"
    echo "  api_key_prefix   实例 API key 前缀，必填，小写字母数字与下划线且以 _ 结尾，例如 xjbk_"
    echo ""
    echo "  If go.mod exists, module_path is auto-detected and only api_key_prefix is passed."
    exit 1
}

if [[ -f "go.mod" ]]; then
    NEW_MODULE="$(grep '^module ' go.mod | awk '{print $2}')"
    if [[ -z "${NEW_MODULE}" ]]; then
        echo "Error: go.mod found but module path could not be detected."
        exit 1
    fi
    echo "Detected module from go.mod: ${NEW_MODULE}"
    if [[ $# -ne 1 ]]; then
        echo "Error: api_key_prefix is required."
        echo ""
        usage
    fi
    API_KEY_PREFIX="$1"
elif [[ $# -eq 2 ]]; then
    NEW_MODULE="$1"
    API_KEY_PREFIX="$2"
else
    echo "Error: no go.mod found; module_path and api_key_prefix are required."
    echo ""
    usage
fi

if [[ ! "${API_KEY_PREFIX}" =~ ^[a-z0-9_]+_$ ]]; then
    echo "Error: api_key_prefix must match ^[a-z0-9_]+_$ (e.g. xjbk_)."
    exit 1
fi


PROJECT_NAME="${NEW_MODULE##*/}"
TARGET_DIR="$(pwd)"
TMP_DIR="$(mktemp -d)"
SCAFFOLD="${TMP_DIR}/scaffold"

trap 'rm -rf "${TMP_DIR}"' EXIT

echo "Template : ${TEMPLATE_MODULE}"
echo "New      : ${NEW_MODULE}"
echo "Prefix   : ${API_KEY_PREFIX}"
echo "Target   : ${TARGET_DIR}"
echo ""

# ── 1. Clone template into tmp ───────────────────────────────────────────────
echo "[1/3] Cloning template..."
git clone --depth=1 --quiet "${TEMPLATE_REPO}" "${SCAFFOLD}"
rm -rf "${SCAFFOLD}/.git"
rm -f  "${SCAFFOLD}/new_project.sh"

# ── 2. Replace all references in every file ──────────────────────────────────
echo "[2/3] Rewriting module paths and project name..."

while IFS= read -r -d '' file; do
    # skip binary files
    grep -qI '' "${file}" 2>/dev/null || continue
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' \
            -e "s|${TEMPLATE_MODULE}|${NEW_MODULE}|g" \
            -e "s|${TEMPLATE_NAME}|${PROJECT_NAME}|g" \
            "${file}"
    else
        sed -i \
            -e "s|${TEMPLATE_MODULE}|${NEW_MODULE}|g" \
            -e "s|${TEMPLATE_NAME}|${PROJECT_NAME}|g" \
            "${file}"
    fi
done < <(find "${SCAFFOLD}" -type f \
    ! -path "*/.git/*" \
    ! -name "*.pb.go" \
    -print0)

# .pb.go 不参与 sed：生成物的 rawDesc 里嵌着 proto 路径字符串且带二进制长度前缀，
# 文本替换会让长度失配，protobuf 解析直接越界 panic。正解是改 proto 源后重新生成。
echo "[2b/3] Regenerating protobuf (proto sources were rewritten)..."
if command -v protoc >/dev/null 2>&1 &&
    command -v protoc-gen-go >/dev/null 2>&1 &&
    command -v protoc-gen-go-grpc >/dev/null 2>&1; then
    (cd "${SCAFFOLD}" && bash gen_proto.sh) ||
        echo "WARN: gen_proto.sh 执行失败，请在新项目内手动重跑（否则 gRPC 面不可用）"
else
    echo "WARN: 未找到 protoc/protoc-gen-go/protoc-gen-go-grpc，.pb.go 仍是模板 module 路径。"
    echo "      新项目内必须安装工具链后执行 ./gen_proto.sh 重新生成（否则 gRPC 面不可用）。"
fi

# ── 3. Patch config/config.go constants ──────────────────────────────────────
echo "[3/3] Patching config/config.go..."

NEW_BUNDLE_ID="com.${PROJECT_NAME}.${PROJECT_NAME}"
CONFIG="${SCAFFOLD}/config/config.go"

if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' \
        -e "s|ProjectName     = \".*\"|ProjectName     = \"${PROJECT_NAME}\"|" \
        -e "s|ProjectVersion  = \".*\"|ProjectVersion  = \"v0.0.1\"|" \
        -e "s|ProjectBundleID = \".*\"|ProjectBundleID = \"${NEW_BUNDLE_ID}\"|" \
        "${CONFIG}"
else
    sed -i \
        -e "s|ProjectName     = \".*\"|ProjectName     = \"${PROJECT_NAME}\"|" \
        -e "s|ProjectVersion  = \".*\"|ProjectVersion  = \"v0.0.1\"|" \
        -e "s|ProjectBundleID = \".*\"|ProjectBundleID = \"${NEW_BUNDLE_ID}\"|" \
        "${CONFIG}"
fi

# ── Rename template-named files in example_files/ ────────────────────────────
while IFS= read -r -d '' f; do
    base="$(basename "$f")"
    new_base="${base//${TEMPLATE_NAME}/${PROJECT_NAME}}"
    [[ "$base" != "$new_base" ]] && mv "$f" "$(dirname "$f")/${new_base}"
done < <(find "${SCAFFOLD}/example_files" -type f -print0)

# ── Clear template changelog entries ─────────────────────────────────────────
if [[ -d "${SCAFFOLD}/changelog" ]]; then
    rm -f "${SCAFFOLD}/changelog"/*.md
fi

# ── Replace the credential prefix token only ─────────────────────────────────
# 只换独立的 key_ 凭证前缀。api_key、key_hash、key_prefix、jwt 与邮件配置不动。
echo "Replacing API key credential prefix..."
SESSION_GO="${SCAFFOLD}/api/api_auth/api_auth_session/session.go"
if [[ ! -f "${SESSION_GO}" ]]; then
    echo "Error: session.go not found; API key prefix was not applied."
    exit 1
fi

while IFS= read -r -d '' file; do
    grep -qI '' "${file}" 2>/dev/null || continue
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' -E \
            "s#(^|[^A-Za-z0-9_])key_([^A-Za-z0-9_]|$)#\\1${API_KEY_PREFIX}\\2#g" \
            "${file}"
    else
        sed -i -E \
            "s#(^|[^A-Za-z0-9_])key_([^A-Za-z0-9_]|$)#\\1${API_KEY_PREFIX}\\2#g" \
            "${file}"
    fi
done < <(find "${SCAFFOLD}" -type f \
    ! -path "*/.git/*" \
    ! -name "*.pb.go" \
    -print0)

if ! grep -F -q "const APIKeyPrefix = \"${API_KEY_PREFIX}\"" "${SESSION_GO}"; then
    echo "Error: failed to replace APIKeyPrefix in session.go."
    exit 1
fi

if [[ "${API_KEY_PREFIX}" != "key_" ]]; then
    set +e
    leftover="$(grep -r --exclude='*.pb.go' --exclude-dir='.git' -E -n -I '(^|[^A-Za-z0-9_])key_([^A-Za-z0-9_]|$)' "${SCAFFOLD}" 2>&1)"
    leftover_status=$?
    set -e
    if [[ "${leftover_status}" -eq 0 ]]; then
        echo "Error: API key prefix replace left credential token key_."
        echo "${leftover}"
        exit 1
    elif [[ "${leftover_status}" -ne 1 ]]; then
        echo "Error: failed to scan for leftover API key prefix."
        echo "${leftover}"
        exit 1
    fi
fi

# ── Copy to target and clean up ──────────────────────────────────────────────
# Preserve existing LICENSE and README.md if present
for f in LICENSE README.md; do
    [[ -f "${TARGET_DIR}/${f}" ]] && cp "${TARGET_DIR}/${f}" "${TMP_DIR}/${f}.bak"
done

# 目标无既有 README 时不落模板使用说明，生成新项目精简 README
if [[ ! -f "${TARGET_DIR}/README.md" ]]; then
    cat > "${SCAFFOLD}/README.md" <<EOF
# ${PROJECT_NAME}

基于 project_template_go 实例化的服务骨架。module：\`${NEW_MODULE}\`。

## 本地运行

- \`go run .\`（Debug 模式读 \`./example_files/config_local.yaml\`）
- 默认端口：JSON-RPC \`13001\`、MCP \`13002\`、test_ui \`13003\`、WebSocket \`13004\`、gRPC \`13005\`

## 文档

- 全量协作契约：\`AGENTS.md\`
- API 方法清单：\`docs/api_methods.md\`（\`gen_api_docs.sh\` 生成，禁手改）
- 各功能域契约：\`docs/ability_*.md\`
EOF
fi

# 目标无既有 LICENSE 时不代选许可证：模板作者 LICENSE 不落入新项目
NEED_LICENSE=0
if [[ ! -f "${TARGET_DIR}/LICENSE" ]]; then
    rm -f "${SCAFFOLD}/LICENSE"
    NEED_LICENSE=1
fi

cp -r "${SCAFFOLD}"/. "${TARGET_DIR}/"

for f in LICENSE README.md; do
    [[ -f "${TMP_DIR}/${f}.bak" ]] && cp "${TMP_DIR}/${f}.bak" "${TARGET_DIR}/${f}"
done

rm -rf "${TMP_DIR}"

echo ""
echo "Done! module: ${NEW_MODULE}"
echo ""
echo "Next steps:"
echo "  git add . && git commit -m 'chore: init from project_template_go'"
if [[ "${NEED_LICENSE}" == "1" ]]; then
    echo "  为新项目补充 LICENSE（模板不代选许可证）"
fi

# 自清理只针对落在目标项目内的脚本副本（wget 下载场景）；引用模板仓路径执行时不得删除模板自身
SCRIPT_PATH="$(cd "$(dirname -- "$0")" 2>/dev/null && pwd)/$(basename -- "$0")"
if [[ "${SCRIPT_PATH}" == "${TARGET_DIR}/"* ]]; then
    rm -f -- "$0"
fi
