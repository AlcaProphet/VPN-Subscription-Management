<!-- admin/RulesView.vue：规则管理（Design2-UI §4.6）——空规则实体 + 首页默认展示 + 双态列表 -->
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import dayjs from 'dayjs'
import { Alert, Button, Input, Modal, Radio, Select, Space, Table, Tabs, Tag, Tooltip, TypographyText, Upload } from 'ant-design-vue'
import {
  listAdminRules, createRule, renameRule, deleteRule, refreshRuleToken, setHomeDefault, type RuleItem,
} from '@/api/rule'
import ConfirmModal from '@/components/ConfirmModal.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'

function fmtTime(t: string | null) {
  return t ? dayjs(t).format('YYYY-MM-DD HH:mm') : '—'
}

const router = useRouter()
const loading = ref(false)
const rules = ref<RuleItem[]>([])
async function load() {
  loading.value = true
  try {
    rules.value = await listAdminRules()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(load)

function goAssembly(r: RuleItem) {
  void router.push(`/admin/assembly?tab=sr-conf&rule_id=${r.id}`)
}
function goAssemblyHeader() {
  if (rules.value.length > 0) {
    goAssembly(rules.value[0])
  } else {
    void router.push('/admin/assembly?tab=sr-conf')
  }
}

// --- 创建弹窗（首版本可选：留空创建空规则实体） ---
const createOpen = ref(false)
const creating = ref(false)
const createMode = ref<'upload' | 'text'>('upload')
const form = reactive({ name: '', client_type: 'shadowrocket', schemes: [''] as string[], text: '' })

function openCreate() {
  form.name = ''
  form.client_type = 'shadowrocket'
  form.schemes = ['']
  form.text = ''
  createOpen.value = true
}

async function doCreate(file?: File) {
  if (!form.name.trim()) {
    Notify.error('请填写名称')
    return
  }
  const schemes = form.schemes.map((s) => s.trim()).filter(Boolean)
  creating.value = true
  try {
    if (file) {
      const payload = new FormData()
      payload.append('name', form.name.trim())
      payload.append('client_type', form.client_type)
      payload.append('schemes', JSON.stringify(schemes))
      payload.append('file', file)
      await createRule(payload)
    } else {
      // 文本可空 = 空规则实体（Design2 §3.4 放宽「创建必带首版」）
      await createRule({ name: form.name.trim(), client_type: form.client_type, schemes, text: form.text })
    }
    Notify.success(form.text ? '规则已创建' : '空规则实体已创建（可作为 SR 分流规则装配目标）')
    createOpen.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    creating.value = false
  }
}
function beforeUpload(file: File) {
  if (file.size > 50 << 20) {
    Notify.error('文件超过 50MB 限制')
    return false
  }
  void doCreate(file)
  return false
}

// --- 改名 / 复制 / 刷新 / 删除 ---
const renameTarget = ref<RuleItem | null>(null)
const renameValue = ref('')
const renaming = ref(false)
async function doRename() {
  if (!renameTarget.value || !renameValue.value.trim()) return
  renaming.value = true
  try {
    await renameRule(renameTarget.value.id, renameValue.value.trim())
    Notify.success('名称已更新')
    renameTarget.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    renaming.value = false
  }
}

async function copyLink(r: RuleItem) {
  if (!r.token) return
  const url = `${location.origin}/rules/${r.slug}/download?token=${r.token}`
  try {
    await navigator.clipboard.writeText(url)
    Notify.success('链接已复制（规则内容公开，请谨慎分发）')
  } catch {
    Notify.error('复制失败，请手动复制')
  }
}

const refreshTarget = ref<RuleItem | null>(null)
const refreshing = ref(false)
async function doRefresh() {
  if (!refreshTarget.value) return
  refreshing.value = true
  try {
    await refreshRuleToken(refreshTarget.value.id)
    Notify.success('Token 已刷新，旧链接立即失效')
    refreshTarget.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    refreshing.value = false
  }
}

const deleteTarget = ref<RuleItem | null>(null)
const deleting = ref(false)
async function doDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    await deleteRule(deleteTarget.value.id)
    Notify.success('规则已删除')
    deleteTarget.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    deleting.value = false
  }
}

// --- 首页默认展示：单选切换（ConfirmModal）+ 默认行专设「取消默认」 ---
const toSetDefault = ref<RuleItem | null>(null)
const settingDefault = ref(false)
const oldDefault = () => rules.value.find((r) => r.is_home_default)?.name ?? ''
function askSetDefault(r: RuleItem) {
  if (r.is_home_default) return
  toSetDefault.value = r
}
async function doSetDefault() {
  if (!toSetDefault.value) return
  settingDefault.value = true
  try {
    await setHomeDefault(toSetDefault.value.id, true)
    Notify.success('已设为首页默认规则')
    toSetDefault.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    settingDefault.value = false
  }
}
const cancelTarget = ref<RuleItem | null>(null)
function cancelDefault(r: RuleItem) {
  cancelTarget.value = r
}
async function doCancelDefault() {
  if (!cancelTarget.value) return
  settingDefault.value = true
  try {
    await setHomeDefault(cancelTarget.value.id, false)
    Notify.success('已取消首页默认')
    cancelTarget.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    settingDefault.value = false
  }
}
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold m-0">规则管理</h2>
      <Space>
        <Button @click="goAssemblyHeader">装配生成</Button>
        <Button type="primary" @click="openCreate">创建规则</Button>
      </Space>
    </div>

    <TriStateList :loading="loading" :empty="rules.length === 0" empty-text="还没有规则">
      <!-- ≥768：表格 -->
      <Table :data-source="rules" row-key="id" :pagination="false" class="hidden md:block">
        <Table.Column key="default" title="首页默认" width="90">
          <template #default="{ record }">
            <Radio :checked="record.is_home_default" @click="askSetDefault(record)" />
          </template>
        </Table.Column>
        <Table.Column key="name" title="名称" data-index="name" />
        <Table.Column key="slug" title="标识" width="180">
          <template #default="{ record }"><TypographyText code>{{ record.slug }}</TypographyText></template>
        </Table.Column>
        <Table.Column key="type" title="客户端类型" width="130">
          <template #default="{ record }"><Tag color="blue">{{ record.client_type }}</Tag></template>
        </Table.Column>
        <Table.Column key="version" title="当前版本" width="110">
          <template #default="{ record }">
            <Tag v-if="record.current_version > 0" color="green">v{{ record.current_version }}</Tag>
            <Tooltip v-else title="可作为 SR 分流规则装配目标">
              <span class="text-gray-400">无激活版本</span>
            </Tooltip>
          </template>
        </Table.Column>
        <Table.Column key="refreshed" title="Token 刷新时间" width="160">
          <template #default="{ record }">{{ fmtTime(record.refreshed_at) }}</template>
        </Table.Column>
        <Table.Column key="actions" title="操作" width="380">
          <template #default="{ record }">
            <Space :wrap="true">
              <Button size="small" @click="renameTarget = record; renameValue = record.name">改名</Button>
              <Button size="small" @click="router.push(`/admin/rules/${record.id}/versions`)">版本管理</Button>
              <Button size="small" @click="goAssembly(record)">装配生成</Button>
              <Button size="small" :disabled="!record.token" @click="copyLink(record)">复制链接</Button>
              <Button size="small" type="primary" ghost @click="refreshTarget = record">刷新 Token</Button>
              <Button v-if="record.is_home_default" size="small" :loading="settingDefault" @click="cancelDefault(record)">取消默认</Button>
              <Button size="small" danger @click="deleteTarget = record">删除</Button>
            </Space>
          </template>
        </Table.Column>
      </Table>

      <!-- <768：卡片 -->
      <div class="grid grid-cols-1 gap-3 md:hidden">
        <div v-for="r in rules" :key="r.id" class="border rounded-lg p-3 bg-white dark:bg-gray-800">
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2 min-w-0">
              <Radio :checked="r.is_home_default" @click="askSetDefault(r)" />
              <span class="font-medium truncate">{{ r.name }}</span>
            </div>
            <Tag color="blue" class="shrink-0">{{ r.client_type }}</Tag>
          </div>
          <div class="text-xs text-gray-500 mt-1 flex flex-wrap items-center gap-2">
            <TypographyText code>{{ r.slug }}</TypographyText>
            <Tag v-if="r.current_version > 0" color="green">v{{ r.current_version }}</Tag>
            <Tooltip v-else title="可作为 SR 分流规则装配目标">
              <span class="text-gray-400">无激活版本</span>
            </Tooltip>
            <span>刷新 {{ fmtTime(r.refreshed_at) }}</span>
          </div>
          <div class="mt-2 flex flex-wrap gap-2">
            <Button size="small" @click="renameTarget = r; renameValue = r.name">改名</Button>
            <Button size="small" @click="router.push(`/admin/rules/${r.id}/versions`)">版本管理</Button>
            <Button size="small" @click="goAssembly(r)">装配生成</Button>
            <Button size="small" :disabled="!r.token" @click="copyLink(r)">复制链接</Button>
            <Button size="small" type="primary" ghost @click="refreshTarget = r">刷新 Token</Button>
            <Button v-if="r.is_home_default" size="small" :loading="settingDefault" @click="cancelDefault(r)">取消默认</Button>
            <Button size="small" danger @click="deleteTarget = r">删除</Button>
          </div>
        </div>
      </div>
    </TriStateList>

    <!-- 创建弹窗（首版本可选） -->
    <Modal v-model:open="createOpen" title="创建规则" :footer="null" :width="560" destroy-on-close>
      <div class="space-y-3">
        <Input v-model:value="form.name" :maxlength="100" placeholder="名称（不强制唯一）" />
        <Select v-model:value="form.client_type" class="w-full" disabled>
          <Select.Option value="shadowrocket">Shadowrocket（当前唯一支持）</Select.Option>
        </Select>
        <div>
          <div class="text-xs text-gray-400 mb-1">scheme（含 {url} 占位符，创建后不可修改）</div>
          <div v-for="(_, i) in form.schemes" :key="i" class="flex gap-2 mb-1">
            <Input v-model:value="form.schemes[i]" placeholder="如 shadowrocket://add/{url}" />
            <Button size="small" danger :disabled="form.schemes.length <= 1" @click="form.schemes.splice(i, 1)">删除</Button>
          </div>
          <Button size="small" @click="form.schemes.push('')">添加 scheme</Button>
        </div>
        <Alert type="info" show-icon message="首版本可选：暂不填写内容将创建空规则实体（可作为 SR 分流规则装配目标）" />
        <Tabs v-model:activeKey="createMode">
          <Tabs.TabPane key="upload" tab="文件上传">
            <Upload :show-upload-list="false" :before-upload="beforeUpload">
              <Button :loading="creating">选择文件（≤50MB）</Button>
            </Upload>
          </Tabs.TabPane>
          <Tabs.TabPane key="text" tab="在线编辑">
            <Input.TextArea v-model:value="form.text" :rows="6" placeholder="粘贴规则内容；留空=创建空规则实体" />
            <Button type="primary" class="mt-2" :loading="creating" @click="doCreate()">创建</Button>
          </Tabs.TabPane>
        </Tabs>
      </div>
    </Modal>

    <Modal :open="renameTarget !== null" title="改名" :footer="null" :width="420" destroy-on-close
           @cancel="renameTarget = null">
      <Input v-model:value="renameValue" :maxlength="100" @press-enter="doRename" />
      <div class="flex justify-end mt-3">
        <Button type="primary" :loading="renaming" @click="doRename">保存</Button>
      </div>
    </Modal>

    <ConfirmModal :open="toSetDefault !== null" title="设为首页默认" :loading="settingDefault"
                  :content="`设为首页默认后，原默认规则「${oldDefault() || '无'}」将自动取消默认`"
                  @confirm="doSetDefault" @update:open="toSetDefault = null" />
    <ConfirmModal :open="cancelTarget !== null" title="取消首页默认" :loading="settingDefault"
                  content="取消后首页分流规则卡片将回到未设置空态" @confirm="doCancelDefault" @update:open="cancelTarget = null" />
    <ConfirmModal :open="refreshTarget !== null" title="刷新 Token" :loading="refreshing"
                  content="刷新后旧链接立即失效；规则 Token 全局共享，不随用户状态变化" @confirm="doRefresh" @update:open="refreshTarget = null" />
    <ConfirmModal :open="deleteTarget !== null" title="删除规则" danger :loading="deleting"
                  content="将删除全部版本文件与 Token，删除后不可恢复" @confirm="doDelete" @update:open="deleteTarget = null" />
  </div>
</template>
