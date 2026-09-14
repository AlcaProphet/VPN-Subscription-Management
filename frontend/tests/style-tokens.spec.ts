// style-tokens.spec.ts：前端颜色 Token 静态门禁（Build26 Step 1 建立，Step 19 清零）。
// 扫描 frontend/src 下源码中的 gray/white/black Tailwind 工具类，禁止新增遗留硬编码颜色。
import { describe, expect, it } from 'vitest'
import { readFileSync, readdirSync } from 'node:fs'
import { join, relative } from 'node:path'

interface ColorViolation {
  file: string
  line: number
  match: string
}

// 当前已确认、待 Step 19 R28-07H 修复的遗留违规。每修复一处必须删除对应允许项。
const styleTokenAllowlist: Array<{ file: string; match: string }> = []

const colorTokenPattern = /\b(?:(?:[a-z-]+):)*(?:bg|text|border|ring|from|to|via|placeholder|divide|shadow|outline|decoration|accent|caret|fill|stroke)-(?:gray|white|black)(?:-[0-9]{1,3})?\b/g

// findColorViolations 暴露为可测试的纯函数，用来验证 whitespace/whitelist 不误报。
export function findColorViolations(source: string): Array<{ line: number; match: string }> {
  const out: Array<{ line: number; match: string }> = []
  source.split('\n').forEach((lineText, index) => {
    colorTokenPattern.lastIndex = 0
    for (const match of lineText.matchAll(colorTokenPattern)) {
      out.push({ line: index + 1, match: match[0] })
    }
  })
  return out
}

function collectSourceViolations(): ColorViolation[] {
  const root = process.cwd() // Vitest 从 frontend/ 运行；测试只读扫描，不依赖浏览器环境
  const srcDir = join(root, 'src')
  const files: string[] = []
  const walk = (dir: string) => {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const full = join(dir, entry.name)
      if (entry.isDirectory()) {
        walk(full)
      } else if (entry.isFile() && (entry.name.endsWith('.vue') || entry.name.endsWith('.ts'))) {
        files.push(full)
      }
    }
  }
  walk(srcDir)
  const out: ColorViolation[] = []
  for (const full of files) {
    const source = readFileSync(full, 'utf8')
    for (const v of findColorViolations(source)) {
      out.push({ file: relative(root, full).split('\\').join('/'), line: v.line, match: v.match })
    }
  }
  return out
}

describe('style token 静态门禁', () => {
  it('frontend/src 中不存在未登记的 gray/white/black Tailwind 颜色工具类', () => {
    const violations = collectSourceViolations()
    const unallowed = violations.filter((v) => !styleTokenAllowlist.some((a) => a.file === v.file && a.match === v.match))
    expect(unallowed, `发现未登记的遗留颜色类：\n${unallowed.map((v) => `  ${v.file}:${v.line} ${v.match}`).join('\n')}`).toEqual([])

    for (const allowed of styleTokenAllowlist) {
      expect(violations.some((v) => v.file === allowed.file && v.match === allowed.match), `允许项已无对应违规，应删除：${allowed.file} ${allowed.match}`).toBe(true)
    }
  })

  it('不误报 whitespace / whitelist，且能命中 gray/white/black 颜色类', () => {
    expect(findColorViolations('<div class="whitespace-nowrap whitelist font-semibold"></div>')).toEqual([])
    expect(findColorViolations('<pre class="bg-gray-50 text-white border-black"></pre>').map((v) => v.match)).toEqual([
      'bg-gray-50',
      'text-white',
      'border-black',
    ])
  })
})
