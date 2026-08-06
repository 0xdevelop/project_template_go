#!/bin/bash
set -e

project_path=$(cd "$(dirname "$0")" && pwd)
cd "${project_path}"

go run gen_api_docs.go "$1"
