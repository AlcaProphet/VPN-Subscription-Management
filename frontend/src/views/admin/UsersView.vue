<!-- UsersView.vue：用户管理（UI §5.5，Design1 §3.4.5）——双态列表（后端分页 20 条/页 + 搜索）+ 全操作 Dropdown + 批量发链接 -->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  Alert, Badge, Button, Dropdown, Input, Menu, Modal, Pagination, Radio, Select, Space, Table, Tabs, Tag, Upload,
} from 'ant-design-vue'
import {
  listUsers, createUser, updateUser, changeRole, revokeTokens, resetPassword, clearOidc, setStatus, deleteUser,
  sendPasswordLinks, type AdminUser,
} from '@/api/user'
import { listGroups, type GroupItem } from '@/api/group'
import { listPlatforms, type PlatformItem } from '@/api/platform'
import { upsertCustom, upsertCustomText, deleteCustom } from '@/api/custom'
import { getSMTP } from '@/api/settings'
import { useAuthStore } from '@/stores/auth'
import ConfirmModal from '@/components/ConfirmModal.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'

const router = useRouter()
const auth = useAuthStore()

// 响应式：≥768 表格 / <768 卡片（展示前 4 字段）
const isMobile = ref(false)
function checkMobile() {
  isMobile.value = window.matchMedia('(max-width: 767px)').matches
}
onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})
onUnmounted(() => window.removeEventListener('resize', checkMobile))

// --- 列表（后端分页 20 条/页 + 用户名/邮箱模糊搜索） ---
const loading = ref(false)
const users = ref<AdminUser[]>([])
const total = ref(0)
const page = ref(1)
const size = ref(20)
const keyword = ref('')
async function load() {
  loading.value = true
  try {
    const res = await listUsers({ page: page.value, size: size.value, keyword: keyword.value.trim() })
    users.value = res.list
    total.value = res.total
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
watch([page, keyword], () => load())
onMounted(load)

// 数据源：组选项 + 平台选项（编辑/上传自定义订阅用）+ SMTP 配置状态（批量发链接置灰依据）
const groupOptions = ref<{ label: string; value: number }[]>([])
const platforms = ref<PlatformItem[]>([])
const smtpConfigured = ref(false)
async function loadMeta() {
  try {
    const [grps, plats, smtp] = await Promise.all([listGroups(), listPlatforms(), getSMTP()])
    groupOptions.value = grps.map((g: GroupItem) => ({ label: g.name, value: g.id }))
    platforms.value = plats
    smtpConfigured.value = !!(smtp.host && smtp.user && smtp.password)
  } catch (err) {
    Notify.error((err as Error).message)
  }
}
onMounted(loadMeta)

// 当前登录管理员（本人入口禁用：重置密码/禁用/删除/角色变更）
const me = computed(() => auth.user)

// --- 头部批量操作：为所有无密码用户发送密码设置链接 ---
const sendingLinks = ref(false)
async function batchSendLinks() {
  sendingLinks.value = true
  try {
    const res = await sendPasswordLinks()
    Notify.success(
      `已发送 ${res.sent} 封；排除：待审批 ${res.skipped_pending}、已禁用 ${res.skipped_disabled}、无邮箱 ${res.skipped_no_email}`,
    )
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    sendingLinks.value = false
  }
}

// --- 新建用户弹窗 ---
const createOpen = ref(false)
const creating = ref(false)
const createForm = reactive({ username: '', email: '', password: '' })
function openCreate() {
  createForm.username = ''
  createForm.email = ''
  createForm.password = ''
  createOpen.value = true
}
async function doCreate() {
  if (!createForm.username.trim() || !createForm.email.trim() || !createForm.password) {
    Notify.error('请完整填写用户名、邮箱与密码')
    return
  }
  creating.value = true
  try {
    await createUser({ username: createForm.username.trim(), email: createForm.email.trim(), password: createForm.password })
    Notify.success('用户已创建')
    createOpen.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message) // 邮箱冲突即时提示
  } finally {
    creating.value = false
  }
}

// --- 编辑弹窗（分组换组 + 无邮箱补填） ---
const editOpen = ref(false)
const editing = ref<AdminUser | null>(null)
const editForm = reactive({ group_id: 0, email: '' })
const saving = ref(false)
function openEdit(u: AdminUser) {
  editing.value = u
  editForm.group_id = u.group_id
  editForm.email = ''
  editOpen.value = true
}
async function doEdit() {
  if (!editing.value) return
  saving.value = true
  try {
    const data: { group_id: number; email?: string } = { group_id: editForm.group_id }
    if (!editing.value.email && editForm.email.trim()) data.email = editForm.email.trim() // 仅无邮箱用户补填
    await updateUser(editing.value.id, data)
    Notify.success('用户已更新')
    editOpen.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    saving.value = false
  }
}

// --- 角色变更（ConfirmModal；降级提示级联清显式 Token） ---
const roleTarget = ref<AdminUser | null>(null)
const changingRole = ref(false)
async function confirmRole() {
  if (!roleTarget.value) return
  changingRole.value = true
  try {
    await changeRole(roleTarget.value.id, roleTarget.value.role === 'admin' ? 'user' : 'admin')
    Notify.success('角色已变更')
    roleTarget.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    changingRole.value = false
  }
}

// --- 吊销所有 Token（ConfirmModal 危险） ---
const revokeTarget = ref<AdminUser | null>(null)
const revoking = ref(false)
async function confirmRevoke() {
  if (!revokeTarget.value) return
  revoking.value = true
  try {
    await revokeTokens(revokeTarget.value.id)
    Notify.success('全部下载 Token 已吊销，用户下次访问首页重新生成')
    revokeTarget.value = null
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    revoking.value = false
  }
}

// --- 设置/重置密码（专属弹窗二选一） ---
const resetOpen = ref(false)
const resetTarget = ref<AdminUser | null>(null)
const resetMode = ref<'send_email' | 'direct'>('direct')
const resetting = ref(false)
const directResult = ref('') // 直接重置成功后的随机密码（仅一次展示）
function openReset(u: AdminUser) {
  if (u.status === 'pending') {
    Notify.error('请先在审批中心处理待审批账号')
    return
  }
  resetTarget.value = u
  resetMode.value = 'direct'
  directResult.value = ''
  resetOpen.value = true
}
async function doReset() {
  if (!resetTarget.value) return
  resetting.value = true
  try {
    const res = await resetPassword(resetTarget.value.id, { mode: resetMode.value })
    if (resetMode.value === 'direct') {
      directResult.value = res.password ?? ''
      Notify.info('该用户全部现有会话已失效')
    } else {
      Notify.success('重置邮件已发送')
      resetOpen.value = false
    }
    await load()
  } catch (err) {
    Notify.error((err as Error).message) // 待审批拒绝 / SMTP 未配置提示
  } finally {
    resetting.value = false
  }
}

// --- 清除 OIDC 绑定（无密码用户先弹显著警告） ---
const oidcTarget = ref<AdminUser | null>(null)
const clearingOidc = ref(false)
async function confirmClearOidc() {
  if (!oidcTarget.value) return
  clearingOidc.value = true
  try {
    const res = await clearOidc(oidcTarget.value.id)
    if (!res.has_password) {
      Modal.warning({
        title: '警告：该用户无本地密码',
        content: '清除绑定后该用户将无法登录，建议先为其设置本地密码。',
      })
    } else {
      Notify.success('OIDC 绑定已清除')
    }
    oidcTarget.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    clearingOidc.value = false
  }
}

// --- 禁用/启用（禁用自己置灰） ---
const statusTarget = ref<AdminUser | null>(null)
const disabling = ref(false)
async function confirmStatus() {
  if (!statusTarget.value) return
  disabling.value = true
  try {
    await setStatus(statusTarget.value.id, statusTarget.value.status !== 'disabled')
    Notify.success(statusTarget.value.status === 'disabled' ? '账号已启用' : '账号已禁用，其会话与全部 Token 已失效')
    statusTarget.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    disabling.value = false
  }
}

// --- 删除（ConfirmModal；待审批账号与「拒绝」同效果说明） ---
const deleteTarget = ref<AdminUser | null>(null)
const deleting = ref(false)
const deleteContent = computed(() =>
  deleteTarget.value
    ? deleteTarget.value.status === 'pending'
      ? '待审批账号删除与审批中心「拒绝」同效果：账号删除、邮箱释放可重新注册'
      : '将级联删除该用户全部 Token、自定义订阅（含版本文件），此操作不可恢复'
    : '',
)
async function confirmDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    await deleteUser(deleteTarget.value.id)
    Notify.success('用户已删除')
    deleteTarget.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    deleting.value = false
  }
}

// --- 上传自定义订阅（平台 Select + 文件/文本） ---
const customOpen = ref(false)
const customTarget = ref<AdminUser | null>(null)
const customForm = reactive({ platform_id: 0, mode: 'upload' as 'upload' | 'text', file: null as File | null, text: '' })
const uploading = ref(false)
const customResult = ref('') // 成功后的 custom- 标识展示
function openCustom(u: AdminUser) {
  customTarget.value = u
  customForm.platform_id = platforms.value[0]?.id ?? 0
  customForm.mode = 'upload'
  customForm.file = null
  customForm.text = ''
  customResult.value = ''
  customOpen.value = true
}
function onFileChange(file: File) {
  if (file.size > 50 << 20) { // 前端预校验对齐后端 MaxContentSize（AGENTS §4.1 双重校验）
    Notify.error('文件超过 50MB 限制')
    return false
  }
  customForm.file = file
  return false // 阻止自动上传
}
async function doCustom() {
  if (!customTarget.value) return
  if (customForm.platform_id <= 0) {
    Notify.error('请选择平台')
    return
  }
  uploading.value = true
  try {
    let slug = ''
    if (customForm.mode === 'upload') {
      if (!customForm.file) {
        Notify.error('请选择文件')
        return
      }
      const fd = new FormData()
      fd.append('platform_id', String(customForm.platform_id))
      fd.append('file', customForm.file)
      slug = (await upsertCustom(customTarget.value.id, fd)).slug
    } else {
      if (!customForm.text.trim()) {
        Notify.error('请输入订阅内容')
        return
      }
      slug = (await upsertCustomText(customTarget.value.id, {
        platform_id: customForm.platform_id,
        text: customForm.text,
      })).slug
    }
    customResult.value = slug
    Notify.success('自定义订阅已上传')
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    uploading.value = false
  }
}

// --- 删除自定义订阅（恢复组分配） ---
const customDeleteTarget = ref<{ user: AdminUser; customId: number; platformId: number } | null>(null)
const customDeleting = ref(false)
async function confirmCustomDelete() {
  if (!customDeleteTarget.value) return
  customDeleting.value = true
  try {
    await deleteCustom(customDeleteTarget.value.user.id, customDeleteTarget.value.platformId)
    Notify.success('自定义订阅已删除，恢复组分配')
    customDeleteTarget.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    customDeleting.value = false
  }
}

// 状态 Badge 颜色（UI §5.5：待审批橙/已激活绿/已禁用灰）
const statusColor: Record<string, string> = { pending: 'orange', active: 'green', disabled: 'default' }
const statusText: Record<string, string> = { pending: '待审批', active: '已激活', disabled: '已禁用' }
const sourceText: Record<string, string> = { oidc: 'OIDC', local: '本地创建', selfreg: '自注册' }

// 操作菜单项（Dropdown）
interface MenuItemDef {
  key: string
  label: string
  disabled?: boolean
}
function menuItems(u: AdminUser): MenuItemDef[] {
  const items: MenuItemDef[] = [
    { key: 'edit', label: '编辑' },
    { key: 'role', label: u.role === 'admin' ? '降级为普通用户' : '提升为管理员' },
    { key: 'custom', label: '上传自定义订阅' },
    ...u.custom_subs.map((c) => ({ key: `versions-${c.id}`, label: `版本管理（${c.platform_name}）` })),
    ...u.custom_subs.map((c) => ({ key: `delcustom-${c.id}`, label: `删除自定义订阅（${c.platform_name}）` })),
    { key: 'revoke', label: '吊销所有 Token' },
    { key: 'reset', label: '设置/重置密码' },
  ]
  if (u.has_oidc_binding) items.push({ key: 'oidc', label: '清除 OIDC 绑定' })
  items.push(
    u.status === 'disabled'
      ? { key: 'enable', label: '启用账号' }
      : { key: 'disable', label: '禁用账号', disabled: u.id === me.value?.id },
  )
  items.push({ key: 'delete', label: '删除用户', disabled: u.id === me.value?.id })
  return items
}

function onMenuClick(key: string, u: AdminUser) {
  if (key === 'edit') openEdit(u)
  else if (key === 'role') roleTarget.value = u
  else if (key === 'custom') openCustom(u)
  else if (key.startsWith('versions-')) {
    const c = u.custom_subs.find((x) => x.id === Number(key.slice(9)))
    if (c) void router.push(`/admin/customs/${c.id}/versions`)
  } else if (key.startsWith('delcustom-')) {
    const c = u.custom_subs.find((x) => x.id === Number(key.slice(10)))
    if (c) customDeleteTarget.value = { user: u, customId: c.id, platformId: c.platform_id }
  } else if (key === 'revoke') revokeTarget.value = u
  else if (key === 'reset') openReset(u)
  else if (key === 'oidc') oidcTarget.value = u
  else if (key === 'disable' || key === 'enable') statusTarget.value = u
  else if (key === 'delete') deleteTarget.value = u
}

// 角色变更确认文案（降级提示级联清显式 Token）
const roleConfirmContent = computed(() => {
  const u = roleTarget.value
  if (!u) return ''
  return u.role === 'admin'
    ? `降级后该用户将失去管理员权限，其全部显式订阅 Token 将被级联清除`
    : '提升后该用户将获得管理面板全部权限'
})
</script>

<template>
  <div>
    <div class="flex flex-wrap items-center justify-between gap-2 mb-4">
      <h2 class="text-lg font-semibold m-0">用户管理</h2>
      <Space>
        <Button :loading="sendingLinks" :disabled="!smtpConfigured"
                :title="smtpConfigured ? '' : 'SMTP 未配置，请先在面板配置'"
                @click="batchSendLinks">为所有无密码用户发送密码设置链接</Button>
        <Button type="primary" @click="openCreate">新建用户</Button>
      </Space>
    </div>

    <div class="mb-3">
      <Input v-model:value="keyword" allow-clear placeholder="搜索用户名 / 邮箱" style="max-width: 320px" />
    </div>

    <TriStateList :loading="loading" :empty="users.length === 0 && total === 0" empty-text="暂无用户">
      <!-- ≥768 表格态 -->
      <template v-if="!isMobile">
        <Table :data-source="users" row-key="id" :pagination="false">
          <Table.Column key="username" title="用户名" data-index="username">
            <template #default="{ record }">
              <Space>
                {{ record.username }}
                <Tag v-if="!record.email" color="default">无邮箱</Tag>
              </Space>
            </template>
          </Table.Column>
          <Table.Column key="email" title="邮箱" data-index="email">
            <template #default="{ record }">{{ record.email || '—' }}</template>
          </Table.Column>
          <Table.Column key="role" title="角色" width="110">
            <template #default="{ record }">
              <Tag :color="record.role === 'admin' ? 'red' : 'blue'">{{ record.role === 'admin' ? '管理员' : '用户' }}</Tag>
            </template>
          </Table.Column>
          <Table.Column key="group" title="所属组" width="120">
            <template #default="{ record }">
              <Tag v-if="record.group_name">{{ record.group_name }}</Tag>
              <span v-else>—</span>
            </template>
          </Table.Column>
          <Table.Column key="source" title="来源" width="100">
            <template #default="{ record }">{{ sourceText[record.source] ?? record.source }}</template>
          </Table.Column>
          <Table.Column key="status" title="状态" width="100">
            <template #default="{ record }">
              <Badge :color="statusColor[record.status]" :text="statusText[record.status]" />
            </template>
          </Table.Column>
          <Table.Column key="custom" title="自定义订阅" width="130">
            <template #default="{ record }">
              <Tag v-for="c in record.custom_subs" :key="c.id" color="purple">{{ c.platform_name }}</Tag>
              <span v-if="!record.custom_subs.length">—</span>
            </template>
          </Table.Column>
          <Table.Column key="actions" title="操作" width="100">
            <template #default="{ record }">
              <Dropdown>
                <Button size="small">操作 <span class="ml-0.5">▾</span></Button>
                <template #overlay>
                  <Menu :items="menuItems(record)" @click="(e: any) => onMenuClick(e.key, record)" />
                </template>
              </Dropdown>
            </template>
          </Table.Column>
        </Table>
      </template>
      <!-- <768 卡片态（前 4 字段：用户名/邮箱/角色/所属组） -->
      <template v-else>
        <div class="space-y-3">
          <div v-for="u in users" :key="u.id" class="border rounded-lg p-3">
            <div class="flex items-center justify-between">
              <Space>
                <span class="font-medium">{{ u.username }}</span>
                <Tag v-if="!u.email" color="default">无邮箱</Tag>
                <Tag :color="u.role === 'admin' ? 'red' : 'blue'">{{ u.role === 'admin' ? '管理员' : '用户' }}</Tag>
              </Space>
              <Dropdown>
                <Button size="small">操作 ▾</Button>
                <template #overlay>
                  <Menu :items="menuItems(u)" @click="(e: any) => onMenuClick(e.key, u)" />
                </template>
              </Dropdown>
            </div>
            <div class="text-sm text-gray-500 mt-1 truncate">{{ u.email || '无邮箱' }} · {{ u.group_name || '无组' }}</div>
          </div>
        </div>
      </template>
    </TriStateList>

    <div class="flex justify-end mt-3">
      <Pagination v-model:current="page" :page-size="size" :total="total"
                  :show-total="(t: number) => `共 ${t} 条`" />
    </div>

    <!-- 新建用户弹窗 -->
    <Modal v-model:open="createOpen" title="新建用户" :footer="null" :width="480" destroy-on-close>
      <div class="space-y-3">
        <div>
          <div class="mb-1 text-sm">用户名</div>
          <Input v-model:value="createForm.username" :maxlength="64" placeholder="登录显示名" />
        </div>
        <div>
          <div class="mb-1 text-sm">邮箱</div>
          <Input v-model:value="createForm.email" :maxlength="254" placeholder="登录邮箱（唯一）" />
        </div>
        <div>
          <div class="mb-1 text-sm">密码（≥8 字符）</div>
          <Input.Password v-model:value="createForm.password" :maxlength="128" placeholder="初始密码" />
        </div>
        <div class="flex justify-end">
          <Button type="primary" :loading="creating" @click="doCreate">创建</Button>
        </div>
      </div>
    </Modal>

    <!-- 编辑弹窗：分组换组 + 无邮箱补填 -->
    <Modal v-model:open="editOpen" :title="`编辑用户：${editing?.username ?? ''}`" :footer="null" :width="480" destroy-on-close>
      <div class="space-y-3">
        <div>
          <div class="mb-1 text-sm">所属组（换组无需清 Token，下载实时解析跟随）</div>
          <Select v-model:value="editForm.group_id" class="w-full" :options="groupOptions" />
        </div>
        <div v-if="editing && !editing.email">
          <div class="mb-1 text-sm">补填邮箱（补填后获得设置密码/重置能力）</div>
          <Input v-model:value="editForm.email" :maxlength="254" placeholder="邮箱（唯一）" />
        </div>
        <div class="flex justify-end">
          <Button type="primary" :loading="saving" @click="doEdit">保存</Button>
        </div>
      </div>
    </Modal>

    <!-- 上传自定义订阅弹窗 -->
    <Modal v-model:open="customOpen" :title="`上传自定义订阅：${customTarget?.username ?? ''}`" :footer="null" :width="560" destroy-on-close>
      <div class="space-y-3">
        <div v-if="customResult" class="mb-2">
          <Alert type="success" show-icon :message="`自定义订阅已上传`" :description="`标识：${customResult}`" />
        </div>
        <div>
          <div class="mb-1 text-sm">适用平台</div>
          <Select v-model:value="customForm.platform_id" class="w-full" placeholder="选择平台">
            <Select.Option v-for="p in platforms" :key="p.id" :value="p.id">{{ p.name }}</Select.Option>
          </Select>
        </div>
        <!-- 文件/文本双页签（与版本管理、分享订阅创建弹窗统一，避免按钮错位） -->
        <Tabs v-model:activeKey="customForm.mode">
          <Tabs.TabPane key="upload" tab="文件上传">
            <Upload :before-upload="onFileChange" accept=".yaml,.yml,.txt,.conf">
              <Button>选择文件（≤50MB）</Button>
            </Upload>
          </Tabs.TabPane>
          <Tabs.TabPane key="text" tab="在线编辑">
            <Input.TextArea v-model:value="customForm.text" :rows="6" placeholder="粘贴订阅配置内容（覆盖该用户该平台的组分配）" />
          </Tabs.TabPane>
        </Tabs>
        <div class="flex justify-end">
          <Button type="primary" :loading="uploading" @click="doCustom">上传</Button>
        </div>
      </div>
    </Modal>

    <!-- 设置/重置密码弹窗（二选一） -->
    <Modal v-model:open="resetOpen" :title="`设置/重置密码：${resetTarget?.username ?? ''}`" :footer="null" :width="480" destroy-on-close>
      <div class="space-y-3">
        <Radio.Group v-model:value="resetMode">
          <Radio value="direct">直接重置（系统生成 8 位密码）</Radio>
          <Radio value="send_email" :disabled="!smtpConfigured">触发重置邮件（用户自设密码）</Radio>
        </Radio.Group>
        <Alert v-if="resetMode === 'direct' && directResult" type="success" show-icon
               :message="`新密码：${directResult}`"
               description="请复制并妥善保管，仅展示一次；该用户全部现有会话已失效" />
        <Alert v-if="resetMode === 'send_email'" type="info" show-icon message="将发送一次性重置链接（1 小时有效），SMTP 未配置时不可用" />
        <div class="flex justify-end">
          <Button type="primary" :loading="resetting" @click="doReset">{{ resetMode === 'direct' ? '确认重置' : '发送邮件' }}</Button>
        </div>
      </div>
    </Modal>

    <!-- 各 ConfirmModal -->
    <ConfirmModal :open="!!roleTarget" title="角色变更" :content="roleConfirmContent" :loading="changingRole"
                  @confirm="confirmRole" @update:open="roleTarget = null" />
    <ConfirmModal :open="!!revokeTarget" title="吊销所有下载 Token" danger
                  content="将物理删除该用户全部下载 Token 记录（无标记态），其现有下载链接立即失效，下次访问首页重新生成。确定继续？"
                  :loading="revoking" @confirm="confirmRevoke" @update:open="revokeTarget = null" />
    <ConfirmModal :open="!!oidcTarget" title="清除 OIDC 绑定" danger
                  :content="oidcTarget && !oidcTarget.has_password
                    ? '该用户无本地密码，清除绑定后将无法登录！建议先为其设置本地密码。确定继续？'
                    : '将清空该用户的 OIDC 绑定，其可在个人中心重新绑定。确定继续？'"
                  :loading="clearingOidc" @confirm="confirmClearOidc" @update:open="oidcTarget = null" />
    <ConfirmModal :open="!!statusTarget" :title="statusTarget?.status === 'disabled' ? '启用账号' : '禁用账号'" danger
                  :content="statusTarget?.status === 'disabled'
                    ? '启用后该用户可重新登录，原 Token 不恢复（访问首页重新生成）'
                    : '禁用后该用户全部会话立即失效、全部下载 Token 被删除，且不可登录。确定继续？'"
                  :loading="disabling" @confirm="confirmStatus" @update:open="statusTarget = null" />
    <ConfirmModal :open="!!deleteTarget" title="删除用户" danger :content="deleteContent" :loading="deleting"
                  @confirm="confirmDelete" @update:open="deleteTarget = null" />
    <ConfirmModal :open="!!customDeleteTarget" title="删除自定义订阅" danger
                  content="删除后该用户该平台恢复组分配（无组选定则变为未分配）。确定继续？"
                  :loading="customDeleting" @confirm="confirmCustomDelete" @update:open="customDeleteTarget = null" />
  </div>
</template>
