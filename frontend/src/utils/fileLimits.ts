// fileLimits.ts：文件导入边界工具（R28-07G）。
// 后端固定 20 MiB 文件上限 / 21 MiB 请求体上限；前端在选择文件阶段提前拒绝超限文件。
export const MAX_IMPORT_FILE_BYTES = 20 * 1024 * 1024

function formatMiB(bytes: number): string {
  return (bytes / (1024 * 1024)).toFixed(1)
}

// importFileError 返回 null 表示允许；否则返回用户可读的拒绝原因。恰好 20 MiB 允许。
export function importFileError(file: Pick<File, 'size' | 'name'>): string | null {
  if (file.size > MAX_IMPORT_FILE_BYTES) {
    return `配置文件不能超过 20 MiB（当前 ${formatMiB(file.size)} MiB）`
  }
  return null
}
