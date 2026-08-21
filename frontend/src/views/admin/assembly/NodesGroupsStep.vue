<!-- NodesGroupsStep.vue：装配步骤③ 节点与代理组（Design2-UI §5.3.1） -->
<script setup lang="ts">
import { Button, Checkbox, Tag } from 'ant-design-vue'
import type { AssemblyContext, TargetSyntax } from '@/api/assembly'
import type { NodeItem } from '@/api/node'
import type { ProxyGroupItem } from '@/api/proxyGroup'

const props = defineProps<{
  form: { node_names: string[]; group_names: string[]; overseas_members: string[] }
  groupNodeOrders: Record<string, string[]>
  context: AssemblyContext | null
  targetSyntax: TargetSyntax
  invalidRefs: Array<{ kind: string; name: string }>
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

function moveOrder(group: string, index: number, delta: number) {
  const arr = [...(props.groupNodeOrders[group] ?? [])]
  const target = index + delta
  if (target < 0 || target >= arr.length) return
  const [item] = arr.splice(index, 1)
  arr.splice(target, 0, item)
  emit('update-group-node-order', group, arr)
}
function removeOrder(group: string, index: number) {
  const arr = [...(props.groupNodeOrders[group] ?? [])]
  arr.splice(index, 1)
  emit('update-group-node-order', group, arr)
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
    <div>
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
          {{ g.name }}<Tag class="ml-1">preset</Tag>
        </Checkbox>
        <Checkbox v-for="g in customGroups" :key="g.name" :checked="form.group_names.includes(g.name)" @change="emit('toggle-group', g.name)">
          {{ g.name }}<Tag class="ml-1">自建</Tag>
        </Checkbox>
      </div>
      <div v-if="form.group_names.length" class="mt-4 space-y-2">
        <div class="text-sm font-medium">已勾选代理组的节点引用顺序</div>
        <div v-for="g in [...presetGroups, ...customGroups].filter((x) => form.group_names.includes(x.name))" :key="g.name" class="border rounded p-2">
          <div class="text-sm font-medium mb-1">{{ g.name }}</div>
          <div v-if="(groupNodeOrders[g.name] ?? []).length" class="space-y-1">
            <div v-for="(name, idx) in groupNodeOrders[g.name]" :key="name" class="flex items-center gap-2 text-sm">
              <span class="flex-1">{{ name }}</span>
              <Button size="small" :disabled="idx === 0" @click="moveOrder(g.name, idx, -1)">上移</Button>
              <Button size="small" :disabled="idx === groupNodeOrders[g.name].length - 1" @click="moveOrder(g.name, idx, 1)">下移</Button>
              <Button size="small" danger @click="removeOrder(g.name, idx)">移除</Button>
            </div>
          </div>
          <div v-else class="text-xs text-gray-400">未设置节点引用顺序（将使用代理组全局定义）</div>
        </div>
      </div>
    </div>
    <div v-if="targetSyntax === 'clash-yaml'">
      <div class="text-sm font-medium mb-1">🌎国外流量成员（仅节点）</div>
      <div class="grid md:grid-cols-3 gap-2">
        <Checkbox v-for="n in manualNodes.concat(xrayNodes)" :key="n.name" :checked="form.overseas_members.includes(n.name)"
                  :disabled="n.source === 'xray' && (!n.allocatable || n.enabled === false)" @change="emit('toggle-overseas', n.name)">
          {{ n.render_name }}
        </Checkbox>
      </div>
    </div>
  </div>
</template>
