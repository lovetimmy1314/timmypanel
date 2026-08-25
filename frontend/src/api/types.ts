// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

export interface User {
  id: number
  username: string
  nickname: string
  role: 'admin' | 'user'
  disabled: boolean
}

export interface Group {
  id: number
  name: string
  sort: number
  collapsed: boolean
}

export type IconType = 'url' | 'iconify' | 'text'

export interface Site {
  id: number
  groupId: number
  title: string
  url: string
  lanUrl: string
  description: string
  iconType: IconType
  iconValue: string
  iconBg: string
  openMode: 'blank' | 'self'
  sort: number
  hidden: boolean
}

export interface SearchEngine {
  name: string
  url: string
  icon: string
}

// 附加搜索框：只有「显不显示」和「默认搜索源」，引擎清单和配色跟主搜索框共用。
export interface SearchBar {
  enabled: boolean
  default: string
}

export interface Settings {
  background: {
    type: 'image' | 'color' | 'gradient'
    value: string
    blur: number
    mask: number
  }
  layout: {
    cardSize: 'sm' | 'md' | 'lg'
    showTitle: boolean
    showLogo: boolean
    showDesc: boolean
    siteName: string
    logoText: string
    showClock: boolean
    groupStyle: 'section' | 'tabs'
    footerHtml: string
  }
  search: {
    enabled: boolean
    default: string
    engines: SearchEngine[]
    bars: SearchBar[]
    style: { bg: string; color: string; border: string }
  }
  weather: {
    enabled: boolean
    // auto 时坐标由浏览器定位给出（只落 localStorage），拿不到就回落到下面这个城市。
    locationMode: 'manual' | 'auto'
    city: string
    lat: number
    lon: number
    unit: 'c' | 'f'
    provider: 'open-meteo' | 'qweather'
    qweatherHost: string
    qweatherKey: string
  }
  calendar: {
    enabled: boolean
    weekStart: 'mon' | 'sun'
  }
  theme: 'auto' | 'light' | 'dark'
  language: 'zh' | 'en'
  network: 'wan' | 'lan'
}

// GET /weather 的返回。温度一律摄氏度，换算成华氏是前端的事。
export interface Weather {
  tempC: number
  feelsLikeC: number
  code: number // WMO weather code
  isDay: boolean
  humidity: number
  windKph: number
  maxC: number | null // 上游偶尔不给当日区间，这时是 null，不是 0
  minC: number | null
  updatedAt: number // unix 秒
  // 下面这些只有和风才给。缺席就藏行，Open-Meteo 详情卡保持原来那几行。
  conditionText?: string
  windDir?: string
  uvIndex?: number | null
  visibilityKm?: number | null
  pressureHpa?: number | null
  precipMm?: number | null
  sunrise?: string
  sunset?: string
  aqi?: number | null
  aqiCategory?: string
  alert?: string
}

// GET /calendar/month 的一天。农历、节气、节日名都是后端拼好的中文，
// 前端只管显示——这些是中文历法专名，没有通行英译，不进 i18n（决策 034）。
export interface CalendarDay {
  date: string // 2026-08-24
  day: number
  weekday: number // 0=周日 … 6=周六
  lunarMonth: string
  lunarDay: string
  ganZhi: string
  zodiac: string
  festival: string
  solarTerm: string
  dayType: '' | 'off' | 'work'
}

// GET /calendar/month 的整月返回。
export interface CalendarMonth {
  y: number
  m: number
  // 这一年有没有法定调休数据。没有时不画班/休角标，只显示节日名。
  hasHolidayData: boolean
  items: CalendarDay[]
}

// GET /weather/geocode 的一条结果。
export interface GeoPlace {
  name: string
  admin1: string
  admin2: string
  country: string
  lat: number
  lon: number
}

// 管理员编辑的实例级配置。siteIcon / loginBackground 存的是 /uploads/ 路径。
export interface SiteConfig {
  siteTitle: string
  siteIcon: string
  loginBackground: string
}

// GET /auth/config 的返回：未登录也能读，图片已换成公开只读地址。
export interface PublicSiteConfig extends SiteConfig {
  allowRegister: boolean
}

export interface UploadItem {
  id: number
  path: string
  kind: 'bg' | 'icons' | ''
  mime: string
  size: number
  createdAt: string
}

export interface SessionInfo {
  id: number
  userAgent: string
  ip: string
  remember: boolean
  current: boolean
  expiresAt: string
  lastUsedAt: string
  createdAt: string
}

export interface ImportItem {
  title: string
  url: string
  lanUrl?: string
  description?: string
  iconType?: IconType
  iconValue?: string
  openMode?: Site['openMode']
  hidden?: boolean
  groupName?: string
}

export interface ParseResult {
  url: string
  title: string
  description: string
  iconUrl: string
  error?: string
}

// 批量补全：给已经存在的卡片补图标 / 描述 / 标题。
export interface BackfillFields {
  icon: boolean
  title: boolean
  description: boolean
}

// 只回填真正写进去的字段，前端据此就地更新卡片，不必整表重拉。
export interface BackfillResult {
  id: number
  title?: string
  description?: string
  iconValue?: string
  changed: boolean
  error?: string
}

export interface BackfillResponse {
  items: BackfillResult[]
  changed: number
  unchanged: number
  failed: number
}

// 收藏书签（浏览器反向上传）：令牌状态只回答「有没有」，
// 原文只在生成那一刻返回一次（决策 026）。
export interface IngestTokenStatus {
  exists: boolean
  createdAt?: string
}

export interface IngestTokenCreated {
  token: string
  createdAt: string
}

export interface AdminUser extends User {
  createdAt: string
  siteCount: number
  groupCount: number
}
