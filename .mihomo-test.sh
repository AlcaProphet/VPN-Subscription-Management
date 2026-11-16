#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

if [[ -z "${MIHOMO_11931_BIN:-}" ]]; then
  echo "MIHOMO_11931_BIN 未设置；固定 Mihomo 1.19.31 验收未执行。" >&2
  exit 1
fi
if [[ ! -x "$MIHOMO_11931_BIN" ]]; then
  echo "MIHOMO_11931_BIN 不存在或不可执行: $MIHOMO_11931_BIN" >&2
  exit 1
fi

if ! VERSION_OUTPUT="$("$MIHOMO_11931_BIN" -v 2>&1)"; then
  echo "无法读取 Mihomo 版本: $MIHOMO_11931_BIN" >&2
  echo "$VERSION_OUTPUT" >&2
  exit 1
fi
read -r PRODUCT VARIANT VERSION _ <<<"${VERSION_OUTPUT%%$'\n'*}"
if [[ "$PRODUCT" != "Mihomo" || "$VARIANT" != "Meta" || "$VERSION" != "v1.19.31" ]]; then
  echo "固定验收要求 Mihomo Meta v1.19.31，实际: ${VERSION_OUTPUT%%$'\n'*}" >&2
  exit 1
fi

cd "$SCRIPT_DIR/backend"
MIHOMO_11931_BIN="$MIHOMO_11931_BIN" \
  go test ./internal/assembly -count=1 -v -run '^TestMihomo11931'
