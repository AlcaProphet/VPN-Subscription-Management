// 确认框实例焦点、确认词及关闭后的延迟任务回归。
import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, ref } from 'vue'
import ConfirmModal from '@/components/ConfirmModal.vue'
import FormOverlay from '@/components/FormOverlay.vue'
import { getActiveOverlay } from '@/utils/overlayManager'

afterEach(() => { document.body.innerHTML = '' })

describe('ConfirmModal', () => {
  it.each([false, true])('嵌套于移动端=%s 的表单时，仅聚焦当前实例，Esc 后回焦且保留表单', async (mobile) => {
    const original = window.matchMedia
    const listeners = new Set<(event: { matches: boolean }) => void>()
    const viewport = {
      matches: mobile, media: '(max-width: 767px)', addListener: vi.fn(), removeListener: vi.fn(),
      addEventListener: vi.fn((_type: string, listener: (event: { matches: boolean }) => void) => listeners.add(listener)),
      removeEventListener: vi.fn((_type: string, listener: (event: { matches: boolean }) => void) => listeners.delete(listener)),
    }
    window.matchMedia = vi.fn().mockImplementation(() => viewport)
    const Host = defineComponent({
      components: { FormOverlay, ConfirmModal },
      setup: () => ({ open: ref(false), formOpen: ref(true) }),
      template: `<FormOverlay v-model:open="formOpen" title="编辑节点">
        <button id="trigger" @click="open = true">切换</button>
      </FormOverlay><ConfirmModal v-model:open="open" title="确认切换" />`,
    })
    const wrapper = mount(Host, { attachTo: document.body })
    try {
      await flushPromises()
      const trigger = document.querySelector<HTMLButtonElement>('#trigger')!
      trigger.focus()
      trigger.click()
      await flushPromises()
      await new Promise(resolve => setTimeout(resolve, 30))
      const id = getActiveOverlay()!.id
      const confirmWrap = document.querySelector<HTMLElement>(`.${id}`)!
      const form = document.querySelector<HTMLElement>(mobile ? '.ant-drawer' : `.ant-modal-wrap:not(.${id})`)!
      expect(Number(getComputedStyle(confirmWrap).zIndex)).toBeGreaterThan(Number(getComputedStyle(form).zIndex))
      expect(document.querySelector(`.${id} .ant-modal-content`)!.contains(document.activeElement)).toBe(true)
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
      await flushPromises()
      expect(wrapper.vm.open).toBe(false)
      expect(wrapper.vm.formOpen).toBe(true)
      expect(document.activeElement).toBe(trigger)
      // 同一确认框已挂载后再切换断点，后来创建的表单也必须处于确认框下方。
      viewport.matches = !mobile
      listeners.forEach(listener => listener({ matches: !mobile }))
      await flushPromises()
      const nextTrigger = document.querySelector<HTMLButtonElement>('#trigger')!
      nextTrigger.focus()
      nextTrigger.click()
      await flushPromises()
      await new Promise(resolve => setTimeout(resolve, 30))
      const nextForm = document.querySelector<HTMLElement>(!mobile ? '.ant-drawer' : `.ant-modal-wrap:not(.${id})`)!
      expect(getActiveOverlay()!.id).toBe(id)
      expect(Number(getComputedStyle(confirmWrap).zIndex)).toBeGreaterThan(Number(getComputedStyle(nextForm).zIndex))
      expect(confirmWrap.contains(document.activeElement)).toBe(true)
    } finally { wrapper.unmount(); window.matchMedia = original }
  })

  it('普通确认词控制按钮；关闭或卸载后不留下聚焦任务与浮层注册', async () => {
    const wrapper = mount(ConfirmModal, { attachTo: document.body,
      props: { open: true, title: '导入', confirmWord: 'IMPORT' } })
    await flushPromises()
    const id = getActiveOverlay()!.id
    const scope = document.querySelector(`.${id}`)!
    const ok = scope.querySelector<HTMLButtonElement>('.ant-btn-primary')!
    expect(ok.disabled).toBe(true)
    const input = scope.querySelector<HTMLInputElement>('input')!
    input.value = 'IMPORT'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await flushPromises()
    expect(ok.disabled).toBe(false)
    ok.click()
    expect(wrapper.emitted('confirm')).toHaveLength(1)
    await wrapper.setProps({ open: false })
    wrapper.unmount()
    await new Promise(resolve => setTimeout(resolve, 30))
    expect(getActiveOverlay()?.id).not.toBe(id)
    expect(document.querySelector(`.${id}`)).toBeNull()
  })
})
