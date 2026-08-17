// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api } from '@/api/http'
import { deepClone } from '@/utils/clone'
import { setLocale } from '@/i18n'
import type { Group, Settings, Site } from '@/api/types'

const DEFAULT_SETTINGS: Settings = {
  background: { type: 'gradient', value: 'linear-gradient(135deg,#1e3a8a 0%,#0f172a 60%,#312e81 100%)', blur: 0, mask: 0.15 },
  layout: {
    cardSize: 'md',
    showTitle: true,
    showLogo: true,
    showDesc: true,
    siteName: '我的导航',
    logoText: '我的导航',
    showClock: true,
    groupStyle: 'section',
    footerHtml: '',
  },
  search: { enabled: true, default: 'local', engines: [], style: { bg: '', color: '', border: '' } },
  theme: 'auto',
  language: 'zh',
  network: 'wan',
}

// panel store 是首页的数据源：分组、卡片和界面设置。
export const usePanelStore = defineStore('panel', () => {
  const groups = ref<Group[]>([])
  const sites = ref<Site[]>([])
  const settings = ref<Settings>(structuredClone(DEFAULT_SETTINGS))
  const loading = ref(false)

  // 未分组的卡片单独归到一个虚拟分组里展示，避免它们"消失"。
  const grouped = computed(() => {
    const buckets = new Map<number, Site[]>()
    for (const g of groups.value) buckets.set(g.id, [])
    buckets.set(0, [])
    for (const s of sites.value) {
      const key = buckets.has(s.groupId) ? s.groupId : 0
      buckets.get(key)!.push(s)
    }
    for (const list of buckets.values()) list.sort((a, b) => a.sort - b.sort || a.id - b.id)

    const out = groups.value.map((g) => ({ group: g, sites: buckets.get(g.id) ?? [] }))
    const loose = buckets.get(0) ?? []
    if (loose.length) {
      out.push({ group: { id: 0, name: '未分组', sort: 9999, collapsed: false }, sites: loose })
    }
    return out
  })

  async function loadAll() {
    loading.value = true
    try {
      const [g, s, st] = await Promise.all([
        api.get<{ items: Group[] }>('/groups'),
        api.get<{ items: Site[] }>('/sites'),
        api.get<Settings>('/settings'),
      ])
      groups.value = g.items ?? []
      sites.value = s.items ?? []
      settings.value = {
        ...structuredClone(DEFAULT_SETTINGS),
        ...st,
        background: { ...DEFAULT_SETTINGS.background, ...st.background },
        layout: { ...DEFAULT_SETTINGS.layout, ...st.layout },
        search: {
          ...DEFAULT_SETTINGS.search,
          ...st.search,
          style: { ...DEFAULT_SETTINGS.search.style, ...st.search?.style },
        },
      }
      // 服务端的语言是权威值，盖掉 localStorage 里那份起手值（见 src/i18n/index.ts）。
      setLocale(settings.value.language)
    } finally {
      loading.value = false
    }
  }

  const reloadSites = async () => {
    sites.value = (await api.get<{ items: Site[] }>('/sites')).items ?? []
  }
  const reloadGroups = async () => {
    groups.value = (await api.get<{ items: Group[] }>('/groups')).items ?? []
  }

  async function saveSettings(next: Settings) {
    settings.value = await api.put<Settings>('/settings', next)
    // 后端会把不认识的 language 打回 zh，所以以返回值为准而不是以提交值为准。
    setLocale(settings.value.language)
    return settings.value
  }

  // 只改一个字段并立即落库，用于主题、内外网这类开关。
  async function patchSettings(patch: Partial<Settings>) {
    return saveSettings({ ...deepClone(settings.value), ...patch })
  }

  const createGroup = (name: string) => api.post<Group>('/groups', { name })
  const updateGroup = (id: number, body: Partial<Group>) => api.put<Group>(`/groups/${id}`, body)
  const deleteGroup = (id: number, cascade: boolean) =>
    api.del(`/groups/${id}${cascade ? '?mode=cascade' : ''}`)

  const createSite = (body: Partial<Site>) => api.post<Site>('/sites', body)
  const updateSite = (id: number, body: Partial<Site>) => api.put<Site>(`/sites/${id}`, body)
  const deleteSite = (id: number) => api.del(`/sites/${id}`)

  // 把拖拽后的完整顺序整体提交，后端在一个事务里写完，不会留下半套顺序。
  // 同时按 id 回写 store 里的那批 Site，这样 grouped 重算出来的就是拖完的样子，
  // 列表重建时不会闪回旧顺序。**按 id 找而不是直接改 boards 里的对象**：
  // 拖拽库有可能把副本塞进 boards（决策 029），改副本等于没改。
  function persistSiteOrder(boards: { group: Group; sites: Site[] }[]) {
    const byId = new Map(sites.value.map((s) => [s.id, s]))
    const items: { id: number; groupId: number; sort: number }[] = []
    for (const bucket of boards) {
      bucket.sites.forEach((s, idx) => {
        const local = byId.get(s.id) ?? s
        local.groupId = bucket.group.id
        local.sort = idx
        items.push({ id: s.id, groupId: bucket.group.id, sort: idx })
      })
    }
    return api.patch('/sites/sort', { items })
  }

  const persistGroupOrder = () =>
    api.patch('/groups/sort', {
      items: groups.value.map((g, idx) => ({ id: g.id, sort: idx })),
    })

  return {
    groups, sites, settings, loading, grouped,
    loadAll, reloadSites, reloadGroups,
    saveSettings, patchSettings,
    createGroup, updateGroup, deleteGroup,
    createSite, updateSite, deleteSite,
    persistSiteOrder, persistGroupOrder,
  }
})
