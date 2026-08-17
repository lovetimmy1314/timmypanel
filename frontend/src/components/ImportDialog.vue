<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, ref } from 'vue'
import { useMessage, type UploadCustomRequestOptions } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { api, request } from '@/api/http'
import { t } from '@/i18n'
import type { Group, ImportItem } from '@/api/types'

const props = defineProps<{ show: boolean; groups: Group[] }>()
const emit = defineEmits<{ 'update:show': [boolean]; imported: [] }>()

const message = useMessage()
const tab = ref<'text' | 'bookmark' | 'json'>('text')
const text = ref('')
const jsonText = ref('')
const targetGroupId = ref(0)
const dedupe = ref(true)
const autoIcon = ref(true)
const parsing = ref(false)
const importing = ref(false)

// 预览列表：三种来源解析完都汇到这里，确认无误再真正写库。
const preview = ref<ImportItem[]>([])
const selected = ref<Set<number>>(new Set())

const groupOptions = computed(() => [
  { label: t('common.ungrouped'), value: 0 },
  ...props.groups.map((g) => ({ label: g.name, value: g.id })),
])

function resetPreview(items: ImportItem[]) {
  preview.value = items
  selected.value = new Set(items.map((_, i) => i))
}

// 文本格式：一行一条，支持 URL / URL,标题 / URL,标题,分组
function parseText() {
  const items: ImportItem[] = []
  for (const line of text.value.split('\n')) {
    const raw = line.trim()
    if (!raw || raw.startsWith('#')) continue
    const parts = raw.split(/[,，\t]/).map((p) => p.trim())
    const url = parts[0]
    if (!url) continue
    items.push({ url, title: parts[1] ?? '', groupName: parts[2] ?? '' })
  }
  if (!items.length) {
    message.warning(t('import.textNone'))
    return
  }
  resetPreview(items)
}

function parseJson() {
  try {
    const data = JSON.parse(jsonText.value)
    // 既支持纯数组，也支持直接粘贴一份备份文件。
    const arr: any[] = Array.isArray(data) ? data : (data.sites ?? data.items ?? [])
    const items = arr
      .filter((it) => it && typeof it.url === 'string')
      .map((it) => ({
        url: it.url,
        title: it.title ?? '',
        lanUrl: it.lanUrl ?? '',
        description: it.description ?? '',
        iconType: it.iconType,
        iconValue: it.iconValue ?? '',
        openMode: it.openMode,
        hidden: !!it.hidden,
        groupName: it.groupName ?? '',
      }))
    if (!items.length) {
      message.warning(t('import.jsonNone'))
      return
    }
    resetPreview(items)
  } catch (e: any) {
    message.error(t('import.jsonBad', { error: e.message }))
  }
}

// 书签文件交给后端解析：Netscape HTML 嵌套深、经常缺闭合标签，
// 后端用 HTML tokenizer 处理比前端正则可靠得多。
async function parseBookmarkFile({ file }: UploadCustomRequestOptions) {
  parsing.value = true
  try {
    const fd = new FormData()
    fd.append('file', file.file as File)
    const res = await request<{ items: ImportItem[]; count: number }>('/sites/bookmarks', {
      method: 'POST',
      form: fd,
    })
    if (!res.count) {
      message.warning(t('import.bookmarkNone'))
      return
    }
    resetPreview(res.items)
    message.success(t('import.bookmarkDone', { n: res.count }))
  } catch (e: any) {
    message.error(e?.message ?? t('common.parseFailed'))
  } finally {
    parsing.value = false
  }
  return false
}

function toggle(i: number) {
  const next = new Set(selected.value)
  next.has(i) ? next.delete(i) : next.add(i)
  selected.value = next
}

const selectedItems = computed(() => preview.value.filter((_, i) => selected.value.has(i)))

async function doImport() {
  if (!selectedItems.value.length) {
    message.warning(t('import.needOne'))
    return
  }
  importing.value = true
  try {
    const res = await api.post<{ created: number; skipped: number; invalid: number }>('/sites/batch', {
      groupId: targetGroupId.value,
      dedupe: dedupe.value,
      autoIcon: autoIcon.value,
      items: selectedItems.value,
    })
    message.success(
      t('import.done', { created: res.created }) +
        (res.skipped ? t('import.doneSkipped', { n: res.skipped }) : '') +
        (res.invalid ? t('import.doneInvalid', { n: res.invalid }) : ''),
    )
    if (autoIcon.value && res.created) {
      message.info(t('import.backgroundFetch'))
    }
    emit('imported')
    close()
  } catch (e: any) {
    message.error(e?.message ?? t('import.failed'))
  } finally {
    importing.value = false
  }
}

function close() {
  preview.value = []
  selected.value = new Set()
  text.value = ''
  jsonText.value = ''
  emit('update:show', false)
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    class="max-w-3xl"
    :title="t('import.title')"
    @update:show="!$event && close()"
  >
    <n-tabs v-model:value="tab" type="line" animated>
      <n-tab-pane name="text" :tab="t('import.tabText')">
        <p class="text-xs opacity-60 mb-2">{{ t('import.textHint') }}</p>
        <n-input
          v-model:value="text"
          type="textarea"
          :rows="9"
          placeholder="https://github.com,GitHub,开发&#10;https://news.ycombinator.com"
        />
        <n-button class="mt-2" @click="parseText">{{ t('common.parse') }}</n-button>
      </n-tab-pane>

      <n-tab-pane name="bookmark" :tab="t('import.tabBookmark')">
        <p class="text-xs opacity-60 mb-3">{{ t('import.bookmarkHint') }}</p>
        <n-upload
          :custom-request="parseBookmarkFile"
          :show-file-list="false"
          accept=".html,.htm"
        >
          <n-upload-dragger>
            <div class="py-6">
              <Icon icon="mdi:bookmark-multiple-outline" class="text-3xl opacity-60" />
              <p class="mt-2 text-sm">{{ t('import.bookmarkDrop') }}</p>
            </div>
          </n-upload-dragger>
        </n-upload>
        <n-spin v-if="parsing" size="small" class="mt-2" />
      </n-tab-pane>

      <n-tab-pane name="json" :tab="t('import.tabJson')">
        <p class="text-xs opacity-60 mb-2">{{ t('import.jsonHint') }}</p>
        <n-input
          v-model:value="jsonText"
          type="textarea"
          :rows="9"
          placeholder='[{"url":"https://github.com","title":"GitHub","groupName":"开发"}]'
        />
        <n-button class="mt-2" @click="parseJson">{{ t('common.parse') }}</n-button>
      </n-tab-pane>
    </n-tabs>

    <template v-if="preview.length">
      <n-divider class="!my-3" />
      <div class="flex flex-wrap items-center gap-3 mb-2">
        <span class="text-sm">
          {{ t('import.selected', { n: selectedItems.length, total: preview.length }) }}
        </span>
        <n-button size="tiny" @click="selected = new Set(preview.map((_, i) => i))">
          {{ t('common.selectAll') }}
        </n-button>
        <n-button size="tiny" @click="selected = new Set()">{{ t('common.selectNone') }}</n-button>
        <div class="flex items-center gap-2 ml-auto">
          <span class="text-sm">{{ t('import.targetGroup') }}</span>
          <n-select v-model:value="targetGroupId" :options="groupOptions" size="small" class="w-32" />
        </div>
      </div>
      <div class="flex gap-4 mb-2">
        <n-checkbox v-model:checked="dedupe">{{ t('import.dedupe') }}</n-checkbox>
        <n-checkbox v-model:checked="autoIcon">{{ t('import.autoIcon') }}</n-checkbox>
      </div>

      <div class="max-h-64 overflow-y-auto rounded border border-black/10 dark:border-white/10">
        <div
          v-for="(item, i) in preview"
          :key="i"
          class="flex items-center gap-2 px-3 py-1.5 text-sm border-b border-black/5 dark:border-white/5 last:border-0"
        >
          <n-checkbox :checked="selected.has(i)" @update:checked="toggle(i)" />
          <span class="truncate w-40">{{ item.title || t('import.autoTitle') }}</span>
          <span class="truncate flex-1 opacity-60 text-xs">{{ item.url }}</span>
          <n-tag v-if="item.groupName" size="tiny">{{ item.groupName }}</n-tag>
        </div>
      </div>
    </template>

    <template #footer>
      <div class="flex justify-end gap-2">
        <n-button @click="close">{{ t('common.cancel') }}</n-button>
        <n-button
          type="primary"
          :disabled="!selectedItems.length"
          :loading="importing"
          @click="doImport"
        >
          {{ t('import.submit', { n: selectedItems.length || '' }) }}
        </n-button>
      </div>
    </template>
  </n-modal>
</template>
