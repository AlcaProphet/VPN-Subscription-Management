<template>
  <div class="rules-container">
    <header class="rules-header">
      <h1 class="rules-title">分流规则</h1>
      <p class="rules-desc">浏览和下载可用的分流规则配置</p>
    </header>

    <main class="rules-main" v-loading="loading">
      <el-empty
        v-if="!loading && rules.length === 0"
        description="暂无可用规则"
      />

      <el-table
        v-else
        :data="rules"
        stripe
        class="rules-table"
      >
        <el-table-column prop="name" label="规则名称" min-width="180" />
        <el-table-column prop="client_type" label="客户端类型" width="140">
          <template #default="{ row }">
            <el-tag size="small">{{ row.client_type || 'Shadowrocket' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="当前版本" width="120">
          <template #default="{ row }">
            <span v-if="currentVersion(row) !== null">
              v{{ currentVersion(row) }}
            </span>
            <span v-else class="no-version">—</span>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="180">
          <template #default="{ row }">
            <span v-if="currentUpdatedAt(row)">
              {{ formatTime(currentUpdatedAt(row)) }}
            </span>
            <span v-else class="no-version">—</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <a
              v-if="row.token"
              :href="getRuleDownloadUrl(row.id, row.token)"
              class="download-btn"
            >
              <el-button type="primary" size="small">
                下载当前版本
              </el-button>
            </a>
            <el-tooltip
              v-else
              content="请联系管理员获取下载链接"
              placement="top"
            >
              <el-button type="primary" size="small" disabled>
                下载当前版本
              </el-button>
            </el-tooltip>
          </template>
        </el-table-column>
      </el-table>
    </main>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { publicApi } from '@/services/api'

const loading = ref(true)
const rules = ref([])

function currentVersion(rule) {
  if (!rule.versions || rule.versions.length === 0) return null
  // Find the version with the highest updated_at (current version)
  const sorted = [...rule.versions].sort(
    (a, b) => new Date(b.updated_at || 0) - new Date(a.updated_at || 0)
  )
  return sorted[0]?.version ?? null
}

function currentUpdatedAt(rule) {
  if (!rule.versions || rule.versions.length === 0) return null
  const sorted = [...rule.versions].sort(
    (a, b) => new Date(b.updated_at || 0) - new Date(a.updated_at || 0)
  )
  return sorted[0]?.updated_at ?? null
}

function getRuleDownloadUrl(ruleId, token) {
  return publicApi.getRuleDownloadUrl(ruleId, token)
}

function formatTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString()
}

onMounted(async () => {
  try {
    const res = await publicApi.getRules()
    rules.value = res.data.rules || []
  } catch (e) {
    // Silently fail — rules page is informational
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.rules-container {
  min-height: 100vh;
  background: var(--el-bg-color-page);
}

.rules-header {
  padding: 24px 24px 0;
}

.rules-title {
  margin: 0 0 4px;
  font-size: 22px;
  font-weight: 700;
  color: var(--el-text-color-primary);
}

.rules-desc {
  margin: 0;
  font-size: 14px;
  color: var(--el-text-color-secondary);
}

.rules-main {
  padding: 24px;
}

.rules-table {
  width: 100%;
}

.download-btn {
  text-decoration: none;
}

.no-version {
  color: var(--el-text-color-placeholder);
  font-style: italic;
}

@media (max-width: 768px) {
  .rules-header {
    padding: 16px 16px 0;
  }

  .rules-main {
    padding: 16px;
  }
}
</style>
