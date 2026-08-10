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

rm -f -- "$0"
