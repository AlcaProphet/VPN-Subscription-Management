#!/bin/bash
set -e
BASE=http://127.0.0.1:18080
J() { python3 -c "import sys,json; print(json.load(sys.stdin)$1)"; }

# 1) Setup 快速开始
curl -s -X POST $BASE/api/setup/quickstart > /dev/null
echo "1) Setup configured=$(curl -s $BASE/api/system/status | J "['data']['configured']")"

# 2) 注册管理员（首管理员直接激活）
TOKEN=$(curl -s -X POST $BASE/api/auth/register -H 'Content-Type: application/json' \
  -d '{"username":"admin1","email":"admin1@x.com","password":"password123"}' | J "['data']['token']")
echo "2) 注册 token=${TOKEN:0:12}..."
AUTH="Authorization: Bearer $TOKEN"

# 3) 创建订阅
SUB=$(curl -s -X POST $BASE/api/admin/subscriptions -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"platform_id":1,"name":"主订阅","slug":"main-sub"}')
SUBID=$(echo "$SUB" | J "['data']['id']")
echo "3) 订阅 id=$SUBID slug=$(echo "$SUB" | J "['data']['slug']")"

# 4) 创建版本（文本）
curl -s -X POST "$BASE/api/admin/subscriptions/$SUBID/versions?mode=text" -H "$AUTH" \
  -H 'Content-Type: application/json' \
  -d '{"text":"proxies:\n  - name: node1\n    server: 1.2.3.4\n    port: 443"}' > /dev/null
echo "4) 版本=$(curl -s $BASE/api/admin/subscriptions/$SUBID/versions -H "$AUTH" | J "['data']['list']")"

# 5) 组关联 + 每平台选定（整体提交）
GID=$(curl -s $BASE/api/admin/groups -H "$AUTH" | J "['data']['list'][0]['id']")
curl -s -X PUT $BASE/api/admin/groups/$GID -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"name\":\"默认组\",\"sub_ids\":[$SUBID],\"selections\":[{\"platform_id\":1,\"subscription_id\":$SUBID}]}" > /dev/null
echo "5) 组选定 group=$GID"

# 6) 用户首页（管理员：池内订阅列表，取首份显式 Token；列表统一 ListData 包裹解包）
CARD=$(curl -s $BASE/api/home/platforms -H "$AUTH")
PLATFORM_SLUG=$(echo "$CARD" | J "['data']['list'][0]['subscriptions'][0]['download_url'].split('/')[2]")
STATUS=$(echo "$CARD" | J "['data']['list'][0]['status']")
TOK=$(echo "$CARD" | J "['data']['list'][0]['subscriptions'][0]['token']")
echo "6) 首页 platform=$PLATFORM_SLUG status=$STATUS"

# 7) 下载（显式 Token → 订阅内容）
echo "7) 下载: $(curl -s "$BASE/subscriptions/$PLATFORM_SLUG/download?token=$TOK" | head -1)"

# 8) 无效 Token → 404
echo "8) 无效Token: HTTP $(curl -s -o /dev/null -w '%{http_code}' "$BASE/subscriptions/$PLATFORM_SLUG/download?token=bad")"

# 9) 切换版本后下载内容变化（版本管理生效）
curl -s -X POST "$BASE/api/admin/subscriptions/$SUBID/versions?mode=text" -H "$AUTH" \
  -H 'Content-Type: application/json' -d '{"text":"proxies:\n  - name: node2\n    server: 5.6.7.8\n    port: 8443"}' > /dev/null
echo "9) 新版本下载: $(curl -s "$BASE/subscriptions/$PLATFORM_SLUG/download?token=$TOK" | grep -o 'node2')"

# 10) 分享订阅：创建 → 公开下载 → 吊销 → 404 → 刷新恢复
SHARE=$(curl -s -X POST "$BASE/api/admin/shares?mode=text" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"测试分享","text":"rules: []"}')
SHAREID=$(echo "$SHARE" | J "['data']['id']")
SHARESLUG=$(echo "$SHARE" | J "['data']['slug']")
SHARETOK=$(echo "$SHARE" | J "['data']['token']")
echo "10) 分享 $SHARESLUG 下载: $(curl -s "$BASE/share/$SHARESLUG/download?token=$SHARETOK" | head -1)"
curl -s -X POST $BASE/api/admin/shares/$SHAREID/token/revoke -H "$AUTH" > /dev/null
echo "    吊销后: HTTP $(curl -s -o /dev/null -w '%{http_code}' "$BASE/share/$SHARESLUG/download?token=$SHARETOK")"
NEWTOK=$(curl -s -X POST $BASE/api/admin/shares/$SHAREID/token/refresh -H "$AUTH" | J "['data']['token']")
echo "    刷新后: $(curl -s "$BASE/share/$SHARESLUG/download?token=$NEWTOK" | head -1)"

# 11) 规则：创建 → 用户端列表 → 公开下载
curl -s -X POST "$BASE/api/admin/rules?mode=text" -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"默认规则","slug":"default-rules","client_type":"shadowrocket","schemes":["shadowrocket://add/{url}"],"text":"rules: []"}' > /dev/null
RULE=$(curl -s $BASE/api/rules -H "$AUTH")
RULESLUG=$(echo "$RULE" | J "['data']['list'][0]['slug']")
RULETOK=$(echo "$RULE" | J "['data']['list'][0]['token']")
echo "11) 规则下载: $(curl -s "$BASE/rules/$RULESLUG/download?token=$RULETOK" | head -1)"

echo "=== SMOKE ALL DONE ==="
