<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed } from 'vue'
import { Icon } from '@iconify/vue'
import { t } from '@/i18n'
import type { Group, Site } from '@/api/types'

// 左下角的分组跳转：点出分组列表，选了之后由 Home 决定怎么跳
// （平铺模式滚动到锚点，页签模式切 tab——它持有 activeTab 和折叠状态）。
const props = defineProps<{
  boards: { group: Group; sites: Site[] }[]
}>()
const emit = defineEmits<{ jump: [id: number] }>()

const options = computed(() =>
  props.boards.map((b) => ({
    label: `${b.group.name} (${b.sites.length})`,
    key: b.group.id,
  })),
)
</script>

<template>
  <!-- 只有一个分组时跳转没有意义，不显示 -->
  <n-dropdown
    v-if="boards.length > 1"
    trigger="click"
    :options="options"
    @select="(id: number) => emit('jump', id)"
  >
    <button
      type="button"
      class="tp-fab tp-fab-bl"
      :aria-label="t('home.jumpGroup')"
      :title="t('home.jumpGroup')"
    >
      <Icon icon="mdi:format-list-bulleted" class="text-xl" />
    </button>
  </n-dropdown>
</template>
