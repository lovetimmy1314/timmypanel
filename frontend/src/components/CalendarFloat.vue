<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
// 首页右上角的悬浮万年历。只在桌面端由 Home 挂载（useIsDesktop），移动端根本不存在
// 这个组件——不是用 CSS 藏起来，那样定时器和请求照跑。
//
// 农历/节气/节日/调休全部由后端算好（决策 034），这里只排版和缓存。
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { api } from '@/api/http'
import { usePanelStore } from '@/stores/panel'
import { dateLocale, t } from '@/i18n'
import type { CalendarDay, CalendarMonth } from '@/api/types'

const emit = defineEmits<{ configure: [] }>()

const panel = usePanelStore()
const conf = computed(() => panel.settings.calendar)

// 本地日期串。**一律按浏览器本地时区算**：后端不知道用户在哪个时区，
// 用 toISOString() 会先转 UTC，东八区的晚上八点就成了「昨天」。
function localKey(d: Date) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

// 反过来把日期串解回 Date。**不能用 new Date('2026-08-24')**：那种写法按 ISO
// 规矩解成 UTC 零点，西半球的浏览器再取 getDate() 就退回前一天了。
function parseKey(key: string) {
  const [y, m, d] = key.split('-').map(Number)
  return new Date(y, m - 1, d)
}

const now = new Date()
const today = ref(localKey(now))
const viewY = ref(now.getFullYear())
const viewM = ref(now.getMonth() + 1)
const expanded = ref(false)
// 翻月方向，只用来决定滑动过渡往哪边走。
const slide = ref<'next' | 'prev'>('next')

// 按 y-m 缓存整月数据。翻过的月份不再请求——连点十几次上/下月只该有十几次
// 首次请求，来回翻不该重复打。
const months = ref<Record<string, CalendarMonth>>({})
const pending = new Set<string>()

const monthKey = (y: number, m: number) => `${y}-${m}`
const viewKey = computed(() => monthKey(viewY.value, viewM.value))
const viewMonth = computed<CalendarMonth | undefined>(() => months.value[viewKey.value])

async function loadMonth(y: number, m: number) {
  const key = monthKey(y, m)
  if (months.value[key] || pending.has(key)) return
  pending.add(key)
  try {
    const q = new URLSearchParams({ y: String(y), m: String(m) })
    months.value[key] = await api.get<CalendarMonth>(`/calendar/month?${q}`)
  } catch {
    // 静默：卡片退回只显示公历那一行。这个接口是纯计算，失败基本只有掉登录
    // 一种可能，弹 message 只会在会话过期时刷屏。
  } finally {
    pending.delete(key)
  }
}

// 今天所在的那个月单独缓存一份引用，药丸那一行只认它。
const todayEntry = computed<CalendarDay | undefined>(() => {
  const d = parseKey(today.value)
  return months.value[monthKey(d.getFullYear(), d.getMonth() + 1)]?.items.find(
    (it) => it.date === today.value,
  )
})

function step(delta: number) {
  slide.value = delta > 0 ? 'next' : 'prev'
  const d = new Date(viewY.value, viewM.value - 1 + delta, 1)
  viewY.value = d.getFullYear()
  viewM.value = d.getMonth() + 1
}

function backToToday() {
  const d = parseKey(today.value)
  const target = d.getFullYear() * 12 + d.getMonth()
  slide.value = target >= viewY.value * 12 + viewM.value - 1 ? 'next' : 'prev'
  viewY.value = d.getFullYear()
  viewM.value = d.getMonth() + 1
}

const isThisMonth = computed(() => today.value.startsWith(`${viewY.value}-${String(viewM.value).padStart(2, '0')}`))

// 跨零点换天。每分钟比一次本地日期串，变了就把「今天」挪过去并把新的月份取回来
// ——半夜挂着的标签页第二天不该还高亮昨天。
let timer: number | undefined
function tick() {
  const key = localKey(new Date())
  if (key === today.value) return
  today.value = key
  const d = parseKey(key)
  loadMonth(d.getFullYear(), d.getMonth() + 1)
  if (!expanded.value) backToToday()
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') expanded.value = false
}

// 点面板外面收起来。面板不小，压着卡片时没法只靠再点一次药丸关掉。
const root = ref<HTMLElement | null>(null)
function onDocClick(e: MouseEvent) {
  if (expanded.value && root.value && !root.value.contains(e.target as Node)) expanded.value = false
}

onMounted(() => {
  loadMonth(viewY.value, viewM.value)
  timer = window.setInterval(tick, 60_000)
  document.addEventListener('keydown', onKey)
  document.addEventListener('click', onDocClick)
})

onUnmounted(() => {
  if (timer) window.clearInterval(timer)
  document.removeEventListener('keydown', onKey)
  document.removeEventListener('click', onDocClick)
})

watch(viewKey, () => loadMonth(viewY.value, viewM.value))

// ---- 显示用的文本 ----

// 公历那一行不依赖接口：接口挂了也照样显示日期和星期。
const dateText = computed(() =>
  parseKey(today.value).toLocaleDateString(dateLocale.value, { month: 'long', day: 'numeric' }),
)
const weekdayText = computed(() =>
  parseKey(today.value).toLocaleDateString(dateLocale.value, { weekday: 'short' }),
)
// 农历那一行在英文界面下**仍然是中文**：农历、节气、节日是中文历法专名，
// 没有通行英译，硬翻出来没人看得懂（决策 034）。
const lunarText = computed(() => {
  const e = todayEntry.value
  if (!e) return ''
  return e.festival || e.solarTerm || `${e.lunarMonth}${e.lunarDay}`
})
const yearText = computed(() => {
  const e = todayEntry.value
  return e?.ganZhi ? `${e.ganZhi}年 ${e.zodiac}` : ''
})
const monthTitle = computed(() =>
  new Date(viewY.value, viewM.value - 1, 1).toLocaleDateString(dateLocale.value, {
    year: 'numeric',
    month: 'long',
  }),
)

const weekdayKeys = ['calendar.w0', 'calendar.w1', 'calendar.w2', 'calendar.w3', 'calendar.w4', 'calendar.w5', 'calendar.w6'] as const
const weekLabels = computed(() =>
  conf.value.weekStart === 'sun'
    ? weekdayKeys.map((k) => t(k))
    : [...weekdayKeys.slice(1), weekdayKeys[0]].map((k) => t(k)),
)

type Cell = { key: string; num: number; day?: CalendarDay; outside: boolean }

// 固定排 6 行 42 格：格数跟着月份变的话，翻月时面板高度会跳一下，
// 滑动过渡看着就像抖动。
const cells = computed<Cell[]>(() => {
  const month = viewMonth.value
  const first = new Date(viewY.value, viewM.value - 1, 1)
  const lead = conf.value.weekStart === 'sun' ? first.getDay() : (first.getDay() + 6) % 7
  const out: Cell[] = []
  for (let i = 0; i < 42; i++) {
    const offset = i - lead
    const d = new Date(viewY.value, viewM.value - 1, offset + 1)
    const inMonth = offset >= 0 && offset < (month?.items.length ?? new Date(viewY.value, viewM.value, 0).getDate())
    out.push({
      key: localKey(d),
      num: d.getDate(),
      day: inMonth ? month?.items[offset] : undefined,
      outside: !inMonth,
    })
  }
  return out
})

// 格子里农历那一行：节日 > 节气 > 农历日；初一显示月名，好认出月份边界。
function subText(day: CalendarDay) {
  if (day.festival) return day.festival
  if (day.solarTerm) return day.solarTerm
  return day.lunarDay === '初一' ? day.lunarMonth : day.lunarDay
}
</script>

<template>
  <div ref="root" class="tp-float tp-float-tr">
    <div class="tp-float-card">
      <button class="tp-float-pill" type="button" @click="expanded = !expanded">
        <Icon icon="mdi:calendar-month-outline" class="text-2xl tp-text-dim shrink-0" />
        <span class="min-w-0 text-left leading-tight">
          <span class="block text-[15px] font-medium tp-text">
            {{ dateText }} <span class="tp-text-dim font-normal">{{ weekdayText }}</span>
          </span>
          <span v-if="lunarText" class="block truncate text-[11px] tp-text-dim">{{ lunarText }}</span>
        </span>
      </button>

      <!-- 月面板。scale+fade 展开，内容区翻月时左右滑动 -->
      <Transition name="tp-cal-pop">
        <div v-if="expanded" class="tp-cal-panel">
          <div class="flex items-center gap-1 px-1 pb-1.5">
            <button class="tp-cal-nav" type="button" :title="t('calendar.prevMonth')" @click="step(-1)">
              <Icon icon="mdi:chevron-left" class="text-lg" />
            </button>
            <span class="flex-1 text-center text-[13px] font-medium tp-text">{{ monthTitle }}</span>
            <button class="tp-cal-nav" type="button" :title="t('calendar.nextMonth')" @click="step(1)">
              <Icon icon="mdi:chevron-right" class="text-lg" />
            </button>
          </div>

          <div class="tp-cal-grid px-1 pb-1">
            <span v-for="w in weekLabels" :key="w" class="tp-cal-weekday">{{ w }}</span>
          </div>

          <div class="tp-cal-slider">
            <Transition :name="slide === 'next' ? 'tp-cal-next' : 'tp-cal-prev'">
              <div :key="viewKey" class="tp-cal-grid px-1">
                <div
                  v-for="c in cells"
                  :key="c.key"
                  class="tp-cal-cell"
                  :class="{
                    'tp-cal-outside': c.outside,
                    'tp-cal-today': c.key === today,
                    'tp-cal-weekend': !c.outside && (c.day?.weekday === 0 || c.day?.weekday === 6),
                    'tp-cal-fest': !!c.day?.festival || !!c.day?.solarTerm,
                  }"
                >
                  <span class="tp-cal-num">{{ c.num }}</span>
                  <span v-if="c.day" class="tp-cal-sub">{{ subText(c.day) }}</span>
                  <span v-else class="tp-cal-sub">&nbsp;</span>
                  <!-- 没有该年调休数据时一个角标都不画，只显示节日名 -->
                  <span
                    v-if="c.day?.dayType && viewMonth?.hasHolidayData"
                    class="tp-cal-badge"
                    :class="c.day.dayType === 'off' ? 'tp-cal-badge-off' : 'tp-cal-badge-work'"
                  >
                    {{ c.day.dayType === 'off' ? t('calendar.off') : t('calendar.work') }}
                  </span>
                </div>
              </div>
            </Transition>
          </div>

          <div class="tp-cal-foot">
            <span class="truncate">
              <template v-if="lunarText">{{ lunarText }}</template>
              <template v-if="yearText"> · {{ yearText }}</template>
            </span>
            <button
              v-if="!isThisMonth"
              class="tp-cal-back"
              type="button"
              @click="backToToday"
            >
              {{ t('calendar.backToToday') }}
            </button>
            <button class="tp-cal-back" type="button" @click="emit('configure')">
              {{ t('calendar.settings') }}
            </button>
          </div>
        </div>
      </Transition>
    </div>
  </div>
</template>
