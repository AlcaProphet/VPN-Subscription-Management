import { computed } from 'vue'
import { useIsMobile } from './useIsMobile'

export function useDialogWidth(desktopWidth = '520px') {
  const isMobile = useIsMobile()
  return computed(() => isMobile.value ? '90%' : desktopWidth)
}
