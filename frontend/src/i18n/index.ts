// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

import { computed, ref } from 'vue'
import { dateEnUS, dateZhCN, enUS, zhCN } from 'naive-ui'
import { zh, type MessageKey } from './zh'
import { en } from './en'

export type Locale = 'zh' | 'en'
export type { MessageKey }

// 语言的权威值在服务端设置里（和 theme 一样，换设备保持一致），但登录页读不到
// /settings，首帧也还没拿到——所以这里镜像一份到 localStorage 当**起手值**。
// 登录后 panel store 会用服务端的值再盖一次。
const STORAGE_KEY = 'tp-locale'

const dicts: Record<Locale, Record<string, string>> = { zh, en }

function stored(): Locale {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    if (v === 'zh' || v === 'en') return v
  } catch {
    // 隐私模式下 localStorage 会抛，忽略即可
  }
  // 不去嗅 navigator.language：后端默认是 zh，猜出别的值只会在登录后被服务端
  // 设置盖掉，界面当场闪一下语言。
  return 'zh'
}

export const locale = ref<Locale>(stored())

export function setLocale(next: Locale) {
  if (next !== 'zh' && next !== 'en') return
  locale.value = next
  document.documentElement.lang = next === 'zh' ? 'zh-CN' : 'en'
  try {
    localStorage.setItem(STORAGE_KEY, next)
  } catch {
    // 同上
  }
}

// t 在渲染期读 locale.value，所以切语言时用到它的模板会自动重算。
export function t(key: MessageKey, vars?: Record<string, string | number>): string {
  const raw = dicts[locale.value][key] ?? zh[key] ?? key
  if (!vars) return raw
  return raw.replace(/\{(\w+)\}/g, (m, name: string) => (name in vars ? String(vars[name]) : m))
}

// naive-ui 自带的文案（日期选择、上传、空状态）也要跟着切。
export const naiveLocale = computed(() => (locale.value === 'zh' ? zhCN : enUS))
export const naiveDateLocale = computed(() => (locale.value === 'zh' ? dateZhCN : dateEnUS))

// 日期/时间用的 BCP 47 标签，给 toLocaleString 系列用。
export const dateLocale = computed(() => (locale.value === 'zh' ? 'zh-CN' : 'en-US'))

export function useI18n() {
  return { t, locale, setLocale, dateLocale }
}

setLocale(locale.value)
