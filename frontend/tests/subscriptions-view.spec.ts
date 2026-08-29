// subscriptions-view.spec.ts：订阅列表的入池提示保持轻量、可访问且可恢复。
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

vi.mock('@/api/subscription', () => ({
  listSubscriptions: vi.fn(),
  createSubscription: vi.fn(),
  updateSubscription: vi.fn(),
  deleteSubscription: vi.fn(),
}))
vi.mock('@/api/platform', () => ({ listPlatforms: vi.fn() }))

import SubscriptionsView from '@/views/admin/SubscriptionsView.vue'
import { listSubscriptions } from '@/api/subscription'
import { listPlatforms } from '@/api/platform'

const subscription = {
  id: 1, slug: 'main-subscription', name: '主力订阅', platform_id: 2,
  platform_name: 'Clash Verge', product_type: 'yaml', current_version: 0, content_kind: 'blueprint',
}
const originalMatchMedia = window.matchMedia

function router() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: SubscriptionsView },
      { path: '/admin/subscriptions/:id/versions', component: { template: '<div />' } },
      { path: '/admin/assembly', component: { template: '<div />' } },
    ],
  })
}

function popoverStub() {
  return {
    props: ['trigger'],
    template: '<div class="app-popover-stub" :data-trigger="JSON.stringify(trigger)"><slot /><div class="popover-content"><slot name="content" /></div></div>',
  }
}

function setMobile(matches: boolean) {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: vi.fn().mockReturnValue({
      matches,
      media: '(max-width: 767px)',
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }),
  })
}

describe('SubscriptionsView 入池提示', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    sessionStorage.clear()
    ;(listSubscriptions as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([subscription])
    ;(listPlatforms as unknown as ReturnType<typeof vi.fn>).mockResolvedValue([])
  })

  afterEach(() => {
    Object.defineProperty(window, 'matchMedia', { configurable: true, writable: true, value: originalMatchMedia })
  })

  it('桌面端移除状态列，以可聚焦浮层保留去激活入口', async () => {
    setMobile(false)
    sessionStorage.setItem('pooled_sub_1', '1')
    const appRouter = router()
    await appRouter.push('/')
    await appRouter.isReady()
    const wrapper = mount(SubscriptionsView, { global: { plugins: [appRouter], stubs: { AppPopover: popoverStub() } } })
    await flushPromises()

    expect(wrapper.findAll('th').map((cell) => cell.text())).not.toContain('状态')
    const trigger = wrapper.get('button.subscription-pooled-trigger')
    expect(trigger.attributes('aria-label')).toBe('查看入池状态并前往激活')
    expect(wrapper.get('.app-popover-stub').attributes('data-trigger')).toBe('["hover","focus","click"]')

    await wrapper.get('button.subscription-pooled-activate').trigger('click')
    await flushPromises()
    expect(appRouter.currentRoute.value.fullPath).toBe('/admin/subscriptions/1/versions')
    expect(sessionStorage.getItem('pooled_sub_1')).toBeNull()
  })

  it('移动端保留同一轻量触发器，不再渲染常驻 Alert', async () => {
    setMobile(true)
    sessionStorage.setItem('pooled_sub_1', '1')
    const wrapper = mount(SubscriptionsView, { global: { plugins: [router()], stubs: { AppPopover: popoverStub() } } })
    await flushPromises()

    expect(wrapper.find('table').exists()).toBe(false)
    expect(wrapper.find('button.subscription-pooled-trigger').exists()).toBe(true)
    expect(wrapper.find('.ant-alert').exists()).toBe(false)
  })
})
