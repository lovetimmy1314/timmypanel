<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
// 天气组件的设置：开关、定位方式、城市、温度单位。
import { computed, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { api } from '@/api/http'
import { usePanelStore } from '@/stores/panel'
import { deepClone } from '@/utils/clone'
import { locale, t } from '@/i18n'
import SettingsSection from './SettingsSection.vue'
import SettingsRow from './SettingsRow.vue'
import type { GeoPlace, Settings } from '@/api/types'

const emit = defineEmits<{ changed: [] }>()

const panel = usePanelStore()
const message = useMessage()
const draft = ref<Settings>(deepClone(panel.settings))
const saving = ref(false)

const keyword = ref('')
const searching = ref(false)
const results = ref<GeoPlace[]>([])
const searched = ref(false)

const currentCity = computed(() => {
  const w = draft.value.weather
  if (w.city) return w.city
  if (w.lat !== 0 || w.lon !== 0) return `${w.lat}, ${w.lon}`
  return t('weather.notSet')
})

// 城市搜索走后端代理（前端直连上游会被 CSP 挡掉，决策 033）。
async function search() {
  const q = keyword.value.trim()
  if (!q) return
  searching.value = true
  try {
    const res = await api.get<{ items: GeoPlace[] }>(
      `/weather/geocode?${new URLSearchParams({ q, lang: locale.value })}`,
    )
    results.value = res.items ?? []
    searched.value = true
  } catch (e: any) {
    message.error(e?.message ?? t('weather.searchFailed'))
  } finally {
    searching.value = false
  }
}

// 选中一条结果：城市名带上省份，同名城市（好几个 Springfield）才分得清。
function pick(p: GeoPlace) {
  draft.value.weather.city = p.admin1 && p.admin1 !== p.name ? `${p.name}·${p.admin1}` : p.name
  draft.value.weather.lat = p.lat
  draft.value.weather.lon = p.lon
  results.value = []
  keyword.value = ''
  searched.value = false
}

function clearCity() {
  draft.value.weather.city = ''
  draft.value.weather.lat = 0
  draft.value.weather.lon = 0
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
    <SettingsSection :title="t('settings.nav.weather')">
      <SettingsRow :label="t('appearance.show')" :hint="t('weather.pcOnlyHint')">
        <n-switch v-model:value="draft.weather.enabled" size="small" />
      </SettingsRow>
      <SettingsRow :label="t('weather.unit')">
        <n-radio-group v-model:value="draft.weather.unit" size="small">
          <n-radio-button value="c">°C</n-radio-button>
          <n-radio-button value="f">°F</n-radio-button>
        </n-radio-group>
      </SettingsRow>
    </SettingsSection>

    <SettingsSection :title="t('weather.location')">
      <SettingsRow :label="t('weather.locationMode')" :hint="draft.weather.locationMode === 'auto' ? t('weather.modeAutoHint') : ''">
        <n-radio-group v-model:value="draft.weather.locationMode" size="small">
          <n-radio-button value="manual">{{ t('weather.modeManual') }}</n-radio-button>
          <n-radio-button value="auto">{{ t('weather.modeAuto') }}</n-radio-button>
        </n-radio-group>
      </SettingsRow>

      <SettingsRow :label="t('weather.current')">
        <div class="flex items-center gap-2 min-w-0">
          <span class="text-[13px] truncate">{{ currentCity }}</span>
          <n-button
            v-if="draft.weather.city || draft.weather.lat || draft.weather.lon"
            size="tiny"
            quaternary
            @click="clearCity"
          >
            {{ t('common.clear') }}
          </n-button>
        </div>
      </SettingsRow>

      <SettingsRow :label="t('weather.searchCity')" stack>
        <n-input-group>
          <n-input
            v-model:value="keyword"
            :placeholder="t('weather.searchPlaceholder')"
            :maxlength="32"
            @keyup.enter="search"
          />
          <n-button type="primary" :loading="searching" @click="search">
            {{ t('common.search') }}
          </n-button>
        </n-input-group>

        <div v-if="results.length" class="mt-2 space-y-1">
          <button
            v-for="p in results"
            :key="`${p.lat},${p.lon}`"
            type="button"
            class="tp-option w-full flex items-center gap-2 rounded-lg px-2 py-1.5 text-left text-[13px]"
            @click="pick(p)"
          >
            <Icon icon="mdi:map-marker-outline" class="text-base opacity-60 shrink-0" />
            <span class="truncate">{{ p.name }}</span>
            <span class="ml-auto shrink-0 text-xs opacity-50">
              {{ [p.admin1, p.country].filter(Boolean).join(' · ') }}
            </span>
          </button>
        </div>
        <p v-else-if="searched && !searching" class="mt-2 text-xs opacity-45">
          {{ t('weather.noResult') }}
        </p>
      </SettingsRow>
    </SettingsSection>

    <div class="sticky bottom-0 -mx-0.5 px-0.5 py-2 flex justify-end bg-gradient-to-t from-black/[0.06] dark:from-white/[0.06] to-transparent backdrop-blur-sm">
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.saveSettings') }}</n-button>
    </div>
  </div>
</template>
