<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
// 万年历组件的设置：开关、每周从哪天起排。农历/节气/节日没有可配的东西——
// 它们由后端按中国历法算死（决策 034）。
import { ref } from 'vue'
import { useMessage } from 'naive-ui'
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
    <SettingsSection :title="t('settings.nav.calendar')">
      <SettingsRow :label="t('appearance.show')" :hint="t('calendar.pcOnlyHint')">
        <n-switch v-model:value="draft.calendar.enabled" size="small" />
      </SettingsRow>
      <SettingsRow :label="t('calendar.weekStart')">
        <n-radio-group v-model:value="draft.calendar.weekStart" size="small">
          <n-radio-button value="mon">{{ t('calendar.weekStartMon') }}</n-radio-button>
          <n-radio-button value="sun">{{ t('calendar.weekStartSun') }}</n-radio-button>
        </n-radio-group>
      </SettingsRow>
      <SettingsRow :label="t('calendar.off') + ' / ' + t('calendar.work')" :hint="t('calendar.holidayHint')" stack />
    </SettingsSection>

    <div class="sticky bottom-0 -mx-0.5 px-0.5 py-2 flex justify-end bg-gradient-to-t from-black/[0.06] dark:from-white/[0.06] to-transparent backdrop-blur-sm">
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.saveSettings') }}</n-button>
    </div>
  </div>
</template>
