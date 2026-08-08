<!-- VersionManageView.vue：通用版本管理视图组件（四类资源复用，props 驱动，UI §5.1/7.1） -->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Alert, Button, Input, Modal, Space, Table, Tabs, Tag, Tooltip, TypographyText, Upload } from 'ant-design-vue'
import { ArrowLeftOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import { versionApi, type VersionItem } from '@/api/version'
import ConfirmModal from '@/components/ConfirmModal.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'

const props = defineProps<{
  ownerType: 'subscription' | 'rule' | 'custom' | 'share'
  ownerId: number
  apiPrefix: string // 如 /api/admin/subscriptions
  resourceName?: string // 标题展示用
  backPath?: string // 返回上级列表页路径（版本子页返回入口，UI §1 PageHeader 面包屑）
}>()

const router = useRouter()

// 返回上级列表页（如订阅管理）；custom 暂回订阅管理（用户管理在 Build3 接通后修正）
function goBack() {
  if (props.backPath) void router.push(props.backPath)
}

const api = versionApi(props.apiPrefix)
const loading = ref(false)
const versions = ref<VersionItem[]>([])
const createOpen = ref(false)
const createMode = ref<'upload' | 'text'>('upload')
const editText = ref('')
const yamlWarning = ref('')
const saving = ref(false)
const previewContent = ref<string | null>(null)
const previewing = ref(false)
const toDelete = ref<number | null>(null)
const deleting = ref(false)
const toSwitch = ref<number | null>(null)
const switching = ref(false)

async function load() {
  loading.value = true
  try {
    versions.value = await api.list(props.ownerId)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(load)

// 文件上传（自动提交新版本）
async function onUpload(file: File) {
  const form = new FormData()
  form.append('file', file)
  saving.value = true
  try {
    const res = await api.create(props.ownerId, form)
    if (res.yaml_warning) yamlWarning.value = res.yaml_warning
    Notify.success('新版本已创建并切换为当前')
    createOpen.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    saving.value = false
  }
  return false // 拦截默认上传
}

// 文本编辑保存前 YAML 语法提示（后端返回 yaml_warning，a-alert warning 展示不阻断）
async function saveText() {
  saving.value = true
  try {
    const res = await api.create(props.ownerId, { text: editText.value })
    if (res.yaml_warning) {
      yamlWarning.value = res.yaml_warning
      Notify.warning(res.yaml_warning)
    }
    Notify.success('新版本已创建并切换为当前')
    createOpen.value = false
    editText.value = ''
    yamlWarning.value = ''
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    saving.value = false
  }
}

async function doSwitch() {
  if (toSwitch.value === null) return
  switching.value = true
  try {
    await api.switchCurrent(props.ownerId, toSwitch.value)
    Notify.success('当前版本已切换')
    toSwitch.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    switching.value = false
  }
}

async function doDelete() {
  if (toDelete.value === null) return
  deleting.value = true
  try {
    await api.remove(props.ownerId, toDelete.value)
    Notify.success('版本已删除')
    toDelete.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message) // 「不可删当前激活版本，请先切换」等
  } finally {
    deleting.value = false
  }
}

async function doPreview(ver: number) {
  previewing.value = true
  try {
    previewContent.value = await api.preview(props.ownerId, ver)
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    previewing.value = false
  }
}

function fmtTime(ts: string): string {
  return dayjs(ts).format('YYYY-MM-DD HH:mm') // 后端 UTC → 本地时区
}
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-4">
      <div class="flex items-center gap-1">
        <Button v-if="backPath" type="text" class="-ml-2" @click="goBack">
          <template #icon><ArrowLeftOutlined /></template>
          返回
        </Button>
        <h2 class="text-lg font-semibold m-0">版本管理{{ resourceName ? `（${resourceName}）` : '' }}</h2>
      </div>
      <Button type="primary" @click="createOpen = true">创建新版本</Button>
    </div>

    <TriStateList :loading="loading" :empty="versions.length === 0" empty-text="暂无版本，请创建第一个版本">
      <Table :data-source="versions" :pagination="false" row-key="version_no" size="middle">
        <Table.Column key="no" title="版本号" width="90">
          <template #default="{ record }">
            <Space>
              <TypographyText code>v{{ record.version_no }}</TypographyText>
              <Tag v-if="record.current" color="green">当前</Tag>
            </Space>
          </template>
        </Table.Column>
        <Table.Column key="created" title="创建时间" width="160">
          <template #default="{ record }">{{ fmtTime(record.created_at) }}</template>
        </Table.Column>
        <Table.Column key="updated" title="更新时间" width="160">
          <template #default="{ record }">{{ fmtTime(record.updated_at) }}</template>
        </Table.Column>
        <Table.Column key="actions" title="操作" width="220">
          <template #default="{ record }">
            <Space>
              <Button size="small" @click="doPreview(record.version_no)">预览</Button>
              <Button v-if="!record.current" size="small" @click="toSwitch = record.version_no">设为当前</Button>
              <Tooltip v-else title="当前激活版本不可删除，请先切换">
                <Button size="small" danger disabled>删除</Button>
              </Tooltip>
              <Button v-if="!record.current" size="small" danger @click="toDelete = record.version_no">删除</Button>
            </Space>
          </template>
        </Table.Column>
      </Table>
    </TriStateList>

    <!-- 创建新版本弹窗：文件上传 / 在线文本编辑双页签 -->
    <Modal v-model:open="createOpen" title="创建新版本" :footer="null" :width="560">
      <Tabs v-model:activeKey="createMode">
        <Tabs.TabPane key="upload" tab="文件上传">
          <Upload :show-upload-list="false" :before-upload="onUpload" :disabled="saving">
            <Button :loading="saving">选择文件上传（≤50MB）</Button>
          </Upload>
        </Tabs.TabPane>
        <Tabs.TabPane key="text" tab="在线编辑">
          <Alert v-if="yamlWarning" type="warning" :message="yamlWarning" class="mb-2" />
          <Input.TextArea v-model:value="editText" :rows="14" placeholder="粘贴订阅/规则内容（YAML 语法问题仅提示，不阻断保存）" />
          <Button type="primary" class="mt-2" :loading="saving" @click="saveText">保存为新版本</Button>
        </Tabs.TabPane>
      </Tabs>
    </Modal>

    <!-- 预览弹窗：宽屏纯文本（禁 HTML） -->
    <Modal :open="previewContent !== null" title="内容预览"
           width="80%" :footer="null" :loading="previewing" @cancel="previewContent = null">
      <pre class="text-xs overflow-auto max-h-[70vh] whitespace-pre-wrap break-all">{{ previewContent }}</pre>
    </Modal>

    <ConfirmModal :open="toDelete !== null" title="删除版本" danger :loading="deleting"
                  content="删除后不可恢复" @confirm="doDelete" @update:open="toDelete = null" />
    <ConfirmModal :open="toSwitch !== null" title="切换当前版本" :loading="switching"
                  content="切换后所有下载立即生效" @confirm="doSwitch" @update:open="toSwitch = null" />
  </div>
</template>
