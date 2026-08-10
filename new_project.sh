#!/usr/bin/env bash
# Usage:
#   bash new_project.sh <module_path>
#   wget -qO- https://raw.githubusercontent.com/0xdevelop/project_template_go/main/new_project.sh | bash -s -- <module_path>
#
# Run from inside your project directory (already cloned or freshly created).
#
# Examples:
#   bash -s -- github.com/myorg/my_service
#   bash -s -- mycompany.com/backend
#   bash -s -- gitlab.com/team/api

set -euo pipefail

TEMPLATE_REPO="https://github.com/0xdevelop/project_template_go.git"
TEMPLATE_MODULE="github.com/0xdevelop/project_template_go"
TEMPLATE_NAME="project_template_go"

usage() {
    echo "Usage: bash new_project.sh [module_path]"
    echo ""
    echo "  module_path   Go module path (optional if go.mod already exists)"
    echo "                e.g. my_project"
    echo "                     github.com/myorg/my_service"
    echo ""
    echo "  If go.mod exists in the current directory, module_path is auto-detected."
    exit 1
}

if [[ -f "go.mod" ]]; then
    NEW_MODULE="$(grep '^module ' go.mod | awk '{print $2}')"
    if [[ -z "${NEW_MODULE}" ]]; then
        echo "Error: go.mod found but module path could not be detected."
        exit 1
    fi
    echo "Detected module from go.mod: ${NEW_MODULE}"
elif [[ $# -ge 1 ]]; then
    NEW_MODULE="$1"
else
    echo "Error: no go.mod found and no module_path provided."
    echo ""
    usage
fi


PROJECT_NAME="${NEW_MODULE##*/}"
TARGET_DIR="$(pwd)"
TMP_DIR="$(mktemp -d)"
SCAFFOLD="${TMP_DIR}/scaffold"

trap 'rm -rf "${TMP_DIR}"' EXIT

echo "Template : ${TEMPLATE_MODULE}"
echo "New      : ${NEW_MODULE}"
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
    -print0)

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
