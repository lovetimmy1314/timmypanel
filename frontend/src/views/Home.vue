<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useDialog, useMessage } from 'naive-ui'
import { Icon } from '@iconify/vue'
import BoardGrid from '@/components/BoardGrid.vue'
import SearchBar from '@/components/SearchBar.vue'
import SiteEditor from '@/components/SiteEditor.vue'
import ImportDialog from '@/components/ImportDialog.vue'
import SettingsHub from '@/components/settings/SettingsHub.vue'
import AccountDialog from '@/components/AccountDialog.vue'
import BackTop from '@/components/BackTop.vue'
import GroupJump from '@/components/GroupJump.vue'
import BrandMark from '@/components/BrandMark.vue'
import { usePanelStore } from '@/stores/panel'
import { useUserStore } from '@/stores/user'
import { useSiteStore } from '@/stores/site'
import { useThemeStore } from '@/stores/theme'
import { deepClone } from '@/utils/clone'
import { dateLocale, locale, t } from '@/i18n'
import type { Group, Settings, Site } from '@/api/types'

const panel = usePanelStore()
const userStore = useUserStore()
// 顶栏图标跟登录页保持一致：管理员配了站点图标就用那张，没配用品牌主标。
const site = useSiteStore()
const themeStore = useThemeStore()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()

const editing = ref(false)
// 分组级编辑：点分组名旁的铅笔只对这一组生效。与全局编辑的区别是拖拽
// 被锁在组内（BoardGrid 的 dragGroup 按组给独立名字，跨组放不下）。
const groupEditing = ref<number | null>(null)
const query = ref('')
const showEditor = ref(false)
const showImport = ref(false)
const showSettings = ref(false)
const showAccount = ref(false)
const editingSite = ref<Site | null>(null)
// SiteEditor 的分组初值与锁定：从分组编辑按钮进的新建不带分组选择，
// 默认落在当前分组（lockGroup=true 时编辑器直接不渲染分组那一项）。
const editorGroupId = ref(0)
const editorLockGroup = ref(false)

const isBoardEditing = (groupId: number) => editing.value || groupEditing.value === groupId

function toggleGroupEdit(groupId: number) {
  groupEditing.value = groupEditing.value === groupId ? null : groupId
}

// boards 是拖拽用的本地视图。直接把 store 的 computed 交给拖拽组件会写不回去，
// 所以这里维护一份可变副本（元素仍是 store 里的同一批对象引用）。
const boards = ref<{ group: Group; sites: Site[] }[]>([])
watch(
  () => panel.grouped,
  (v) => (boards.value = v.map((b) => ({ group: b.group, sites: [...b.sites] }))),
  { immediate: true, deep: false },
)

// 可拖拽的前提是渲染的就是 boards 里那个数组本身。filter 会产生新数组，
// 拖拽结果只会写进这个临时数组，persistSiteOrder(boards) 拿到的还是旧顺序 ——
// 表现为拖完看着对了，刷新就打回原形。所以能拖的时候直接返回源数组：
// 编辑模式（全局或分组级）下 hidden 本就不过滤，再没有搜索词，这一趟 filter 是恒等的。
const canDrag = computed(() => (editing.value || groupEditing.value !== null) && !query.value.trim())
const boardCanDrag = (groupId: number) => canDrag.value && isBoardEditing(groupId)
// 全局编辑下各组共用一个拖拽组名（可跨组）；分组级编辑下每组一个独立名字，
// 其它组的列表不接受它，于是只能组内排序。
const dragGroup = (groupId: number) => (editing.value ? 'sites' : `sites-${groupId}`)

const visibleBoards = computed(() => {
  if (canDrag.value) return boards.value
  const q = query.value.trim().toLowerCase()
  return boards.value
    .map((b) => ({
      group: b.group,
      sites: b.sites.filter((s) => {
        if (s.hidden && !isBoardEditing(b.group.id)) return false
        if (!q) return true
        return (
          s.title.toLowerCase().includes(q) ||
          s.url.toLowerCase().includes(q) ||
          s.description.toLowerCase().includes(q)
        )
      }),
    }))
    .filter((b) => b.sites.length > 0 || ((editing.value || groupEditing.value !== null) && !q))
})

// 页签模式下当前选中的分组。分组被删、或首次加载完成时回落到第一个。
const activeTab = ref('')
watch(
  () => visibleBoards.value.map((b) => String(b.group.id)),
  (ids) => {
    if (!ids.includes(activeTab.value)) activeTab.value = ids[0] ?? ''
    // 正在编辑的分组被删掉（或搜索后不可见）时退出分组编辑
    if (groupEditing.value !== null && !ids.includes(String(groupEditing.value)))
      groupEditing.value = null
  },
  { immediate: true },
)

// ---- 背景 ----
const bgStyle = computed(() => {
  const bg = panel.settings.background
  const style: Record<string, string> = { filter: bg.blur ? `blur(${bg.blur}px)` : 'none' }
  // 模糊会把边缘晕开，稍微放大一点盖住露白
  if (bg.blur) style.transform = 'scale(1.06)'
  // 后端已经把 background.value 限制为 /uploads/ 或 http(s) 地址，这里再转义引号即可。
  if (bg.type === 'image' && bg.value) style.backgroundImage = `url("${bg.value.replace(/["\\]/g, '')}")`
  else if (bg.type === 'color') style.background = bg.value
  else style.background = bg.value
  return style
})
// 浅色模式下遮罩是白的（--tp-scrim），且有保底不透明度：壁纸是用户随便传的，
// 深色壁纸配深色文字直接读不了，而遮罩的滑块值是照着深色模式调的。
const maskStyle = computed(() => {
  const mask = panel.settings.background.mask ?? 0
  return { opacity: String(themeStore.isDark ? mask : Math.max(mask, 0.62)) }
})

// ---- 时钟 ----
const now = ref(new Date())
let timer: number | undefined
onMounted(() => {
  timer = window.setInterval(() => (now.value = new Date()), 1000)
})
onUnmounted(() => window.clearInterval(timer))
const clock = computed(() =>
  now.value.toLocaleTimeString(dateLocale.value, { hour12: false, hour: '2-digit', minute: '2-digit' }),
)
const dateText = computed(() =>
  now.value.toLocaleDateString(dateLocale.value, { month: 'long', day: 'numeric', weekday: 'long' }),
)

// ---- 搜索栏设默认引擎（点亮下拉里的星星，立即落库）----
async function setDefaultEngine(name: string) {
  try {
    const next = deepClone(panel.settings)
    next.search.default = name
    await panel.saveSettings(next)
    message.success(t('common.saved'))
  } catch (e: any) {
    message.error(e?.message ?? t('common.saveFailed'))
  }
}

// ---- 内外网切换 ----
async function toggleNetwork() {
  const next = panel.settings.network === 'lan' ? 'wan' : 'lan'
  await panel.patchSettings({ network: next })
  message.success(next === 'lan' ? t('home.switchedLan') : t('home.switchedWan'))
}

// ---- 明暗与语言 ----
// 两个都是「点一下立刻落库」的开关，和内外网切换一路。语言另有一份在设置里，
// 那边是下拉框；这里只在中英之间对切。
async function toggleTheme() {
  try {
    await themeStore.toggle()
  } catch (e: any) {
    message.error(e?.message ?? t('common.saveFailed'))
  }
}

async function pickLanguage(next: string) {
  if (next !== 'zh' && next !== 'en') return
  try {
    await panel.patchSettings({ language: next as Settings['language'] })
  } catch (e: any) {
    message.error(e?.message ?? t('common.saveFailed'))
  }
}

// ---- 卡片操作 ----
function openNew() {
  editingSite.value = null
  editorGroupId.value = panel.groups[0]?.id ?? 0
  editorLockGroup.value = false
  showEditor.value = true
}
// 分组编辑按钮里的「添加」：分组已定，编辑器不再问分组
function openNewInGroup(g: Group) {
  editingSite.value = null
  editorGroupId.value = g.id
  editorLockGroup.value = true
  showEditor.value = true
}
function openEdit(site: Site) {
  editingSite.value = site
  editorLockGroup.value = false
  showEditor.value = true
}
function removeSite(site: Site) {
  dialog.warning({
    title: t('home.deleteCard'),
    content: t('home.deleteCardTip', { title: site.title }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await panel.deleteSite(site.id)
        await panel.reloadSites()
        message.success(t('common.deleted'))
      } catch (e: any) {
        message.error(e?.message ?? t('common.deleteFailed'))
      }
    },
  })
}

async function onDragEnd() {
  try {
    await panel.persistSiteOrder(boards.value)
  } catch (e: any) {
    message.error(e?.message ?? t('common.sortFailed'))
    await panel.reloadSites()
  }
}

// ---- 分组操作 ----
async function addGroup() {
  const name = window.prompt(t('home.newGroupPrompt'))
  if (!name?.trim()) return
  try {
    await panel.createGroup(name.trim())
    await panel.reloadGroups()
  } catch (e: any) {
    message.error(e?.message ?? t('home.newGroupFailed'))
  }
}

function renameGroup(g: Group) {
  if (!g.id) return
  const name = window.prompt(t('home.renameGroupPrompt'), g.name)
  if (!name?.trim() || name === g.name) return
  panel
    .updateGroup(g.id, { name: name.trim() })
    .then(() => panel.reloadGroups())
    .catch((e: any) => message.error(e?.message ?? t('common.renameFailed')))
}

function removeGroup(g: Group) {
  if (!g.id) return
  dialog.warning({
    title: t('home.deleteGroup'),
    content: t('home.deleteGroupTip', { name: g.name }),
    positiveText: t('home.deleteGroup'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await panel.deleteGroup(g.id, false)
        await panel.loadAll()
      } catch (e: any) {
        message.error(e?.message ?? t('home.deleteGroupFailed'))
      }
    },
  })
}

// 折叠状态存在服务端，换台设备也保持一致。「未分组」(id=0) 是虚拟分组，存不了。
// 编辑模式（全局或该组的分组编辑）下不折叠，否则拖拽时看不见目标分组。
function toggleCollapse(g: Group) {
  if (!g.id || isBoardEditing(g.id)) return
  const next = !g.collapsed
  g.collapsed = next
  panel.updateGroup(g.id, { collapsed: next }).catch(async (e: any) => {
    g.collapsed = !next
    message.error(e?.message ?? t('home.collapseFailed'))
  })
}

const isCollapsed = (g: Group) => !!g.collapsed && !isBoardEditing(g.id)

// ---- 分组跳转（左下角悬浮按钮）----
// 平铺模式滚动到对应 section（折叠的先展开，否则跳过去只有一个标题）；
// 页签模式没有锚点可跳，等价于切到那个 tab 再滚回页签顶部。
async function jumpToGroup(id: number) {
  if (panel.settings.layout.groupStyle === 'tabs') {
    activeTab.value = String(id)
    await nextTick()
    document.getElementById('tp-tabs')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    return
  }
  const board = boards.value.find((b) => b.group.id === id)
  if (board && isCollapsed(board.group)) toggleCollapse(board.group)
  await nextTick()
  document.getElementById(`tp-group-${id}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

async function logout() {
  await userStore.logout()
  router.replace({ name: 'login' })
}

function onUserMenu(key: string) {
  if (key === 'logout') logout()
  else if (key === 'account') showAccount.value = true
  else router.push('/admin')
}

const gridClass = computed(
  () =>
    ({
      sm: 'grid-cols-2 sm:grid-cols-3 md:grid-cols-5 lg:grid-cols-7 xl:grid-cols-8',
      md: 'grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6',
      lg: 'grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5',
    })[panel.settings.layout.cardSize],
)

const refreshAll = () => panel.loadAll()

onMounted(async () => {
  try {
    await panel.loadAll()
  } catch (e: any) {
    message.error(e?.message ?? t('common.loadFailed'))
  }
})
</script>

<template>
  <div class="min-h-screen">
    <div class="tp-bg" :style="bgStyle" />
    <div class="tp-bg-mask" :style="maskStyle" />

    <div class="max-w-7xl mx-auto px-4 py-6">
      <!-- 顶栏。毛玻璃容器保证压在壁纸上也可读；小屏下按钮区会把标题挤窄，
           允许换行，时钟另起一行（见 header 下方） -->
      <header
        class="flex flex-wrap items-center gap-x-3 gap-y-2 mb-3 sm:mb-6 tp-surface-glass rounded-2xl px-3 py-2.5"
      >
        <template v-if="panel.settings.layout.showLogo">
          <img
            v-if="site.config.siteIcon"
            :src="site.config.siteIcon"
            class="w-9 h-9 rounded-xl object-contain shrink-0"
            alt=""
          />
          <BrandMark v-else :size="36" class="rounded-xl" />
        </template>

        <div class="min-w-0">
          <h1 v-if="panel.settings.layout.showLogo" class="tp-text text-lg font-semibold truncate">
            {{ panel.settings.layout.logoText || panel.settings.layout.siteName || t('home.defaultTitle') }}
          </h1>
          <!-- 内联时钟只在中屏以上显示：小屏下它被按钮区挤压会换行错位 -->
          <div v-if="panel.settings.layout.showClock" class="tp-text-dim text-xs hidden sm:block">
            {{ clock }} · {{ dateText }}
          </div>
        </div>

        <div class="ml-auto flex items-center gap-1.5">
          <n-tooltip>
            <template #trigger>
              <n-button
                quaternary
                circle
                :type="panel.settings.network === 'lan' ? 'success' : 'default'"
                @click="toggleNetwork"
              >
                <Icon
                  :icon="panel.settings.network === 'lan' ? 'mdi:lan-connect' : 'mdi:earth'"
                  class="text-lg tp-text"
                />
              </n-button>
            </template>
            {{ panel.settings.network === 'lan' ? t('home.networkLan') : t('home.networkWan') }}
          </n-tooltip>

          <!-- 明暗切换：写的是 settings.theme，所以换台设备也保持一致 -->
          <n-tooltip>
            <template #trigger>
              <n-button quaternary circle @click="toggleTheme">
                <Icon
                  :icon="themeStore.isDark ? 'mdi:weather-sunny' : 'mdi:weather-night'"
                  class="text-lg tp-text"
                />
              </n-button>
            </template>
            {{ themeStore.isDark ? t('home.toLight') : t('home.toDark') }}
          </n-tooltip>

          <n-tooltip>
            <template #trigger>
              <n-dropdown
                trigger="click"
                :options="[
                  { label: '中文', key: 'zh' },
                  { label: 'English', key: 'en' },
                ]"
                @select="pickLanguage"
              >
                <!-- 图标看不出当前是哪种语言，所以这颗按钮直接写着语言本身 -->
                <n-button quaternary circle>
                  <span class="tp-text text-xs font-semibold">{{ locale === 'zh' ? '中' : 'EN' }}</span>
                </n-button>
              </n-dropdown>
            </template>
            {{ t('home.language') }}
          </n-tooltip>

          <n-tooltip>
            <template #trigger>
              <n-button quaternary circle :type="editing ? 'primary' : 'default'" @click="editing = !editing">
                <Icon :icon="editing ? 'mdi:check' : 'mdi:pencil-outline'" class="text-lg tp-text" />
              </n-button>
            </template>
            {{ editing ? t('home.editDone') : t('home.editStart') }}
          </n-tooltip>

          <n-tooltip>
            <template #trigger>
              <n-button quaternary circle @click="openNew">
                <Icon icon="mdi:plus" class="text-lg tp-text" />
              </n-button>
            </template>
            {{ t('home.addCard') }}
          </n-tooltip>

          <n-tooltip>
            <template #trigger>
              <n-button quaternary circle @click="showImport = true">
                <Icon icon="mdi:import" class="text-lg tp-text" />
              </n-button>
            </template>
            {{ t('home.import') }}
          </n-tooltip>

          <n-tooltip>
            <template #trigger>
              <n-button quaternary circle @click="showSettings = true">
                <Icon icon="mdi:cog-outline" class="text-lg tp-text" />
              </n-button>
            </template>
            {{ t('home.settings') }}
          </n-tooltip>

          <n-dropdown
            trigger="click"
            :options="[
              { label: t('home.menuAccount'), key: 'account' },
              ...(userStore.user?.role === 'admin' ? [{ label: t('home.menuAdmin'), key: 'admin' }] : []),
              { label: t('home.menuLogout'), key: 'logout' },
            ]"
            @select="onUserMenu"
          >
            <n-button quaternary circle>
              <Icon icon="mdi:account-circle-outline" class="text-lg tp-text" />
            </n-button>
          </n-dropdown>
        </div>
      </header>

      <!-- 手机端时钟：独占一行，不被顶栏按钮挤压 -->
      <div v-if="panel.settings.layout.showClock" class="tp-text-dim text-xs mb-5 sm:hidden">
        {{ clock }} · {{ dateText }}
      </div>

      <!-- 搜索 -->
      <div v-if="panel.settings.search.enabled" class="mb-6 sm:mb-8">
        <SearchBar
          :sites="panel.sites"
          :engines="panel.settings.search.engines"
          :default-engine="panel.settings.search.default"
          :network="panel.settings.network"
          :bar-style="panel.settings.search.style"
          @update:query="query = $event"
          @set-default="setDefaultEngine"
        />
      </div>

      <!-- 分组与卡片 -->
      <n-spin :show="panel.loading">
        <div v-if="!visibleBoards.length" class="text-center tp-text-dim py-20">
          <Icon icon="mdi:bookmark-plus-outline" class="text-5xl opacity-50" />
          <p class="mt-3 text-sm">{{ t('home.empty') }}</p>
          <div class="flex gap-2 justify-center mt-4">
            <n-button size="small" @click="openNew">{{ t('home.emptyAdd') }}</n-button>
            <n-button size="small" type="primary" @click="showImport = true">
              {{ t('home.emptyImport') }}
            </n-button>
          </div>
        </div>

        <!-- 页签模式：一次只显示一个分组，适合分组多的情况 -->
        <!-- 不要加 animated：pane 由 v-for 动态生成时，Naive UI 的动画模式会把
             两个 pane 同时留在 DOM 里用 opacity 过渡，实测切换后激活的那个反而
             是 opacity:0，看起来就是"点了没反应"。默认的销毁重建反而正确。 -->
        <div v-if="panel.settings.layout.groupStyle === 'tabs' && visibleBoards.length" id="tp-tabs">
          <n-tabs v-model:value="activeTab" type="line">
          <n-tab-pane
            v-for="board in visibleBoards"
            :key="board.group.id"
            :name="String(board.group.id)"
            :tab="`${board.group.name} (${board.sites.length})`"
          >
            <div class="flex items-center gap-2 mb-3">
              <n-tooltip>
                <template #trigger>
                  <n-button
                    size="tiny"
                    quaternary
                    :type="groupEditing === board.group.id ? 'primary' : 'default'"
                    @click="toggleGroupEdit(board.group.id)"
                  >
                    <Icon
                      :icon="groupEditing === board.group.id ? 'mdi:check' : 'mdi:pencil-outline'"
                      class="tp-text-soft"
                    />
                  </n-button>
                </template>
                {{ groupEditing === board.group.id ? t('home.editDone') : t('home.groupEditStart') }}
              </n-tooltip>
              <n-tooltip v-if="isBoardEditing(board.group.id)">
                <template #trigger>
                  <n-button size="tiny" quaternary @click="openNewInGroup(board.group)">
                    <Icon icon="mdi:plus" class="tp-text-soft" />
                  </n-button>
                </template>
                {{ t('home.quickAdd') }}
              </n-tooltip>
              <template v-if="editing && board.group.id">
                <n-button size="tiny" quaternary @click="renameGroup(board.group)">
                  <Icon icon="mdi:rename-outline" class="tp-text-soft" />
                </n-button>
                <n-button size="tiny" quaternary @click="removeGroup(board.group)">
                  <Icon icon="mdi:trash-can-outline" class="tp-text-soft" />
                </n-button>
              </template>
            </div>
            <BoardGrid
              :board="board"
              :size="panel.settings.layout.cardSize"
              :show-desc="panel.settings.layout.showDesc"
              :network="panel.settings.network"
              :editing="isBoardEditing(board.group.id)"
              :can-drag="boardCanDrag(board.group.id)"
              :drag-group="dragGroup(board.group.id)"
              :grid-class="gridClass"
              @drag-end="onDragEnd"
              @edit="openEdit"
              @remove="removeSite"
            />
          </n-tab-pane>
          </n-tabs>
        </div>

        <!-- 分段模式：所有分组平铺，可逐个折叠 -->
        <template v-else>
          <section
            v-for="board in visibleBoards"
            :id="`tp-group-${board.group.id}`"
            :key="board.group.id"
            class="mb-8 scroll-mt-4"
          >
            <div class="flex items-center gap-2 mb-3">
              <button
                class="flex items-center gap-1.5 bg-transparent border-0 p-0 text-left"
                :class="board.group.id && !isBoardEditing(board.group.id) ? 'cursor-pointer' : 'cursor-default'"
                @click="toggleCollapse(board.group)"
              >
                <Icon
                  v-if="board.group.id && !isBoardEditing(board.group.id)"
                  :icon="isCollapsed(board.group) ? 'mdi:chevron-right' : 'mdi:chevron-down'"
                  class="tp-text-dim shrink-0"
                />
                <h2 class="tp-text-soft text-sm font-medium tracking-wide">{{ board.group.name }}</h2>
                <span class="tp-pill">{{ board.sites.length }}</span>
              </button>
              <n-tooltip>
                <template #trigger>
                  <n-button
                    size="tiny"
                    quaternary
                    :type="groupEditing === board.group.id ? 'primary' : 'default'"
                    @click="toggleGroupEdit(board.group.id)"
                  >
                    <Icon
                      :icon="groupEditing === board.group.id ? 'mdi:check' : 'mdi:pencil-outline'"
                      class="tp-text-soft"
                    />
                  </n-button>
                </template>
                {{ groupEditing === board.group.id ? t('home.editDone') : t('home.groupEditStart') }}
              </n-tooltip>
              <n-tooltip v-if="isBoardEditing(board.group.id)">
                <template #trigger>
                  <n-button size="tiny" quaternary @click="openNewInGroup(board.group)">
                    <Icon icon="mdi:plus" class="tp-text-soft" />
                  </n-button>
                </template>
                {{ t('home.quickAdd') }}
              </n-tooltip>
              <template v-if="editing && board.group.id">
                <n-button size="tiny" quaternary @click="renameGroup(board.group)">
                  <Icon icon="mdi:rename-outline" class="tp-text-soft" />
                </n-button>
                <n-button size="tiny" quaternary @click="removeGroup(board.group)">
                  <Icon icon="mdi:trash-can-outline" class="tp-text-soft" />
                </n-button>
              </template>
            </div>

            <BoardGrid
              v-show="!isCollapsed(board.group)"
              :board="board"
              :size="panel.settings.layout.cardSize"
              :show-desc="panel.settings.layout.showDesc"
              :network="panel.settings.network"
              :editing="isBoardEditing(board.group.id)"
              :can-drag="boardCanDrag(board.group.id)"
              :drag-group="dragGroup(board.group.id)"
              :grid-class="gridClass"
              @drag-end="onDragEnd"
              @edit="openEdit"
              @remove="removeSite"
            />
          </section>
        </template>

        <div v-if="editing" class="mb-10">
          <n-button dashed size="small" @click="addGroup">{{ t('home.newGroup') }}</n-button>
        </div>
      </n-spin>

      <footer
        v-if="panel.settings.layout.footerHtml"
        class="tp-footer"
        v-html="panel.settings.layout.footerHtml"
      />
    </div>

    <SiteEditor
      v-model:show="showEditor"
      :site="editingSite"
      :groups="panel.groups"
      :default-group-id="editorGroupId"
      :lock-group="editorLockGroup"
      @saved="refreshAll"
    />
    <ImportDialog v-model:show="showImport" :groups="panel.groups" @imported="refreshAll" />
    <SettingsHub v-model:show="showSettings" @changed="refreshAll" />
    <AccountDialog v-model:show="showAccount" />
    <BackTop />
    <GroupJump :boards="visibleBoards" @jump="jumpToGroup" />
  </div>
</template>
