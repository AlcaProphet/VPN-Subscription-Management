port: 7890
socks-port: 7891
redir-port: 7892
tproxy-port: 7893
allow-lan: false
find-process-mode: strict
mode: rule
geox-url:
  geoip: "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.dat"
  geosite: "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat"
  mmdb: "https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.metadb"
geo-auto-update: true
geo-update-interval: 168
log-level: warning
ipv6: false
ntp:
  enable: true
  write-to-system: false
  server: ntp.aliyun.com
  port: 123
  interval: 30
dns:
  enable: true
  ipv6: false
  listen: '0.0.0.0:53'
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  default-nameserver:
    - 223.5.5.5
    - 119.29.29.29
  fallback:
    - '1.1.1.1'
    - '8.8.8.8'
  fallback-filter:
    geoip: true
    geoip-code: CN
    geosite:
      - gfw
    ipcidr:
      - 240.0.0.0/4
  fake-ip-filter:
    - '*.lan'
    - localhost.ptlogin2.qq.com
    - '*.linksys.com'
    - '*.linksyssmartwifi.com'
    - swscan.apple.com
    - mesu.apple.com
    - '*.msftconnecttest.com'
    - captive.apple.com
    - '*.msftncsi.com'
    - time.*.com
    - stun.*.*
    - stun.*.*.*
    - '*.mcdn.bilivideo.cn'
proxies:
  -
    name: '🇺🇸US-1(电信移动联通)'
    type: vless
    server: vpn.example.com
    port: 443
    uuid: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
    network: tcp
    tls: true
    udp: true
    flow: xtls-rprx-vision
    client-fingerprint: random
    servername: www.cloudflare-cn.com
    reality-opts:
      public-key: samplePublicKeySamplePublicKey
      short-id: sampleShortId
  -
    name: '🇺🇸US-2(移动联通)'
    type: vless
    server: vpn.example.com
    port: 443
    uuid: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
    network: tcp
    tls: true
    udp: true
    flow: xtls-rprx-vision
    client-fingerprint: random
    servername: www.cloudflare-cn.com
    reality-opts:
      public-key: samplePublicKeySamplePublicKey
      short-id: sampleShortId
  -
    name: '🇭🇰HK(仅电信)'
    type: vless
    server: vpn.example.com
    port: 443
    uuid: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
    network: tcp
    tls: true
    udp: true
    flow: xtls-rprx-vision
    client-fingerprint: random
    servername: www.cloudflare-cn.com
    reality-opts:
      public-key: samplePublicKeySamplePublicKey
      short-id: sampleShortId
  -
    name: '🛟US-1(备用)'
    type: vmess
    server: vpn.example.com
    port: 1234
    uuid: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
    alterId: 0
    cipher: auto
    udp: true
  -
    name: '🛟US-2(备用)'
    type: vmess
    server: vpn.example.com
    port: 1234
    uuid: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
    alterId: 0
    cipher: auto
    udp: true
  -
    name: '🛟HK(备用)'
    type: vmess
    server: vpn.example.com
    port: 1234
    uuid: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
    alterId: 0
    cipher: auto
    udp: true

proxy-groups:
  -
    name: '🌎国外流量'
    type: select
    proxies:
      - '🇺🇸US-1(电信移动联通)'
      - '🇺🇸US-2(移动联通)'
      - '🇭🇰HK(仅电信)'
      - '🛟US-1(备用)'
      - '🛟US-2(备用)'
      - '🇭🇰HK(备用)'
      - '🚀直接连接'
  -
    name: 🛟无法归属的流量
    type: select
    proxies:
      - '🚀直接连接'
      - '🌎国外流量'
  -
    name: 🎬YouTube
    type: select
    proxies:
      - '🇺🇸US-1(电信移动联通)'
      - '🇺🇸US-2(移动联通)'
      - '🇭🇰HK(仅电信)'
      - '🛟US-1(备用)'
      - '🛟US-2(备用)'
      - '🇭🇰HK(备用)'
  -
    name: 🍿Netflix
    type: select
    proxies:
      - '🇺🇸US-1(电信移动联通)'
      - '🇺🇸US-2(移动联通)'
      - '🇭🇰HK(仅电信)'
      - '🛟US-1(备用)'
      - '🛟US-2(备用)'
      - '🇭🇰HK(备用)'
  -
    name: 🍻哔哩哔哩
    type: select
    proxies:
      - '🚀直接连接'
      - '🇺🇸US-1(电信移动联通)'
      - '🇺🇸US-2(移动联通)'
      - '🇭🇰HK(仅电信)'
      - '🛟US-1(备用)'
      - '🛟US-2(备用)'
      - '🇭🇰HK(备用)'
  -
    name: 📽️国外流媒体
    type: select
    proxies:
      - '🇺🇸US-1(电信移动联通)'
      - '🇺🇸US-2(移动联通)'
      - '🇭🇰HK(仅电信)'
      - '🛟US-1(备用)'
      - '🛟US-2(备用)'
      - '🇭🇰HK(备用)'
  -
    name: 🍎苹果海外服务
    type: select
    proxies:
      - '🚀直接连接'
      - '🇺🇸US-1(电信移动联通)'
      - '🇺🇸US-2(移动联通)'
      - '🇭🇰HK(仅电信)'
      - '🛟US-1(备用)'
      - '🛟US-2(备用)'
      - '🇭🇰HK(备用)'
  -
    name: 🍏苹果国内服务
    type: select
    proxies:
      - '🚀直接连接'
      - '🇺🇸US-1(电信移动联通)'
      - '🇺🇸US-2(移动联通)'
      - '🇭🇰HK(仅电信)'
      - '🛟US-1(备用)'
      - '🛟US-2(备用)'
      - '🇭🇰HK(备用)'
  -
    name: 🤖AI
    type: select
    proxies:
      - '🇺🇸US-1(电信移动联通)'
      - '🇺🇸US-2(移动联通)'
      - '🛟US-1(备用)'
      - '🛟US-2(备用)'
  -
    name: 🎮Steam
    type: select
    proxies:
      - '🚀直接连接'
      - '🇺🇸US-1(电信移动联通)'
      - '🇺🇸US-2(移动联通)'
      - '🇭🇰HK(仅电信)'
      - '🛟US-1(备用)'
      - '🛟US-2(备用)'
      - '🇭🇰HK(备用)'
  -
    name: 🧩Steam下载
    type: select
    proxies:
      - '🚀直接连接'
      - '🇺🇸US-1(电信移动联通)'
      - '🇺🇸US-2(移动联通)'
      - '🇭🇰HK(仅电信)'
      - '🛟US-1(备用)'
      - '🛟US-2(备用)'
      - '🇭🇰HK(备用)'
  -
    name: '🚀直接连接'
    type: select
    proxies:
      - DIRECT

rules:
# 下载
- PROCESS-NAME,aria2c,DIRECT
- DOMAIN-KEYWORD,aria2,DIRECT
- DOMAIN-SUFFIX,localhost,DIRECT
# REGEX表达式
- PROCESS-NAME-REGEX,.*1Password.*,🌎国外流量
# Windows
- PROCESS-NAME,WeChat.exe,DIRECT
# macOS
- PROCESS-NAME,WeChat,DIRECT
# Android（mihomo 内核，使用包名）
- PROCESS-NAME,com.tencent.mm,DIRECT
- PROCESS-NAME,com.valvesoftware.android.steam.community,🌎国外流量
# 其他
# IP地址
- IP-CIDR,192.168.0.0/16,DIRECT,no-resolve
- IP-CIDR6,fd00::/8,DIRECT,no-resolve
# 路由器
- DOMAIN,router.asus.com,DIRECT
- DOMAIN-SUFFIX,captive.apple.com,DIRECT
# 手动
- DOMAIN-SUFFIX,luneflare.com,DIRECT
# AI
# Anthropic
- DOMAIN,servd-anthropic-website.b-cdn.net,🤖AI
- DOMAIN-SUFFIX,anthropic.com,🤖AI
# OpenAI
- DOMAIN,browser-intake-datadoghq.com,🤖AI
- DOMAIN-SUFFIX,chat.com,🤖AI
# Steam海外
- DOMAIN,f3b7q2p3.ssl.hwcdn.net,🎮Steam
- DOMAIN-SUFFIX,playartifact.com,🎮Steam
# Steam国内CDN
- DOMAIN,cm.steampowered.com,🧩Steam下载
- DOMAIN-SUFFIX,gstore.val.manlaxy.com,🧩Steam下载
# 苹果无国内CDN
- DOMAIN,apple-relay.apple.com,🍎苹果海外服务
- DOMAIN-SUFFIX,tv.apple.com,🍎苹果海外服务
# 苹果需直连
- DOMAIN,api-edge.apps.apple.com,🍏苹果国内服务
- DOMAIN-KEYWORD,buy.itunes.apple.com,🍏苹果国内服务
- DOMAIN-SUFFIX,aaplimg.com,🍏苹果国内服务
# 苹果国内CDN
- DOMAIN,aod.itunes.apple.com,🍏苹果国内服务
- DOMAIN-SUFFIX,origin-apple.com.akadns.net,🍏苹果国内服务
# YouTube
- DOMAIN,youtubei.googleapis.com,🎬YouTube
- DOMAIN-SUFFIX,youtube.com.vn,🎬YouTube
# Netflix
- IP-CIDR,198.45.48.0/20,🍿Netflix,no-resolve
- DOMAIN,netflix.com.edgesuite.net,🍿Netflix
- DOMAIN-SUFFIX,fast.com,🍿Netflix
# Bilibili
- DOMAIN,manga.bilibili.com,🍻哔哩哔哩
- DOMAIN-SUFFIX,bilivideo.com,🍻哔哩哔哩
# 国外流媒体合集
- DOMAIN-SUFFIX,soundcloud.com,📽️国外流媒体
- DOMAIN-KEYWORD,spotify.com,📽️国外流媒体
- DOMAIN,audio-ak-spotify-com.akamaized.net,📽️国外流媒体
# 国外流量
- DOMAIN-KEYWORD,google,🌎国外流量
- IP-CIDR,3.123.36.126/32,🌎国外流量,no-resolve
- PROCESS-NAME,LookupViewService,🌎国外流量
- DOMAIN,adservice.google.com,🌎国外流量
- DOMAIN-SUFFIX,google.com,🌎国外流量
# 直连流量
- DOMAIN-KEYWORD,weixin,DIRECT
- IP-CIDR,218.85.142.99/32,DIRECT
- PROCESS-NAME,weixin,DIRECT
- DOMAIN,adservice.weixin.com,DIRECT
- DOMAIN-SUFFIX,weixin.com,DIRECT
# 国内IP
- GEOIP,CN,DIRECT
# 兜底
- MATCH,🛟无法归属的流量