// 草稿回执仅表示已应用到内存表单，不代表已保存到数据库。
export interface DraftApplication {
  path: string
  applied: () => void
}
