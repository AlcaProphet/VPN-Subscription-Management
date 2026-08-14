# Luneflare Shadowrocket Conf:2026 FEB
[General]
bypass-system = true
skip-proxy = 192.168.0.0/16, 10.0.0.0/8, 172.16.0.0/12, localhost, *.local, captive.apple.com, *.local, *.crashlytics.com
tun-excluded-routes = 10.0.0.0/8, 100.64.0.0/10, 127.0.0.0/8, 169.254.0.0/16, 172.16.0.0/12, 192.0.0.0/24, 192.0.2.0/24, 192.88.99.0/24, 192.168.0.0/16, 198.51.100.0/24, 203.0.113.0/24, 224.0.0.0/4, 255.255.255.255/32, 239.255.255.250/32
dns-server = 223.6.6.6, 119.29.29.29
fallback-dns-server = 114.114.114.114, 1.1.1.1
ipv6 = false
# prefer-ipv6 = false
dns-direct-system = false
icmp-auto-reply = true
always-reject-url-rewrite = false
private-ip-answer = true
# direct domain fail to resolve use proxy rule
dns-direct-fallback-proxy = true
# The fallback behavior when UDP traffic matches a policy that doesn't support the UDP relay. Possible values: DIRECT, REJECT.
udp-policy-not-supported-behaviour = REJECT

[Rule]

# 手动处理
DOMAIN-SUFFIX,luneflare.com,DIRECT
DOMAIN-KEYWORD,aws,PROXY

# 苹果无国内CDN
DOMAIN,apple-relay.apple.com,PROXY
USER-AGENT,AppleNews*,PROXY
# 苹果需直连
DOMAIN,api-edge.apps.apple.com,DIRECT
DOMAIN-SUFFIX,aaplimg.com,DIRECT
# 苹果国内CDN
DOMAIN,updates.cdn-apple.com,DIRECT
DOMAIN-SUFFIX,aapl-edge0.qtlcdn.com,DIRECT
USER-AGENT,com.apple.appstored*,DIRECT
IP-CIDR,143.198.200.27/32,PROXY,no-resolve
IP-CIDR,159.89.204.203/32,PROXY,no-resolve
# China
GEOIP,CN,DIRECT
# Final
FINAL,PROXY

[Host]
localhost = 127.0.0.1

[URL Rewrite]
^https?://(www.)?g.cn https://www.google.com 302
^https?://(www.)?google.cn https://www.google.com 302