<!-- EmergencyView.vue：应急恢复页（UI §三，Design1 §3.8）——独立全屏路由；操作码校验 → 能力分级 →
     重置管理员密码 / 重新初始化（本页不依赖业务 API） -->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Alert, Button, Input, Modal, Result, Select, Space, Tag } from 'ant-design-vue'
import { emergencyVerify, emergencyResetPassword, emergencyReinitialize, type AdminOption } from '@/api/emergency'
import { useSystemStore } from '@/stores/system'
import { Notify } from '@/components/Notify'

const system = useSystemStore()
const router = useRouter()
const statusLoading = ref(true)
const statusError = ref('')
const emergency = computed(() => system.status?.emergency === true)
const reason = computed(() => system.status?.emergency_reason ?? '')
const reasonText = computed(() => {
  const map: Record<string, string> = {
    manual: '管理员手动触发（RESET_ADMIN_PASSWORD 环境变量）',
    db_corrupt: '数据库无法连接或已损坏',
    key_missing: '关键配置损坏（签名密钥缺失）',
  }
  return map[reason.value] ?? '未知原因'
})

async function loadStatus() {
  statusLoading.value = true
  statusError.value = ''
  try {
    await system.fetchStatus(true)
  } catch (err) {
    statusError.value = (err as Error).message || '无法获取系统状态'
  } finally {
    statusLoading.value = false
  }
}

onMounted(() => { void loadStatus() })

// ① 操作码输入（8 位大字号等宽输入框 + 校验按钮）
const opCode = ref('')
const verifying = ref(false)
const verified = ref(false)
const canReset = ref(false)
const admins = ref<AdminOption[]>([])
async function verify() {
  if (opCode.value.length !== 8) {
    Notify.error('请输入 8 位操作码（从运行日志获取）')
    return
  }
  verifying.value = true
  try {
    const res = await emergencyVerify({ op_code: opCode.value })
    verified.value = true
    canReset.value = res.can_reset_password
    admins.value = res.admins ?? []
  } catch (err) {
    Notify.error((err as Error).message) // 「操作码已失效，请重新从运行日志获取」
    opCode.value = '' // 操作码已消耗（每次提交即失效），需重新从日志获取
  } finally {
    verifying.value = false
  }
}

// ② 重置管理员密码（仅 manual + 库可读时提供）
const selectedAdmin = ref<number | undefined>(undefined)
const newPwd = ref('')
const newPwd2 = ref('')
const resetting = ref(false)
async function doResetPassword() {
  if (!selectedAdmin.value) {
    Notify.error('请选择管理员账号')
    return
  }
  if (newPwd.value.length < 8) {
    Notify.error('新密码至少 8 个字符')
    return
  }
  if (newPwd.value !== newPwd2.value) {
    Notify.error('两次输入的密码不一致')
    return
  }
  resetting.value = true
  try {
    await emergencyResetPassword({ op_code: opCode.value, user_id: selectedAdmin.value, new_password: newPwd.value })
    Modal.success({
      title: '密码已重置',
      content: '进程即将退出重启。请从 docker-compose 移除 RESET_ADMIN_PASSWORD 环境变量并重启容器，以恢复正常服务。',
    })
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    resetting.value = false
  }
}

// ③ 重新初始化（应急全清；操作码 + 二次确认）
const reinitConfirm = ref(false)
const reinitializing = ref(false)
async function doReinitialize() {
  reinitializing.value = true
  try {
    await emergencyReinitialize({ op_code: opCode.value, confirm: '确认重新初始化' })
    Modal.success({
      title: '系统已重新初始化',
      content: '进程即将退出重启，重启后将进入首次配置。',
    })
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    reinitializing.value = false
    reinitConfirm.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-100 p-4">
    <div class="w-full max-w-lg bg-white rounded-xl shadow p-6 space-y-4">
      <Result v-if="statusLoading" status="info" title="正在检查系统状态" sub-title="请稍候…" />
      <Result v-else-if="statusError" status="warning" title="暂时无法确认应急状态" :sub-title="statusError">
        <template #extra>
          <Space>
            <Button type="primary" @click="loadStatus">重试</Button>
            <Button @click="router.push('/login')">返回登录</Button>
          </Space>
        </template>
      </Result>
      <Result v-else-if="!emergency" status="info" title="当前未处于应急恢复模式" sub-title="系统服务正常，无需使用应急操作码。">
        <template #extra>
          <Space>
            <Button type="primary" @click="router.push('/login')">返回登录</Button>
            <Button @click="router.push('/')">返回首页</Button>
          </Space>
        </template>
      </Result>
      <template v-else>
      <div class="flex items-center gap-3">
        <span class="text-3xl">🚨</span>
        <div>
          <div class="text-lg font-semibold">应急恢复模式</div>
          <div class="text-xs text-gray-500">正常服务已暂停，业务 API 与下载端点暂不可用</div>
        </div>
      </div>

      <Alert type="error" show-icon :message="reasonText" />

      <!-- ① 操作码校验 -->
      <div v-if="!verified" class="space-y-2">
        <div class="text-sm">请输入一次性操作码（8 位，见运行日志 docker compose logs）</div>
        <div class="flex gap-2">
          <Input v-model:value="opCode" :maxlength="8" class="font-mono text-2xl tracking-widest text-center"
                 placeholder="••••••••" @press-enter="verify" />
          <Button type="primary" :loading="verifying" @click="verify">校验</Button>
        </div>
        <div class="text-xs text-gray-400">操作码严格一次性：每次提交即消耗，失败后需重新从运行日志获取新码</div>
      </div>

      <!-- ② 校验通过后按能力分级渲染 -->
      <div v-else class="space-y-4">
        <!-- 重置管理员密码（仅 manual + 库可读） -->
        <div v-if="canReset" class="space-y-3">
          <div class="font-medium">重置管理员密码</div>
          <div>
            <div class="mb-1 text-sm">选择管理员账号</div>
            <Select v-model:value="selectedAdmin" class="w-full" placeholder="选择账号（验码前名单不暴露）">
              <Select.Option v-for="a in admins" :key="a.id" :value="a.id">
                {{ a.username }}（{{ a.email || '无邮箱' }}）<Tag v-if="!a.has_password" color="orange">纯 OIDC</Tag>
              </Select.Option>
            </Select>
            <div class="text-xs text-gray-400 mt-1">纯 OIDC 管理员（无本地密码）重置后仍无法本地登录</div>
          </div>
          <div>
            <div class="mb-1 text-sm">新密码（≥8 字符）</div>
            <Input.Password v-model:value="newPwd" :maxlength="128" placeholder="新密码" />
          </div>
          <div>
            <div class="mb-1 text-sm">确认新密码</div>
            <Input.Password v-model:value="newPwd2" :maxlength="128" placeholder="再次输入" />
          </div>
          <Button type="primary" :loading="resetting" @click="doResetPassword">确认重置</Button>
        </div>

        <!-- 重新初始化（应急全清；自动触发仅保留此按钮） -->
        <div class="space-y-2">
          <div class="font-medium">重新初始化（应急全清）</div>
          <Alert type="warning" show-icon message="将清空全部数据回到首次配置状态；数据库损坏时自动降级为删除数据库文件重建" />
          <Button v-if="!reinitConfirm" danger @click="reinitConfirm = true">重新初始化</Button>
          <template v-else>
            <Alert type="error" show-icon message="此操作不可恢复！再次点击确认执行全清（操作码兼承担确认词职能）" />
            <Space>
              <Button danger :loading="reinitializing" @click="doReinitialize">确认重新初始化</Button>
              <Button @click="reinitConfirm = false">取消</Button>
            </Space>
          </template>
        </div>
      </div>
      </template>
    </div>
  </div>
</template>
