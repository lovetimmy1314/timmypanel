<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { ref } from 'vue'
import { useMessage } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { usePanelStore } from '@/stores/panel'
import { deepClone } from '@/utils/clone'
import { t } from '@/i18n'
import SettingsSection from './SettingsSection.vue'
import SettingsRow from './SettingsRow.vue'
import type { Settings } from '@/api/types'

const emit = defineEmits<{ changed: [] }>()

const panel = usePanelStore()
const message = useMessage()
const draft = ref<Settings>(deepClone(panel.settings))
const saving = ref(false)

function addEngine() {
  draft.value.search.engines.push({ name: '', url: 'https://example.com/search?q=%s', icon: '' })
}
function removeEngine(i: number) {
  const name = draft.value.search.engines[i]?.name
  draft.value.search.engines.splice(i, 1)
  if (draft.value.search.default === name) {
    draft.value.search.default = 'local'
  }
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
    <SettingsSection :title="t('searchSet.default')">
      <SettingsRow :hint="t('searchSet.defaultHint')" stack>
        <n-select
          v-model:value="draft.search.default"
          size="small"
          :options="[
            { label: t('search.local'), value: 'local' },
            ...draft.search.engines.map((e) => ({ label: e.name, value: e.name })),
          ]"
        />
      </SettingsRow>
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
