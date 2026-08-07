// tests/import-url.spec.ts：一键导入 URL 拼接（Build2 Step 6 验收：URL 编码特殊字符）
import { describe, expect, it } from 'vitest'
import { buildImportUrl } from '@/utils/importUrl'

describe('一键导入 URL 拼接', () => {
  it('下载地址中的特殊字符应被 URL 编码', () => {
    const scheme = 'shadowrocket://add/{url}'
    const downloadUrl = '/subscriptions/platform-abc/download?token=aB3+/=xy&extra=1'
    const url = buildImportUrl(scheme, downloadUrl)
    expect(url).toBe(
      'shadowrocket://add/%2Fsubscriptions%2Fplatform-abc%2Fdownload%3Ftoken%3DaB3%2B%2F%3Dxy%26extra%3D1',
    )
    // 编码后不应含原始特殊字符（? & = + /）
    expect(url).not.toContain('?token=')
    expect(url).not.toContain('+')
  })

  it('无特殊字符时保持原样替换', () => {
    expect(buildImportUrl('clash://install-config?url={url}', '/subscriptions/x/download?token=abc')).toBe(
      'clash://install-config?url=%2Fsubscriptions%2Fx%2Fdownload%3Ftoken%3Dabc',
    )
  })
})
