<!-- NodesGroupsStep.vue：装配步骤③ 节点与代理组（Design2-UI §5.3.1） -->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { Button, Checkbox, Tag } from 'ant-design-vue'
import AppModal from '@/components/AppModal.vue'
import type { AssemblyContext, TargetSyntax } from '@/api/assembly'
import type { NodeItem } from '@/api/node'
import type { ProxyGroupItem } from '@/api/proxyGroup'

const props = defineProps<{
  form: { node_names: string[]; group_names: string[]; overseas_members: string[]; fallback_group_members: string[] }
  groupNodeOrders: Record<string, string[]>
  context: AssemblyContext | null
  targetSyntax: TargetSyntax
  invalidRefs: Array<{ kind: string; name: string }>
  showXray?: boolean
  manualNodes: NodeItem[]
  xrayNodes: NodeItem[]
  presetGroups: ProxyGroupItem[]
  customGroups: ProxyGroupItem[]
}>()

const emit = defineEmits<{
  'toggle-node': [name: string]
  'toggle-group': [name: string]
  'update-group-node-order': [group: string, nodes: string[]]
  'update-force-members': [group: string, members: string[]]
}>()

const FORCE_DIRECT = '🚀直接连接'
const FORCE_OVERSEAS = '🌎国外流量'
const FORCE_FALLBACK = '🛟无法归属的流量'

// “选择与排序”弹窗状态
const selectingGroup = ref<string | null>(null)
const draftSelected = ref<string[]>([])
const dragIndex = ref<number | null>(null)

const availableNodes = computed(() => {
  const all = props.showXray === false ? props.manualNodes : [...props.manualNodes, ...props.xrayNodes]
  return all.filter((n) => props.form.node_names.includes(n.name) && (n.source === 'manual' || (n.allocatable && n.enabled !== false)))
})
const selectedManualCount = computed(() => props.manualNodes.filter((n) => props.form.node_names.includes(n.name)).length)
const selectedXrayCount = computed(() => props.xrayNodes.filter((n) => props.form.node_names.includes(n.name)).length)
const selectedGroupCount = computed(() => props.form.group_names.length)
const selectingForceGroup = computed(() => selectingGroup.value === FORCE_OVERSEAS || selectingGroup.value === FORCE_FALLBACK)
const availableForceGroups = computed(() => {
  if (selectingGroup.value === FORCE_OVERSEAS) return [FORCE_DIRECT]
  if (selectingGroup.value === FORCE_FALLBACK) return [FORCE_DIRECT, FORCE_OVERSEAS]
  return []
})
function nodeLabel(name: string) {
  return [...props.manualNodes, ...props.xrayNodes].find((n) => n.name === name)?.render_name ?? name
}
function nodeSubLabel(name: string) {
  return [...props.manualNodes, ...props.xrayNodes].find((n) => n.name === name)?.display_name ?? ''
}
function openSelector(group: string) {
  selectingGroup.value = group
  if (group === FORCE_OVERSEAS) {
    draftSelected.value = [...props.form.overseas_members]
  } else if (group === FORCE_FALLBACK) {
    draftSelected.value = [...props.form.fallback_group_members]
  } else {
    draftSelected.value = [...(props.groupNodeOrders[group] ?? [])]
  }
}
function closeSelector() {
  selectingGroup.value = null
  dragIndex.value = null
}
function toggleDraftNode(name: string) {
  draftSelected.value = draftSelected.value.includes(name)
    ? draftSelected.value.filter((n) => n !== name)
    : [...draftSelected.value, name]
}
function moveDraft(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= draftSelected.value.length) return
  const arr = [...draftSelected.value]
  const [item] = arr.splice(index, 1)
  arr.splice(target, 0, item)
  draftSelected.value = arr
}
function saveSelector() {
  if (selectingGroup.value) {
    if (selectingForceGroup.value) emit('update-force-members', selectingGroup.value, draftSelected.value)
    else emit('update-group-node-order', selectingGroup.value, draftSelected.value)
  }
  closeSelector()
}
function onDragStart(idx: number) { dragIndex.value = idx }
function onDrop(idx: number) {
  if (dragIndex.value !== null && dragIndex.value !== idx) {
    const arr = [...draftSelected.value]
    const [item] = arr.splice(dragIndex.value, 1)
    arr.splice(idx, 0, item)
    draftSelected.value = arr
  }
  dragIndex.value = null
}
</script>

<template>
  <div class="space-y-3">
    <section class="rounded-lg border border-border-subtle p-3">
      <div class="flex items-center justify-between mb-2">
        <div class="text-sm font-medium">手动添加的节点</div>
        <span class="text-xs text-text-tertiary">已选 {{ selectedManualCount }} / {{ manualNodes.length }}</span>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-2">
        <div v-for="n in manualNodes" :key="n.name"
             class="flex items-center gap-2 rounded border px-2 py-1.5"
             :class="form.node_names.includes(n.name) ? 'border-primary bg-primary-soft' : 'border-border-subtle'">
          <Checkbox :checked="form.node_names.includes(n.name)" @change="emit('toggle-node', n.name)">
            <span>{{ n.render_name }}</span><Tag class="ml-1">{{ n.protocol }}</Tag>
            <Tag v-if="invalidRefs.some((r) => r.kind === 'node' && r.name === n.name)" color="red">已失效</Tag>
          </Checkbox>
        </div>
        <div v-if="manualNodes.length === 0" class="text-xs text-text-tertiary">暂无手动添加的节点</div>
      </div>
    </section>

    <section v-if="showXray !== false" class="rounded-lg border border-border-subtle p-3">
      <div class="flex items-center justify-between mb-2">
        <div class="text-sm font-medium">Xray 节点</div>
        <span class="text-xs text-text-tertiary">已选 {{ selectedXrayCount }} / {{ xrayNodes.length }}</span>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-2">
        <div v-for="n in xrayNodes" :key="n.name"
             class="flex items-center gap-2 rounded border px-2 py-1.5"
             :class="form.node_names.includes(n.name) ? 'border-primary bg-primary-soft' : 'border-border-subtle'">
          <Checkbox :checked="form.node_names.includes(n.name)"
                    :disabled="!n.allocatable || n.enabled === false" @change="emit('toggle-node', n.name)">
            <span>{{ n.render_name }}</span>
            <span v-if="n.display_name" class="block text-xs text-text-tertiary font-mono">{{ n.name }}</span>
            <Tag v-if="!n.allocatable || n.enabled === false" class="ml-1">不可用</Tag>
          </Checkbox>
        </div>
        <div v-if="xrayNodes.length === 0" class="text-xs text-text-tertiary">未检测到 Xray 节点（高级模式录入实例后刷新节点发现）</div>
      </div>
    </section>

    <section v-if="targetSyntax === 'clash-yaml'" class="rounded-lg border border-border-subtle p-3">
      <div class="flex items-center justify-between mb-2">
        <div class="text-sm font-medium">代理组</div>
        <span class="text-xs text-text-tertiary">已选 {{ selectedGroupCount }} / {{ presetGroups.length + customGroups.length }}（另有 3 个强制组）</span>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-2">
        <div class="rounded border border-border-subtle p-2">
          <Checkbox :checked="true" disabled>{{ FORCE_DIRECT }}</Checkbox>
          <Tag class="ml-1">强制</Tag>
          <div class="mt-1 text-xs text-text-tertiary">系统固定的直连出口，不提供成员选择</div>
        </div>
        <div data-testid="force-overseas-group" class="rounded border border-border-subtle p-2">
          <div class="flex items-center gap-1 flex-wrap">
            <Checkbox :checked="true" disabled>{{ FORCE_OVERSEAS }}</Checkbox>
            <Tag>强制</Tag>
            <Button data-testid="select-overseas-members" size="small" @click="openSelector(FORCE_OVERSEAS)">选择与排序</Button>
          </div>
          <div class="mt-1 text-xs text-text-tertiary">可选成员：本次已勾选节点、{{ FORCE_DIRECT }}</div>
        </div>
        <div data-testid="force-fallback-group" class="rounded border border-border-subtle p-2">
          <div class="flex items-center gap-1 flex-wrap">
            <Checkbox :checked="true" disabled>{{ FORCE_FALLBACK }}</Checkbox>
            <Tag>强制</Tag>
            <Button data-testid="select-fallback-members" size="small" @click="openSelector(FORCE_FALLBACK)">选择与排序</Button>
          </div>
          <div class="mt-1 text-xs text-text-tertiary">可选成员：本次已勾选节点、{{ FORCE_DIRECT }}、{{ FORCE_OVERSEAS }}</div>
        </div>
        <div v-for="g in presetGroups" :key="g.name"
             class="flex items-center gap-2 rounded border px-2 py-1.5"
             :class="form.group_names.includes(g.name) ? 'border-primary bg-primary-soft' : 'border-border-subtle'">
          <Checkbox :checked="form.group_names.includes(g.name)" :disabled="!g.enabled" @change="emit('toggle-group', g.name)">
            <span>{{ g.name }}</span>
            <Button v-if="form.group_names.includes(g.name)" size="small" class="ml-1" @click.stop="openSelector(g.name)">选择与排序</Button>
          </Checkbox>
        </div>
        <div v-for="g in customGroups" :key="g.name"
             class="flex items-center gap-2 rounded border px-2 py-1.5"
             :class="form.group_names.includes(g.name) ? 'border-primary bg-primary-soft' : 'border-border-subtle'">
          <Checkbox :checked="form.group_names.includes(g.name)" @change="emit('toggle-group', g.name)">
            <span>{{ g.name }}</span>
            <Tag v-if="!form.group_names.includes(g.name)" class="ml-1">自建</Tag>
            <Button v-else size="small" class="ml-1" @click.stop="openSelector(g.name)">选择与排序</Button>
          </Checkbox>
        </div>
      </div>
    </section>

    <AppModal :open="!!selectingGroup" :title="`成员选择与排序 · ${selectingGroup ?? ''}`" :footer="null" :width="640" destroy-on-close @update:open="closeSelector">
      <div class="space-y-3">
        <div v-if="selectingForceGroup && availableForceGroups.length">
          <div class="text-sm font-medium mb-1">可引用组</div>
          <div class="grid md:grid-cols-2 gap-2">
            <Checkbox v-for="group in availableForceGroups" :key="group" :checked="draftSelected.includes(group)" @change="toggleDraftNode(group)">
              {{ group }}
            </Checkbox>
          </div>
          <div class="mt-1 text-xs text-text-tertiary">底层直连不会作为候选显示；需要直连时请选择「{{ FORCE_DIRECT }}」。</div>
        </div>
        <div>
          <div class="text-sm font-medium mb-1">本次已勾选节点</div>
          <div class="grid md:grid-cols-2 gap-2">
            <Checkbox v-for="n in availableNodes" :key="n.name" :checked="draftSelected.includes(n.name)" @change="toggleDraftNode(n.name)">
              <span>{{ n.render_name }}</span>
              <Tag class="ml-1">{{ n.protocol }}</Tag>
            </Checkbox>
            <div v-if="availableNodes.length === 0" class="text-xs text-text-tertiary">暂无已勾选且可用的节点</div>
          </div>
        </div>
        <div>
          <div class="text-sm font-medium mb-1">已选成员（有序）</div>
          <div v-if="draftSelected.length === 0" class="text-xs text-danger">至少选择一个成员</div>
          <div v-for="(name, idx) in draftSelected" :key="name" :draggable="true"
               class="flex items-center gap-2 border rounded p-2 mb-2 cursor-move"
               @dragstart="onDragStart(idx)" @dragover.prevent @drop="onDrop(idx)">
            <span class="flex-1">{{ availableForceGroups.includes(name) ? name : nodeLabel(name) }}</span>
            <span v-if="nodeSubLabel(name)" class="text-xs text-text-tertiary font-mono">{{ name }}</span>
            <Button size="small" :disabled="idx === 0" @click="moveDraft(idx, -1)">上移</Button>
            <Button size="small" :disabled="idx === draftSelected.length - 1" @click="moveDraft(idx, 1)">下移</Button>
          </div>
        </div>
        <div class="flex justify-end gap-2">
          <Button @click="closeSelector">取消</Button>
          <Button data-testid="save-member-selector" type="primary" :disabled="selectingForceGroup && draftSelected.length === 0" @click="saveSelector">保存</Button>
        </div>
      </div>
    </AppModal>
  </div>
</template>
