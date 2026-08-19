<!-- PlatformsView.vue：平台管理双态列表（UI §5.4）——≥768 表格 / <768 卡片 -->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Button, Space, Table, Tag, TypographyText } from 'ant-design-vue'
import { listPlatforms, deletePlatform, type PlatformItem } from '@/api/platform'
import ConfirmModal from '@/components/ConfirmModal.vue'
import TriStateList from '@/components/TriStateList.vue'
import { Notify } from '@/components/Notify'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const platforms = ref<PlatformItem[]>([])
const toDelete = ref<PlatformItem | null>(null)
const deleting = ref(false)

async function load() {
  loading.value = true
  try {
    platforms.value = await listPlatforms()
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    loading.value = false
  }
}
onMounted(() => {
  void load()
  // 新建平台成功返回列表：引导前往订阅管理创建条目或装配生成模板
  if (route.query.created === '1') {
    Notify.info('平台已创建，可前往订阅管理创建订阅条目，或前往订阅装配生成模板')
    void router.replace({ path: '/admin/platforms' })
  }
})

// 安装包状态：本地包数量 / 外链数量 / 无（多包并存）
function installerStatus(p: PlatformItem): { text: string; color: string } {
  const files = p.installer_files?.length ?? 0
  const urls = p.installer_urls?.length ?? 0
  if (files > 0 && urls > 0) return { text: `本地 ${files} · 外链 ${urls}`, color: 'cyan' }
  if (files > 0) return { text: `本地 ${files}`, color: 'green' }
  if (urls > 0) return { text: `外链 ${urls}`, color: 'blue' }
  return { text: '无', color: 'default' }
}

const productTypeMeta: Record<string, { label: string; color: string }> = {
  yaml: { label: 'yaml', color: 'blue' },
  subs: { label: 'subs', color: 'cyan' },
  'generic-subs': { label: 'generic-subs', color: 'purple' },
}

// 删除确认：逐项列出影响清单（N 份订阅、M 个 Token、K 份自定义订阅 + 文件不可恢复提示）
const deleteContent = computed(() => {
  const p = toDelete.value
  if (!p) return ''
  const parts = [
    `将删除平台「${p.name}」及其标识 ${p.slug}`,
    `影响：${p.cascade?.subscriptions ?? 0} 份订阅、${p.cascade?.tokens ?? 0} 个下载 Token、${p.cascade?.customs ?? 0} 份自定义订阅`,
  ]
  if ((p.installer_files?.length ?? 0) > 0) parts.push('本地安装包文件将一并删除')
  parts.push('删除后不可恢复，请谨慎操作')
  return parts.join('\n')
})

async function confirmDelete() {
  if (!toDelete.value) return
  deleting.value = true
  try {
    await deletePlatform(toDelete.value.id)
    Notify.success('平台已删除')
    toDelete.value = null
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
      <h2 class="text-lg font-semibold m-0">平台管理</h2>
      <Button type="primary" @click="router.push('/admin/platforms/new')">新建平台</Button>
    </div>

    <TriStateList :loading="loading" :empty="platforms.length === 0" empty-text="暂无平台">
      <!-- ≥768：表格 -->
      <Table :data-source="platforms" row-key="id" :pagination="false" class="hidden md:block">
        <Table.Column key="name" title="名称" data-index="name" />
        <Table.Column key="product_type" title="产物格式">
          <template #default="{ record }">
            <Tag :color="productTypeMeta[record.product_type]?.color">{{ record.product_type }}</Tag>
          </template>
        </Table.Column>
        <Table.Column key="slug" title="标识" data-index="slug">
          <template #default="{ record }">
            <TypographyText code>{{ record.slug }}</TypographyText>
          </template>
        </Table.Column>
        <Table.Column key="installer" title="安装包" data-index="installer_file">
          <template #default="{ record }">
            <Tag :color="installerStatus(record).color">{{ installerStatus(record).text }}</Tag>
          </template>
        </Table.Column>
        <Table.Column key="actions" title="操作" width="160">
          <template #default="{ record }">
            <Space>
              <Button size="small" @click="router.push(`/admin/platforms/${record.id}/edit`)">编辑</Button>
              <Button size="small" danger @click="toDelete = record">删除</Button>
            </Space>
          </template>
        </Table.Column>
      </Table>

      <!-- <768：卡片 -->
      <div class="grid grid-cols-1 gap-3 md:hidden">
        <div v-for="p in platforms" :key="p.id" class="border rounded-lg p-3 bg-white dark:bg-gray-800">
          <div class="flex items-center justify-between">
            <span class="font-medium">{{ p.name }}</span>
            <Tag :color="installerStatus(p).color">{{ installerStatus(p).text }}</Tag>
          </div>
          <!-- 标识只读展示 -->
          <div class="text-xs text-gray-500 mt-1">
            <TypographyText code>{{ p.slug }}</TypographyText>
              <Tag class="ml-1" :color="productTypeMeta[p.product_type]?.color">{{ p.product_type }}</Tag>
          </div>
          <div class="mt-2 flex gap-2">
            <Button size="small" @click="router.push(`/admin/platforms/${p.id}/edit`)">编辑</Button>
            <Button size="small" danger @click="toDelete = p">删除</Button>
          </div>
        </div>
      </div>
    </TriStateList>

    <!-- 删除确认（影响清单 + 文件不可恢复提示） -->
    <ConfirmModal :open="toDelete !== null" title="删除平台" danger :loading="deleting"
                  :content="deleteContent" @confirm="confirmDelete" @update:open="toDelete = null" />
  </div>
</template>
