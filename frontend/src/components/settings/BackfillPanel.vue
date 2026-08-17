<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { api } from '@/api/http'
import { usePanelStore } from '@/stores/panel'
import { t } from '@/i18n'
import SettingsSection from './SettingsSection.vue'
import SettingsRow from './SettingsRow.vue'
import type { BackfillResponse, BackfillResult, Site } from '@/api/types'

const emit = defineEmits<{ changed: [] }>()

const panel = usePanelStore()
const message = useMessage()

// 后端单次上限是 20（见 backfill.go 的 maxBackfillPerRequest）。这里发得更小，
// 是为了让进度条动得细一点 —— 一批 10 条最坏也就几十秒。
const BATCH = 10

const groupId = ref<number | 'all'>('all')
const fields = ref({ icon: true, description: true, title: false })
const overwrite = ref(false)
const onlyMissing = ref(true)

const running = ref(false)
const stopping = ref(false)
const done = ref(0)
const total = ref(0)
const results = ref<BackfillResult[]>([])
const selected = ref<Set<number>>(new Set())

const groupOptions = computed(() => [
  { label: t('common.allGroups'), value: 'all' as const },
  { label: t('common.ungrouped'), value: 0 },
  ...panel.groups.map((g) => ({ label: g.name, value: g.id })),
])

// 卡片标题为空、或还是后端拿域名兜底填的，都算「没有标题」——
// 判断口径要和后端 backfillSite 保持一致，否则列表里显示缺、跑完却没变化。
function hostOf(url: string) {
  try {
    return new URL(url).host
  } catch {
    return ''
  }
}
function lacksTitle(s: Site) {
  return !s.title.trim() || s.title === hostOf(s.url)
}

// 一张卡片按当前勾选项还缺哪些内容。勾了「覆盖已有」时一切都算要处理。
function missingOf(s: Site) {
  const out: string[] = []
  if (fields.value.icon && !s.iconValue.trim()) out.push(t('backfill.fieldIcon'))
  if (fields.value.description && !s.description.trim()) out.push(t('backfill.fieldDesc'))
  if (fields.value.title && lacksTitle(s)) out.push(t('backfill.fieldTitle'))
  return out
}

const inScope = computed(() =>
  panel.sites.filter((s) => groupId.value === 'all' || s.groupId === groupId.value),
)

// 覆盖模式下「只处理缺内容的」自相矛盾，直接按全部算（开关也会置灰）。
const filterMissing = computed(() => onlyMissing.value && !overwrite.value)

const candidates = computed(() =>
  filterMissing.value ? inScope.value.filter((s) => missingOf(s).length > 0) : inScope.value,
)

const anyField = computed(() => fields.value.icon || fields.value.description || fields.value.title)

// 候选集变了就重选一遍：默认全选，让「改条件 → 直接开跑」是顺的。
watch(
  candidates,
  (list) => {
    if (!running.value) selected.value = new Set(list.map((s) => s.id))
  },
  { immediate: true },
)

function toggle(id: number) {
  const next = new Set(selected.value)
  next.has(id) ? next.delete(id) : next.add(id)
  selected.value = next
}

const selectedIds = computed(() => candidates.value.filter((s) => selected.value.has(s.id)).map((s) => s.id))
const failedResults = computed(() => results.value.filter((r) => r.error))
const changedCount = computed(() => results.value.filter((r) => r.changed).length)

// ---- 浏览器逐个补（档 2，决策 026）----
// 失败的站点多半是对方拦了服务器抓取（Cloudflare 按 TLS 指纹认机器人），
// 但拦不了用户自己的浏览器。把失败 URL 塞进服务端队列，用户逐个打开、
// 每个页面点一次收藏书签，书签提交成功后靠响应里的 next 自动跳下一个。

// idle=还没查令牌；need-token=没书签可先去生成；ready=可以开始；started=队列已下发
const fixState = ref<'idle' | 'need-token' | 'ready' | 'started'>('idle')

const failedSites = computed(() =>
  failedResults.value
    .map((r) => panel.sites.find((s) => s.id === r.id))
    .filter((s): s is Site => !!s),
)

// 失败列表出现了才去查令牌，省得每次打开面板都打一发。
watch(failedResults, async (list) => {
  if (!list.length || fixState.value === 'started') return
  fixState.value = 'idle'
  try {
    const res = await api.get<{ exists: boolean }>('/ingest/token')
    fixState.value = res.exists ? 'ready' : 'need-token'
  } catch {
    fixState.value = 'idle'
  }
})

async function startBrowserFix() {
  const urls = failedSites.value.map((s) => s.url)
  if (!urls.length) return
  await api.put('/ingest/queue', { urls })
  fixState.value = 'started'
  window.open(urls[0], '_blank')
  message.success(t('backfill.browserFixStarted'))
}

async function stopBrowserFix() {
  await api.put('/ingest/queue', { urls: [] })
  fixState.value = 'ready'
}

const titleOf = (id: number) => panel.sites.find((s) => s.id === id)?.title || `#${id}`

async function run() {
  const ids = selectedIds.value
  if (!ids.length) {
    message.warning(t('backfill.noCandidate'))
    return
  }
  if (!anyField.value) {
    message.warning(t('backfill.noField'))
    return
  }
  running.value = true
  stopping.value = false
  results.value = []
  done.value = 0
  total.value = ids.length

  try {
    for (let i = 0; i < ids.length; i += BATCH) {
      if (stopping.value) break
      const chunk = ids.slice(i, i + BATCH)
      try {
        const res = await api.post<BackfillResponse>('/sites/backfill', {
          ids: chunk,
          fields: fields.value,
          overwrite: overwrite.value,
        })
        results.value = results.value.concat(res.items)
      } catch (e: any) {
        // 单批失败（网络断了、超时）不该让整轮白跑，记下来接着下一批。
        results.value = results.value.concat(
          chunk.map((id) => ({ id, changed: false, error: e?.message ?? t('common.requestFailed') })),
        )
      }
      done.value += chunk.length
    }
    // 抓来的图标和标题已经落库了，把本地状态同步过来，首页立刻能看到。
    await panel.reloadSites()
    emit('changed')
    const failed = failedResults.value.length
    message.success(
      t('backfill.done', { changed: changedCount.value }) +
        (failed ? t('backfill.doneFailed', { n: failed }) : '') +
        (stopping.value ? t('backfill.doneStopped') : ''),
    )
  } finally {
    running.value = false
    stopping.value = false
  }
}
</script>

<template>
  <div class="space-y-3">
    <SettingsSection :title="t('backfill.scope')">
      <SettingsRow :label="t('backfill.group')" grow>
        <n-select v-model:value="groupId" :options="groupOptions" size="small" :disabled="running" />
      </SettingsRow>
      <SettingsRow :label="t('backfill.onlyMissing')" :hint="t('backfill.onlyMissingHint')">
        <n-switch
          v-model:value="onlyMissing"
          size="small"
          :disabled="running || overwrite"
        />
      </SettingsRow>
    </SettingsSection>

    <SettingsSection :title="t('backfill.what')">
      <div class="flex flex-wrap gap-4">
        <n-checkbox v-model:checked="fields.icon" :disabled="running">
          {{ t('backfill.fieldIcon') }}
        </n-checkbox>
        <n-checkbox v-model:checked="fields.description" :disabled="running">
          {{ t('backfill.fieldDesc') }}
        </n-checkbox>
        <n-checkbox v-model:checked="fields.title" :disabled="running">
          {{ t('backfill.fieldTitle') }}
        </n-checkbox>
      </div>
      <SettingsRow :label="t('backfill.overwrite')" :hint="t('backfill.overwriteHint')">
        <n-switch v-model:value="overwrite" size="small" :disabled="running" />
      </SettingsRow>
    </SettingsSection>

    <SettingsSection :title="t('backfill.pending', { n: selectedIds.length, total: candidates.length })">
      <template #extra>
        <div class="flex gap-1.5">
          <n-button
            size="tiny"
            :disabled="running"
            @click="selected = new Set(candidates.map((s) => s.id))"
          >
            {{ t('common.selectAll') }}
          </n-button>
          <n-button size="tiny" :disabled="running" @click="selected = new Set()">
            {{ t('common.selectNone') }}
          </n-button>
        </div>
      </template>

      <p v-if="!anyField" class="text-sm opacity-60">{{ t('backfill.needField') }}</p>
      <p v-else-if="!candidates.length" class="text-sm opacity-60">{{ t('backfill.nothing') }}</p>

      <div
        v-else
        class="max-h-56 overflow-y-auto rounded-lg border border-black/5 dark:border-white/10"
      >
        <label
          v-for="s in candidates"
          :key="s.id"
          class="flex items-center gap-2 px-3 py-1.5 text-sm border-b border-black/5 dark:border-white/5 last:border-0 cursor-pointer"
        >
          <n-checkbox
            :checked="selected.has(s.id)"
            :disabled="running"
            @update:checked="toggle(s.id)"
          />
          <span class="truncate w-36">{{ s.title || t('backfill.noTitle') }}</span>
          <span class="truncate flex-1 opacity-50 text-xs">{{ s.url }}</span>
          <n-tag v-for="m in missingOf(s)" :key="m" size="tiny" type="warning">
            {{ t('backfill.missing', { field: m }) }}
          </n-tag>
        </label>
      </div>
    </SettingsSection>

    <!-- 抓取是逐批同步的，进度条是这个面板里唯一能告诉人"还在跑"的东西 -->
    <SettingsSection v-if="running || results.length" :title="t('backfill.progress')">
      <div class="h-1.5 rounded-full bg-black/10 dark:bg-white/10 overflow-hidden">
        <div
          class="h-full bg-sky-500 transition-[width] duration-300"
          :style="{ width: total ? `${Math.round((done / total) * 100)}%` : '0%' }"
        />
      </div>
      <p class="text-xs opacity-60">
        {{
          t('backfill.progressText', {
            done,
            total,
            changed: changedCount,
            failed: failedResults.length,
          })
        }}
      </p>

      <div
        v-if="failedResults.length"
        class="max-h-32 overflow-y-auto rounded-lg border border-black/5 dark:border-white/10"
      >
        <div
          v-for="r in failedResults"
          :key="r.id"
          class="flex items-center gap-2 px-3 py-1.5 text-xs border-b border-black/5 dark:border-white/5 last:border-0"
        >
          <Icon icon="mdi:alert-circle-outline" class="opacity-60 shrink-0 text-amber-500" />
          <span class="truncate w-32">{{ titleOf(r.id) }}</span>
          <span class="truncate flex-1 opacity-60">{{ r.error }}</span>
        </div>
      </div>
      <p v-if="failedResults.length" class="text-xs opacity-45">{{ t('backfill.failHint') }}</p>
    </SettingsSection>

    <!-- 有失败项才出现：引导用户走收藏书签这条路（档 2） -->
    <SettingsSection v-if="failedResults.length" :title="t('backfill.browserFix')">
      <template v-if="fixState === 'need-token'">
        <p class="text-sm opacity-70">{{ t('backfill.browserFixNeedToken') }}</p>
      </template>
      <template v-else>
        <p class="text-sm opacity-70">{{ t('backfill.browserFixHint') }}</p>
        <div class="flex justify-end gap-2">
          <n-button v-if="fixState === 'started'" size="small" @click="stopBrowserFix">
            {{ t('backfill.browserFixStop') }}
          </n-button>
          <n-button
            size="small"
            type="primary"
            :disabled="fixState !== 'ready' && fixState !== 'started'"
            @click="startBrowserFix"
          >
            {{ t('backfill.browserFixStart', { n: failedSites.length }) }}
          </n-button>
        </div>
      </template>
    </SettingsSection>

    <div
      class="sticky bottom-0 py-2 flex justify-end gap-2 bg-gradient-to-t from-black/[0.06] dark:from-white/[0.06] to-transparent backdrop-blur-sm"
    >
      <n-button v-if="running" :disabled="stopping" @click="stopping = true">
        {{ stopping ? t('backfill.stopping') : t('backfill.stop') }}
      </n-button>
      <n-button
        type="primary"
        :loading="running"
        :disabled="!selectedIds.length || !anyField"
        @click="run"
      >
        {{ t('backfill.start') }} {{ selectedIds.length || '' }}
      </n-button>
    </div>
  </div>
</template>
