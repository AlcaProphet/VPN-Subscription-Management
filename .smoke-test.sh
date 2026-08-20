#!/bin/bash
set -e
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

echo "=== SMOKE ALL DONE ==="
