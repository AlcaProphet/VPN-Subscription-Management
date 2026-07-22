import { ref } from 'vue'

const toasts = ref([])
let id = 0

export function useToast() {
  function show(message, type = 'info') {
    const toast = { id: ++id, message, type }
    toasts.value.push(toast)
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
