<!-- AdminOverviewView.vue：管理员概览，严格只读取一个 /admin/overview 汇总接口。 -->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Button, Card, List, Result, Space, Tag } from 'ant-design-vue'
import { CheckCircleOutlined, ClockCircleOutlined, ReloadOutlined, RightOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import { getAdminOverview, type AdminOverview, type OverviewChecklistItem } from '@/api/overview'
import PageShell from '@/components/PageShell.vue'

const overview = ref<AdminOverview | null>(null)
const loading = ref(false)
const error = ref('')

const counts = computed(() => overview.value?.counts)
const quickLinks = computed(() => {
  const base = [
    { label: '平台', value: counts.value?.platforms ?? 0, path: '/admin/platforms' },
    { label: '订阅', value: counts.value?.subscriptions ?? 0, path: '/admin/subscriptions' },
    { label: '可用节点', value: counts.value?.usable_nodes ?? 0, path: '/admin/nodes' },
    { label: '用户', value: counts.value?.users ?? 0, path: '/admin/users' },
  ]
  if (overview.value?.status.advanced_mode) {
    base.push({ label: 'Xray 实例', value: counts.value?.xray_instances ?? 0, path: '/admin/xray' })
  }
  return base
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    overview.value = await getAdminOverview()
  } catch (err) {
    error.value = (err as Error).message
  } finally {
    loading.value = false
  }
}

function itemIcon(item: OverviewChecklistItem) {
  return item.done ? CheckCircleOutlined : ClockCircleOutlined
}

function itemColor(item: OverviewChecklistItem) {
  if (item.done) return 'text-green-600'
  return item.manual ? 'text-amber-500' : 'text-gray-400'
}

function fmtTime(value: string) {
  return dayjs(value).format('MM-DD HH:mm')
}

onMounted(() => { void load() })
</script>

<template>
  <PageShell title="概览" description="查看服务状态、首次发布进度与近期管理活动。" :loading="loading" :error="error"
             @retry="load">
    <template #actions>
      <Button :loading="loading" @click="load"><template #icon><ReloadOutlined /></template>刷新</Button>
    </template>

    <template v-if="overview">
      <div class="grid gap-4 xl:grid-cols-[1.15fr_0.85fr]">
        <Card title="服务状态" size="small">
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
            <div>
              <p class="metric-label">运行模式</p>
              <Tag :color="overview.status.app_mode === 'prod' ? 'green' : 'orange'">
                {{ overview.status.app_mode === 'prod' ? 'Production' : 'Dev' }}
              </Tag>
            </div>
            <div>
              <p class="metric-label">高级模式</p>
              <Tag :color="overview.status.advanced_mode ? 'green' : 'default'">
                {{ overview.status.advanced_mode ? '已开启' : '未开启' }}
              </Tag>
            </div>
            <div>
              <p class="metric-label">应急状态</p>
              <Tag :color="overview.status.emergency ? 'red' : 'green'">
                {{ overview.status.emergency ? '应急中' : '正常' }}
              </Tag>
            </div>
          </div>
        </Card>

        <Card title="首次发布清单" size="small">
          <List size="small" :data-source="overview.checklist">
            <template #renderItem="{ item }">
              <List.Item class="!px-0">
                <div class="flex min-w-0 flex-1 items-center gap-2">
                  <component :is="itemIcon(item)" :class="itemColor(item)" />
                  <span class="flex-1 text-sm" :class="item.done ? 'text-gray-500 line-through dark:text-gray-400' : ''">{{ item.label }}</span>
                  <Tag v-if="item.manual" color="gold">人工</Tag>
                  <Button v-if="!item.done" type="link" size="small" @click="$router.push(item.action_path)">
                    {{ item.action_label }}
                  </Button>
                </div>
              </List.Item>
            </template>
          </List>
        </Card>
      </div>

      <section class="mt-4">
        <h2 class="section-title">资源概况</h2>
        <div class="grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-5">
          <button v-for="link in quickLinks" :key="link.path" type="button" class="overview-metric" @click="$router.push(link.path)">
            <span class="text-sm text-gray-500 dark:text-gray-400">{{ link.label }}</span>
            <strong>{{ link.value }}</strong>
            <RightOutlined class="text-xs text-gray-400" />
          </button>
        </div>
      </section>

      <div class="mt-4 grid gap-4 xl:grid-cols-2">
        <Card title="最近待审批" size="small">
          <template v-if="overview.recent.pending_users.length">
            <List size="small" :data-source="overview.recent.pending_users">
              <template #renderItem="{ item }">
                <List.Item class="!px-0">
                  <div class="min-w-0"><strong class="mr-2">{{ item.username }}</strong><span class="text-sm text-gray-500">{{ item.email || '未提供邮箱' }}</span></div>
                  <span class="text-xs text-gray-400">{{ fmtTime(item.created_at) }}</span>
                </List.Item>
              </template>
            </List>
          </template>
          <Result v-else status="info" title="暂无待审批用户" sub-title="新注册或 OIDC 待审批用户会显示在这里。" />
          <template #actions><Button type="link" @click="$router.push('/admin/approvals')">前往审批中心</Button></template>
        </Card>

        <Card title="最近访问日志" size="small">
          <template v-if="overview.recent.access_logs.length">
            <List size="small" :data-source="overview.recent.access_logs">
              <template #renderItem="{ item }">
                <List.Item class="!px-0">
                  <div class="min-w-0 truncate"><strong class="mr-2">{{ item.username || '匿名' }}</strong><span class="text-sm text-gray-500">{{ item.resource_name || item.resource_slug || '资源访问' }}</span></div>
                  <Space size="small"><Tag :color="item.status === 'success' ? 'green' : 'red'">{{ item.status === 'success' ? '成功' : '失败' }}</Tag><span class="text-xs text-gray-400">{{ fmtTime(item.created_at) }}</span></Space>
                </List.Item>
              </template>
            </List>
          </template>
          <Result v-else status="info" title="暂无访问日志" sub-title="用户下载订阅或规则后，最近记录会显示在这里。" />
          <template #actions><Button type="link" @click="$router.push('/admin/logs')">查看全部日志</Button></template>
        </Card>
      </div>
    </template>
  </PageShell>
</template>

<style scoped>
.metric-label { margin: 0 0 .35rem; color: rgb(107 114 128); font-size: .75rem; }
.section-title { margin: 0 0 .75rem; font-size: 1rem; font-weight: 600; }
.overview-metric { min-height: 5.5rem; display: grid; grid-template-columns: 1fr auto; align-items: center; gap: .25rem; text-align: left; border: 1px solid rgb(229 231 235); border-radius: .5rem; padding: .75rem; background: transparent; cursor: pointer; }
.overview-metric:hover { border-color: rgb(96 165 250); background: rgb(239 246 255); }
.overview-metric strong { grid-column: 1; font-size: 1.5rem; line-height: 1; }
.overview-metric :last-child { grid-column: 2; grid-row: 1 / span 2; }
</style>
