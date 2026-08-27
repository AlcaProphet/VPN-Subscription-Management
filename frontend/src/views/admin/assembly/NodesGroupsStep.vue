<!-- NodesGroupsStep.vue：装配步骤③ 节点与代理组（Design2-UI §5.3.1） -->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { Button, Checkbox, Modal, Tag } from 'ant-design-vue'
import type { AssemblyContext, TargetSyntax } from '@/api/assembly'
import type { NodeItem } from '@/api/node'
import type { ProxyGroupItem } from '@/api/proxyGroup'

const props = defineProps<{
  form: { node_names: string[]; group_names: string[]; overseas_members: string[] }
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
  'toggle-overseas': [name: string]
  'update-group-node-order': [group: string, nodes: string[]]
}>()

const FORCE_GROUPS = ['🚀直接连接', '🌎国外流量', '🛟无法归属的流量']

// “选择与排序”弹窗状态
const selectingGroup = ref<string | null>(null)
const draftSelected = ref<string[]>([])
const dragIndex = ref<number | null>(null)

const availableNodes = computed(() => {
  const all = props.showXray === false ? props.manualNodes : [...props.manualNodes, ...props.xrayNodes]
  return all.filter((n) => n.source === 'manual' || (n.allocatable && n.enabled !== false))
})
function nodeLabel(name: string) {
  return [...props.manualNodes, ...props.xrayNodes].find((n) => n.name === name)?.render_name ?? name
}
function nodeSubLabel(name: string) {
  return [...props.manualNodes, ...props.xrayNodes].find((n) => n.name === name)?.display_name ?? ''
}
function openSelector(group: string) {
  selectingGroup.value = group
  draftSelected.value = [...(props.groupNodeOrders[group] ?? [])]
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
  if (selectingGroup.value) emit('update-group-node-order', selectingGroup.value, draftSelected.value)
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
    <div>
      <div class="text-sm font-medium mb-1">manual 节点</div>
      <div class="grid md:grid-cols-3 gap-2">
        <Checkbox v-for="n in manualNodes" :key="n.name" :checked="form.node_names.includes(n.name)" @change="emit('toggle-node', n.name)">
          <span>{{ n.render_name }}</span><Tag class="ml-1">{{ n.protocol }}</Tag>
          <Tag v-if="invalidRefs.some((r) => r.kind === 'node' && r.name === n.name)" color="red">已失效</Tag>
        </Checkbox>
        <div v-if="manualNodes.length === 0" class="text-xs text-gray-400">暂无 manual 节点</div>
      </div>
    </div>
    <div v-if="showXray !== false">
      <div class="text-sm font-medium mb-1">xray 节点</div>
      <div class="grid md:grid-cols-3 gap-2">
        <Checkbox v-for="n in xrayNodes" :key="n.name" :checked="form.node_names.includes(n.name)"
                  :disabled="!n.allocatable || n.enabled === false" @change="emit('toggle-node', n.name)">
          <span>{{ n.render_name }}</span>
          <span v-if="n.display_name" class="block text-xs text-gray-400 font-mono">{{ n.name }}</span>
          <Tag v-if="!n.allocatable || n.enabled === false" class="ml-1">不可用</Tag>
        </Checkbox>
        <div v-if="xrayNodes.length === 0" class="text-xs text-gray-400">未检测到 Xray 节点（高级模式录入实例后刷新节点发现）</div>
      </div>
    </div>
    <div v-if="targetSyntax === 'clash-yaml'">
      <div class="text-sm font-medium mb-1">代理组</div>
      <div class="grid md:grid-cols-3 gap-2">
        <Checkbox v-for="g in FORCE_GROUPS" :key="g" :checked="true" disabled>{{ g }}<Tag class="ml-1">强制</Tag></Checkbox>
        <Checkbox v-for="g in presetGroups" :key="g.name" :checked="form.group_names.includes(g.name)" :disabled="!g.enabled" @change="emit('toggle-group', g.name)">
          <span>{{ g.name }}</span>
          <Button v-if="form.group_names.includes(g.name)" size="small" class="ml-1" @click.stop="openSelector(g.name)">选择与排序</Button>
        </Checkbox>
        <Checkbox v-for="g in customGroups" :key="g.name" :checked="form.group_names.includes(g.name)" @change="emit('toggle-group', g.name)">
          <span>{{ g.name }}</span>
          <Tag v-if="!form.group_names.includes(g.name)" class="ml-1">自建</Tag>
          <Button v-else size="small" class="ml-1" @click.stop="openSelector(g.name)">选择与排序</Button>
        </Checkbox>
      </div>
    </div>
    <div v-if="targetSyntax === 'clash-yaml'">
      <div class="text-sm font-medium mb-1">🌎国外流量成员（仅节点）</div>
      <div class="grid md:grid-cols-3 gap-2">
        <Checkbox v-for="n in manualNodes.concat(showXray !== false ? xrayNodes : [])" :key="n.name" :checked="form.overseas_members.includes(n.name)"
                  :disabled="n.source === 'xray' && (!n.allocatable || n.enabled === false)" @change="emit('toggle-overseas', n.name)">
          {{ n.render_name }}
        </Checkbox>
      </div>
    </div>

    <Modal :open="!!selectingGroup" :title="`节点选择与排序 · ${selectingGroup ?? ''}`" :footer="null" :width="640" destroy-on-close @cancel="closeSelector">
      <div class="space-y-3">
        <div>
          <div class="text-sm font-medium mb-1">可选节点</div>
          <div class="grid md:grid-cols-2 gap-2">
            <Checkbox v-for="n in availableNodes" :key="n.name" :checked="draftSelected.includes(n.name)" @change="toggleDraftNode(n.name)">
              <span>{{ n.render_name }}</span>
              <Tag class="ml-1">{{ n.protocol }}</Tag>
            </Checkbox>
          </div>
        </div>
        <div>
          <div class="text-sm font-medium mb-1">已选节点（有序）</div>
          <div v-if="draftSelected.length === 0" class="text-xs text-gray-400">尚未选择节点，将使用子组引用</div>
          <div v-for="(name, idx) in draftSelected" :key="name" :draggable="true"
               class="flex items-center gap-2 border rounded p-2 mb-2 cursor-move"
               @dragstart="onDragStart(idx)" @dragover.prevent @drop="onDrop(idx)">
            <span class="flex-1">{{ nodeLabel(name) }}</span>
            <span v-if="nodeSubLabel(name)" class="text-xs text-gray-400 font-mono">{{ name }}</span>
            <Button size="small" :disabled="idx === 0" @click="moveDraft(idx, -1)">上移</Button>
            <Button size="small" :disabled="idx === draftSelected.length - 1" @click="moveDraft(idx, 1)">下移</Button>
          </div>
        </div>
        <div class="flex justify-end gap-2">
          <Button @click="closeSelector">取消</Button>
          <Button type="primary" @click="saveSelector">保存</Button>
        </div>
      </div>
    </Modal>
  </div>
</template>
