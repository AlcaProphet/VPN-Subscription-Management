// lib/diff.ts：轻量行级 Diff（R12-21 环境无 npm 依赖时本地实现，接口与 jsdiff 的 diffLines 兼容）
export interface DiffPart {
  value: string
  added?: boolean
  removed?: boolean
}

export function diffLines(oldText: string, newText: string): DiffPart[] {
  const a = oldText.split('\n')
  const b = newText.split('\n')
  if (a.length === 1 && a[0] === '') {
    return [{ value: newText, added: true }]
  }
  const n = a.length
  const m = b.length
  const dp: number[][] = Array.from({ length: n + 1 }, () => new Array<number>(m + 1).fill(0))
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1])
    }
  }
  const parts: DiffPart[] = []
  let i = 0
  let j = 0
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      parts.push({ value: a[i] })
      i++
      j++
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      parts.push({ value: a[i], removed: true })
      i++
    } else {
      parts.push({ value: b[j], added: true })
      j++
    }
  }
  while (i < n) {
    parts.push({ value: a[i], removed: true })
    i++
  }
  while (j < m) {
    parts.push({ value: b[j], added: true })
    j++
  }
  return parts
}
