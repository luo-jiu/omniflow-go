#!/usr/bin/env sh
set -eu

ROOT_DIR=${OMNIFLOW_ROOT:-/srv/omniflow}
APP_DIR="$ROOT_DIR/app"

if [ ! -f "$ROOT_DIR/compose.yaml" ] || [ ! -f "$ROOT_DIR/.env" ]; then
  echo "missing compose.yaml or .env under $ROOT_DIR" >&2
  exit 1
fi

if [ ! -f "$APP_DIR/Dockerfile" ]; then
  echo "missing application source under $APP_DIR" >&2
  exit 1
fi

if command -v git >/dev/null 2>&1 && git -C "$APP_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  IMAGE_TAG=$(git -C "$APP_DIR" rev-parse --short=12 HEAD)
else
  IMAGE_TAG=$(date -u +%Y%m%d%H%M%S)
fi

cd "$ROOT_DIR"
export OMNIFLOW_IMAGE_TAG="$IMAGE_TAG"
docker compose build api
docker compose up -d --no-deps api

attempt=0
until curl --fail --silent --show-error http://127.0.0.1:8850/healthz >/dev/null; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 20 ]; then
    docker compose logs --tail=100 api >&2
    exit 1
  fi
  sleep 2
done

# 公开只读接口会实际查询 PostgreSQL，用于避免仅验证到 HTTP 进程存活。
curl --fail --silent --show-error \
  "http://127.0.0.1:8850/api/v1/user/exists?username=__omniflow_deploy_probe__" >/dev/null

echo "deployed omniflow-go image tag: $IMAGE_TAG"
