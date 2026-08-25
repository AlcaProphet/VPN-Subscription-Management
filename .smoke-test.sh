#!/bin/bash
set -e

# L05 安全门禁：本地 smoke 时同步检查前端依赖不存在 high/critical 漏洞
if [ -d frontend ] && command -v npm >/dev/null 2>&1; then
  (cd frontend && npm audit --audit-level=high)
fi

BASE="${BASE:-http://127.0.0.1:18080}"
J() { python3 -c "import sys,json; d=json.load(sys.stdin); print(d$1)"; }

# 1) Setup 快速开始
curl -s -X POST $BASE/api/setup/quickstart > /dev/null
echo "1) Setup configured=$(curl -s $BASE/api/system/status | J "['data']['configured']")"

# 2) 注册管理员（首管理员直接激活）
TOKEN=$(curl -s -X POST $BASE/api/auth/register -H 'Content-Type: application/json' \
  -d '{"username":"admin1","email":"admin1@x.com","password":"password123"}' | J "['data']['token']")
echo "2) 注册 token=${TOKEN:0:12}..."
AUTH="Authorization: Bearer $TOKEN"

# 3) 创建订阅（每平台一份，product_type 来自平台默认 yaml）
SUB=$(curl -s -X POST $BASE/api/admin/subscriptions -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"platform_id":1,"name":"主订阅","slug":"main-sub"}')
SUBID=$(echo "$SUB" | J "['data']['id']")
echo "3) 订阅 id=$SUBID slug=$(echo "$SUB" | J "['data']['slug']")"

# 4) 创建版本（文本，订阅池首版自动激活）
curl -s -X POST "$BASE/api/admin/subscriptions/$SUBID/versions?mode=text" -H "$AUTH" \
  -H 'Content-Type: application/json' \
  -d '{"text":"proxies:\n  - name: node1\n    server: 1.2.3.4\n    port: 443"}' > /dev/null
echo "4) 版本=$(curl -s $BASE/api/admin/subscriptions/$SUBID/versions -H "$AUTH" | J "['data']['list'][0]['version_no']")"

# 5) 订阅单条 GET（R14-01 回归）
echo "5) 单条订阅 GET=$(curl -s $BASE/api/admin/subscriptions/$SUBID -H "$AUTH" | J "['data']['name']")"

# 6) 首页平台卡（管理员预览形态）
CARD=$(curl -s $BASE/api/home/platforms -H "$AUTH")
STATUS=$(echo "$CARD" | J "['data']['list'][0]['status']")
PREVIEW=$(echo "$CARD" | J "['data']['list'][0]['preview_available']")
echo "6) 首页 platform status=$STATUS preview=$PREVIEW"

# 7) 按平台预览（管理员会话，文本/纯文本）
PREVIEW_TEXT=$(curl -s "$BASE/api/subscriptions/preview?platform=$(echo "$CARD" | J "['data']['list'][0]['slug']")" -H "$AUTH")
echo "7) 预览首行=${PREVIEW_TEXT%%$'\n'*}"

# 8) 规则：创建文本规则并设置首页默认
RULE=$(curl -s -X POST "$BASE/api/admin/rules?mode=text" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"默认规则","slug":"default-rules","client_type":"shadowrocket","schemes":["shadowrocket://add/{url}"],"text":"[Rule]\nGEOIP,CN,DIRECT"}')
RULEID=$(echo "$RULE" | J "['data']['id']")
curl -s -X PUT "$BASE/api/admin/rules/$RULEID/home-default" -H "$AUTH" -H 'Content-Type: application/json' -d '{"is_default":true}' > /dev/null
RULES=$(curl -s $BASE/api/rules -H "$AUTH")
RULE_TOKEN=$(echo "$RULES" | J "['data']['list'][0]['token']")
RULE_SLUG=$(echo "$RULES" | J "['data']['list'][0]['slug']")
echo "8) 规则下载=$(curl -s "$BASE/rules/$RULE_SLUG/download?token=$RULE_TOKEN" | head -1)"

# 9) 分享订阅：创建 → 公开下载 → 吊销 → 404 → 刷新恢复
SHARE=$(curl -s -X POST "$BASE/api/admin/shares?mode=text" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"测试分享","text":"rules: []"}')
SHAREID=$(echo "$SHARE" | J "['data']['id']")
SHARESLUG=$(echo "$SHARE" | J "['data']['slug']")
SHARETOK=$(echo "$SHARE" | J "['data']['token']")
echo "9) 分享 $SHARESLUG 下载=$(curl -s "$BASE/share/$SHARESLUG/download?token=$SHARETOK" | head -1)"
curl -s -X POST $BASE/api/admin/shares/$SHAREID/token/revoke -H "$AUTH" > /dev/null
echo "   吊销后 HTTP=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/share/$SHARESLUG/download?token=$SHARETOK")"
NEWTOK=$(curl -s -X POST $BASE/api/admin/shares/$SHAREID/token/refresh -H "$AUTH" | J "['data']['token']")
echo "   刷新后=$(curl -s "$BASE/share/$SHARESLUG/download?token=$NEWTOK" | head -1)"

# 10) 无效 Token → 404
echo "10) 无效Token HTTP=$(curl -s -o /dev/null -w '%{http_code}' "$BASE/subscriptions/platform-1/download?token=bad")"

# --- Build4~7 核心路径 ---
# 11) 规则素材池 CRUD + 手动条目
POOL=$(curl -s -X POST $BASE/api/admin/pools -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"smoke-pool","urls":[],"auto_sync":false,"sync_time":"04:00"}')
POOLID=$(echo "$POOL" | J "['data']['id']")
curl -s -X POST "$BASE/api/admin/pools/$POOLID/entries" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"rule_type":"DOMAIN-SUFFIX","match_value":"smoke.example"}' > /dev/null
echo "11) 素材池 id=$POOLID entries=$(curl -s "$BASE/api/admin/pools/$POOLID/entries" -H "$AUTH" | J "['data']['total']")"

# 12) manual 节点 + 代理组
NODE=$(curl -s -X POST $BASE/api/admin/nodes -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"smoke-node","protocol":"vless","host":"1.2.3.4","port":443,"protocol_json":{"uuid":"11111111-2222-3333-4444-555555555555"}}')
NODEID=$(echo "$NODE" | J "['data']['id']")
curl -s -X POST $BASE/api/admin/proxy-groups -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"smoke-group","group_type":"select","definition":{"type":"select","nodes":["smoke-node"],"groups":[]}}' > /dev/null
echo "12) manual 节点 id=$NODEID 代理组已建"

# 13) 装配生成（Clash YAML，自动激活首版）
GEN=$(curl -s -X POST $BASE/api/admin/assembly/generate -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"target_syntax":"clash-yaml","platform_id":1,"node_names":["smoke-node"],"group_names":["smoke-group"],"overseas_members":["smoke-node"],"pools":[{"pool_id":'$POOLID',"target":"smoke-group"}],"custom_rules":[],"final_direction":"DIRECT"}')
GENID=$(echo "$GEN" | J "['data']['version_id']")
echo "13) 装配生成 version_id=$GENID auto=$(echo "$GEN" | J "['data']['auto_activated']")"

# 13b) 其他三类装配器（generic-subs / sr-subs / sr-conf）
SUB2=$(curl -s -X POST $BASE/api/admin/subscriptions -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"platform_id":2,"name":"generic-smoke-sub","slug":"generic-smoke-sub"}')
SUB2ID=$(echo "$SUB2" | J "['data']['id']")
GEN2=$(curl -s -X POST $BASE/api/admin/assembly/generate -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"target_syntax":"generic-subs","platform_id":2,"node_names":["smoke-node"],"group_names":[],"pools":[],"custom_rules":[],"final_direction":"DIRECT"}')
GEN2ID=$(echo "$GEN2" | J "['data']['version_id']")
echo "13b) generic-subs 装配 version_id=$GEN2ID"

SUB3=$(curl -s -X POST $BASE/api/admin/subscriptions -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"platform_id":3,"name":"sr-smoke-sub","slug":"sr-smoke-sub"}')
SUB3ID=$(echo "$SUB3" | J "['data']['id']")
GEN3=$(curl -s -X POST $BASE/api/admin/assembly/generate -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"target_syntax":"sr-subs","platform_id":3,"node_names":["smoke-node"],"group_names":[],"pools":[],"custom_rules":[],"final_direction":"DIRECT"}')
GEN3ID=$(echo "$GEN3" | J "['data']['version_id']")
echo "13c) sr-subs 装配 version_id=$GEN3ID"

GENR=$(curl -s -X POST $BASE/api/admin/assembly/generate -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"target_syntax":"sr-conf","rule_id":'$RULEID',"node_names":[],"group_names":[],"pools":[],"custom_rules":[],"final_direction":"PROXY"}')
GENRID=$(echo "$GENR" | J "['data']['version_id']")
echo "13d) sr-conf 装配 version_id=$GENRID"

# 14) Xray 高级模式：开启 + 实例（可选；无 Xray 时跳过检测只验证接口不 5xx）
curl -s -X PUT $BASE/api/admin/settings/advanced -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"advanced_mode":true,"collect_interval_minutes":10,"traffic_card_enabled":true}' > /dev/null
echo "14) 高级模式=$(curl -s $BASE/api/system/status | J "['data']['advanced_mode']")"
if [ -n "${XRAY_FAKE_ADDR:-}" ]; then
  XR=$(curl -s -X POST $BASE/api/admin/xray/instances -H "$AUTH" -H 'Content-Type: application/json' \
    -d "{\"name\":\"smoke-xray\",\"slug\":\"instance-smoke\",\"api_addr\":\"$XRAY_FAKE_ADDR\",\"api_tag\":\"smoke\",\"enabled\":true}")
  echo "15) Xray 实例=$(echo "$XR" | J "['data']['id']")"
else
  echo "15) Xray 实例跳过（未设置 XRAY_FAKE_ADDR）"
fi

# 15/16) v2 导出导入往返（必须 Production；Dev 会 403，禁止假绿）
APP_MODE=$(curl -s $BASE/api/system/status | J "['data']['app_mode']")
if [ "$APP_MODE" != "prod" ]; then
  echo "16) v2 导出/导入要求 Production 模式，当前 app_mode=$APP_MODE；请使用 Production 临时实例运行" >&2
  exit 1
fi
HTTP=$(curl -s -o /tmp/vpn-smoke-export.enc -w '%{http_code}' -X POST $BASE/api/admin/settings/export -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"password":"smoke-pass-123"}')
if [ "$HTTP" != "200" ]; then
  echo "16) v2 导出失败 HTTP=$HTTP body=$(cat /tmp/vpn-smoke-export.enc)" >&2
  exit 1
fi
if head -c 1 /tmp/vpn-smoke-export.enc | grep -q '{'; then
  echo "16) v2 导出返回 JSON 错误：$(cat /tmp/vpn-smoke-export.enc)" >&2
  exit 1
fi
echo "16) v2 导出 HTTP=$HTTP 文件大小=$(wc -c < /tmp/vpn-smoke-export.enc) 字节"

if [ "${SMOKE_IMPORT:-0}" = "1" ]; then
  IMPORT_HTTP=$(curl -s -o /tmp/vpn-smoke-import.out -w '%{http_code}' -X POST $BASE/api/admin/settings/import -H "$AUTH" \
    -F "file=@/tmp/vpn-smoke-export.enc" -F "password=smoke-pass-123" -F "confirm_word=IMPORT" -F "disable_confirm_word=DISABLE")
  echo "17) v2 导入 HTTP=$IMPORT_HTTP body=$(cat /tmp/vpn-smoke-import.out)"
  if [ "$IMPORT_HTTP" != "200" ]; then
    echo "17) v2 导入失败" >&2
    exit 1
  fi
fi

echo "=== SMOKE ALL DONE ==="
