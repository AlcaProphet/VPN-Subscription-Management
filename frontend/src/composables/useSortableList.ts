// useSortableList：轻量有序列表排序（上移/下移/拖拽辅助；无外部拖拽库）
import type { Ref } from 'vue'

export function useSortableList<T>(list: Ref<T[]>) {
  function move(from: number, to: number) {
    if (from < 0 || from >= list.value.length || to < 0 || to >= list.value.length) return
    const next = [...list.value]
    const [item] = next.splice(from, 1)
    next.splice(to, 0, item)
    list.value = next
  }
  function up(index: number) {
    if (index > 0) move(index, index - 1)
  }
  function down(index: number) {
    if (index < list.value.length - 1) move(index, index + 1)
  }
  return { move, up, down }
}
