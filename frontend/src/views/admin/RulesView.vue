<!-- admin/RulesView.vue：规则管理（UI §5.7）——双态列表 + 创建弹窗（手填标识 + scheme + 首版本） -->
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Button, Input, Modal, Select, Space, Table, Tabs, Tag, TypographyText, Upload } from 'ant-design-vue'
import { listAdminRules, createRule, renameRule, deleteRule, refreshRuleToken, type RuleItem } from '@/api/rule'
import { checkSlug } from '@/api/subscription'
import ConfirmModal from '@/components/ConfirmModal.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'

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

// --- 创建弹窗 ---
const createOpen = ref(false)
const creating = ref(false)
const createMode = ref<'upload' | 'text'>('upload')
const form = reactive({ name: '', slug: '', client_type: 'shadowrocket', schemes: [''] as string[], text: '' })
const slugStatus = ref<'' | 'error'>('')
const slugTip = ref('')
const slugRe = /^[a-z0-9-]{3,64}$/
let slugTimer: ReturnType<typeof setTimeout> | undefined
async function onSlugChange() {
  clearTimeout(slugTimer)
  if (!form.slug) {
    slugStatus.value = ''
    slugTip.value = ''
    return
  }
  if (!slugRe.test(form.slug)) {
    slugStatus.value = 'error'
    slugTip.value = '须为小写字母数字连字符，长度 3~64'
    return
  }
  slugTimer = setTimeout(async () => {
    try {
      const res = await checkSlug(form.slug, 'rule')
      slugStatus.value = res.available ? '' : 'error'
      slugTip.value = res.available ? '标识可用' : '标识已被使用（四类资源全局唯一）'
    } catch {
      slugStatus.value = ''
      slugTip.value = ''
    }
  }, 300)
}
function openCreate() {
  form.name = ''
  form.slug = ''
  form.client_type = 'shadowrocket'
  form.schemes = ['']
  form.text = ''
  slugStatus.value = ''
  slugTip.value = ''
  createOpen.value = true
}

async function doCreate(file?: File) {
  if (!form.name.trim() || !form.slug.trim()) {
    Notify.error('请填写名称与标识')
    return
  }
  if (slugStatus.value === 'error') {
    Notify.error('标识不可用，请更换')
    return
  }
  const schemes = form.schemes.map((s) => s.trim()).filter(Boolean)
  creating.value = true
  try {
    const payload = new FormData()
    payload.append('name', form.name.trim())
    payload.append('slug', form.slug)
    payload.append('client_type', form.client_type)
    payload.append('schemes', JSON.stringify(schemes))
    if (file) {
      payload.append('file', file)
      await createRule(payload)
    } else {
      if (!form.text) {
        Notify.error('请填写内容')
        return
      }
      payload.append('text', form.text)
      await createRule(payload)
    }
    Notify.success('规则已创建')
    createOpen.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    creating.value = false
  }
}
function beforeUpload(file: File) {
  void doCreate(file)
  return false // 拦截默认上传
}

// --- 改名 ---
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

// --- 复制链接（全局 Token）---
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

// --- 刷新 Token / 删除 ---
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
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold m-0">规则管理</h2>
      <Button type="primary" @click="openCreate">创建规则</Button>
    </div>

    <TriStateList :loading="loading" :empty="rules.length === 0" empty-text="还没有规则">
      <Table :data-source="rules" row-key="id" :pagination="false">
        <Table.Column key="name" title="名称" data-index="name" />
        <Table.Column key="slug" title="标识" width="180">
          <template #default="{ record }"><TypographyText code>{{ record.slug }}</TypographyText></template>
        </Table.Column>
        <Table.Column key="type" title="客户端类型" width="130">
          <template #default="{ record }"><Tag color="blue">{{ record.client_type }}</Tag></template>
        </Table.Column>
        <Table.Column key="version" title="当前版本" width="100">
          <template #default="{ record }">
            <Tag v-if="record.current_version > 0" color="green">v{{ record.current_version }}</Tag>
            <Tag v-else color="default">无版本</Tag>
          </template>
        </Table.Column>
        <Table.Column key="refreshed" title="Token 刷新时间" width="160">
          <template #default="{ record }">{{ record.refreshed_at || '—' }}</template>
        </Table.Column>
        <Table.Column key="actions" title="操作" width="330">
          <template #default="{ record }">
            <Space :wrap="true">
              <Button size="small" @click="renameTarget = record; renameValue = record.name">改名</Button>
              <Button size="small" @click="router.push(`/admin/rules/${record.id}/versions`)">版本管理</Button>
              <Button size="small" :disabled="!record.token" @click="copyLink(record)">复制链接</Button>
              <Button size="small" type="primary" ghost @click="refreshTarget = record">刷新 Token</Button>
              <Button size="small" danger @click="deleteTarget = record">删除</Button>
            </Space>
          </template>
        </Table.Column>
      </Table>
    </TriStateList>

    <!-- 创建弹窗：名称 + 标识 + 客户端类型 + scheme + 首版本（文件/文本） -->
    <Modal v-model:open="createOpen" title="创建规则" :footer="null" :width="560" destroy-on-close>
      <div class="space-y-3">
        <Input v-model:value="form.name" :maxlength="100" placeholder="名称（不强制唯一）" />
        <div>
          <Input v-model:value="form.slug" :status="slugStatus || undefined" placeholder="标识（小写字母数字连字符 3~64，四类全局唯一）"
                 @change="onSlugChange" />
          <div v-if="slugTip" class="text-xs mt-1" :class="slugStatus === 'error' ? 'text-red-500' : 'text-green-600'">
            {{ slugTip }}
          </div>
        </div>
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
        <Tabs v-model:activeKey="createMode">
          <Tabs.TabPane key="upload" tab="文件上传">
            <Upload :show-upload-list="false" :before-upload="beforeUpload">
              <Button :loading="creating">选择文件（≤50MB）</Button>
            </Upload>
          </Tabs.TabPane>
          <Tabs.TabPane key="text" tab="在线编辑">
            <Input.TextArea v-model:value="form.text" :rows="6" placeholder="粘贴规则内容" />
            <Button type="primary" class="mt-2" :loading="creating" @click="doCreate()">创建</Button>
          </Tabs.TabPane>
        </Tabs>
      </div>
    </Modal>

    <!-- 改名弹窗 -->
    <Modal :open="renameTarget !== null" title="改名" :footer="null" :width="420" destroy-on-close
           @cancel="renameTarget = null">
      <Input v-model:value="renameValue" :maxlength="100" @press-enter="doRename" />
      <div class="flex justify-end mt-3">
        <Button type="primary" :loading="renaming" @click="doRename">保存</Button>
      </div>
    </Modal>

    <ConfirmModal :open="refreshTarget !== null" title="刷新 Token" :loading="refreshing"
                  content="刷新后旧链接立即失效；规则 Token 全局共享，不随用户状态变化" @confirm="doRefresh" @update:open="refreshTarget = null" />
    <ConfirmModal :open="deleteTarget !== null" title="删除规则" danger :loading="deleting"
                  content="将删除全部版本文件与 Token，删除后不可恢复" @confirm="doDelete" @update:open="deleteTarget = null" />
  </div>
</template>
