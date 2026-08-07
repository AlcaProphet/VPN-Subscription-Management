// utils/importUrl.ts：一键导入 URL 拼接（scheme {url} 占位符替换，下载地址 URL 编码）
// 抽离为独立工具便于单元测试（Build2 Step 6 验收：encodeURIComponent 验证特殊字符）

/**
 * buildImportUrl 将下载地址 URL 编码后替换 scheme 中的 {url} 占位符
 * @param scheme 客户端导入 scheme，如 shadowrocket://add/{url}
 * @param downloadUrl 订阅下载地址（含 token）
 */
export function buildImportUrl(scheme: string, downloadUrl: string): string {
  return scheme.replace('{url}', encodeURIComponent(downloadUrl))
}
