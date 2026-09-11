<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
// 快捷访问卡片的设置：开关、从已收录网站中勾选与排序。
import { computed, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { VueDraggable } from 'vue-draggable-plus'
import { usePanelStore } from '@/stores/panel'
import { deepClone } from '@/utils/clone'
import { iconSetReady, loadIconSet } from '@/icons'
import { t } from '@/i18n'
import SettingsSection from './SettingsSection.vue'
import SettingsRow from './SettingsRow.vue'
import type { Group, Settings, Site } from '@/api/types'

const emit = defineEmits<{ changed: [] }>()

const panel = usePanelStore()
const message = useMessage()
const draft = ref<Settings>(deepClone(panel.settings))
const saving = ref(false)
const searchKeyword = ref('')

// 确保 quickAccess 字段及其 siteIds 数组存在
if (!draft.value.quickAccess) {
  draft.value.quickAccess = { enabled: true, siteIds: [] }
}
if (!draft.value.quickAccess.siteIds) {
  draft.value.quickAccess.siteIds = []
}

// 加载图标库（如果有使用 iconify 图标的站点）
watch(
  () => panel.sites,
  (sites) => {
    if (sites.some((s) => s.iconType === 'iconify')) {
      loadIconSet()
    }
  },
  { immediate: true },
)

const siteMap = computed(() => {
  const map = new Map<number, Site>()
  for (const s of panel.sites) {
    map.set(s.id, s)
  }
  return map
})

const selectedSet = computed(() => new Set(draft.value.quickAccess.siteIds))

// 已选网站列表（保持用户调整的顺序，过滤掉已被删除的站点）
const selectedSitesList = computed({
  get() {
    return draft.value.quickAccess.siteIds
      .map((id) => siteMap.value.get(id))
      .filter((s): s is Site => !!s)
  },
  set(newSites: Site[]) {
    draft.value.quickAccess.siteIds = newSites.map((s) => s.id)
  },
})

function isSelected(siteId: number): boolean {
  return selectedSet.value.has(siteId)
}

function toggleSite(siteId: number) {
  const idx = draft.value.quickAccess.siteIds.indexOf(siteId)
  if (idx >= 0) {
    draft.value.quickAccess.siteIds.splice(idx, 1)
  } else {
    draft.value.quickAccess.siteIds.push(siteId)
  }
}

function removeSelected(siteId: number) {
  const idx = draft.value.quickAccess.siteIds.indexOf(siteId)
  if (idx >= 0) {
    draft.value.quickAccess.siteIds.splice(idx, 1)
  }
}

// 搜索过滤后的分组网站
const filteredGroups = computed(() => {
  const q = searchKeyword.value.trim().toLowerCase()
  return panel.grouped
    .map((g) => {
      const filtered = g.sites.filter((s) => {
        if (!q) return true
        return (
          s.title.toLowerCase().includes(q) ||
          s.url.toLowerCase().includes(q) ||
          (s.description && s.description.toLowerCase().includes(q))
        )
      })
      return { group: g.group, sites: filtered }
    })
    .filter((g) => g.sites.length > 0)
})

const allVisibleSiteIds = computed(() => {
  const ids: number[] = []
  for (const g of filteredGroups.value) {
    for (const s of g.sites) {
      ids.push(s.id)
    }
  }
  return ids
})

function selectAllVisible() {
  const set = new Set(draft.value.quickAccess.siteIds)
  for (const id of allVisibleSiteIds.value) {
    if (!set.has(id)) {
      draft.value.quickAccess.siteIds.push(id)
      set.add(id)
    }
  }
}

function clearAll() {
  draft.value.quickAccess.siteIds = []
}

function isGroupAllSelected(sites: Site[]): boolean {
  if (!sites.length) return false
  return sites.every((s) => selectedSet.value.has(s.id))
}

function toggleGroupSites(sites: Site[]) {
  if (isGroupAllSelected(sites)) {
    const toRemove = new Set(sites.map((s) => s.id))
    draft.value.quickAccess.siteIds = draft.value.quickAccess.siteIds.filter((id) => !toRemove.has(id))
  } else {
    const set = new Set(draft.value.quickAccess.siteIds)
    for (const s of sites) {
      if (!set.has(s.id)) {
        draft.value.quickAccess.siteIds.push(s.id)
        set.add(s.id)
      }
    }
  }
}

// 图标辅助函数
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

async function save() {
  saving.value = true
  try {
    await panel.saveSettings(draft.value)
    draft.value = deepClone(panel.settings)
    message.success(t('common.saved'))
    emit('changed')
  } catch (e: any) {
    message.error(e?.message ?? t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="space-y-3">
    <SettingsSection :title="t('settings.nav.quickAccess')">
      <SettingsRow :label="t('appearance.show')" :hint="t('quickAccess.pcOnlyHint')">
        <n-switch v-model:value="draft.quickAccess.enabled" size="small" />
      </SettingsRow>
    </SettingsSection>

    <SettingsSection :title="t('quickAccess.selectSites')">
      <p class="text-xs opacity-50 mb-3">{{ t('quickAccess.selectSitesHint') }}</p>

      <!-- 搜索与快捷操作 -->
      <div class="flex flex-wrap items-center justify-between gap-2 mb-3">
        <div class="w-full sm:w-64">
          <n-input
            v-model:value="searchKeyword"
            size="small"
            :placeholder="t('quickAccess.searchPlaceholder')"
            clearable
          >
            <template #prefix>
              <Icon icon="mdi:magnify" class="opacity-50" />
            </template>
          </n-input>
        </div>
        <div class="flex items-center gap-2">
          <n-button size="tiny" tertiary @click="selectAllVisible">
            {{ t('quickAccess.selectAll') }}
          </n-button>
          <n-button size="tiny" tertiary @click="clearAll">
            {{ t('quickAccess.clearAll') }}
          </n-button>
          <span class="tp-pill text-xs">
            {{ t('quickAccess.selectedCount', { n: draft.quickAccess.siteIds.length }) }}
          </span>
        </div>
      </div>

      <!-- 已选列表（可拖拽排序） -->
      <div v-if="selectedSitesList.length > 0" class="mb-4 space-y-1.5">
        <div class="text-xs font-medium opacity-60">{{ t('quickAccess.selectedOrder') }}</div>
        <VueDraggable
          v-model="selectedSitesList"
          :animation="150"
          handle=".tp-drag-handle"
          class="flex flex-wrap gap-1.5 p-2 rounded-lg bg-black/[0.03] dark:bg-white/[0.04] border border-black/5 dark:border-white/[0.06] max-h-36 overflow-y-auto"
        >
          <div
            v-for="site in selectedSitesList"
            :key="site.id"
            class="flex items-center gap-1.5 px-2 py-1 rounded-md bg-white/70 dark:bg-black/30 border border-black/5 dark:border-white/10 text-xs select-none shadow-xs group"
          >
            <Icon icon="mdi:drag" class="tp-drag-handle cursor-grab opacity-40 hover:opacity-100" />
            <div
              class="w-4 h-4 rounded shrink-0 overflow-hidden flex items-center justify-center text-[9px] text-white font-semibold"
              :style="site.iconType === 'url' && site.iconValue ? undefined : { background: siteFallbackBg(site) }"
            >
              <img
                v-if="site.iconType === 'url' && site.iconValue"
                :src="site.iconValue"
                class="w-full h-full object-contain"
                alt=""
              />
              <Icon
                v-else-if="site.iconType === 'iconify' && site.iconValue && iconSetReady"
                :icon="site.iconValue"
                class="text-xs"
              />
              <span v-else>{{ siteGlyph(site) }}</span>
            </div>
            <span class="max-w-[90px] truncate">{{ site.title }}</span>
            <button
              type="button"
              class="opacity-40 hover:opacity-100 hover:text-red-500 cursor-pointer p-0 bg-transparent border-0 flex items-center"
              @click.stop="removeSelected(site.id)"
            >
              <Icon icon="mdi:close" class="text-xs" />
            </button>
          </div>
        </VueDraggable>
      </div>

      <!-- 无收录网站提示 -->
      <div v-if="!panel.sites.length" class="text-center py-8 text-xs opacity-50">
        {{ t('quickAccess.noSitesAvailable') }}
      </div>

      <!-- 按分组勾选网站 -->
      <div v-else class="space-y-4 max-h-[380px] overflow-y-auto pr-1">
        <div
          v-for="groupItem in filteredGroups"
          :key="groupItem.group.id"
          class="rounded-lg border border-black/5 dark:border-white/[0.06] p-2.5 bg-black/[0.01] dark:bg-white/[0.02]"
        >
          <div class="flex items-center justify-between gap-2 mb-2 pb-1.5 border-b border-black/5 dark:border-white/[0.06]">
            <div class="flex items-center gap-1.5 text-xs font-medium opacity-75">
              <span>{{ groupItem.group.name }}</span>
              <span class="tp-pill text-[10px]">{{ groupItem.sites.length }}</span>
            </div>
            <n-button
              size="tiny"
              quaternary
              @click="toggleGroupSites(groupItem.sites)"
            >
              {{ isGroupAllSelected(groupItem.sites) ? t('quickAccess.unselectGroup') : t('quickAccess.selectGroup') }}
            </n-button>
          </div>

          <div class="grid grid-cols-1 sm:grid-cols-2 gap-1.5">
            <div
              v-for="site in groupItem.sites"
              :key="site.id"
              class="flex items-center gap-2 p-1.5 rounded-md hover:bg-black/5 dark:hover:bg-white/10 transition-colors cursor-pointer"
              @click="toggleSite(site.id)"
            >
              <n-checkbox
                :checked="isSelected(site.id)"
                size="small"
                @click.stop="toggleSite(site.id)"
              />
              <div
                class="w-6 h-6 rounded-md shrink-0 overflow-hidden flex items-center justify-center text-xs text-white font-semibold"
                :style="site.iconType === 'url' && site.iconValue ? undefined : { background: siteFallbackBg(site) }"
              >
                <img
                  v-if="site.iconType === 'url' && site.iconValue"
                  :src="site.iconValue"
                  class="w-full h-full object-contain"
                  alt=""
                />
                <Icon
                  v-else-if="site.iconType === 'iconify' && site.iconValue && iconSetReady"
                  :icon="site.iconValue"
                  class="text-sm"
                />
                <span v-else>{{ siteGlyph(site) }}</span>
              </div>
              <div class="min-w-0 flex-1">
                <div class="text-xs font-medium truncate">{{ site.title }}</div>
                <div v-if="site.description" class="text-[11px] opacity-45 truncate">
                  {{ site.description }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </SettingsSection>

    <div class="sticky bottom-0 -mx-0.5 px-0.5 py-2 flex justify-end bg-gradient-to-t from-black/[0.06] dark:from-white/[0.06] to-transparent backdrop-blur-sm">
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.saveSettings') }}</n-button>
    </div>
  </div>
</template>
