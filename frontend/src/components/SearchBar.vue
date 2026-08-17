<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, h, onMounted, onUnmounted, ref, watch, type VNode } from 'vue'
import Fuse from 'fuse.js'
import { Icon } from '@iconify/vue'
import type { SelectOption } from 'naive-ui'
import { t } from '@/i18n'
import type { SearchEngine, Site } from '@/api/types'

const props = defineProps<{
  sites: Site[]
  engines: SearchEngine[]
  defaultEngine: string
  network: 'wan' | 'lan'
  barStyle?: { bg: string; color: string; border: string }
}>()

const wrapStyle = computed(() => {
  const s = props.barStyle
  if (!s) return undefined
  const out: Record<string, string> = {}
  if (s.bg) out.background = s.bg
  if (s.border) out.borderColor = s.border
  // 文字颜色不能只写在外层：naive-ui 给 input 和 select 内部元素直接上了
  // color: var(--n-text-color)，继承色盖不过去（实测外层 color 完全无效）。
  // 所以这里出一个自定义属性，由下面的 :deep 规则命中内部元素。
  if (s.color) {
    out.color = s.color
    out['--tp-search-color'] = s.color
  }
  return Object.keys(out).length ? out : undefined
})

// 没配颜色时不加这个类，保持 naive 默认外观。
const tinted = computed(() => !!props.barStyle?.color)

const emit = defineEmits<{ 'update:query': [string]; 'set-default': [string] }>()

const query = ref('')
const engine = ref(props.defaultEngine || 'local')
const inputRef = ref<any>(null)
const activeIndex = ref(0)

watch(
  () => props.defaultEngine,
  (v) => (engine.value = v || 'local'),
)
watch(query, (v) => {
  emit('update:query', engine.value === 'local' ? v : '')
  activeIndex.value = 0
})
watch(engine, () => emit('update:query', engine.value === 'local' ? query.value : ''))

// Fuse 索引随卡片变化重建。个人站点量级下重建开销可以忽略。
const fuse = computed(
  () =>
    new Fuse(
      props.sites.filter((s) => !s.hidden),
      {
        keys: [
          { name: 'title', weight: 3 },
          { name: 'description', weight: 1 },
          { name: 'url', weight: 1 },
        ],
        threshold: 0.4,
        ignoreLocation: true,
      },
    ),
)

const matches = computed(() => {
  if (engine.value !== 'local' || !query.value.trim()) return []
  return fuse.value
    .search(query.value.trim())
    .slice(0, 8)
    .map((r) => r.item)
})

function hrefOf(s: Site) {
  return props.network === 'lan' && s.lanUrl ? s.lanUrl : s.url
}

function submit() {
  const q = query.value.trim()
  if (!q) return
  if (engine.value === 'local') {
    const target = matches.value[activeIndex.value] ?? matches.value[0]
    if (target) {
      window.open(hrefOf(target), target.openMode === 'self' ? '_self' : '_blank')
      query.value = ''
    }
    return
  }
  const eng = props.engines.find((e) => e.name === engine.value)
  if (eng) window.open(eng.url.replace('%s', encodeURIComponent(q)), '_blank')
}

function move(delta: number) {
  if (!matches.value.length) return
  activeIndex.value = (activeIndex.value + delta + matches.value.length) % matches.value.length
}

// 全局快捷键：/ 或 Ctrl+K 聚焦搜索框，Esc 清空并失焦。
function onKeydown(e: KeyboardEvent) {
  const el = e.target as HTMLElement | null
  const typing = el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable)
  if ((e.key === '/' && !typing) || ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k')) {
    e.preventDefault()
    inputRef.value?.focus()
  } else if (e.key === 'Escape' && typing) {
    query.value = ''
    inputRef.value?.blur()
  }
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))

const engineOptions = computed(() => [
  { label: t('search.local'), value: 'local' },
  ...props.engines.map((e) => ({ label: e.name, value: e.name })),
])

// 下拉项右侧放一颗星：点亮即把该引擎设为默认（立即落库，由父组件处理）。
// 下拉是 teleport 到 body 的，scoped 样式够不着，所以这里全部用行内样式。
// 点星星要 stopPropagation，否则会触发选项选中、菜单也关了，看不见星星亮起来。
function renderEngineOption({ node, option }: { node: VNode; option: SelectOption }) {
  const name = String(option.value)
  const isDefault = name === (props.defaultEngine || 'local')
  return h('div', { style: 'display:flex; align-items:center; gap:8px; width:100%' }, [
    node,
    h(
      'span',
      {
        title: t('search.setDefault'),
        style: `margin-left:auto; display:flex; cursor:pointer; font-size:15px; ${
          isDefault ? 'color:#f0a020;' : 'opacity:0.35;'
        }`,
        onClick: (e: MouseEvent) => {
          e.stopPropagation()
          e.preventDefault()
          if (!isDefault) emit('set-default', name)
        },
      },
      [h(Icon, { icon: isDefault ? 'mdi:star' : 'mdi:star-outline' })],
    ),
  ])
}
</script>

<template>
  <div class="relative w-full max-w-xl mx-auto">
    <div
      class="tp-surface-glass tp-searchbar rounded-full flex items-center pl-1 pr-1.5 py-1"
      :class="{ 'tp-search-tinted': tinted }"
      :style="wrapStyle"
    >
      <n-select
        v-model:value="engine"
        :options="engineOptions"
        :render-option="renderEngineOption"
        size="small"
        class="w-28 shrink-0"
        :consistent-menu-width="false"
        :placeholder="t('search.engine')"
      />
      <n-input
        ref="inputRef"
        v-model:value="query"
        :placeholder="
          engine === 'local' ? t('search.placeholderLocal') : t('search.placeholderEngine', { engine })
        "
        class="flex-1"
        :bordered="false"
        @keydown.enter.prevent="submit"
        @keydown.down.prevent="move(1)"
        @keydown.up.prevent="move(-1)"
      />
      <n-button quaternary circle size="small" @click="submit">
        <Icon icon="mdi:magnify" class="text-lg" />
      </n-button>
    </div>

    <!-- 站内搜索的候选列表，回车直接打开第一个（或方向键选中的那个） -->
    <div
      v-if="matches.length"
      class="absolute left-0 right-0 mt-2 rounded-xl overflow-hidden tp-surface-glass z-30"
    >
      <a
        v-for="(m, i) in matches"
        :key="m.id"
        :href="hrefOf(m)"
        :target="m.openMode === 'self' ? '_self' : '_blank'"
        rel="noopener noreferrer"
        class="tp-option flex items-center gap-2 px-4 py-2 no-underline text-sm"
        :class="{ 'tp-option-active': i === activeIndex }"
        @mouseenter="activeIndex = i"
      >
        <Icon icon="mdi:link-variant" class="opacity-60 shrink-0" />
        <span class="truncate">{{ m.title }}</span>
        <span class="tp-text-faint text-xs truncate ml-auto">{{ m.url }}</span>
      </a>
    </div>
  </div>
</template>

<style scoped>
/* 设置里的「搜索栏文字颜色」只有命中 naive 内部元素才有效，见 wrapStyle 的注释。 */
.tp-search-tinted :deep(.n-input__input-el),
.tp-search-tinted :deep(.n-base-selection-label),
.tp-search-tinted :deep(.n-base-selection-input) {
  color: var(--tp-search-color);
}

.tp-search-tinted :deep(.n-input__placeholder) {
  color: var(--tp-search-color);
  opacity: 0.55;
}
</style>
