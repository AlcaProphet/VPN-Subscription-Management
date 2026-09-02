// clashHeaderDefaults.ts：Clash/Mihomo 头部的默认值，和用户提供的配置模板保持一致。
export const CLASH_HEADER_DEFAULTS = {
  port: 7890,
  'socks-port': 7891,
  'redir-port': 7892,
  'tproxy-port': 7893,
  'allow-lan': false,
  'find-process-mode': 'strict',
  mode: 'rule',
  'geox-url': {
    geoip: 'https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.dat',
    geosite: 'https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat',
    mmdb: 'https://fastly.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip.metadb',
  },
  'geo-auto-update': true,
  'geo-update-interval': 168,
  'log-level': 'warning',
  ipv6: false,
  ntp: {
    enable: true,
    'write-to-system': false,
    server: 'ntp.aliyun.com',
    port: 123,
    interval: 30,
  },
  dns: {
    enable: true,
    ipv6: false,
    listen: '0.0.0.0:53',
    'enhanced-mode': 'fake-ip',
    'fake-ip-range': '198.18.0.1/16',
    'default-nameserver': ['223.5.5.5', '119.29.29.29'],
    fallback: ['1.1.1.1', '8.8.8.8'],
    'fallback-filter': {
      geoip: true,
      'geoip-code': 'CN',
      geosite: ['gfw'],
      ipcidr: ['240.0.0.0/4'],
    },
    'fake-ip-filter': [
      '*.lan',
      'localhost.ptlogin2.qq.com',
      '*.linksys.com',
      '*.linksyssmartwifi.com',
      'swscan.apple.com',
      'mesu.apple.com',
      '*.msftconnecttest.com',
      'captive.apple.com',
      '*.msftncsi.com',
      'time.*.com',
      'stun.*.*',
      'stun.*.*.*',
      '*.mcdn.bilivideo.cn',
    ],
  },
} as const

// defaultClashHeaderText 每次返回新的文本，避免调用方共享可变对象。
export function defaultClashHeaderText() {
  return JSON.stringify(CLASH_HEADER_DEFAULTS, null, 2)
}
