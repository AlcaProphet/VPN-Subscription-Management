// import-limit.spec.ts：两个导入入口的 20 MiB 前端提前拒绝（R28-07G Step 15）。
import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { importFileError, MAX_IMPORT_FILE_BYTES } from '@/utils/fileLimits'

describe('导入文件前端提前拒绝', () => {
  it('恰好 20 MiB 允许，20 MiB+1 拒绝', () => {
    expect(importFileError({ size: MAX_IMPORT_FILE_BYTES, name: 'exact.enc' } as File)).toBeNull()
    const err = importFileError({ size: MAX_IMPORT_FILE_BYTES + 1, name: 'over.enc' } as File)
    expect(err).toContain('20 MiB')
  })

  it('Setup 与管理端设置入口均接入同一提前拒绝规则', () => {
    const root = process.cwd()
    const setup = readFileSync(join(root, 'src/views/SetupView.vue'), 'utf8')
    const settings = readFileSync(join(root, 'src/views/admin/SettingsView.vue'), 'utf8')
    expect(setup).toContain('importFileError')
    expect(settings).toContain('importFileError')
  })
})
