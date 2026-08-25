// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

import type { MessageKey } from '@/i18n'

// WMO weather code 有几十个取值，图标和文案不需要分那么细，
// 收成这 8 类。上游给出未知取值时回落到 cloudy（画一朵云，不至于开天窗）。
export type WeatherGroup =
  | 'clear'
  | 'partly'
  | 'cloudy'
  | 'fog'
  | 'drizzle'
  | 'rain'
  | 'snow'
  | 'thunder'

// 取值表见 https://open-meteo.com 的 WMO Weather interpretation codes。
const GROUPS: [number[], WeatherGroup][] = [
  [[0], 'clear'],
  [[1, 2], 'partly'],
  [[3], 'cloudy'],
  [[45, 48], 'fog'],
  [[51, 53, 55, 56, 57], 'drizzle'],
  [[61, 63, 65, 66, 67, 80, 81, 82], 'rain'],
  [[71, 73, 75, 77, 85, 86], 'snow'],
  [[95, 96, 99], 'thunder'],
]

export function weatherGroup(code: number): WeatherGroup {
  for (const [codes, group] of GROUPS) {
    if (codes.includes(code)) return group
  }
  return 'cloudy'
}

// 天气文案的词条键。i18n 的 t() 要的是 MessageKey，这里拼出来的字符串得断言回去。
export function weatherLabelKey(code: number): MessageKey {
  return `weather.cond.${weatherGroup(code)}` as MessageKey
}

const WIND_DIRS = new Set([
  'n', 'nne', 'ne', 'ene', 'e', 'ese', 'se', 'sse',
  's', 'ssw', 'sw', 'wsw', 'w', 'wnw', 'nw', 'nnw',
])

export function windDirKey(dir: string): MessageKey | '' {
  const k = dir.toLowerCase()
  if (!WIND_DIRS.has(k)) return ''
  return `weather.dir.${k}` as MessageKey
}

// 摄氏转华氏。接口一律回摄氏度，单位只是显示偏好（决策 033）。
export function formatTemp(celsius: number, unit: 'c' | 'f'): string {
  const v = unit === 'f' ? celsius * 1.8 + 32 : celsius
  return `${Math.round(v)}°`
}
