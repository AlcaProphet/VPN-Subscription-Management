// app-header.spec.ts：通用顶栏的管理入口权限、响应式可用性与路由回归。
import { beforeEach, describe, expect, it } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import AppHeader from '@/components/AppHeader.vue'
import { useAuthStore } from '@/stores/auth'

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/admin', component: { template: '<div />' } },
    ],
  })
}

async function mountHeader(role: 'admin' | 'user', manageBtn: boolean) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const auth = useAuthStore()
  auth.setSession('tok', {
    id: 1,
    username: role === 'admin' ? 'Admin' : 'Member',
    email: `${role}@example.com`,
    role,
    group_id: 1,
    status: 'active',
    user_source: 'local',
  })

  const router = makeRouter()
  await router.push('/')
  await router.isReady()
  const wrapper = mount(AppHeader, {
    props: { manageBtn },
    global: { plugins: [pinia, router] },
  })
  await flushPromises()
  return { wrapper, router }
}

describe('AppHeader', () => {
  beforeEach(() => localStorage.clear())

  it('管理员主页入口在手机宽度不隐藏，且保持 44px 触控高度', async () => {
    const { wrapper, router } = await mountHeader('admin', true)
    const manageButton = wrapper.findAll('button').find((button) => button.text().trim() === '管理面板')

    expect(manageButton).toBeDefined()
    expect(manageButton!.classes()).toContain('inline-flex')
    expect(manageButton!.classes()).toContain('min-h-11')
    expect(manageButton!.classes()).toContain('items-center')
    expect(manageButton!.classes()).toContain('justify-center')
    expect(manageButton!.classes()).toContain('md:min-h-0')
    expect(manageButton!.classes()).not.toContain('hidden')

    await manageButton!.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/admin')
  })

  it('普通用户或未启用入口时不显示管理面板按钮', async () => {
    const member = await mountHeader('user', true)
    expect(member.wrapper.text()).not.toContain('管理面板')
    member.wrapper.unmount()

    const adminWithoutEntry = await mountHeader('admin', false)
    expect(adminWithoutEntry.wrapper.text()).not.toContain('管理面板')
  })
})
