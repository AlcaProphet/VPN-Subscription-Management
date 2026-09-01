// overlayManager.ts：全局浮层管理器（Build12 Step 2）
// 负责：
// - 非模态浮层唯一：新浮层打开时自动关闭上一个非模态浮层；
// - Modal/Drawer 允许合法层叠；
// - Esc 关闭栈顶浮层并回焦；
// - 焦点保存/恢复。
export type OverlayType = 'dropdown' | 'popover' | 'select' | 'picker' | 'modal' | 'drawer'

export interface OverlayHandle {
  id: string
  type: OverlayType
  close: () => void
  focusTrigger?: () => void
}

interface OverlayRecord extends OverlayHandle {
  previousFocus: HTMLElement | null
}

const activeOverlays: OverlayRecord[] = []
let escapeHandlerInstalled = false

let idCounter = 0
export function nextOverlayId(prefix: string): string {
  idCounter += 1
  return `${prefix}-${idCounter}`
}

function isModalType(type: OverlayType): boolean {
  return type === 'modal' || type === 'drawer'
}

export function registerOverlay(handle: OverlayHandle): () => void {
  // 新浮层打开时先关闭现有非模态浮层；Modal/Drawer 之间允许层叠。
  for (let i = activeOverlays.length - 1; i >= 0; i--) {
    const item = activeOverlays[i]
    if (!isModalType(item.type)) {
      item.close()
      unregisterOverlay(item.id)
    }
  }
  const record: OverlayRecord = {
    ...handle,
    previousFocus: document.activeElement as HTMLElement | null,
  }
  activeOverlays.push(record)
  ensureEscapeHandler()
  return () => unregisterOverlay(handle.id)
}

export function unregisterOverlay(id: string): void {
  const idx = activeOverlays.findIndex((item) => item.id === id)
  if (idx >= 0) {
    const [removed] = activeOverlays.splice(idx, 1)
    if (removed.previousFocus) {
      removed.previousFocus.focus?.()
    }
  }
}

export function getActiveOverlay(): OverlayHandle | null {
  return activeOverlays[activeOverlays.length - 1] ?? null
}

export function closeTopOverlay(): boolean {
  const top = activeOverlays[activeOverlays.length - 1]
  if (!top) return false
  top.close()
  unregisterOverlay(top.id)
  return true
}

export function closeNonModalOverlays(): void {
  for (let i = activeOverlays.length - 1; i >= 0; i--) {
    const item = activeOverlays[i]
    if (!isModalType(item.type)) {
      item.close()
      unregisterOverlay(item.id)
    }
  }
}

function ensureEscapeHandler(): void {
  if (escapeHandlerInstalled) return
  escapeHandlerInstalled = true
  document.addEventListener('keydown', (event) => {
    if (event.key !== 'Escape') return
    const top = activeOverlays[activeOverlays.length - 1]
    if (!top) return
    event.preventDefault()
    top.focusTrigger?.()
    closeTopOverlay()
  })
}

export function saveFocus(id: string): void {
  const item = activeOverlays.find((x) => x.id === id)
  if (item) item.previousFocus = document.activeElement as HTMLElement | null
}

export function restoreFocus(id: string): void {
  const item = activeOverlays.find((x) => x.id === id)
  if (item) item.previousFocus?.focus?.()
}

export function focusFirstInContainer(container: HTMLElement): void {
  const selector = 'input, textarea, select, button, [tabindex]:not([tabindex="-1"])'
  const el = container.querySelector<HTMLElement>(selector)
  el?.focus()
}
