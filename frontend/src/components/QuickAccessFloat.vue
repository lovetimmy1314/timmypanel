<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
// 首页右侧悬浮快捷访问卡片。只在桌面端由 Home 挂载（useIsDesktop）。
// 卡片上的网站只保留图标和名字，描述只有鼠标悬停的时候在 tooltip 出现。
import { computed, ref, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { usePanelStore } from '@/stores/panel'
import { iconSetReady, loadIconSet } from '@/icons'
import { t } from '@/i18n'
import type { Site } from '@/api/types'

const emit = defineEmits<{ configure: [] }>()

const panel = usePanelStore()
const collapsed = ref(false)
const failedIcons = ref<Set<number>>(new Set())

// 根据 siteIds 获取对应的站点列表
const quickSites = computed(() => {
  const map = new Map<number, Site>()
  for (const s of panel.sites) {
    if (!s.hidden) {
      map.set(s.id, s)
    }
  }
  const ids = panel.settings.quickAccess?.siteIds ?? []
  return ids.map((id) => map.get(id)).filter((s): s is Site => !!s)
})

// 图标库类型卡片按需异步加载 mdi 图标集
watch(
  quickSites,
  (sites) => {
    if (sites.some((s) => s.iconType === 'iconify')) {
      loadIconSet()
    }
  },
  { immediate: true },
)

function getHref(s: Site): string {
  return panel.settings.network === 'lan' && s.lanUrl ? s.lanUrl : s.url
}

function isLan(s: Site): boolean {
  return panel.settings.network === 'lan' && !!s.lanUrl
}

function markIconFailed(siteId: number) {
  failedIcons.value.add(siteId)
}

function siteGlyph(s: Site) {
  const v = s.iconValue.trim()
  if (s.iconType === 'text' && v) return v
  return (s.title || s.url).trim().charAt(0).toUpperCase()
}

function siteFallbackBg(s: Site) {
  if (s.iconBg) return s.iconBg
  let hash = 0
  for (const ch of s.title || s.url) hash = (hash * 31 + ch.charCodeAt(0)) >>> 0
  return `hsl(${hash % 360}, 55%, 45%)`
}
</script>

<template>
  <div>
    <!-- 折叠状态下的迷你侧边栏按钮 -->
    <button
      v-if="collapsed"
      type="button"
      class="tp-quick-collapsed-tab"
      :title="t('quickAccess.expand')"
      @click="collapsed = false"
    >
      <Icon icon="mdi:flash-outline" class="text-base text-sky-500 mb-1" />
      <span>{{ t('quickAccess.title') }}</span>
    </button>

    <!-- 展开状态下的悬浮卡片 -->
    <div v-else class="tp-quick-float">
      <div class="tp-quick-card">
        <!-- 卡片头部 -->
        <div class="flex items-center justify-between gap-1.5 px-1.5 py-1 mb-1 border-b border-[var(--tp-surface-border)]">
          <div class="flex items-center gap-1.5 min-w-0">
            <Icon icon="mdi:flash-outline" class="text-base text-sky-500 shrink-0" />
            <span class="text-xs font-semibold tp-text tracking-wide truncate">
              {{ t('quickAccess.title') }}
            </span>
            <span v-if="quickSites.length > 0" class="tp-pill text-[10px]">
              {{ quickSites.length }}
            </span>
          </div>
          <div class="flex items-center gap-0.5 shrink-0">
            <button
              type="button"
              class="w-5 h-5 rounded flex items-center justify-center text-xs opacity-50 hover:opacity-100 hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer p-0 bg-transparent border-0"
              :title="t('quickAccess.settings')"
              @click="emit('configure')"
            >
              <Icon icon="mdi:cog-outline" />
            </button>
            <button
              type="button"
              class="w-5 h-5 rounded flex items-center justify-center text-xs opacity-50 hover:opacity-100 hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer p-0 bg-transparent border-0"
              :title="t('quickAccess.collapse')"
              @click="collapsed = true"
            >
              <Icon icon="mdi:chevron-right" />
            </button>
          </div>
        </div>

        <!-- 网站列表内容区 -->
        <div v-if="quickSites.length > 0" class="space-y-0.5 overflow-y-auto max-h-[calc(100vh-220px)] p-0.5 tp-quick-list">
          <template v-for="site in quickSites" :key="site.id">
            <!-- 有描述时鼠标悬停展示描述，无描述时不展示 tooltip -->
            <n-tooltip v-if="site.description" trigger="hover" placement="left" :delay="150">
              <template #trigger>
                <a
                  class="tp-quick-item"
                  :href="getHref(site)"
                  :target="site.openMode === 'self' ? '_self' : '_blank'"
                  rel="noopener noreferrer"
                >
                  <div
                    class="w-5 h-5 rounded shrink-0 overflow-hidden flex items-center justify-center text-[10px] text-white font-semibold"
                    :style="
                      site.iconType === 'url' && site.iconValue && !failedIcons.has(site.id)
                        ? undefined
                        : { background: siteFallbackBg(site) }
                    "
                  >
                    <img
                      v-if="site.iconType === 'url' && site.iconValue && !failedIcons.has(site.id)"
                      :src="site.iconValue"
                      class="w-full h-full object-contain"
                      loading="lazy"
                      alt=""
                      @error="markIconFailed(site.id)"
                    />
                    <Icon
                      v-else-if="site.iconType === 'iconify' && site.iconValue && iconSetReady"
                      :icon="site.iconValue"
                      class="text-xs"
                    />
                    <span v-else>{{ siteGlyph(site) }}</span>
                  </div>
                  <span class="tp-text text-xs font-medium truncate flex-1 min-w-0">
                    {{ site.title }}
                  </span>
                  <Icon
                    v-if="isLan(site)"
                    icon="mdi:lan-connect"
                    class="shrink-0 text-emerald-500 dark:text-emerald-300 text-[10px]"
                    :title="t('home.lanBadge')"
                  />
                </a>
              </template>
              <span class="inline-block max-w-xs whitespace-pre-wrap break-words text-xs">
                {{ site.description }}
              </span>
            </n-tooltip>

            <!-- 无描述时直接展示链接 -->
            <a
              v-else
              class="tp-quick-item"
              :href="getHref(site)"
              :target="site.openMode === 'self' ? '_self' : '_blank'"
              rel="noopener noreferrer"
            >
              <div
                class="w-5 h-5 rounded shrink-0 overflow-hidden flex items-center justify-center text-[10px] text-white font-semibold"
                :style="
                  site.iconType === 'url' && site.iconValue && !failedIcons.has(site.id)
                    ? undefined
                    : { background: siteFallbackBg(site) }
                "
              >
                <img
                  v-if="site.iconType === 'url' && site.iconValue && !failedIcons.has(site.id)"
                  :src="site.iconValue"
                  class="w-full h-full object-contain"
                  loading="lazy"
                  alt=""
                  @error="markIconFailed(site.id)"
                />
                <Icon
                  v-else-if="site.iconType === 'iconify' && site.iconValue && iconSetReady"
                  :icon="site.iconValue"
                  class="text-xs"
                />
                <span v-else>{{ siteGlyph(site) }}</span>
              </div>
              <span class="tp-text text-xs font-medium truncate flex-1 min-w-0">
                {{ site.title }}
              </span>
              <Icon
                v-if="isLan(site)"
                icon="mdi:lan-connect"
                class="shrink-0 text-emerald-500 dark:text-emerald-300 text-[10px]"
                :title="t('home.lanBadge')"
              />
            </a>
          </template>
        </div>

        <!-- 暂无勾选网站时的空状态 -->
        <div v-else class="text-center py-6 px-2">
          <Icon icon="mdi:bookmark-plus-outline" class="text-2xl tp-text-dim mx-auto mb-1.5 opacity-60" />
          <p class="text-xs tp-text-dim mb-2.5">{{ t('quickAccess.empty') }}</p>
          <n-button size="tiny" type="primary" secondary @click="emit('configure')">
            {{ t('quickAccess.toSettings') }}
          </n-button>
        </div>
      </div>
    </div>
  </div>
</template>
