<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
// 首页左上角的悬浮天气。只在桌面端由 Home 挂载（useIsDesktop），移动端根本不存在
// 这个组件——不是用 CSS 藏起来，那样定时器和请求照跑。
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { api } from '@/api/http'
import { usePanelStore } from '@/stores/panel'
import { dateLocale, t } from '@/i18n'
import { formatTemp, weatherLabelKey } from '@/utils/weather'
import WeatherIcon from './WeatherIcon.vue'
import type { Weather } from '@/api/types'

const emit = defineEmits<{ configure: [] }>()

const panel = usePanelStore()
const conf = computed(() => panel.settings.weather)

// 刷新间隔。上游数据本身就是 15 分钟一档，再密只是白打接口
// （而且服务端还有 10 分钟缓存，密了也只会拿到同一份）。
const REFRESH_MS = 10 * 60 * 1000
// 浏览器定位结果的本地有效期。定位是要弹授权的，拿到一次就先用一天。
const GEO_TTL_MS = 24 * 60 * 60 * 1000
const GEO_KEY = 'tp-weather-geo'

type Coords = { lat: number; lon: number }

const data = ref<Weather | null>(null)
// unset = 还没配城市（此时一个请求都不发），error 只在手里连旧数据都没有时才显示。
const status = ref<'unset' | 'loading' | 'ok' | 'error'>('loading')
const expanded = ref(false)
const root = ref<HTMLElement | null>(null)
let lastAt = 0
let timer: number | undefined

function readGeoCache(): Coords | null {
  try {
    const raw = localStorage.getItem(GEO_KEY)
    if (!raw) return null
    const v = JSON.parse(raw) as Coords & { at: number }
    if (typeof v.lat !== 'number' || typeof v.lon !== 'number') return null
    if (Date.now() - v.at > GEO_TTL_MS) return null
    return { lat: v.lat, lon: v.lon }
  } catch {
    return null
  }
}

function writeGeoCache(c: Coords) {
  try {
    localStorage.setItem(GEO_KEY, JSON.stringify({ ...c, at: Date.now() }))
  } catch {
    // 隐私模式下会抛，忽略：大不了每次都重新问一次定位
  }
}

// 浏览器定位。需要 HTTPS 和用户授权，两者缺一都会失败——失败不是错误路径，
// 回落到设置里那个城市就行。
function locate(): Promise<Coords | null> {
  if (!navigator.geolocation) return Promise.resolve(null)
  return new Promise((resolve) => {
    navigator.geolocation.getCurrentPosition(
      (pos) => resolve({ lat: pos.coords.latitude, lon: pos.coords.longitude }),
      () => resolve(null),
      { timeout: 8000, maximumAge: GEO_TTL_MS },
    )
  })
}

// 设置里选过城市：city 非空，或坐标不是 (0,0)。判坐标是因为浏览器定位写回来的
// 那一份没有城市名。
const hasManual = computed(() => !!conf.value.city || conf.value.lat !== 0 || conf.value.lon !== 0)

async function resolveCoords(): Promise<Coords | null> {
  if (conf.value.locationMode === 'auto') {
    const cached = readGeoCache()
    if (cached) return cached
    const pos = await locate()
    if (pos) {
      writeGeoCache(pos)
      return pos
    }
  }
  if (hasManual.value) return { lat: conf.value.lat, lon: conf.value.lon }
  return null
}

async function load() {
  const coords = await resolveCoords()
  if (!coords) {
    status.value = 'unset'
    data.value = null
    return
  }
  if (!data.value) status.value = 'loading'
  try {
    const q = new URLSearchParams({ lat: String(coords.lat), lon: String(coords.lon) })
    data.value = await api.get<Weather>(`/weather?${q}`)
    status.value = 'ok'
    lastAt = Date.now()
  } catch {
    // 上游不通是常态（见 plans.md 的「已知限制」）。静默：手里有旧数据就继续显示，
    // 没有就显示 --。**不要**在这里弹 message，10 分钟一次的轮询会把界面刷爆。
    status.value = data.value ? 'ok' : 'error'
  }
}

// 到点才刷：定时器每分钟醒一次只是比时间戳，真正出站是 10 分钟一回。
function tick() {
  if (Date.now() - lastAt >= REFRESH_MS) load()
}

function onVisible() {
  if (document.visibilityState === 'visible') tick()
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') expanded.value = false
}

function onDocClick(e: MouseEvent) {
  if (expanded.value && root.value && !root.value.contains(e.target as Node)) expanded.value = false
}

onMounted(() => {
  load()
  timer = window.setInterval(tick, 60_000)
  document.addEventListener('visibilitychange', onVisible)
  document.addEventListener('keydown', onKey)
  document.addEventListener('click', onDocClick)
})

onUnmounted(() => {
  if (timer) window.clearInterval(timer)
  document.removeEventListener('visibilitychange', onVisible)
  document.removeEventListener('keydown', onKey)
  document.removeEventListener('click', onDocClick)
})

// 设置里换了城市/定位方式就立刻重取。lastAt 归零，免得被 tick 的间隔挡住。
watch(
  () =>
    [
      conf.value.locationMode,
      conf.value.city,
      conf.value.lat,
      conf.value.lon,
      conf.value.provider,
      conf.value.qweatherHost,
    ].join('|'),
  () => {
    lastAt = 0
    load()
  },
)

const tempText = computed(() =>
  data.value ? formatTemp(data.value.tempC, conf.value.unit) : '--',
)
const cityText = computed(() => {
  if (conf.value.city) return conf.value.city
  return conf.value.locationMode === 'auto' ? t('weather.myLocation') : ''
})
const condText = computed(() => (data.value ? t(weatherLabelKey(data.value.code)) : ''))
const rangeText = computed(() => {
  const d = data.value
  if (!d || d.maxC === null || d.minC === null) return '--'
  return `${formatTemp(d.minC, conf.value.unit)} / ${formatTemp(d.maxC, conf.value.unit)}`
})
const updatedText = computed(() =>
  lastAt || data.value
    ? new Date((data.value?.updatedAt ?? 0) * 1000).toLocaleTimeString(dateLocale.value, {
        hour: '2-digit',
        minute: '2-digit',
      })
    : '',
)
</script>

<template>
  <div ref="root" class="tp-float tp-float-tl">
    <div class="tp-float-card">
      <!-- 还没配城市：给一个入口，不发任何请求，也不弹定位授权 -->
      <button
        v-if="status === 'unset'"
        class="tp-float-pill"
        type="button"
        @click="emit('configure')"
      >
        <Icon icon="mdi:map-marker-plus-outline" class="text-xl tp-text-soft" />
        <span class="text-[13px] tp-text-soft">{{ t('weather.setCity') }}</span>
      </button>

      <template v-else>
        <button class="tp-float-pill" type="button" :title="condText" @click="expanded = !expanded">
          <WeatherIcon
            v-if="data"
            :code="data.code"
            :is-day="data.isDay"
            :size="30"
            class="shrink-0"
          />
          <Icon
            v-else
            :icon="status === 'loading' ? 'mdi:weather-cloudy-clock' : 'mdi:weather-cloudy-alert'"
            class="text-2xl tp-text-dim shrink-0"
            :class="status === 'loading' ? 'tp-float-pulse' : ''"
          />
          <span class="min-w-0 text-left leading-tight">
            <span class="block text-[15px] font-medium tp-text tabular-nums">{{ tempText }}</span>
            <span v-if="cityText" class="block max-w-[92px] truncate text-[11px] tp-text-dim">
              {{ cityText }}
            </span>
          </span>
        </button>

        <Transition name="tp-float-detail">
          <div v-if="expanded" class="tp-float-detail">
            <div v-if="data" class="space-y-1 text-[11px] tp-text-dim">
              <div class="tp-text-soft text-[12px]">{{ condText }}</div>
              <div>{{ t('weather.feelsLike') }} {{ formatTemp(data.feelsLikeC, conf.unit) }}</div>
              <div>{{ t('weather.range') }} {{ rangeText }}</div>
              <div>{{ t('weather.humidity') }} {{ data.humidity }}%</div>
              <div>{{ t('weather.wind') }} {{ Math.round(data.windKph) }} km/h</div>
              <div class="tp-text-faint">{{ t('weather.updatedAt', { time: updatedText }) }}</div>
            </div>
            <div v-else class="text-[11px] tp-text-dim">{{ t('weather.unavailable') }}</div>
            <div class="mt-1.5 flex justify-end">
              <button class="tp-cal-back" type="button" @click="emit('configure')">
                {{ t('weather.settings') }}
              </button>
            </div>
          </div>
        </Transition>
      </template>
    </div>
  </div>
</template>
