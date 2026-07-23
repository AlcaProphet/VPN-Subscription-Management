import { ref } from 'vue'

const MAX_TOASTS = 5
const toasts = ref([])
let id = 0

export function useToast() {
  function show(message, type = 'info') {
    const toast = { id: ++id, message, type }
    toasts.value.push(toast)
    if (toasts.value.length > MAX_TOASTS) {
      toasts.value = toasts.value.slice(-MAX_TOASTS)
    }
    setTimeout(() => {
      toasts.value = toasts.value.filter(t => t.id !== toast.id)
    }, 3000)
  }
  return {
    toasts,
    success: m => show(m, 'success'),
    error: m => show(m, 'error'),
    info: m => show(m, 'info'),
    warning: m => show(m, 'warning'),
  }
}
