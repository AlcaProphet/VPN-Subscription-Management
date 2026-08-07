<!-- SubscriptionsView.vue：订阅池管理（UI §5.1）——按平台分组，双态行 -->
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Alert, Button, Collapse, Input, Modal, Select, Space, Tag, TypographyText } from 'ant-design-vue'
import { listSubscriptions, createSubscription, updateSubscription, deleteSubscription, checkSlug, type PlatformSubs, type SubscriptionItem } from '@/api/subscription'
import { listPlatforms, type PlatformItem } from '@/api/platform'
import { listGroups } from '@/api/group'
import ConfirmModal from '@/components/ConfirmModal.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'

const router = useRouter()
const loading = ref(false)
const groups = ref<PlatformSubs[]>([])
const platforms = ref<PlatformItem[]>([])

async function load() {
  loading.value = true
  try {
    const [g, p, grps] = await Promise.all([listSubscriptions(), listPlatforms(), listGroups()])
    groups.value = g
    platforms.value = p
    groupOptions.value = grps.map((x) => ({ label: x.name, value: x.id })) // 关联组多选数据源（Step 3 接通）
    openPlatforms.value = g.map((pg) => pg.platform_id) // 默认全展开
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}

// 平台分组展开控制（默认全展开）
const openPlatforms = ref<number[]>([])
function onCollapseChange(keys: (string | number) | (string | number)[]) {
  openPlatforms.value = Array.isArray(keys) ? keys.map(Number) : [Number(keys)]
}
onMounted(load)

// 版本跳转辅助（版本管理页复用 VersionManageView）
function goVersions(sub: SubscriptionItem) {
  void router.push(`/admin/subscriptions/${sub.id}/versions`)
}

// --- 新建/编辑弹窗 ---
const modalOpen = ref(false)
const editing = ref<SubscriptionItem | null>(null) // null = 新建
const saving = ref(false)
const form = reactive({ platform_id: 0, name: '', slug: '', group_ids: [] as number[] })

// 关联组多选数据源（组列表接口 Build2 Step 3 已建立）
const groupOptions = ref<{ label: string; value: number }[]>([])

// 标识即时校验：格式 + 防抖唯一性提示（调 /api/admin/slug/check）；success 态用提示文字呈现，status 仅 error/warning
const slugStatus = ref<'' | 'error' | 'warning'>('')
const slugTip = ref('')
let slugTimer: ReturnType<typeof setTimeout> | undefined
const slugRe = /^[a-z0-9-]{3,64}$/
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
      const res = await checkSlug(form.slug, 'subscription', editing.value?.id)
      slugStatus.value = res.available ? '' : 'error'
      slugTip.value = res.available ? '标识可用' : '标识已被使用（四类资源全局唯一）'
    } catch {
      slugStatus.value = ''
      slugTip.value = ''
    }
  }, 300)
}

function openCreate() {
  editing.value = null
  form.platform_id = platforms.value[0]?.id ?? 0
  form.name = ''
  form.slug = ''
  form.group_ids = []
  slugStatus.value = ''
  slugTip.value = ''
  modalOpen.value = true
}
function openEdit(sub: SubscriptionItem) {
  editing.value = sub
  form.platform_id = sub.platform_id
  form.name = sub.name
  form.slug = sub.slug
  form.group_ids = sub.groups.map((g) => g.id)
  slugStatus.value = ''
  slugTip.value = ''
  modalOpen.value = true
}

async function save() {
  if (!form.name.trim() || form.platform_id <= 0) {
    Notify.error('请填写名称并选择平台')
    return
  }
  if (slugStatus.value === 'error') {
    Notify.error('标识不可用，请更换')
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await updateSubscription(editing.value.id, { name: form.name.trim(), group_ids: form.group_ids })
      Notify.success('订阅已更新')
    } else {
      if (!slugRe.test(form.slug)) {
        Notify.error('标识须为小写字母数字连字符，长度 3~64')
        return
      }
      await createSubscription({ platform_id: form.platform_id, name: form.name.trim(), slug: form.slug, group_ids: form.group_ids })
      Notify.success('订阅已创建')
      guideOpen.value = true // 创建成功引导（Step 3 接通「每平台选定」直达）
    }
    modalOpen.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    saving.value = false
  }
}

// --- 删除 ---
const toDelete = ref<SubscriptionItem | null>(null)
const deleting = ref(false)
async function confirmDelete() {
  if (!toDelete.value) return
  deleting.value = true
  try {
    await deleteSubscription(toDelete.value.id)
    Notify.success('订阅已删除')
    toDelete.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    deleting.value = false
  }
}

// 创建成功引导：关联用户组 → 每平台选定（组管理页在 Step 3 建立，本 Step 仅提示）
const guideOpen = ref(false)
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-semibold m-0">订阅管理</h2>
      <Button type="primary" @click="openCreate">新建订阅</Button>
    </div>

    <TriStateList :loading="loading" :empty="groups.length === 0" empty-text="还没有订阅">
      <Collapse v-model:active-key="openPlatforms" class="bg-transparent" @change="onCollapseChange">
        <Collapse.Panel v-for="g in groups" :key="g.platform_id" :header="`${g.platform_name}（${g.subscriptions.length}）`">
          <div class="space-y-2">
            <div v-for="sub in g.subscriptions" :key="sub.id"
                 class="flex flex-wrap items-center gap-2 border rounded-lg p-3 bg-white dark:bg-gray-800">
              <span class="font-medium">{{ sub.name }}</span>
              <TypographyText code class="text-xs">{{ sub.slug }}</TypographyText>
              <Tag v-if="sub.current_version > 0" color="green">v{{ sub.current_version }}</Tag>
              <Tag v-else color="default">无版本</Tag>
              <Tag v-for="grp in sub.groups" :key="grp.id" color="blue">{{ grp.name }}</Tag>
              <Tag v-if="sub.selected_by > 0" color="cyan">被 {{ sub.selected_by }} 个组选定中</Tag>
              <Space class="ml-auto">
                <Button size="small" @click="goVersions(sub)">版本管理</Button>
                <Button size="small" @click="openEdit(sub)">编辑</Button>
                <Button size="small" danger @click="toDelete = sub">删除</Button>
              </Space>
            </div>
          </div>
        </Collapse.Panel>
      </Collapse>
    </TriStateList>

    <!-- 新建/编辑弹窗 -->
    <Modal v-model:open="modalOpen" :title="editing ? '编辑订阅' : '新建订阅'" :footer="null" :width="520"
           destroy-on-close>
      <Alert v-if="!editing" type="info" show-icon class="mb-3"
             message="标识为四类资源（订阅/分享/规则/自定义）全局唯一命名空间，创建后不可修改" />
      <div class="space-y-4">
        <div>
          <div class="mb-1 text-sm">平台</div>
          <Select v-model:value="form.platform_id" class="w-full" :disabled="!!editing" placeholder="选择平台">
            <Select.Option v-for="p in platforms" :key="p.id" :value="p.id">{{ p.name }}</Select.Option>
          </Select>
          <div v-if="editing" class="text-xs text-gray-400 mt-1">平台创建后不可修改</div>
        </div>
        <div>
          <div class="mb-1 text-sm">名称</div>
          <Input v-model:value="form.name" :maxlength="100" placeholder="订阅名称（不强制唯一）" />
        </div>
        <div v-if="!editing">
          <div class="mb-1 text-sm">标识</div>
          <Input v-model:value="form.slug" :status="slugStatus || undefined" placeholder="小写字母数字连字符，3~64"
                 @change="onSlugChange" />
          <div v-if="slugTip" class="text-xs mt-1" :class="slugStatus === 'error' ? 'text-red-500' : 'text-green-600'">
            {{ slugTip }}
          </div>
        </div>
        <div v-else>
          <div class="mb-1 text-sm">标识</div>
          <TypographyText code>{{ editing.slug }}</TypographyText>
        </div>
        <div>
          <div class="mb-1 text-sm">关联用户组</div>
          <!-- 组数据源在 Build2 Step 3 接通（/api/admin/groups），本 Step 先渲染占位 -->
          <Select v-model:value="form.group_ids" class="w-full" mode="multiple" :options="groupOptions"
                  placeholder="选择关联的用户组（可空）" />
        </div>
        <div class="flex justify-end gap-2">
          <Button @click="modalOpen = false">取消</Button>
          <Button type="primary" :loading="saving" @click="save">{{ editing ? '保存' : '创建' }}</Button>
        </div>
      </div>
    </Modal>

    <!-- 创建成功引导：关联用户组 → 每平台选定（Step 3 接通组管理直达按钮） -->
    <ConfirmModal v-model:open="guideOpen" title="订阅已创建"
                  content="请前往用户组管理：① 为组关联该订阅；② 按平台选定该订阅，组内用户即可通过无标识链接获取内容。"
                  :ok-button-props="{ danger: false }" @confirm="guideOpen = false" />

    <!-- 删除确认 -->
    <ConfirmModal :open="toDelete !== null" title="删除订阅" danger :loading="deleting"
                  content="将删除全部版本文件及相关关联，删除后不可恢复" @confirm="confirmDelete" @update:open="toDelete = null" />
  </div>
</template>
