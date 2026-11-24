<!-- ProviderTrafficCard.vue：基础模式首页服务器剩余流量卡片（Build41 Step 5 / Design7 §四）。 -->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Alert, Card } from 'ant-design-vue'
import dayjs from 'dayjs'
import {
  getHomeProviderTraffic,
  PROVIDER_TRAFFIC_LABELS,
  PROVIDER_TRAFFIC_POLL_INTERVAL_MS,
  PROVIDER_TRAFFIC_PROVIDERS,
  type Provider,
  type ProviderTrafficResult,
} from '@/api/providerTraffic'

const items = ref<ProviderTrafficResult[]>([])
const loadError = ref('')
let timer: number | undefined
let requestSeq = 0
let active = false

const visible = computed(() => loadError.value !== '' || items.value.length > 0)
const rows = computed(() => {
  const order = new Map<Provider, number>(PROVIDER_TRAFFIC_PROVIDERS.map((provider, index) => [provider, index]))
  return [...items.value].sort((a, b) => (order.get(a.provider) ?? 99) - (order.get(b.provider) ?? 99))
})

function label(provider: Provider): string {
  return PROVIDER_TRAFFIC_LABELS[provider] ?? provider
}

function formatRemaining(value?: number): string {
  return typeof value === 'number' && Number.isFinite(value) ? value.toFixed(2) : '—'
}

function formatSampledAt(value?: string): string {
  return value ? dayjs(value).format('MM-DD HH:mm') : '—'
}

async function load() {
  const seq = ++requestSeq
  try {
    const data = await getHomeProviderTraffic()
    if (!active || seq !== requestSeq) return
    items.value = data.list ?? []
    loadError.value = ''
  } catch {
    if (!active || seq !== requestSeq) return
    // 首页读取失败只影响本区域；不向 HomeView 抛出，也不影响订阅、规则和公告。
    loadError.value = '服务器余量读取失败'
  }
}

onMounted(() => {
  active = true
  void load()
  timer = window.setInterval(() => {
    void load()
  }, PROVIDER_TRAFFIC_POLL_INTERVAL_MS)
})

onBeforeUnmount(() => {
  active = false
  requestSeq += 1
  if (timer !== undefined) {
    window.clearInterval(timer)
    timer = undefined
  }
})
</script>

<template>
  <Card v-if="visible" id="provider-traffic-home" class="shadow-xs">
    <div class="mb-2 text-sm font-medium">服务器剩余流量</div>
    <Alert v-if="loadError" type="warning" show-icon :message="loadError" />
    <div v-else class="flex flex-col gap-2">
      <div v-for="item in rows" :key="item.host_id ?? item.provider"
           class="flex flex-wrap items-baseline gap-x-2 gap-y-1 text-sm">
        <span class="min-w-16 max-w-full break-all text-text-secondary">
          {{ label(item.provider) }}<template v-if="item.name && item.name !== label(item.provider)"> · {{ item.name }}</template>
        </span>
        <template v-if="item.status === 'waiting'">
          <span class="text-text-tertiary">等待采集</span>
        </template>
        <template v-else-if="item.status === 'ok'">
          <span>剩余 <span class="font-medium">{{ formatRemaining(item.remaining) }}</span> {{ item.unit }}</span>
          <span v-if="item.half_remaining" class="text-text-tertiary">双向流量计算，显示 50% 余量</span>
          <span class="text-text-tertiary">· 采集于 {{ formatSampledAt(item.sampled_at) }}</span>
        </template>
        <template v-else>
          <span class="text-danger">采集失败</span>
          <span class="text-text-secondary">· {{ item.message || '服务商返回错误' }}</span>
        </template>
      </div>
    </div>
  </Card>
</template>
