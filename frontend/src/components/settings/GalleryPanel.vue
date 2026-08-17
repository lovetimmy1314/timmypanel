<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, onMounted, ref } from 'vue'
import { useDialog, useMessage, type UploadCustomRequestOptions } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { api } from '@/api/http'
import { t } from '@/i18n'
import type { UploadItem } from '@/api/types'

// kind 固定住时（壁纸选图、卡片图标选图）不显示筛选条，上传也直接用这个类型。
const props = defineProps<{ kind?: 'bg' | 'icons'; selectable?: boolean }>()
const emit = defineEmits<{ changed: []; select: [string] }>()

const message = useMessage()
const dialog = useDialog()

const items = ref<UploadItem[]>([])
const loading = ref(false)
const filter = ref<'all' | 'bg' | 'icons'>('all')
const uploadKind = computed(() => props.kind ?? (filter.value === 'icons' ? 'icons' : 'bg'))

const shown = computed(() => {
  if (props.kind) return items.value.filter((i) => i.kind === props.kind)
  if (filter.value === 'all') return items.value
  return items.value.filter((i) => i.kind === filter.value)
})

async function load() {
  loading.value = true
  try {
    items.value = (await api.get<{ items: UploadItem[] }>('/uploads')).items ?? []
  } catch (e: any) {
    message.error(e?.message ?? t('gallery.loadFailed'))
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function doUpload({ file }: UploadCustomRequestOptions) {
  try {
    const fd = new FormData()
    fd.append('file', file.file as File)
    fd.append('kind', uploadKind.value)
    await api.upload<{ path: string }>('/upload', fd)
    await load()
    emit('changed')
    message.success(t('common.uploaded'))
  } catch (e: any) {
    message.error(e?.message ?? t('common.uploadFailed'))
  }
  return false
}

function onDrop(e: DragEvent) {
  const file = e.dataTransfer?.files?.[0]
  if (!file || !file.type.startsWith('image/')) return
  doUpload({ file: { file } } as unknown as UploadCustomRequestOptions)
}

function remove(item: UploadItem) {
  dialog.warning({
    title: t('gallery.deleteTitle'),
    content: t('gallery.deleteTip'),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await api.del(`/uploads/${item.id}`)
        items.value = items.value.filter((i) => i.id !== item.id)
        emit('changed')
      } catch (e: any) {
        message.error(e?.message ?? t('common.deleteFailed'))
      }
    },
  })
}

const sizeText = (n: number) => (n < 1024 * 1024 ? `${Math.round(n / 1024)} KB` : `${(n / 1024 / 1024).toFixed(1)} MB`)
</script>

<template>
  <div class="space-y-3" @dragover.prevent @drop.prevent="onDrop">
    <div class="flex items-center gap-2">
      <n-radio-group v-if="!props.kind" v-model:value="filter" size="small">
        <n-radio-button value="all">{{ t('gallery.all') }}</n-radio-button>
        <n-radio-button value="bg">{{ t('gallery.bg') }}</n-radio-button>
        <n-radio-button value="icons">{{ t('gallery.icons') }}</n-radio-button>
      </n-radio-group>
      <span v-else class="text-xs opacity-55">
        {{ props.kind === 'bg' ? t('gallery.kindBg') : t('gallery.kindIcons') }}
      </span>

      <n-upload
        class="ml-auto w-auto"
        :custom-request="doUpload"
        :show-file-list="false"
        accept="image/png,image/jpeg,image/webp,image/gif"
      >
        <n-button size="small">
          <Icon icon="mdi:upload" class="mr-1" />{{ t('common.upload') }}
        </n-button>
      </n-upload>
      <n-button size="small" quaternary :title="t('common.refresh')" @click="load">
        <Icon icon="mdi:refresh" />
      </n-button>
    </div>

    <n-spin :show="loading">
      <p v-if="!shown.length" class="text-sm opacity-55 py-6 text-center">{{ t('gallery.empty') }}</p>

      <div v-else class="grid grid-cols-3 sm:grid-cols-4 gap-2 max-h-80 overflow-y-auto">
        <div
          v-for="item in shown"
          :key="item.id"
          class="relative group rounded-lg overflow-hidden border border-black/5 dark:border-white/10"
        >
          <img :src="item.path" class="w-full h-20 object-cover bg-black/5 dark:bg-white/5" loading="lazy" alt="" />
          <div class="px-1.5 py-1 text-[10px] opacity-50 truncate">{{ sizeText(item.size) }}</div>

          <div
            class="absolute inset-0 bg-black/55 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center gap-1"
          >
            <n-button v-if="selectable" size="tiny" type="primary" @click="emit('select', item.path)">
              {{ t('common.use') }}
            </n-button>
            <n-button size="tiny" @click="remove(item)">
              <Icon icon="mdi:trash-can-outline" />
            </n-button>
          </div>
        </div>
      </div>
    </n-spin>

    <p class="text-xs opacity-45">{{ t('gallery.footer') }}</p>
  </div>
</template>
