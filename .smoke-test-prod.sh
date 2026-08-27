#!/bin/bash
# .smoke-test-prod.sh —— 自动拉起临时 Production 容器并执行完整 smoke（四类装配器 + v2 导出/导入往返）。
# 用法：
#   bash .smoke-test-prod.sh
# 环境变量（可选）：
#   SMOKE_PROD_PORT   临时容器对外端口，默认 18081
#   SMOKE_IMAGE       镜像名，默认 vpn-sub:smoke-prod
set -euo pipefail

PORT="${SMOKE_PROD_PORT:-18081}"
IMAGE="${SMOKE_IMAGE:-vpn-sub:smoke-prod}"
NAME="vpn-sub-smoke-prod"
VOL="vpn-sub-smoke-prod-data"
BASE="http://127.0.0.1:${PORT}"

cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  docker volume rm "$VOL" >/dev/null 2>&1 || true
}
trap cleanup EXIT

cleanup

echo "==> 构建镜像 $IMAGE"
docker build -t "$IMAGE" .

echo "==> 启动临时 Production 容器（端口 ${PORT}）"
docker run -d --name "$NAME" \
  -p "127.0.0.1:${PORT}:8080" \
  -e APP_MODE=prod \
  -e LOG_LEVEL=warn \
  -v "$VOL:/data" \
  "$IMAGE"

echo "==> 等待服务就绪"
for _ in $(seq 1 60); do
  if curl -sf "$BASE/health" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if ! curl -sf "$BASE/health" >/dev/null 2>&1; then
  echo "临时 Production 容器未在 60s 内就绪" >&2
  docker logs "$NAME" --tail 100 >&2 || true
  exit 1
fi

echo "==> 运行 .smoke-test.sh（SMOKE_IMPORT=1 触发 v2 导入往返）"
BASE="$BASE" SMOKE_IMPORT=1 bash .smoke-test.sh

echo "=== PROD SMOKE ALL DONE ==="
