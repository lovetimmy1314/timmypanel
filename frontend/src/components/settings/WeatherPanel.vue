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

// 地点搜索走后端代理（前端直连上游会被 CSP 挡掉，决策 033）。
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

// 「广州」和「广州市」算同一层，叠上去卡片上是废话。只剥行政区后缀，
// 不用 startsWith：否则 Springfield 会把 Spring 吃掉。
function foldPlace(s: string): string {
  return s.replace(/(特别行政区|自治区|自治州|地区|市|区|县|省|州|盟|旗)$/u, '')
}

function samePlace(a: string, b: string): boolean {
  return a === b || foldPlace(a) === foldPlace(b)
}

function uniqueParts(values: string[], skip: string[] = []): string[] {
  const out: string[] = []
  const seen = [...skip]
  for (const v of values) {
    const s = (v ?? '').trim()
    if (!s || seen.some((x) => samePlace(x, s))) continue
    seen.push(s)
    out.push(s)
  }
  return out
}

// 选中一条结果：显示名尽量精确到区。admin2 是市/区，admin1 是省/州。
function placeLabel(p: GeoPlace): string {
  return uniqueParts([p.name, p.admin2, p.admin1]).join('·') || p.name
}

function placeHint(p: GeoPlace): string {
  return uniqueParts([p.admin2, p.admin1, p.country], [p.name]).join(' · ')
}

function pick(p: GeoPlace) {
  draft.value.weather.city = placeLabel(p)
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
      <SettingsRow :label="t('weather.provider')" :hint="t('weather.providerHint')" stack>
        <n-radio-group v-model:value="draft.weather.provider" size="small">
          <n-radio-button value="open-meteo">{{ t('weather.providerOpenMeteo') }}</n-radio-button>
          <n-radio-button value="qweather">{{ t('weather.providerQWeather') }}</n-radio-button>
        </n-radio-group>
      </SettingsRow>
      <template v-if="draft.weather.provider === 'qweather'">
        <SettingsRow :label="t('weather.qweatherHost')" :hint="t('weather.qweatherHostHint')" stack>
          <n-input
            v-model:value="draft.weather.qweatherHost"
            placeholder="xxxxx.xx.qweatherapi.com"
            :maxlength="253"
          />
        </SettingsRow>
        <SettingsRow :label="t('weather.qweatherKey')" :hint="t('weather.qweatherKeyHint')" stack>
          <n-input
            v-model:value="draft.weather.qweatherKey"
            type="password"
            show-password-on="click"
            :maxlength="128"
            autocomplete="off"
          />
        </SettingsRow>
      </template>
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
              {{ placeHint(p) }}
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
