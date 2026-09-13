#!/usr/bin/env node
// check-md-links.mjs：最小仓库内 Markdown 链接检查。
// 只检查相对路径链接；外部 URL、锚点、mailto、代码块和示例占位符会被跳过。
import { existsSync, readFileSync } from 'node:fs'
import { dirname, isAbsolute, resolve } from 'node:path'

const files = process.argv.slice(2)
if (files.length === 0) {
  console.error('用法：node scripts/check-md-links.mjs <Markdown 文件...>')
  process.exit(2)
}

// 用空白替换围栏代码块和行内代码，保留原始行数，避免把代码示例当成链接。
function stripCode(text) {
  const lines = text.split('\n')
  let inFence = false
  let fenceChar = ''
  const withoutFences = lines.map((line) => {
    const match = line.match(/^\s*(`{3,}|~{3,})/)
    if (match) {
      const currentChar = match[1][0]
      if (!inFence) {
        inFence = true
        fenceChar = currentChar
      } else if (currentChar === fenceChar) {
        inFence = false
        fenceChar = ''
      }
      return ''
    }
    return inFence ? '' : line
  }).join('\n')
  return withoutFences.replace(/`[^`\n]*`/g, (match) => ' '.repeat(match.length))
}

// 解析 Markdown 的 []() 目标，允许目标内部存在成对括号。
function findLinks(text) {
  const links = []
  let searchFrom = 0
  while (true) {
    const marker = text.indexOf('](', searchFrom)
    if (marker === -1) break
    let index = marker + 2
    let depth = 0
    while (index < text.length) {
      const char = text[index]
      if (char === ')' && depth === 0) break
      if (char === '(') depth++
      else if (char === ')') depth--
      index++
    }
    if (index >= text.length) break
    links.push({ raw: text.slice(marker + 2, index), index: marker })
    searchFrom = index + 1
  }
  return links
}

function isExternal(target) {
  return /^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(target) || target.startsWith('//')
}

function skipTarget(target) {
  if (!target) return true
  if (target.startsWith('#')) return true
  if (target.startsWith('/')) return true
  if (isExternal(target)) return true
  if (target.startsWith('localhost') || target.includes('127.0.0.1')) return true
  if (target.includes('{') || target.includes('}') || target.startsWith('...')) return true
  return false
}

let checked = 0
let missing = 0

for (const file of files) {
  if (!existsSync(file)) {
    console.error(`MISSING FILE ${file}`)
    missing++
    continue
  }
  const source = readFileSync(file, 'utf8')
  const text = stripCode(source)
  for (const link of findLinks(text)) {
    let target = link.raw.trim()
    if (target.startsWith('<') && target.endsWith('>')) {
      target = target.slice(1, -1).trim()
    } else {
      const whitespace = target.search(/\s/)
      if (whitespace !== -1) target = target.slice(0, whitespace)
    }
    target = target.split('#', 1)[0].split('?', 1)[0]
    if (skipTarget(target)) continue

    let decoded
    try {
      decoded = decodeURIComponent(target)
    } catch {
      console.error(`INVALID ${file}:${text.slice(0, link.index).split('\n').length} -> ${target}`)
      missing++
      continue
    }
    if (skipTarget(decoded)) continue

    checked++
    const resolved = isAbsolute(decoded) ? decoded : resolve(dirname(file), decoded)
    if (!existsSync(resolved)) {
      const line = text.slice(0, link.index).split('\n').length
      console.error(`MISSING ${file}:${line} -> ${target}`)
      missing++
    }
  }
}

console.log(`checked_links=${checked} missing=${missing} files=${files.length}`)
process.exit(missing === 0 ? 0 : 1)
