#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "${ROOT_DIR}"

CONFIG_PATH="${OMNIFLOW_CONFIG:-./configs/config.yaml}"

# 一键重置：
# 1) 目录树相关表（自动探测存在的表）
# 2) MinIO 桶对象
go run ./cmd/reset -config "${CONFIG_PATH}" --yes "$@"
