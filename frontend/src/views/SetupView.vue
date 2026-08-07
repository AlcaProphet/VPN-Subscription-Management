<!-- SetupView.vue：首次配置向导（UI §2.1）快速开始 + 高级配置（OIDC）+ 完成页抢注提示 -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Steps, Card, Button, Tag, Alert, Result, Radio, Form, Input, Collapse, Space } from 'ant-design-vue'
import { useSystemStore } from '@/stores/system'
import { useTheme } from '@/theme'
import { http } from '@/api/request'
import { Notify } from '@/components/Notify'
import { oidcTest, setupOidc } from '@/api/oidc'

const router = useRouter()
const system = useSystemStore()
const { dark, toggle } = useTheme()
const current = ref(0)                     // a-steps 当前步：认证方式 → 完成
const submitting = ref(false)
const done = ref(false)
const isProd = computed(() => system.status?.app_mode === 'prod')
// <768 步骤条转竖向（matchMedia 监听）
const isMobile = ref(false)
function checkMobile() {
  isMobile.value = window.matchMedia('(max-width: 767px)').matches
}
onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

async function quickStart() {
  submitting.value = true
  try {
    await http.post('/setup/quickstart')
    current.value = 1
    done.value = true
    await system.fetchStatus(true)         // 刷新守卫状态，后续访问 /setup 将被守卫跳到 /login
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    submitting.value = false
  }
}

// --- 高级配置（OIDC，Step 6）---
const advancedOpen = ref(false)
const providerType = ref('generic')
const oidcForm = reactive({ base_url: '', realm: '', client_id: '', client_secret: '' })
const testResult = ref<{ ok: boolean; message: string; warnings?: string[] } | null>(null)
const testing = ref(false)
const saving = ref(false)

// 提供商选项：模拟 OIDC 仅 Dev 可选
const providerOptions = computed(() => {
  const opts = [
    { label: 'Keycloak', value: 'keycloak' },
    { label: 'Auth0', value: 'auth0' },
    { label: '通用 OIDC', value: 'generic' },
  ]
  if (!isProd.value) opts.push({ label: '模拟 OIDC（Dev）', value: 'mock' })
  return opts
})

// 模拟提供商无需参数；真实提供商按类型动态提示字段
const isMock = computed(() => providerType.value === 'mock')
const origin = window.location.origin // 高级折叠面板展示前端/回调地址推导值

async function runTest() {
  testing.value = true
  testResult.value = null
  try {
    testResult.value = await oidcTest({
      provider_type: providerType.value,
      base_url: oidcForm.base_url,
      realm: oidcForm.realm,
      client_id: oidcForm.client_id,
      client_secret: oidcForm.client_secret,
    })
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    testing.value = false
  }
}

async function completeOidc() {
  saving.value = true
  try {
    await setupOidc({
      provider_type: providerType.value,
      base_url: oidcForm.base_url,
      realm: oidcForm.realm,
      client_id: oidcForm.client_id,
      client_secret: oidcForm.client_secret,
    })
    current.value = 1
    done.value = true
    await system.fetchStatus(true)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    saving.value = false
  }
}

// 「导入已有配置」卡片：仅 Production 模式渲染；本 Step 直接隐藏（Build3 补充）
</script>

<template>
  <!-- 独立全屏路由：居中单列卡片 max-w-720px；顶部 ICON + 「首次配置」+ 模式徽标；右上角暗色切换 -->
  <div class="w-full max-w-3xl">
    <div class="flex justify-end mb-2">
      <Button size="small" @click="toggle">{{ dark ? '浅色模式' : '暗色模式' }}</Button>
    </div>
    <Card>
      <div class="flex items-center gap-3 mb-6">
        <h1 class="text-xl font-semibold">首次配置</h1>
        <Tag :color="isProd ? 'green' : 'blue'">{{ isProd ? 'Production' : 'Dev' }}</Tag>
      </div>
      <Steps :current="current" :items="[{ title: '认证方式' }, { title: '完成' }]"
             class="mb-6" :direction="isMobile ? 'vertical' : 'horizontal'" />

      <template v-if="!done">
        <!-- 快速开始卡片（本 Step 唯一可用入口） -->
        <Card class="mb-4" hoverable>
          <div class="flex items-center justify-between">
            <div>
              <div class="font-medium">快速开始 <Tag color="processing">推荐</Tag></div>
              <div class="text-gray-500 text-sm mt-1">本地账号模式，零配置一键完成</div>
            </div>
            <Button type="primary" :loading="submitting" @click="quickStart">完成配置</Button>
          </div>
        </Card>
        <!-- 高级配置卡片：选中后展开 OIDC 配置（Step 6 填充） -->
        <Card class="mb-4" hoverable @click="advancedOpen = !advancedOpen">
          <div class="flex items-center justify-between">
            <div>
              <div class="font-medium">高级配置</div>
              <div class="text-gray-500 text-sm mt-1">接入单点登录（OIDC）</div>
            </div>
            <Tag color="blue">{{ advancedOpen ? '展开' : '收起' }}</Tag>
          </div>
        </Card>
        <!-- OIDC 参数配置区（选中高级配置后展开） -->
        <Card v-if="advancedOpen" class="mb-4">
          <div class="mb-4">
            <div class="font-medium mb-2">提供商</div>
            <Radio.Group v-model:value="providerType" :options="providerOptions" option-type="button" />
          </div>
          <template v-if="!isMock">
            <Form layout="vertical" class="mt-4">
              <Form.Item label="Base URL">
                <Input v-model:value="oidcForm.base_url" placeholder="https://auth.example.com" />
              </Form.Item>
              <Form.Item v-if="providerType === 'keycloak'" label="Realm">
                <Input v-model:value="oidcForm.realm" placeholder="master" />
              </Form.Item>
              <Form.Item label="Client ID">
                <Input v-model:value="oidcForm.client_id" />
              </Form.Item>
              <Form.Item label="Client Secret">
                <Input.Password v-model:value="oidcForm.client_secret" />
              </Form.Item>
            </Form>
            <Collapse class="mb-4">
              <Collapse.Panel key="1" header="高级（前端地址 / 回调地址）">
                <p class="text-sm text-gray-500">前端地址：{{ origin }}</p>
                <p class="text-sm text-gray-500">回调地址：{{ origin }}/api/auth/oidc/callback</p>
              </Collapse.Panel>
            </Collapse>
            <Space>
              <Button :loading="testing" @click="runTest">测试连接</Button>
              <Button type="primary" :loading="saving" @click="completeOidc">完成配置</Button>
            </Space>
            <Alert v-if="testResult" class="mt-4"
                   :type="testResult.ok ? 'success' : 'error'"
                   :message="testResult.message"
                   :description="testResult.warnings?.join('；')" show-icon />
          </template>
          <template v-else>
            <Alert type="info" class="mt-4 mb-4" message="模拟 OIDC：无需参数，登录页将显示 Dev 模拟登录表单" />
            <Button type="primary" :loading="saving" @click="completeOidc">完成配置</Button>
          </template>
        </Card>
      </template>

      <!-- 完成页：显著步骤式提示 + 抢注风险（Design1 §3.1） -->
      <Result v-else status="success" title="配置完成" sub-title="请部署者本人立即注册成为管理员">
        <template #extra>
          <Alert type="warning" show-icon class="text-left mb-4"
                 message="抢注风险提示"
                 description="首个完成注册的用户将自动成为管理员。公网部署下，请尽快完成注册以关闭抢注窗口。" />
          <Button type="primary" @click="router.push('/login')">前往登录</Button>
        </template>
      </Result>
    </Card>
  </div>
</template>
