import { ref, onMounted, onUnmounted } from 'vue'

export function useIsMobile(breakpoint = 768) {
  const isMobile = ref(window.innerWidth < breakpoint)

  const mql = window.matchMedia(`(max-width: ${breakpoint - 1}px)`)
  const handler = (e) => {
    isMobile.value = e.matches
  }

  onMounted(() => {
    mql.addEventListener('change', handler)
  })

  onUnmounted(() => {
    mql.removeEventListener('change', handler)
  })

  return isMobile
}
