<!-- ApprovalsView.vue：审批中心（UI §5.6，Design1 §3.4.6）——双态列表 + 批量选择 + 通过/拒绝 -->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import dayjs from 'dayjs'
import { Button, Checkbox, Pagination, Space, Table, Tag, TypographyParagraph } from 'ant-design-vue'
import { listApprovals, approve, reject, batchApproveApi, type PendingUser } from '@/api/approval'
import ConfirmModal from '@/components/ConfirmModal.vue'
import PageHeader from '@/components/PageHeader.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'

// 响应式：≥768 表格 / <768 卡片
const isMobile = ref(false)
function checkMobile() {
  isMobile.value = window.matchMedia('(max-width: 767px)').matches
}
onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})
onUnmounted(() => window.removeEventListener('resize', checkMobile))

// --- 列表（后端分页） ---
const loading = ref(false)
const list = ref<PendingUser[]>([])
const total = ref(0)
const page = ref(1)
const size = ref(20)
async function load() {
  loading.value = true
  try {
    const res = await listApprovals({ page: page.value, size: size.value })
    list.value = res.list
    total.value = res.total
    // 清理已被处理（他端操作）的选中项
    const ids = new Set(res.list.map((u) => u.id))
    selected.value = selected.value.filter((id) => ids.has(id))
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(load)

// 申请时间 UTC → 本地化展示
function fmtTime(t: string) {
  return t ? dayjs(t).format('YYYY-MM-DD HH:mm') : '—'
}

// --- 批量选择 ---
const selected = ref<number[]>([])
const allChecked = computed(() => list.value.length > 0 && selected.value.length === list.value.length)
function toggleAll(checked: boolean) {
  selected.value = checked ? list.value.map((u) => u.id) : []
}
function toggleOne(id: number, checked: boolean) {
  if (checked) {
    if (!selected.value.includes(id)) selected.value.push(id)
  } else {
    selected.value = selected.value.filter((x) => x !== id)
  }
}

// --- 行操作：通过 / 拒绝（ConfirmModal） ---
const operating = ref(false)
async function approveOne(u: PendingUser) {
  operating.value = true
  try {
    await approve(u.id)
    Notify.success('已通过，欢迎邮件按配置发送')
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    operating.value = false
  }
}
const rejectTarget = ref<PendingUser | null>(null)
const rejectContent = computed(() =>
  rejectTarget.value ? `账号「${rejectTarget.value.username}」将删除、邮箱释放可重新注册，拒绝通知按配置发送` : '',
)
async function confirmReject() {
  if (!rejectTarget.value) return
  operating.value = true
  try {
    await reject(rejectTarget.value.id)
    Notify.success('已拒绝，账号已删除')
    rejectTarget.value = null
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    operating.value = false
  }
}

// --- 头部「批量通过」 ---
const batchOpen = ref(false)
const batchLoading = ref(false)
const batchContent = computed(() => `确定通过选中的 ${selected.value.length} 条申请？`)
async function confirmBatch() {
  if (selected.value.length === 0) return
  batchLoading.value = true
  try {
    const res = await batchApproveApi(selected.value)
    Notify.success(`通过 ${res.succeeded} 条${res.failed ? `，失败 ${res.failed} 条` : ''}`)
    selected.value = []
    batchOpen.value = false
    await load()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    batchLoading.value = false
  }
}

// claims 展开（JSON 只读）
const expandedClaims = ref<Record<number, boolean>>({})
</script>

<template>
  <div>
    <PageHeader title="审批中心">
      <template #actions>
        <Space>
          <Button type="primary" :disabled="selected.length === 0" @click="batchOpen = true">
            批量通过{{ selected.length ? `（${selected.length}）` : '' }}
          </Button>
        </Space>
      </template>
    </PageHeader>

    <TriStateList :loading="loading" :empty="list.length === 0 && total === 0" empty-text="暂无待审批用户">
      <!-- ≥768 表格态 -->
      <template v-if="!isMobile">
        <Table :data-source="list" row-key="id" :pagination="false">
          <Table.Column key="check" title="" width="40">
            <template #header>
              <Checkbox :checked="allChecked" @change="(e: any) => toggleAll(e.target.checked)" />
            </template>
            <template #default="{ record }">
              <Checkbox :checked="selected.includes(record.id)" @change="(e: any) => toggleOne(record.id, e.target.checked)" />
            </template>
          </Table.Column>
          <Table.Column key="source" title="来源" width="90">
            <template #default="{ record }">
              <Tag :color="record.source === 'oidc' ? 'geekblue' : 'cyan'">
                {{ record.source === 'oidc' ? 'OIDC' : '自注册' }}
              </Tag>
            </template>
          </Table.Column>
          <Table.Column key="username" title="用户名" data-index="username" />
          <Table.Column key="email" title="邮箱" data-index="email">
            <template #default="{ record }">{{ record.email || '—' }}</template>
          </Table.Column>
          <Table.Column key="created" title="申请时间" width="150">
            <template #default="{ record }">{{ fmtTime(record.created_at) }}</template>
          </Table.Column>
          <Table.Column key="claims" title="OIDC Claims" width="140">
            <template #default="{ record }">
              <template v-if="record.oidc_claims">
                <TypographyParagraph v-if="expandedClaims[record.id]" class="mb-0" :code="true" style="white-space: pre-wrap">
                  {{ record.oidc_claims }}
                </TypographyParagraph>
                <Button v-else size="small" type="link" @click="expandedClaims[record.id] = true">展开</Button>
              </template>
              <span v-else>—</span>
            </template>
          </Table.Column>
          <Table.Column key="actions" title="操作" width="140">
            <template #default="{ record }">
              <Space>
                <Button size="small" type="primary" ghost @click="approveOne(record)">通过</Button>
                <Button size="small" danger @click="rejectTarget = record">拒绝</Button>
              </Space>
            </template>
          </Table.Column>
        </Table>
      </template>
      <!-- <768 卡片态 -->
      <template v-else>
        <div class="space-y-3">
          <div v-for="u in list" :key="u.id" class="border rounded-lg p-3">
            <div class="flex items-center justify-between">
              <Space>
                <Checkbox :checked="selected.includes(u.id)" @change="(e: any) => toggleOne(u.id, e.target.checked)" />
                <span class="font-medium">{{ u.username }}</span>
                <Tag :color="u.source === 'oidc' ? 'geekblue' : 'cyan'">{{ u.source === 'oidc' ? 'OIDC' : '自注册' }}</Tag>
              </Space>
              <Space>
                <Button size="small" type="primary" ghost @click="approveOne(u)">通过</Button>
                <Button size="small" danger @click="rejectTarget = u">拒绝</Button>
              </Space>
            </div>
            <div class="text-sm text-gray-500 mt-1">
              {{ u.email || '无邮箱' }} · {{ fmtTime(u.created_at) }}
            </div>
          </div>
        </div>
      </template>
    </TriStateList>

    <div class="flex justify-end mt-3">
      <Pagination v-model:current="page" :page-size="size" :total="total" :show-total="(t: number) => `共 ${t} 条`" />
    </div>

    <!-- 拒绝确认 -->
    <ConfirmModal :open="!!rejectTarget" title="拒绝申请" danger :content="rejectContent" :loading="operating"
                  @confirm="confirmReject" @update:open="rejectTarget = null" />
    <!-- 批量通过确认 -->
    <ConfirmModal :open="batchOpen" title="批量通过" :content="batchContent" :loading="batchLoading"
                  @confirm="confirmBatch" @update:open="batchOpen = false" />
  </div>
</template>
