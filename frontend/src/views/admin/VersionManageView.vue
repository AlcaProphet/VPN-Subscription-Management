<!-- VersionManageView.vue：通用版本管理视图组件（四类资源复用，props 驱动，UI §5.1/7.1） -->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Button, Input, Menu, Space, Spin, Table, Tabs, Tag, Tooltip, TypographyText, Upload, type MenuProps } from 'ant-design-vue'
import { ArrowLeftOutlined } from '@ant-design/icons-vue'
import AppDropdown from '@/components/AppDropdown.vue'
import dayjs from 'dayjs'
import { versionApi, getVersionOwner, type VersionItem, type VersionOwner } from '@/api/version'
import { getSubscription } from '@/api/subscription'
import ConfirmModal from '@/components/ConfirmModal.vue'
import FormOverlay from '@/components/FormOverlay.vue'
import PageHeader from '@/components/PageHeader.vue'
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

// 订阅地址池与规则使用「入池 + 显式分发」语义；分享/自定义保持「设为当前」
const activationOwner = computed(() => props.ownerType === 'subscription' || props.ownerType === 'rule')
const switchLabel = computed(() => (activationOwner.value ? '激活/分发' : '设为当前'))
const switchContent = computed(() =>
  activationOwner.value ? '激活后对全体用户生效' : '切换后所有下载立即生效',
)
// UI §10.2：版本列表空态引导仅订阅/规则页显示「装配生成」，分享/自定义不显示
const emptyText = computed(() =>
  activationOwner.value
    ? '暂无版本，可通过上传文件 / 在线编辑 / 装配生成创建'
    : '暂无版本，可通过上传文件 / 在线编辑创建',
)

// 装配生成入口：订阅/规则页显示；订阅页额外带 platform_id 与对应装配器页签
const assemblyUrl = ref('')
async function refreshAssemblyUrl() {
  if (props.ownerType === 'rule') {
    assemblyUrl.value = `/admin/assembly?tab=sr-conf&rule_id=${props.ownerId}`
    return
  }
  if (props.ownerType !== 'subscription') {
    assemblyUrl.value = ''
    return
  }
  try {
    const sub = await getSubscription(props.ownerId)
    const tab = sub.product_type === 'yaml' ? 'clash-yaml' : sub.product_type === 'subs' ? 'sr-subs' : 'generic-subs'
    assemblyUrl.value = `/admin/assembly?tab=${tab}&platform_id=${sub.platform_id}`
  } catch (err) {
    Notify.error((err as Error).message)
    assemblyUrl.value = '/admin/assembly'
  }
}

// 响应式：≥768 表格 / <768 卡片（与其他管理页一致）
const isMobile = ref(false)
function checkMobile() {
  isMobile.value = window.matchMedia('(max-width: 767px)').matches
}

// 返回上级列表页（如订阅管理）；custom 暂回订阅管理（用户管理在 Build3 接通后修正）
function goBack() {
  if (props.backPath) void router.push(props.backPath)
}

const api = versionApi(props.apiPrefix)
const loading = ref(false)
const versions = ref<VersionItem[]>([])
const createOpen = ref(false)
const createMode = ref<'upload' | 'text'>('upload')
const editOpen = ref(false)
const editTarget = ref<number | null>(null) // 正在编辑的版本号（编辑起点）
const editLoading = ref(false) // 拉取编辑起点内容中
const editText = ref('')
const saving = ref(false)
const owner = ref<VersionOwner | null>(null)
const previewOpen = ref(false) // 预览弹窗独立开关：点击立即打开显示加载态
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
    // 有版本时优先以版本 ID 反查真实资源名；空列表不额外请求，避免无意义 404。
    owner.value = versions.value.length > 0 ? await getVersionOwner(versions.value[0].id) : null
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}

const fallbackTitle = computed(() => {
  const labels = { subscription: '订阅', rule: '规则', share: '分享', custom: '自定义订阅' } as const
  return `${labels[props.ownerType]} #${props.ownerId} · 版本管理`
})
const pageTitle = computed(() => owner.value ? `${owner.value.name} · 版本管理` : fallbackTitle.value)
const pageSubtitle = computed(() => owner.value ? `${owner.value.type_label}资源的版本、预览与激活管理。` : '正在读取资源名称；无版本资源将使用安全的类型与编号回退。')
onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  void load()
  void refreshAssemblyUrl()
})
onUnmounted(() => window.removeEventListener('resize', checkMobile))

// 移动端卡片「更多 ▾」菜单：编辑 / 删除（当前激活版本禁删）
function cardMenuItems(v: VersionItem) {
  return [
    { key: 'edit', label: '编辑' },
    {
      key: 'delete', label: '删除', danger: true, disabled: !!v.current,
      title: v.current ? '当前激活版本不可删除，请先切换' : '',
    },
  ] as MenuProps['items']
}
function onCardMenuClick(key: string, v: VersionItem) {
  if (key === 'edit') void openEdit(v.version_no)
  if (key === 'delete') toDelete.value = v.version_no
}

// 创建新版本：文件上传（自动提交新版本；前端 50MB 预校验对齐后端 MaxContentSize，AGENTS §4.1 双重校验）
async function onUpload(file: File) {
  if (file.size > 50 << 20) {
    Notify.error('文件超过 50MB 限制')
    return false
  }
  const form = new FormData()
  form.append('file', file)
  saving.value = true
  try {
    const res = await api.create(props.ownerId, form)
    if (res.auto_activated) {
      Notify.success('首个版本已自动激活')
    } else if (activationOwner.value) {
      if (props.ownerType === 'subscription') sessionStorage.setItem(`pooled_sub_${props.ownerId}`, '1')
      Notify.success('已入池未生效，请激活')
    } else {
      Notify.success('新版本已创建并切换为当前')
    }
    createOpen.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    saving.value = false
  }
  return false // 拦截默认上传
}

// 打开创建弹窗（在线编辑页签从空白起点开始）
function openCreate() {
  createMode.value = 'upload'
  editText.value = ''
  createOpen.value = true
}

// 按版本在线编辑：拉取所选版本内容预填 → 修改 → 保存为新版本（原版本保留不覆盖）
async function openEdit(ver: number) {
  editTarget.value = ver
  editText.value = ''
  editLoading.value = true
  editOpen.value = true
  try {
    editText.value = await api.preview(props.ownerId, ver) // 复用指定版本预览端点取内容
  } catch (err) {
    Notify.error((err as Error).message)
    editOpen.value = false
    editTarget.value = null
  } finally {
    editLoading.value = false
  }
}

// 文本保存（创建弹窗在线编辑页签 / 按版本编辑弹窗共用）：保存为新版本并切换为当前
async function saveText() {
  saving.value = true
  try {
    const res = await api.create(props.ownerId, { text: editText.value })
    if (res.auto_activated) {
      Notify.success('首个版本已自动激活')
    } else if (activationOwner.value) {
      if (props.ownerType === 'subscription') sessionStorage.setItem(`pooled_sub_${props.ownerId}`, '1')
      Notify.success('已入池未生效，请激活')
    } else {
      Notify.success('新版本已创建并切换为当前')
    }
    createOpen.value = false
    editOpen.value = false
    editTarget.value = null
    editText.value = ''
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

// 预览：先开弹窗显示 Spin 占位，内容返回后渲染（与编辑弹窗加载样式一致，避免点击后无反馈）
async function doPreview(ver: number) {
  previewing.value = true
  previewOpen.value = true
  previewContent.value = null
  try {
    previewContent.value = await api.preview(props.ownerId, ver)
  } catch (err) {
    Notify.error((err as Error).message)
    previewOpen.value = false
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
    <PageHeader :title="pageTitle" :subtitle="pageSubtitle">
      <template #actions>
        <Space>
          <Tag v-if="owner && ownerType !== 'subscription'" color="blue">{{ owner.type_label }}</Tag>
          <Button v-if="backPath" type="text" class="-ml-2" @click="goBack">
            <template #icon><ArrowLeftOutlined /></template>
            返回
          </Button>
          <Button type="primary" @click="openCreate">创建新版本</Button>
        </Space>
      </template>
    </PageHeader>

    <TriStateList :loading="loading" :empty="versions.length === 0"
                  :empty-text="emptyText">
      <!-- ≥768 表格态 -->
      <template v-if="!isMobile">
        <Table :data-source="versions" :pagination="false" row-key="version_no" size="middle">
          <Table.Column key="no" title="版本号" width="90">
            <template #default="{ record }">
              <Space>
                <TypographyText code>v{{ record.version_no }}</TypographyText>
                <Tag v-if="record.current" color="green">当前</Tag>
                  <Tag v-if="record.blueprint" color="purple">装配</Tag>
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
                <Button size="small" @click="openEdit(record.version_no)">编辑</Button>
                <Button v-if="!record.current" size="small" @click="toSwitch = record.version_no">{{ switchLabel }}</Button>
                <Tooltip v-else title="当前激活版本不可删除，请先切换">
                  <Button size="small" danger disabled>删除</Button>
                </Tooltip>
                <Button v-if="!record.current" size="small" danger @click="toDelete = record.version_no">删除</Button>
              </Space>
            </template>
          </Table.Column>
        </Table>
      </template>
      <!-- <768 卡片态：预览/设为当前直显，编辑/删除进「更多 ▾」 -->
      <template v-else>
        <div class="space-y-3">
          <div v-for="v in versions" :key="v.version_no" class="mobile-actions border rounded-lg p-3">
            <div class="flex items-center justify-between gap-2">
              <Space>
                <TypographyText code>v{{ v.version_no }}</TypographyText>
                <Tag v-if="v.current" color="green">当前</Tag>
                <Tag v-if="v.blueprint" color="purple">装配</Tag>
              </Space>
              <Space>
                <Button size="small" @click="doPreview(v.version_no)">预览</Button>
                <Button v-if="!v.current" size="small" @click="toSwitch = v.version_no">{{ switchLabel }}</Button>
                <AppDropdown>
                  <Button size="small">更多 ▾</Button>
                  <template #overlay>
                    <Menu :items="cardMenuItems(v)" @click="(e: any) => onCardMenuClick(e.key, v)" />
                  </template>
                </AppDropdown>
              </Space>
            </div>
            <div class="text-sm text-text-secondary mt-1">
              创建 {{ fmtTime(v.created_at) }} · 更新 {{ fmtTime(v.updated_at) }}
            </div>
          </div>
        </div>
      </template>
    </TriStateList>

    <!-- 创建新版本弹窗：文件上传 / 在线文本编辑双页签（在线编辑从空白起点开始） -->
    <FormOverlay v-model:open="createOpen" title="创建新版本" :width="560" :loading="saving">
      <Tabs v-model:activeKey="createMode">
        <template #tabBarExtraContent>
          <Button v-if="assemblyUrl" size="small" @click="router.push(assemblyUrl)">装配生成</Button>
        </template>
        <Tabs.TabPane key="upload" tab="文件上传">
          <Upload :show-upload-list="false" :before-upload="onUpload" :disabled="saving">
            <Button :loading="saving">选择文件上传（≤50MB）</Button>
          </Upload>
        </Tabs.TabPane>
        <Tabs.TabPane key="text" tab="在线编辑">
          <Input.TextArea v-model:value="editText" :rows="14" placeholder="粘贴订阅/规则内容，保存后将创建为新版本" />
        </Tabs.TabPane>
      </Tabs>
      <template #footer>
        <Button class="touch-target" @click="createOpen = false">取消</Button>
        <Button v-if="createMode === 'text'" type="primary" class="touch-target" :loading="saving" @click="saveText">保存为新版本</Button>
      </template>
    </FormOverlay>

    <!-- 按版本在线编辑弹窗：预填所选版本内容，修改后保存为新版本（原版本保留不覆盖）；
         加载中 Spin 占位，完成后一次性渲染编辑区（避免空白可输入框闪烁） -->
    <FormOverlay :open="editOpen" :title="editTarget !== null ? `编辑版本 v${editTarget}` : '在线编辑'"
                 :width="720" :loading="saving" @update:open="editOpen = false; editTarget = null">
      <div v-if="editLoading" class="py-12 text-center">
        <Spin size="large" />
        <div class="mt-2 text-text-secondary">加载版本内容中…</div>
      </div>
      <template v-else>
        <Input.TextArea v-model:value="editText" :rows="14"
                        placeholder="基于所选版本内容修改，保存后将创建为新版本" />
      </template>
      <template #footer>
        <Button class="touch-target" @click="editOpen = false; editTarget = null">取消</Button>
        <Button v-if="!editLoading" type="primary" class="touch-target" :loading="saving" @click="saveText">保存为新版本</Button>
      </template>
    </FormOverlay>

    <!-- 预览弹窗：宽屏纯文本（禁 HTML）；加载中 Spin 占位，完成后一次性渲染内容 -->
    <FormOverlay :open="previewOpen" title="内容预览" width="80%" @update:open="previewOpen = false; previewContent = null">
      <div v-if="previewing" class="py-12 text-center">
        <Spin size="large" />
        <div class="mt-2 text-text-secondary">加载内容中…</div>
      </div>
      <pre v-else class="text-xs overflow-auto max-h-[70vh] whitespace-pre-wrap break-all">{{ previewContent }}</pre>
      <template #footer>
        <Button type="primary" class="touch-target" @click="previewOpen = false; previewContent = null">关闭</Button>
      </template>
    </FormOverlay>

    <ConfirmModal :open="toDelete !== null" title="删除版本" danger :loading="deleting"
                  content="删除后不可恢复" @confirm="doDelete" @update:open="toDelete = null" />
    <ConfirmModal :open="toSwitch !== null" title="切换当前版本" :loading="switching"
                  :content="switchContent" @confirm="doSwitch" @update:open="toSwitch = null" />
  </div>
</template>
