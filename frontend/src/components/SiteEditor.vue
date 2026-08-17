<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, ref, watch } from 'vue'
import { useMessage, type UploadCustomRequestOptions } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { api } from '@/api/http'
import GalleryPanel from '@/components/settings/GalleryPanel.vue'
import { t } from '@/i18n'
import { hasOfflineIcon, iconSetReady, loadIconSet } from '@/icons'
import type { Group, ParseResult, Site } from '@/api/types'

const props = withDefaults(
  defineProps<{
    show: boolean
    site: Site | null
    groups: Group[]
    defaultGroupId: number
    // 从分组名旁的「添加」进来时为 true：分组已确定为 defaultGroupId，
    // 新建表单不再渲染分组选择。编辑已有卡片时不受影响（仍可调组）。
    lockGroup?: boolean
  }>(),
  { lockGroup: false },
)

const emit = defineEmits<{ 'update:show': [boolean]; saved: [] }>()

const message = useMessage()
const saving = ref(false)
const fetching = ref(false)
const showGallery = ref(false)

function pickIcon(path: string) {
  form.value.iconType = 'url'
  form.value.iconValue = path
  showGallery.value = false
}

const form = ref({
  groupId: 0,
  title: '',
  url: '',
  lanUrl: '',
  description: '',
  iconType: 'url' as Site['iconType'],
  iconValue: '',
  iconBg: '',
  openMode: 'blank' as Site['openMode'],
  hidden: false,
})

watch(
  () => props.show,
  (open) => {
    if (!open) return
    if (props.site?.iconType === 'iconify') loadIconSet()
    form.value = props.site
      ? { ...props.site }
      : {
          groupId: props.defaultGroupId,
          title: '',
          url: '',
          lanUrl: '',
          description: '',
          iconType: 'url',
          iconValue: '',
          iconBg: '',
          openMode: 'blank',
          hidden: false,
        }
  },
)

const groupOptions = computed(() => [
  { label: t('common.ungrouped'), value: 0 },
  ...props.groups.map((g) => ({ label: g.name, value: g.id })),
])

// 图标集是打包进来的（决策 019），所以能当场告诉用户这个名字有没有货，
// 不用等保存完回首页看到一个空方块。图标集还没加载完时不下结论。
watch(
  () => form.value.iconType,
  (type) => {
    if (type === 'iconify') loadIconSet()
  },
)

const iconNameMissing = computed(() => {
  const v = form.value.iconValue.trim()
  return iconSetReady.value && form.value.iconType === 'iconify' && v !== '' && !hasOfflineIcon(v)
})

// 自动抓取：填了网址后点一下，后端去取标题和图标并把图标存到本地。
async function autoFetch() {
  if (!form.value.url.trim()) {
    message.warning(t('editor.needUrl'))
    return
  }
  fetching.value = true
  try {
    const res = await api.post<{ items: ParseResult[] }>('/sites/parse', { urls: [form.value.url] })
    const r = res.items[0]
    if (!r) return
    if (r.title && !form.value.title) form.value.title = r.title
    if (r.description && !form.value.description) form.value.description = r.description
    if (r.iconUrl) {
      form.value.iconType = 'url'
      form.value.iconValue = r.iconUrl
    }
    if (r.error) message.warning(t('editor.fetchPartial', { error: r.error }))
    else message.success(t('editor.fetchDone'))
  } catch (e: any) {
    message.error(e?.message ?? t('editor.fetchFailed'))
  } finally {
    fetching.value = false
  }
}

async function uploadIcon({ file }: UploadCustomRequestOptions) {
  const fd = new FormData()
  fd.append('file', file.file as File)
  fd.append('kind', 'icons')
  const res = await api.upload<{ path: string }>('/upload', fd)
  form.value.iconType = 'url'
  form.value.iconValue = res.path
  message.success(t('editor.iconUploaded'))
  return false
}

async function submit() {
  if (!form.value.url.trim()) {
    message.warning(t('editor.urlRequired'))
    return
  }
  saving.value = true
  try {
    if (props.site) await api.put(`/sites/${props.site.id}`, form.value)
    else await api.post('/sites', form.value)
    emit('saved')
    emit('update:show', false)
  } catch (e: any) {
    message.error(e?.message ?? t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    class="max-w-lg"
    :title="site ? t('editor.titleEdit') : t('editor.titleNew')"
    @update:show="emit('update:show', $event)"
  >
    <n-form label-placement="top" size="small">
      <n-form-item :label="t('editor.url')">
        <n-input-group>
          <n-input v-model:value="form.url" placeholder="https://example.com" />
          <n-button :loading="fetching" @click="autoFetch">
            <Icon icon="mdi:auto-fix" class="mr-1" />{{ t('editor.autoFetch') }}
          </n-button>
        </n-input-group>
      </n-form-item>

      <n-form-item :label="t('editor.name')">
        <n-input v-model:value="form.title" :placeholder="t('editor.namePlaceholder')" />
      </n-form-item>

      <n-form-item :label="t('editor.lanUrl')">
        <n-input v-model:value="form.lanUrl" placeholder="http://192.168.1.10:8080" />
      </n-form-item>

      <n-form-item :label="t('editor.desc')">
        <n-input v-model:value="form.description" :placeholder="t('editor.descPlaceholder')" />
      </n-form-item>

      <n-form-item v-if="!lockGroup || site" :label="t('editor.group')">
        <n-select v-model:value="form.groupId" :options="groupOptions" />
      </n-form-item>

      <n-form-item :label="t('editor.icon')">
        <div class="w-full space-y-2">
          <n-radio-group v-model:value="form.iconType" size="small">
            <n-radio-button value="url">{{ t('editor.iconImage') }}</n-radio-button>
            <n-radio-button value="iconify">{{ t('editor.iconLibrary') }}</n-radio-button>
            <n-radio-button value="text">{{ t('editor.iconText') }}</n-radio-button>
          </n-radio-group>

          <n-input-group v-if="form.iconType === 'url'">
            <n-input v-model:value="form.iconValue" :placeholder="t('editor.iconImagePlaceholder')" />
            <n-upload :custom-request="uploadIcon" :show-file-list="false">
              <n-button>{{ t('common.upload') }}</n-button>
            </n-upload>
            <n-button @click="showGallery = true">
              <Icon icon="mdi:image-multiple-outline" />
            </n-button>
          </n-input-group>

          <template v-else-if="form.iconType === 'iconify'">
            <n-input v-model:value="form.iconValue" :placeholder="t('editor.iconNamePlaceholder')" />
            <p v-if="iconNameMissing" class="text-xs text-amber-500">{{ t('editor.iconNameMissing') }}</p>
          </template>

          <div v-else class="flex gap-2">
            <n-input v-model:value="form.iconValue" :placeholder="t('editor.iconTextPlaceholder')" class="flex-1" />
            <n-color-picker v-model:value="form.iconBg" class="w-32" :show-alpha="false" />
          </div>
        </div>
      </n-form-item>

      <n-form-item :label="t('editor.openMode')">
        <n-radio-group v-model:value="form.openMode" size="small">
          <n-radio-button value="blank">{{ t('editor.openBlank') }}</n-radio-button>
          <n-radio-button value="self">{{ t('editor.openSelf') }}</n-radio-button>
        </n-radio-group>
      </n-form-item>

      <n-form-item :label="t('editor.hidden')">
        <div class="flex items-center gap-2">
          <n-switch v-model:value="form.hidden" />
          <span class="text-xs opacity-50">{{ t('editor.hiddenHint') }}</span>
        </div>
      </n-form-item>
    </n-form>

    <template #footer>
      <div class="flex justify-end gap-2">
        <n-button @click="emit('update:show', false)">{{ t('common.cancel') }}</n-button>
        <n-button type="primary" :loading="saving" @click="submit">{{ t('common.save') }}</n-button>
      </div>
    </template>
  </n-modal>

  <n-modal
    v-model:show="showGallery"
    preset="card"
    :title="t('editor.pickFromGallery')"
    class="max-w-xl"
    :bordered="false"
  >
    <GalleryPanel kind="icons" selectable @select="pickIcon" />
  </n-modal>
</template>
