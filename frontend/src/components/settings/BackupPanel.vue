<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { ref } from 'vue'
import { useDialog, useMessage, type UploadCustomRequestOptions } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { request } from '@/api/http'
import { usePanelStore } from '@/stores/panel'
import { t } from '@/i18n'
import SettingsSection from './SettingsSection.vue'
import SettingsRow from './SettingsRow.vue'

const emit = defineEmits<{ changed: [] }>()

const panel = usePanelStore()
const message = useMessage()
const dialog = useDialog()

// 导出走普通 GET 链接：会话 Cookie 会自动带上，浏览器直接触发下载。
const exportUrl = (withFiles: boolean) => `/api/v1/backup/export${withFiles ? '?withFiles=1' : ''}`

const importMode = ref<'merge' | 'overwrite'>('merge')
const importing = ref(false)

async function doImport({ file }: UploadCustomRequestOptions) {
  const run = async () => {
    importing.value = true
    try {
      const fd = new FormData()
      fd.append('file', file.file as File)
      const res = await request<{ imported: number; skipped: number }>(
        `/backup/import?mode=${importMode.value}`,
        { method: 'POST', form: fd },
      )
      message.success(t('backup.done', { imported: res.imported, skipped: res.skipped }))
      await panel.loadAll()
      emit('changed')
    } catch (e: any) {
      message.error(e?.message ?? t('backup.failed'))
    } finally {
      importing.value = false
    }
  }
  if (importMode.value === 'overwrite') {
    dialog.warning({
      title: t('backup.confirmTitle'),
      content: t('backup.confirmTip'),
      positiveText: t('backup.confirmOk'),
      negativeText: t('common.cancel'),
      onPositiveClick: run,
    })
  } else {
    await run()
  }
  return false
}
</script>

<template>
  <div class="space-y-3">
    <SettingsSection :title="t('backup.export')">
      <div class="flex gap-2">
        <n-button size="small" tag="a" :href="exportUrl(false)" download>
          <Icon icon="mdi:download" class="mr-1" />{{ t('backup.exportJson') }}
        </n-button>
        <n-button size="small" tag="a" :href="exportUrl(true)" download>
          <Icon icon="mdi:folder-zip-outline" class="mr-1" />{{ t('backup.exportZip') }}
        </n-button>
      </div>
      <p class="text-xs opacity-45">{{ t('backup.exportHint') }}</p>
    </SettingsSection>

    <SettingsSection :title="t('backup.import')">
      <SettingsRow :label="t('backup.mode')">
        <n-radio-group v-model:value="importMode" size="small">
          <n-radio-button value="merge">{{ t('backup.merge') }}</n-radio-button>
          <n-radio-button value="overwrite">{{ t('backup.overwrite') }}</n-radio-button>
        </n-radio-group>
      </SettingsRow>

      <n-upload :custom-request="doImport" :show-file-list="false" accept=".json,.zip">
        <n-button size="small" :loading="importing" block dashed>
          <Icon icon="mdi:upload" class="mr-1" />{{ t('backup.pick') }}
        </n-button>
      </n-upload>
    </SettingsSection>
  </div>
</template>
