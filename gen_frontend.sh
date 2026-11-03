#!/bin/bash
set -e

project_path=$(cd "$(dirname "$0")" && pwd)
frontend_path="${project_path}/frontend"
bun_tmp_path="${project_path}/tmp/frontend-bun-tmp"
bun_cache_path="${project_path}/tmp/frontend-bun-cache"

if ! command -v bun >/dev/null 2>&1; then
    echo "bun is required to build frontend"
    exit 1
fi

mkdir -p "${bun_tmp_path}" "${bun_cache_path}"

echo "build frontend"
(
    cd "${frontend_path}"
    TMPDIR="${bun_tmp_path}" \
    BUN_INSTALL_CACHE_DIR="${bun_cache_path}" \
    XDG_CACHE_HOME="${bun_cache_path}" \
    bun run build
)
