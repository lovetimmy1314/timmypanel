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
    style: { bg: string; color: string; border: string }
  }
  theme: 'auto' | 'light' | 'dark'
  language: 'zh' | 'en'
  network: 'wan' | 'lan'
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
