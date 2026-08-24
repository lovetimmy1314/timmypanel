<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { usePanelStore } from '@/stores/panel'
import { deepClone } from '@/utils/clone'
import { t } from '@/i18n'
import SettingsSection from './SettingsSection.vue'
import type { Settings } from '@/api/types'

const emit = defineEmits<{ changed: [] }>()

const panel = usePanelStore()
const message = useMessage()
const draft = ref<Settings>(deepClone(panel.settings))
const saving = ref(false)

// 与后端 maxSearchBars 保持一致：超出的部分保存时会被截掉，不如在界面上就拦住。
const MAX_BARS = 8

// 「默认搜索源」的可选项：站内搜索 + 当前这份草稿里的引擎。主搜索框和附加搜索框共用。
const sourceOptions = computed(() => [
  { label: t('search.local'), value: 'local' },
  ...draft.value.search.engines.map((e) => ({ label: e.name, value: e.name })),
])

function addEngine() {
  draft.value.search.engines.push({ name: '', url: 'https://example.com/search?q=%s', icon: '' })
}
function removeEngine(i: number) {
  const name = draft.value.search.engines[i]?.name
  draft.value.search.engines.splice(i, 1)
  if (!name) return
  // 删掉的引擎可能正被某个搜索框选作默认源，那些框要一起回落到站内搜索，
  // 否则下拉框选中一个不存在的项（后端也会这么归一化，这里只是别让界面先错一拍）。
  if (draft.value.search.default === name) draft.value.search.default = 'local'
  for (const bar of draft.value.search.bars) {
    if (bar.default === name) bar.default = 'local'
  }
}

function addBar() {
  if (draft.value.search.bars.length >= MAX_BARS) return
  draft.value.search.bars.push({ enabled: true, default: 'local' })
}
function removeBar(i: number) {
  draft.value.search.bars.splice(i, 1)
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
    <SettingsSection :title="t('searchSet.bars')">
      <!-- 主搜索框：勾上才在首页显示，右边选它的默认搜索源 -->
      <div class="flex items-center gap-2">
        <n-checkbox v-model:checked="draft.search.enabled" class="shrink-0">
          {{ t('searchSet.showBar') }}
        </n-checkbox>
        <n-select
          v-model:value="draft.search.default"
          size="small"
          class="flex-1 min-w-0"
          :placeholder="t('searchSet.default')"
          :options="sourceOptions"
        />
        <!-- 占位：和下面几行的删除按钮对齐，主搜索框删不掉 -->
        <span class="w-8 shrink-0" />
      </div>

      <!-- 附加搜索框：首页里按这个顺序排在主搜索框下面 -->
      <div v-for="(bar, i) in draft.search.bars" :key="i" class="flex items-center gap-2">
        <n-checkbox v-model:checked="bar.enabled" class="shrink-0">
          {{ t('searchSet.showBar') }}
        </n-checkbox>
        <n-select
          v-model:value="bar.default"
          size="small"
          class="flex-1 min-w-0"
          :placeholder="t('searchSet.default')"
          :options="sourceOptions"
        />
        <n-button size="small" quaternary class="w-8 shrink-0" @click="removeBar(i)">
          <Icon icon="mdi:delete-outline" />
        </n-button>
      </div>

      <n-button
        size="small"
        dashed
        block
        :disabled="draft.search.bars.length >= MAX_BARS"
        @click="addBar"
      >
        {{ t('searchSet.addBar') }}
      </n-button>
      <p class="text-xs opacity-45">{{ t('searchSet.barsHint') }}</p>
      <p class="text-xs opacity-45">{{ t('searchSet.defaultHint') }}</p>
    </SettingsSection>

    <SettingsSection :title="t('searchSet.engines')">
      <div v-for="(e, i) in draft.search.engines" :key="i" class="flex gap-2">
        <n-input v-model:value="e.name" :placeholder="t('searchSet.engineName')" class="w-24" size="small" />
        <n-input v-model:value="e.url" placeholder="https://...q=%s" size="small" class="flex-1" />
        <n-button size="small" quaternary @click="removeEngine(i)">
          <Icon icon="mdi:delete-outline" />
        </n-button>
      </div>
      <n-button size="small" dashed block @click="addEngine">{{ t('searchSet.addEngine') }}</n-button>
      <p class="text-xs opacity-45">{{ t('searchSet.placeholderHint') }}</p>
    </SettingsSection>

    <div class="sticky bottom-0 py-2 flex justify-end bg-gradient-to-t from-black/[0.06] dark:from-white/[0.06] to-transparent backdrop-blur-sm">
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.saveSettings') }}</n-button>
    </div>
  </div>
</template>
