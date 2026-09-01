<!-- OverlayStep.vue：Clash 覆盖层（Merge + Rules/Proxies/Groups Seq，Design2-UI §5.3.4） -->
<script setup lang="ts">
import { Button, Input } from 'ant-design-vue'

const props = defineProps<{
  form: {
    overlay: {
      merge_yaml: string
      rules_yaml: string
      proxies_yaml: string
      groups_yaml: string
    }
  }
}>()

const emptySeqTemplate = `prepend: []
append: []
delete: []`

function fillTemplate() {
  props.form.overlay.merge_yaml = '{}'
  props.form.overlay.rules_yaml = emptySeqTemplate
  props.form.overlay.proxies_yaml = emptySeqTemplate
  props.form.overlay.groups_yaml = emptySeqTemplate
}
</script>

<template>
  <div class="space-y-3">
    <div class="flex items-center justify-between flex-wrap gap-2">
      <div class="text-xs text-text-secondary">
        覆盖层仅作用于 Clash YAML：Merge 深合并，Rules/Proxies/Groups 按 prepend / delete / append 顺序覆盖。
      </div>
      <Button size="small" @click="fillTemplate">填入空模板</Button>
    </div>

    <div>
      <div class="text-sm font-medium mb-1">Merge YAML</div>
      <Input.TextArea v-model:value="form.overlay.merge_yaml" :rows="5" placeholder="顶层键深合并，其余以覆盖层为准；控制面键与 dns.ipv6 受保护" />
    </div>

    <div>
      <div class="text-sm font-medium mb-1">Rules Seq</div>
      <Input.TextArea v-model:value="form.overlay.rules_yaml" :rows="4" placeholder="prepend / append / delete" />
    </div>

    <div>
      <div class="text-sm font-medium mb-1">Proxies Seq</div>
      <Input.TextArea v-model:value="form.overlay.proxies_yaml" :rows="4" placeholder="prepend / append / delete；新增节点会自动插入第一个 selector 组" />
    </div>

    <div>
      <div class="text-sm font-medium mb-1">Groups Seq</div>
      <Input.TextArea v-model:value="form.overlay.groups_yaml" :rows="4" placeholder="prepend / append / delete" />
    </div>
  </div>
</template>
