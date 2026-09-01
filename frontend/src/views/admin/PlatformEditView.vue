<!-- PlatformEditView.vue：平台编辑独立页（UI §5.4/7.4）——新建 /admin/platforms/new、编辑 /admin/platforms/:id/edit -->
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Alert, Button, Card, Form, Input, Radio, Space, TypographyText, Upload } from 'ant-design-vue'
import {
  getPlatform, createPlatform, updatePlatform, uploadInstaller, deleteInstallerFile,
  type InstallerFileItem, type InstallerURLItem,
} from '@/api/platform'
import { Notify } from '@/components/Notify'
import PageHeader from '@/components/PageHeader.vue'
import PlatformHeadersEditor from '@/components/PlatformHeadersEditor.vue'
import { ApiError } from '@/api/request'
import { useSystemStore } from '@/stores/system'

const route = useRoute()
const router = useRouter()
const id = computed(() => (route.params.id ? Number(route.params.id) : 0))
const isEdit = computed(() => id.value > 0)

const system = useSystemStore()
const advancedMode = computed(() => system.status?.advanced_mode === true)
const saving = ref(false)
const formError = ref('')
const uploading = ref(false)
const uploadPct = ref(0)
const deletingFile = ref<string>('') // 正在删除的磁盘文件名
const isDefault = ref(false)
const form = reactive({
  name: '',
  description: '',
  product_type: 'yaml' as 'yaml' | 'subs' | 'generic-subs',
  slug: '',
  schemes: [''] as string[],
  installer_urls: [] as InstallerURLItem[],
})
const extraHeaders = ref<Record<string, string>>({})
const headersEditorRef = ref<InstanceType<typeof PlatformHeadersEditor> | null>(null)
const installerFiles = ref<InstallerFileItem[]>([])

onMounted(async () => {
  if (!isEdit.value) return
  try {
    const p = await getPlatform(id.value)
    form.name = p.name
    form.description = p.description
    form.product_type = p.product_type
    isDefault.value = p.is_default === true
    form.slug = p.slug
    form.schemes = p.schemes?.length ? [...p.schemes] : ['']
    extraHeaders.value = { ...(p.extra_headers ?? {}) }
    form.installer_urls = p.installer_urls?.length ? p.installer_urls.map((it) => ({ ...it })) : []
    installerFiles.value = p.installer_files ?? []
  } catch (err) {
    Notify.error((err as Error).message)
  }
})

// --- scheme 动态列表：拖拽排序（原生 draggable），首项即一键导入默认唤起方式 ---
const dragIndex = ref(-1)
function onDragStart(i: number) { dragIndex.value = i }
function onDrop(i: number) {
  if (dragIndex.value < 0 || dragIndex.value === i) return
  const [item] = form.schemes.splice(dragIndex.value, 1)
  form.schemes.splice(i, 0, item)
  dragIndex.value = -1
}

// --- 本地安装包：追加上传 ≤300MB（前端预校验 + 进度条），多包并存，逐个可删 ---
const MAX_INSTALLER = 300 << 20
function beforeUpload(file: File) {
  if (file.size > MAX_INSTALLER) {
    Notify.error('安装包超过 300MB 限制')
    return false
  }
  uploadInstallerFile(file)
  return false // 拦截默认上传，手动流式提交
}
async function uploadInstallerFile(file: File) {
  uploading.value = true
  uploadPct.value = 0
  try {
    installerFiles.value = await uploadInstaller(id.value, file, (pct) => { uploadPct.value = pct })
    Notify.success('安装包已上传')
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    uploading.value = false
  }
}
async function removeInstallerFile(item: InstallerFileItem) {
  deletingFile.value = item.file
  try {
    await deleteInstallerFile(id.value, item.file)
    installerFiles.value = installerFiles.value.filter((it) => it.file !== item.file)
    Notify.success('安装包已删除')
  } catch (err) {
    Notify.error((err as Error).message)
  } finally {
    deletingFile.value = ''
  }
}

// --- 保存 ---
async function save() {
  const schemes = form.schemes.map((s) => s.trim()).filter(Boolean)
  const headers = headersEditorRef.value?.getValue() ?? null
  if (!headers) {
    Notify.error('附加响应头格式不正确，请先修正')
    return
  }
  // 外部下载链接：剔除全空行（地址为空的行直接忽略，其余整行提交由后端校验）
  const installer_urls = form.installer_urls.filter((it) => it.url.trim() !== '')
  saving.value = true
  try {
    if (isEdit.value) {
      await updatePlatform(id.value, { name: form.name, description: form.description, product_type: form.product_type, schemes, extra_headers: headers, installer_urls })
      Notify.success('平台已更新')
      router.push('/admin/platforms')
    } else {
      await createPlatform({ name: form.name, description: form.description, product_type: form.product_type, schemes, extra_headers: headers, installer_urls })
      Notify.success('平台已创建')
      router.push({ path: '/admin/platforms', query: { created: '1' } }) // 返回列表触发「可前往订阅管理/订阅装配」引导（R14-22 新语义）
    }
  } catch (err) {
    if (err instanceof ApiError && err.status === 400) {
      formError.value = err.message // R14-13：400 冲突/校验改为表单级展示
    } else {
      Notify.error((err as Error).message)
    }
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="max-w-2xl">
    <PageHeader :title="isEdit ? '编辑平台' : '新建平台'">
      <template #actions>
        <Button @click="router.back()">返回</Button>
      </template>
    </PageHeader>

    <Card>
      <Form layout="vertical" @submit.prevent="save">
        <Alert v-if="formError" type="error" show-icon class="mb-3" :message="formError" />
        <Form.Item label="名称" required>
          <Input v-model:value="form.name" :maxlength="100" placeholder="平台名称（不强制唯一）" />
        </Form.Item>
        <Form.Item label="描述">
          <Input.TextArea v-model:value="form.description" :maxlength="500" :rows="2" placeholder="平台描述" />
        </Form.Item>
        <Form.Item label="产物格式">
          <Radio.Group v-model:value="form.product_type" :disabled="isDefault && isEdit">
            <Radio.Button value="yaml">Clash YAML 订阅</Radio.Button>
            <Radio.Button value="subs">Shadowrocket 节点订阅</Radio.Button>
            <Radio.Button value="generic-subs">通用节点订阅（v2rayNG/v2rayN 等）</Radio.Button>
          </Radio.Group>
          <div v-if="isDefault && isEdit" class="text-xs text-text-tertiary mt-1">默认平台产物格式固定，不可修改</div>
          <div v-else class="text-xs text-text-tertiary mt-1">已有订阅条目时，与条目格式不一致的变更将被后端拒绝</div>
        </Form.Item>
        <Form.Item label="标识">
          <TypographyText v-if="isEdit" code>{{ form.slug }}</TypographyText>
          <TypographyText v-else type="secondary">保存后自动生成（platform- 前缀）</TypographyText>
        </Form.Item>

        <!-- scheme 动态列表（支持拖拽排序；首项即首页「一键导入」默认唤起方式） -->
        <Form.Item label="scheme 列表" required>
          <Alert type="info" show-icon class="mb-2"
                 message="支持 {url} 占位符，下载地址将替换其中；拖拽排序，列表首项为「一键导入」默认唤起方式" />
          <div v-for="(_, i) in form.schemes" :key="i" class="flex gap-2 mb-2 items-center"
               draggable="true" @dragstart="onDragStart(i)" @drop="onDrop(i)" @dragover.prevent>
            <span class="cursor-move text-text-tertiary" title="拖拽排序">⠿</span>
            <Input v-model:value="form.schemes[i]" placeholder="如 clash://install-config?url={url}" />
            <Button size="small" danger :disabled="form.schemes.length <= 1" @click="form.schemes.splice(i, 1)">删除</Button>
          </div>
          <Button size="small" @click="form.schemes.push('')">添加 scheme</Button>
        </Form.Item>

        <!-- 附加响应头：结构化编辑 / 高级 JSON 双模式（R25-02） -->
        <Form.Item label="附加响应头">
          <PlatformHeadersEditor ref="headersEditorRef" :model-value="extraHeaders" :advanced-mode="advancedMode" />
        </Form.Item>

        <!-- 安装包区：本地上传（多包并存，追加上传 + 逐个删除）/ 外部下载链接（动态列表），两种来源并存 -->
        <Form.Item label="客户端安装包">
          <Space direction="vertical" class="w-full">
            <Alert v-if="!isEdit" type="info" show-icon
                   message="安装包需在平台创建后上传" />
            <template v-else>
              <Upload :show-upload-list="false" :before-upload="beforeUpload">
                <Button :loading="uploading">追加安装包（≤300MB）</Button>
              </Upload>
              <div v-if="uploading" class="text-xs text-text-secondary">上传中 {{ uploadPct }}%</div>
              <div v-if="installerFiles.length" class="border rounded divide-y divide-border-subtle w-full">
                <div v-for="item in installerFiles" :key="item.file" class="flex items-center gap-2 px-2 py-1">
                  <span class="text-xs text-text-secondary flex-none">📦</span>
                  <TypographyText code class="flex-1 min-w-0 truncate">{{ item.name || item.file }}</TypographyText>
                  <Button size="small" danger :loading="deletingFile === item.file"
                          @click="removeInstallerFile(item)">删除</Button>
                </div>
              </div>
              <div v-else class="text-xs text-text-tertiary">暂无本地安装包</div>
            </template>
          </Space>
        </Form.Item>

        <Form.Item label="外部下载链接">
          <div v-for="(row, i) in form.installer_urls" :key="i" class="flex gap-2 mb-2 items-center">
            <Input v-model:value="row.name" class="w-40" placeholder="展示名（可选）" :maxlength="200" />
            <Input v-model:value="row.url" placeholder="https://…（http/https 地址）" />
            <Button size="small" danger @click="form.installer_urls.splice(i, 1)">删除</Button>
          </div>
          <Button size="small" @click="form.installer_urls.push({ name: '', url: '' })">添加下载链接</Button>
        </Form.Item>

        <Space>
          <Button type="primary" :loading="saving" @click="save">{{ isEdit ? '保存' : '创建' }}</Button>
          <Button @click="router.back()">取消</Button>
        </Space>
      </Form>
    </Card>
  </div>
</template>
