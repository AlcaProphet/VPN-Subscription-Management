<!-- SharesView.vue：分享订阅管理（UI §5.3）——双态列表 + 吊销矩阵按钮态 -->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import dayjs from 'dayjs'
import { Badge, Button, Input, Space, Table, Tabs, Tag, TypographyText, Upload } from 'ant-design-vue'
import { listShares, createShare, renameShare, deleteShare, refreshShareToken, revokeShareToken, type ShareItem } from '@/api/share'
import ConfirmModal from '@/components/ConfirmModal.vue'
import FormOverlay from '@/components/FormOverlay.vue'
import PageHeader from '@/components/PageHeader.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'

// 创建时间 UTC → 本地化展示（R07-04 后端已返回 RFC3339）
function fmtTime(t: string | null) {
  return t ? dayjs(t).format('YYYY-MM-DD HH:mm') : '—'
}

const router = useRouter()
const loading = ref(false)
const shares = ref<ShareItem[]>([])
async function load() {
  loading.value = true
  try {
    shares.value = await listShares()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(load)

// 吊销矩阵判定
const revoked = (s: ShareItem) => s.token_status === 'revoked'

// 复制分享链接（仅 active 可用；链接 = {origin}/share/{slug}/download?token=）
async function copyLink(s: ShareItem) {
  if (!s.token) return
  const url = `${location.origin}/share/${s.slug}/download?token=${s.token}`
  try {
    await navigator.clipboard.writeText(url)
    Notify.success('分享链接已复制')
  } catch {
    Notify.error('复制失败，请手动复制')
  }
}

// --- 创建对话框：名称 + 首版本（文件/文本页签）→ 成功后显著展示自动生成标识供复制 ---
const createOpen = ref(false)
const creating = ref(false)
const createMode = ref<'upload' | 'text'>('upload')
const createName = ref('')
const createText = ref('')
const createdShare = ref<ShareItem | null>(null)
async function doCreate() {
  if (!createName.value.trim()) {
    Notify.error('请填写名称')
    return
  }
  creating.value = true
  try {
    if (createMode.value === 'text') {
      if (!createText.value) {
        Notify.error('请填写内容')
        return
      }
      // 文本模式：JSON body + ?mode=text（FormData 会导致后端走文件分支报「未接收到文件」）
      createdShare.value = await createShare({ name: createName.value.trim(), text: createText.value })
    } else {
      // 文件模式由 beforeUpload 触发
      return
    }
    Notify.success('分享已创建')
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    creating.value = false
  }
}
function beforeUpload(file: File) {
  if (file.size > 50 << 20) { // 前端预校验对齐后端 MaxContentSize（AGENTS §4.1 双重校验）
    Notify.error('文件超过 50MB 限制')
    return false
  }
  const form = new FormData()
  form.append('name', createName.value.trim())
  form.append('file', file)
  createShare(form)
    .then((s) => {
      createdShare.value = s
      Notify.success('分享已创建')
      return load()
    })
    .catch((err) => Notify.error((err as Error).message))
    .finally(() => { creating.value = false })
  creating.value = true
  return false // 拦截默认上传
}
function closeCreate() {
  createOpen.value = false
  createName.value = ''
  createText.value = ''
  createdShare.value = null
}

// --- 改名 ---
const renameTarget = ref<ShareItem | null>(null)
const renameValue = ref('')
const renaming = ref(false)
function openRename(s: ShareItem) {
  renameTarget.value = s
  renameValue.value = s.name
}
async function doRename() {
  if (!renameTarget.value || !renameValue.value.trim()) return
  renaming.value = true
  try {
    await renameShare(renameTarget.value.id, renameValue.value.trim())
    Notify.success('名称已更新')
    renameTarget.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    renaming.value = false
  }
}

// --- 刷新 Token（吊销状态恢复） ---
const refreshTarget = ref<ShareItem | null>(null)
const refreshing = ref(false)
async function doRefresh() {
  if (!refreshTarget.value) return
  refreshing.value = true
  try {
    await refreshShareToken(refreshTarget.value.id)
    Notify.success('Token 已刷新（吊销状态已恢复）')
    refreshTarget.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    refreshing.value = false
  }
}

// --- 吊销 ---
const revokeTarget = ref<ShareItem | null>(null)
const revoking = ref(false)
async function doRevoke() {
  if (!revokeTarget.value) return
  revoking.value = true
  try {
    await revokeShareToken(revokeTarget.value.id)
    Notify.success('Token 已吊销，链接立即失效')
    revokeTarget.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    revoking.value = false
  }
}

// --- 删除 ---
const deleteTarget = ref<ShareItem | null>(null)
const deleting = ref(false)
async function doDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    await deleteShare(deleteTarget.value.id)
    Notify.success('分享已删除')
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
    <PageHeader title="分享订阅">
      <template #actions>
        <Button type="primary" @click="createOpen = true">创建分享</Button>
      </template>
    </PageHeader>

    <TriStateList :loading="loading" :empty="shares.length === 0" empty-text="还没有分享订阅">
      <!-- ≥768：表格 -->
      <Table :data-source="shares" row-key="id" :pagination="false" class="hidden md:block">
        <Table.Column key="name" title="名称" data-index="name" />
        <Table.Column key="created" title="创建时间" width="160">
          <template #default="{ record }">{{ fmtTime(record.created_at) }}</template>
        </Table.Column>
        <Table.Column key="version" title="当前版本" width="100">
          <template #default="{ record }">
            <Tag v-if="record.current_version > 0" color="green">v{{ record.current_version }}</Tag>
            <Tag v-else color="default">无版本</Tag>
          </template>
        </Table.Column>
        <Table.Column key="status" title="Token 状态" width="110">
          <template #default="{ record }">
            <Badge :status="revoked(record) ? 'error' : 'success'" :text="revoked(record) ? '已吊销' : '有效'" />
          </template>
        </Table.Column>
        <Table.Column key="actions" title="操作" width="330">
          <template #default="{ record }">
            <Space :wrap="true">
              <Button size="small" @click="openRename(record)">改名</Button>
              <Button size="small" @click="router.push(`/admin/shares/${record.id}/versions`)">版本管理</Button>
              <Button size="small" :disabled="revoked(record)" @click="copyLink(record)">复制链接</Button>
              <Button size="small" type="primary" ghost @click="refreshTarget = record">刷新 Token</Button>
              <Button size="small" danger :disabled="revoked(record)" @click="revokeTarget = record">吊销</Button>
              <Button size="small" danger @click="deleteTarget = record">删除</Button>
            </Space>
          </template>
        </Table.Column>
      </Table>

      <!-- <768：卡片（移动端易用性，与平台/订阅卡片风格一致） -->
      <div class="grid grid-cols-1 gap-3 md:hidden">
        <div v-for="s in shares" :key="s.id" class="border rounded-lg p-3 bg-surface">
          <div class="flex items-center justify-between gap-2">
            <span class="font-medium truncate">{{ s.name }}</span>
            <Badge :status="revoked(s) ? 'error' : 'success'" :text="revoked(s) ? '已吊销' : '有效'" />
          </div>
          <div class="text-xs text-text-secondary mt-1">创建 {{ fmtTime(s.created_at) }}</div>
          <div class="mt-1">
            <Tag v-if="s.current_version > 0" color="green">v{{ s.current_version }}</Tag>
            <Tag v-else color="default">无版本</Tag>
          </div>
          <div class="mobile-actions mt-2 flex flex-wrap gap-2">
            <Button size="small" @click="openRename(s)">改名</Button>
            <Button size="small" @click="router.push(`/admin/shares/${s.id}/versions`)">版本管理</Button>
            <Button size="small" :disabled="revoked(s)" @click="copyLink(s)">复制链接</Button>
            <Button size="small" type="primary" ghost @click="refreshTarget = s">刷新 Token</Button>
            <Button size="small" danger :disabled="revoked(s)" @click="revokeTarget = s">吊销</Button>
            <Button size="small" danger @click="deleteTarget = s">删除</Button>
          </div>
        </div>
      </div>
    </TriStateList>

    <!-- 创建对话框：名称 + 首版本（文件/文本页签） -->
    <FormOverlay v-model:open="createOpen" title="创建分享订阅" :width="560" :loading="creating" destroy-on-close
                 @cancel="closeCreate">
      <div v-if="createdShare" class="mb-4 p-3 rounded border border-green-300 bg-green-50 dark:bg-green-900/20">
        <div class="text-sm mb-1">分享已创建，标识：<TypographyText code>{{ createdShare.slug }}</TypographyText></div>
      </div>
      <template v-else>
        <Input v-model:value="createName" :maxlength="100" placeholder="名称（创建后仅可改名）" class="mb-3" />
        <Tabs v-model:activeKey="createMode">
          <Tabs.TabPane key="upload" tab="文件上传">
            <Upload :show-upload-list="false" :before-upload="beforeUpload">
              <Button :loading="creating">选择文件（≤50MB）</Button>
            </Upload>
          </Tabs.TabPane>
          <Tabs.TabPane key="text" tab="在线编辑">
            <Input.TextArea v-model:value="createText" :rows="8" placeholder="粘贴分享内容" />
          </Tabs.TabPane>
        </Tabs>
      </template>
      <template #footer>
        <template v-if="createdShare">
          <Button class="touch-target" @click="copyLink(createdShare)">复制分享链接</Button>
          <Button type="primary" class="touch-target" @click="closeCreate">完成</Button>
        </template>
        <template v-else>
          <Button class="touch-target" @click="closeCreate">取消</Button>
          <Button v-if="createMode === 'text'" type="primary" class="touch-target" :loading="creating" @click="doCreate">创建</Button>
        </template>
      </template>
    </FormOverlay>

    <!-- 改名弹窗 -->
    <FormOverlay :open="renameTarget !== null" title="改名" :width="420" :loading="renaming" destroy-on-close
                 @submit="doRename" @update:open="renameTarget = null">
      <div class="mb-2 text-xs text-text-secondary">
        标识：<TypographyText code>{{ renameTarget?.slug ?? '' }}</TypographyText>
      </div>
      <Input v-model:value="renameValue" :maxlength="100" @press-enter="doRename" />
    </FormOverlay>

    <!-- 刷新（含吊销恢复）/ 吊销 / 删除 确认 -->
    <ConfirmModal :open="refreshTarget !== null" title="刷新 Token" :loading="refreshing"
                  content="刷新后旧链接立即失效；已吊销状态将恢复并生成新链接" @confirm="doRefresh" @update:open="refreshTarget = null" />
    <ConfirmModal :open="revokeTarget !== null" title="吊销 Token" danger :loading="revoking"
                  content="链接立即失效，内容与版本保留；可随时刷新恢复" @confirm="doRevoke" @update:open="revokeTarget = null" />
    <ConfirmModal :open="deleteTarget !== null" title="删除分享" danger :loading="deleting"
                  content="将删除全部版本文件与 Token，删除后不可恢复" @confirm="doDelete" @update:open="deleteTarget = null" />
  </div>
</template>
