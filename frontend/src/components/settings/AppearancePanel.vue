<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { ref } from 'vue'
import { useMessage, type UploadCustomRequestOptions } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { api } from '@/api/http'
import { usePanelStore } from '@/stores/panel'
import { deepClone } from '@/utils/clone'
import { t } from '@/i18n'
import GalleryPanel from './GalleryPanel.vue'
import SettingsSection from './SettingsSection.vue'
import SettingsRow from './SettingsRow.vue'
import type { Settings } from '@/api/types'

const emit = defineEmits<{ changed: [] }>()

const panel = usePanelStore()
const message = useMessage()
const draft = ref<Settings>(deepClone(panel.settings))
const saving = ref(false)
const showGallery = ref(false)

function pickFromGallery(path: string) {
  draft.value.background.type = 'image'
  draft.value.background.value = path
  showGallery.value = false
  message.success(t('appearance.pickedHint'))
}

const footerExample =
  '<div class="flex justify-center text-slate-300" style="margin-top:100px">Powered By <a href="https://github.com/hslr-s/sun-panel" target="_blank" class="ml-[5px]">Sun-Panel</a></div>'

const presetGradients = [
  'linear-gradient(135deg,#1e3a8a 0%,#0f172a 60%,#312e81 100%)',
  'linear-gradient(135deg,#0f2027 0%,#203a43 50%,#2c5364 100%)',
  'linear-gradient(135deg,#42275a 0%,#734b6d 100%)',
  'linear-gradient(135deg,#134e5e 0%,#71b280 100%)',
  'linear-gradient(135deg,#232526 0%,#414345 100%)',
  'linear-gradient(135deg,#ff6e7f 0%,#bfe9ff 100%)',
]

async function applyImage(file: File) {
  const fd = new FormData()
  fd.append('file', file)
  fd.append('kind', 'bg')
  const res = await api.upload<{ path: string }>('/upload', fd)
  draft.value.background.type = 'image'
  draft.value.background.value = res.path
}

async function uploadBackground({ file }: UploadCustomRequestOptions) {
  try {
    await applyImage(file.file as File)
    message.success(t('appearance.uploadedHint'))
  } catch (e: any) {
    message.error(e?.message ?? t('common.uploadFailed'))
  }
  return false
}

function onDrop(e: DragEvent) {
  const file = e.dataTransfer?.files?.[0]
  if (!file || !file.type.startsWith('image/')) return
  applyImage(file)
    .then(() => message.success(t('appearance.uploadedHint')))
    .catch((err: any) => message.error(err?.message ?? t('common.uploadFailed')))
}

function resetSearchStyle() {
  draft.value.search.style = { bg: 'rgba(255,255,255,0.12)', color: '#ffffff', border: 'rgba(255,255,255,0.16)' }
}

// 三种类型的 value 形状互不兼容。不重置的话，保存时 normalizeSettings
// 会把非法值连同 type 一起打回默认渐变，界面上看着切了、一刷新又回去。
const bgDefaults: Record<Settings['background']['type'], string> = {
  gradient: presetGradients[0],
  color: '#0f172a',
  image: '',
}

function onBgType(next: string) {
  if (next !== 'gradient' && next !== 'color' && next !== 'image') return
  if (draft.value.background.type === next) return
  draft.value.background.type = next
  draft.value.background.value = bgDefaults[next]
}

async function save() {
  saving.value = true
  try {
    draft.value.layout.siteName = draft.value.layout.logoText
    draft.value.layout.showTitle = draft.value.layout.showLogo
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
    <SettingsSection :title="t('appearance.logo')">
      <SettingsRow :label="t('appearance.show')">
        <n-switch v-model:value="draft.layout.showLogo" size="small" />
      </SettingsRow>
      <SettingsRow :label="t('appearance.logoText')" stack>
        <n-input
          v-model:value="draft.layout.logoText"
          :placeholder="t('home.defaultTitle')"
          :disabled="!draft.layout.showLogo"
          :maxlength="32"
          show-count
        />
      </SettingsRow>
    </SettingsSection>

    <SettingsSection :title="t('appearance.clock')">
      <SettingsRow :label="t('appearance.show')">
        <n-switch v-model:value="draft.layout.showClock" size="small" />
      </SettingsRow>
    </SettingsSection>

    <SettingsSection :title="t('appearance.searchBar')">
      <template #extra>
        <n-button size="tiny" quaternary type="warning" @click="resetSearchStyle">
          {{ t('appearance.resetColors') }}
        </n-button>
      </template>
      <SettingsRow :label="t('appearance.show')">
        <n-switch v-model:value="draft.search.enabled" size="small" />
      </SettingsRow>
      <SettingsRow :label="t('appearance.bgColor')" grow>
        <n-color-picker
          :value="draft.search.style.bg || 'rgba(255,255,255,0.12)'"
          size="small"
          show-alpha
          @update:value="draft.search.style.bg = $event"
        />
      </SettingsRow>
      <SettingsRow :label="t('appearance.textColor')" grow>
        <n-color-picker
          :value="draft.search.style.color || '#ffffff'"
          size="small"
          :show-alpha="false"
          @update:value="draft.search.style.color = $event"
        />
      </SettingsRow>
      <SettingsRow :label="t('appearance.borderColor')" grow>
        <n-color-picker
          :value="draft.search.style.border || 'rgba(255,255,255,0.16)'"
          size="small"
          show-alpha
          @update:value="draft.search.style.border = $event"
        />
      </SettingsRow>
    </SettingsSection>

    <SettingsSection :title="t('appearance.wallpaper')">
      <SettingsRow :label="t('appearance.type')">
        <n-radio-group :value="draft.background.type" size="small" @update:value="onBgType">
          <n-radio-button value="gradient">{{ t('appearance.gradient') }}</n-radio-button>
          <n-radio-button value="color">{{ t('appearance.solid') }}</n-radio-button>
          <n-radio-button value="image">{{ t('appearance.image') }}</n-radio-button>
        </n-radio-group>
      </SettingsRow>

      <SettingsRow v-if="draft.background.type === 'gradient'" :label="t('appearance.presetGradients')" stack>
        <div class="grid grid-cols-3 gap-2">
          <button
            v-for="g in presetGradients"
            :key="g"
            type="button"
            class="h-12 rounded-lg border-2"
            :class="draft.background.value === g ? 'border-sky-400' : 'border-transparent'"
            :style="{ background: g }"
            @click="draft.background.value = g"
          />
        </div>
      </SettingsRow>

      <SettingsRow v-else-if="draft.background.type === 'color'" :label="t('appearance.bgColor')" grow>
        <n-color-picker v-model:value="draft.background.value" size="small" :show-alpha="false" />
      </SettingsRow>

      <SettingsRow v-else :label="t('appearance.bgImage')" stack>
        <div class="space-y-2" @dragover.prevent @drop.prevent="onDrop">
          <n-input v-model:value="draft.background.value" :placeholder="t('appearance.bgImagePlaceholder')" />
          <n-upload
            :custom-request="uploadBackground"
            :show-file-list="false"
            accept="image/png,image/jpeg,image/webp,image/gif"
          >
            <n-upload-dragger>
              <div class="py-3 text-center">
                <Icon icon="mdi:image-plus" class="text-2xl opacity-60" />
                <p class="text-xs opacity-60 mt-1">{{ t('appearance.dropHint') }}</p>
              </div>
            </n-upload-dragger>
          </n-upload>
          <n-button size="small" block dashed @click="showGallery = true">
            <Icon icon="mdi:image-multiple-outline" class="mr-1" />{{ t('appearance.pickGallery') }}
          </n-button>
        </div>
      </SettingsRow>

      <SettingsRow :label="t('appearance.blur', { n: draft.background.blur })" stack>
        <n-slider v-model:value="draft.background.blur" :min="0" :max="30" />
      </SettingsRow>
      <SettingsRow :label="t('appearance.mask', { n: Math.round(draft.background.mask * 100) })" stack>
        <n-slider v-model:value="draft.background.mask" :min="0" :max="0.85" :step="0.05" />
      </SettingsRow>
    </SettingsSection>

    <SettingsSection :title="t('appearance.cards')">
      <SettingsRow :label="t('appearance.cardSize')">
        <n-radio-group v-model:value="draft.layout.cardSize" size="small">
          <n-radio-button value="sm">{{ t('appearance.sizeSm') }}</n-radio-button>
          <n-radio-button value="md">{{ t('appearance.sizeMd') }}</n-radio-button>
          <n-radio-button value="lg">{{ t('appearance.sizeLg') }}</n-radio-button>
        </n-radio-group>
      </SettingsRow>
      <SettingsRow :label="t('appearance.showDesc')">
        <n-switch v-model:value="draft.layout.showDesc" size="small" />
      </SettingsRow>
      <SettingsRow :label="t('appearance.groupStyle')">
        <n-radio-group v-model:value="draft.layout.groupStyle" size="small">
          <n-radio-button value="section">{{ t('appearance.groupSection') }}</n-radio-button>
          <n-radio-button value="tabs">{{ t('appearance.groupTabs') }}</n-radio-button>
        </n-radio-group>
      </SettingsRow>
      <SettingsRow :label="t('appearance.theme')">
        <n-radio-group v-model:value="draft.theme" size="small">
          <n-radio-button value="auto">{{ t('appearance.themeAuto') }}</n-radio-button>
          <n-radio-button value="light">{{ t('appearance.themeLight') }}</n-radio-button>
          <n-radio-button value="dark">{{ t('appearance.themeDark') }}</n-radio-button>
        </n-radio-group>
      </SettingsRow>
      <!-- 语言存在服务端设置里，和主题一路；顶栏那颗按钮改的是同一个字段。 -->
      <SettingsRow :label="t('appearance.language')">
        <n-radio-group v-model:value="draft.language" size="small">
          <n-radio-button value="zh">中文</n-radio-button>
          <n-radio-button value="en">English</n-radio-button>
        </n-radio-group>
      </SettingsRow>
    </SettingsSection>

    <SettingsSection :title="t('appearance.footer')">
      <n-input
        v-model:value="draft.layout.footerHtml"
        type="textarea"
        :autosize="{ minRows: 3, maxRows: 6 }"
        :placeholder="footerExample"
      />
    </SettingsSection>

    <!-- 内容区是滚动的，保存按钮贴在底部，滚到哪都点得到 -->
    <div class="sticky bottom-0 -mx-0.5 px-0.5 py-2 flex justify-end bg-gradient-to-t from-black/[0.06] dark:from-white/[0.06] to-transparent backdrop-blur-sm">
      <n-button type="primary" :loading="saving" @click="save">{{ t('common.saveSettings') }}</n-button>
    </div>
  </div>

  <n-modal
    v-model:show="showGallery"
    preset="card"
    :title="t('appearance.pickBgTitle')"
    class="max-w-xl"
    :bordered="false"
  >
    <GalleryPanel kind="bg" selectable @select="pickFromGallery" />
  </n-modal>
</template>
