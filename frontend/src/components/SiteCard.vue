<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, ref, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { t } from '@/i18n'
import { iconSetReady, loadIconSet } from '@/icons'
import type { Site } from '@/api/types'

const props = defineProps<{
  site: Site
  size: 'sm' | 'md' | 'lg'
  showDesc: boolean
  network: 'wan' | 'lan'
  editing: boolean
}>()

const emit = defineEmits<{ edit: [Site]; remove: [Site] }>()

// 内网模式下优先用内网地址，没配就退回公网地址。
const href = computed(() =>
  props.network === 'lan' && props.site.lanUrl ? props.site.lanUrl : props.site.url,
)
const usingLan = computed(() => props.network === 'lan' && !!props.site.lanUrl)

const iconFailed = ref(false)

// 图标库类型的卡片才需要整份 mdi（3MB，见 src/icons/index.ts）。
// 只有图片和文字图标的账号一辈子不会下载它。
watch(
  () => props.site.iconType,
  (type) => {
    if (type === 'iconify') loadIconSet()
  },
  { immediate: true },
)
const initial = computed(() => (props.site.title || props.site.url).trim().charAt(0).toUpperCase())
const glyph = computed(() => {
  const v = props.site.iconValue.trim()
  if (props.site.iconType === 'text' && v) return v
  return initial.value
})

// 没有图标时按标题生成一个稳定的色块，比统一灰块好认。
const fallbackBg = computed(() => {
  if (props.site.iconBg) return props.site.iconBg
  let hash = 0
  for (const ch of props.site.title || props.site.url) hash = (hash * 31 + ch.charCodeAt(0)) >>> 0
  return `hsl(${hash % 360}, 55%, 45%)`
})

const sizeClass = computed(
  () =>
    ({
      sm: 'p-2.5 gap-2',
      md: 'p-3 gap-3',
      lg: 'p-4 gap-3.5',
    })[props.size],
)
const iconSize = computed(() => ({ sm: 'w-8 h-8', md: 'w-10 h-10', lg: 'w-12 h-12' })[props.size])
</script>

<template>
  <div class="relative group">
    <a
      class="tp-card rounded-xl flex items-center no-underline select-none"
      :class="[sizeClass, editing ? 'cursor-move' : 'cursor-pointer']"
      :href="editing ? undefined : href"
      :target="site.openMode === 'self' ? '_self' : '_blank'"
      rel="noopener noreferrer"
      :title="site.description ? undefined : href"
      @click="editing && $event.preventDefault()"
    >
      <div
        class="shrink-0 rounded-lg overflow-hidden flex items-center justify-center text-white font-semibold"
        :class="iconSize"
        :style="
          site.iconType === 'url' && site.iconValue && !iconFailed
            ? undefined
            : { background: fallbackBg }
        "
      >
        <img
          v-if="site.iconType === 'url' && site.iconValue && !iconFailed"
          :src="site.iconValue"
          class="w-full h-full object-contain"
          loading="lazy"
          alt=""
          @error="iconFailed = true"
        />
        <!-- 图标集没就位就先显示文字兜底：这时候渲染 <Icon> 会让它去打已经被清空的
             API，那一次失败之后即便图标集加载好了也不会自己重画。 -->
        <Icon
          v-else-if="site.iconType === 'iconify' && site.iconValue && iconSetReady"
          :icon="site.iconValue"
          class="text-xl"
        />
        <span v-else>{{ glyph }}</span>
      </div>

      <div class="min-w-0 flex-1">
        <div class="tp-text text-sm font-medium truncate flex items-center gap-1">
          <span class="truncate">{{ site.title }}</span>
          <Icon
            v-if="usingLan"
            icon="mdi:lan-connect"
            class="shrink-0 text-emerald-500 dark:text-emerald-300 text-xs"
            :title="t('home.lanBadge')"
          />
        </div>
        <n-tooltip v-if="showDesc && site.description" trigger="hover" :delay="200">
          <template #trigger>
            <div class="tp-text-dim text-xs truncate mt-0.5">{{ site.description }}</div>
          </template>
          <span class="inline-block max-w-xs whitespace-pre-wrap break-words">{{ site.description }}</span>
        </n-tooltip>
      </div>
    </a>

    <!-- 编辑模式下才出现的操作按钮，平时不打扰浏览 -->
    <div v-if="editing" class="absolute -top-2 -right-2 flex gap-1">
      <button
        class="w-6 h-6 rounded-full bg-sky-500 text-white text-xs flex items-center justify-center shadow"
        :title="t('home.cardEdit')"
        @click.stop.prevent="emit('edit', site)"
      >
        <Icon icon="mdi:pencil" />
      </button>
      <button
        class="w-6 h-6 rounded-full bg-red-500 text-white text-xs flex items-center justify-center shadow"
        :title="t('home.cardDelete')"
        @click.stop.prevent="emit('remove', site)"
      >
        <Icon icon="mdi:close" />
      </button>
    </div>
  </div>
</template>
