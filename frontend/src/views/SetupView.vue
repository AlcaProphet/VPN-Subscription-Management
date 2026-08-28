<!-- SetupView.vue：首次配置向导（UI §2.1）快速开始（含确认步）+ 高级配置（OIDC）+ 导入已有配置 + 完成页抢注提示 -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Steps, Card, Button, Tag, Alert, Result, Radio, Form, Input, Collapse, Space, Upload, Modal } from 'ant-design-vue'
import { useSystemStore } from '@/stores/system'
import { useAuthStore } from '@/stores/auth'
import { useTheme } from '@/theme'
import { http } from '@/api/request'
import { Notify } from '@/components/Notify'
import { oidcTest, setupOidc } from '@/api/oidc'
import { setupImportConfig } from '@/api/settings'
import ConfirmModal from '@/components/ConfirmModal.vue'

const router = useRouter()
const system = useSystemStore()
const auth = useAuthStore()
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
    current.value = 2
    done.value = true
    await system.fetchStatus(true)         // 刷新守卫状态，后续访问 /setup 将被守卫跳到 /login
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    submitting.value = false
  }
}

// 「导入已有配置」卡片：仅 Production 模式渲染（Build3 Step 4 补充：Setup 导入双入口）
const importOpen = ref(false)
const importFile = ref<File | null>(null)
const importPwd = ref('')
const importing = ref(false)

function onImportFile(file: File) {
  importFile.value = file
  return false
}

// 导入：上传文件 + 导出密码 + IMPORT 确认词 → POST /api/setup/import（未配置状态暴露，限流 5/min）；
// 校验失败不做任何变更（后端事务内校验），错误原因直接展示；成功后 configured=true 由守卫跳转登录
async function doSetupImport() {
  if (!importFile.value || !importPwd.value) {
    Notify.error('请选择文件并输入导出密码')
    return
  }
  importing.value = true
  try {
    const fd = new FormData()
    fd.append('file', importFile.value)
    fd.append('password', importPwd.value)
    fd.append('confirm_word', 'IMPORT')
    await setupImportConfig(fd)
    importOpen.value = false
    await system.fetchStatus(true) // configured=true，守卫将后续访问跳转登录
    Modal.warning({
      title: '导入完成',
      content: '配置已整体覆盖（导出文件中不存在的配置键已清除）；签名密钥已替换，如有旧会话将全部失效。请立即重启容器后再重新登录。',
      okText: '前往登录',
      onOk: async () => {
        // 签名密钥已替换，旧会话全部失效：先清本地凭据再跳登录（R07-08，防残留失效 token 触发首页 me() 401 全局提示）
        await auth.logoutAction()
        void router.push('/login')
      },
    })
  } catch (err) {
    Notify.error((err as Error).message) // 确认词/密码错误或文件损坏提示
  } finally {
    importing.value = false
  }
}

// 高级配置（OIDC，Step 6）
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
// OIDC 字段随提供商动态显隐（R10-02）：Auth0 用 Domain 标识；Realm 仅 Keycloak 适用
const urlLabel = computed(() => (providerType.value === 'auth0' ? 'Domain' : 'Base URL'))
const urlPlaceholder = computed(() => (providerType.value === 'auth0' ? 'your-tenant.auth0.com' : 'https://auth.example.com'))
// 切换提供商：清空不适用字段（realm 残留会导致 Auth0/通用发现文档 URL 拼接错误）
function onProviderChange(e: { target: { value?: unknown } }) {
  const v = String(e.target.value ?? '')
  providerType.value = v
  if (v !== 'keycloak') oidcForm.realm = ''
}

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
    current.value = 2
    done.value = true
    await system.fetchStatus(true)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    saving.value = false
  }
}
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
      <Steps :current="current" :items="[{ title: '认证方式' }, { title: '确认' }, { title: '完成' }]"
             class="mb-6" :direction="isMobile ? 'vertical' : 'horizontal'" />

      <template v-if="!done && current === 0">
        <!-- 快速开始卡片：点击「下一步」进入确认步（不直接提交） -->
        <Card class="mb-4" hoverable>
          <div class="flex items-center justify-between">
            <div>
              <div class="font-medium">快速开始 <Tag color="processing">推荐</Tag></div>
              <div class="text-text-secondary text-sm mt-1">本地账号模式，零配置一键完成</div>
            </div>
            <Button type="primary" @click="current = 1">下一步</Button>
          </div>
        </Card>
        <!-- 高级配置卡片：选中后展开 OIDC 配置（Step 6 填充） -->
        <Card class="mb-4" hoverable @click="advancedOpen = !advancedOpen">
          <div class="flex items-center justify-between">
            <div>
              <div class="font-medium">高级配置</div>
              <div class="text-text-secondary text-sm mt-1">接入单点登录（OIDC）</div>
            </div>
            <Tag color="blue">{{ advancedOpen ? '展开' : '收起' }}</Tag>
          </div>
        </Card>
        <!-- 导入已有配置卡片：仅 Production 模式渲染（Dev 显示说明文案，UI §2.1） -->
        <Card v-if="isProd" class="mb-4" hoverable>
          <div class="mb-3">
            <div class="font-medium">导入已有配置</div>
            <div class="text-text-secondary text-sm mt-1">从其他实例导出的加密配置文件恢复全部配置（整体覆盖）</div>
          </div>
          <Space class="w-full">
            <Upload :before-upload="onImportFile" :max-count="1">
              <Button>{{ importFile ? importFile.name : '选择配置文件' }}</Button>
            </Upload>
            <Input.Password v-model:value="importPwd" placeholder="导出密码（≥8 字符）" style="max-width: 220px" />
            <Button danger @click="importOpen = true">导入</Button>
          </Space>
          <div class="text-xs text-text-tertiary mt-2">导入将整体覆盖全部配置并替换签名密钥，完成后需重启容器再重新登录</div>
        </Card>
        <Card v-else class="mb-4" hoverable>
          <div class="font-medium">导入已有配置</div>
          <Alert type="info" class="mt-2" show-icon message="Dev 模式不提供配置导入"
                 description="避免模拟 OIDC 等调试配置外流（Design1 §3.4.8）" />
        </Card>
        <!-- OIDC 参数配置区（选中高级配置后展开） -->
        <Card v-if="advancedOpen" class="mb-4">
          <div class="mb-4">
            <div class="font-medium mb-2">提供商</div>
            <Radio.Group v-model:value="providerType" :options="providerOptions" option-type="button" @change="onProviderChange" />
          </div>
          <template v-if="!isMock">
            <Form layout="vertical" class="mt-4">
              <Form.Item :label="urlLabel">
                <Input v-model:value="oidcForm.base_url" :placeholder="urlPlaceholder" />
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
                <p class="text-sm text-text-secondary">前端地址：{{ origin }}</p>
                <p class="text-sm text-text-secondary">回调地址：{{ origin }}/api/auth/oidc/callback</p>
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
      <!-- 快速开始确认步：仅快速开始路径进入（current===1），「返回」回到选择界面，确认后才提交 -->
      <template v-else-if="!done && current === 1">
        <Card class="mb-4">
          <div class="font-medium mb-2">确认快速开始</div>
          <p class="text-text-secondary text-sm mb-4">
            将采用本地账号模式完成配置：注册本地账号即可登录，无需接入外部身份提供商（OIDC）。
            点击「确认完成」后系统立即进入已配置状态，此操作不可撤销。
          </p>
          <Alert type="warning" show-icon class="mb-4" message="配置完成后请部署者本人立即注册成为管理员" />
          <Space>
            <Button @click="current = 0">返回</Button>
            <Button type="primary" :loading="submitting" @click="quickStart">确认完成</Button>
          </Space>
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
      <!-- 导入确认（IMPORT 确认词 + 二次确认） -->
      <ConfirmModal :open="importOpen" title="导入配置（整体覆盖）" danger confirm-word="IMPORT" :loading="importing"
                    content="导入将整体覆盖全部配置：导出文件中不存在的配置键一并清除；签名密钥替换后如有旧会话将全部失效；导入完成后请立即重启容器再重新登录。"
                    @confirm="doSetupImport" @update:open="importOpen = false" />
    </Card>
  </div>
</template>
