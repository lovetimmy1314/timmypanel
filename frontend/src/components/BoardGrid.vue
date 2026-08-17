<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { VueDraggable } from 'vue-draggable-plus'
import SiteCard from '@/components/SiteCard.vue'
import type { Group, Site } from '@/api/types'

// board 是 Home.vue 里 boards 的元素，不是副本 —— VueDraggable 的 v-model
// 必须写回那个数组本身，否则拖拽结果提交时读到的还是旧顺序（见 CLAUDE.md）。
const props = defineProps<{
  board: { group: Group; sites: Site[] }
  size: 'sm' | 'md' | 'lg'
  showDesc: boolean
  network: 'wan' | 'lan'
  editing: boolean
  canDrag: boolean
  // 拖拽组名：全局编辑时各组同名（可跨组拖动）；分组级编辑时每组一个独立
  // 名字，别的组不接受，自然就把拖拽锁在组内了。
  dragGroup: string
  gridClass: string
}>()

const emit = defineEmits<{ dragEnd: []; edit: [Site]; remove: [Site] }>()

// 跨组拖动时 vue-draggable-plus 默认拿 JSON.parse(JSON.stringify(x)) 造一个新对象
// 插进目标数组，于是落到新分组里的那张卡片不再是 store 里的那个 Site。
// persistSiteOrder 就地改的 groupId/sort 只写进了这个副本，store 仍是旧分组，
// 下一次 grouped 重算就把它打回原处（见决策 029）。这里换成恒等函数保住引用。
// 必须是模块级的稳定引用，写成模板里的内联箭头会让 options 每次渲染都变。
const keepRef = (s: Site) => s
</script>

<template>
  <VueDraggable
    v-model="props.board.sites"
    :animation="160"
    :group="dragGroup"
    :clone="keepRef"
    ghost-class="tp-ghost"
    :disabled="!canDrag"
    class="grid gap-3"
    :class="gridClass"
    @end="emit('dragEnd')"
  >
    <SiteCard
      v-for="site in props.board.sites"
      :key="site.id"
      :site="site"
      :size="size"
      :show-desc="showDesc"
      :network="network"
      :editing="editing"
      @edit="emit('edit', $event)"
      @remove="emit('remove', $event)"
    />
  </VueDraggable>
</template>
