<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, ref } from 'vue'
import { VueDraggable } from 'vue-draggable-plus'
import { useDialog, useMessage } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { usePanelStore } from '@/stores/panel'
import { t } from '@/i18n'
import SettingsSection from './SettingsSection.vue'
import type { Group } from '@/api/types'

const emit = defineEmits<{ changed: [] }>()

const panel = usePanelStore()
const message = useMessage()
const dialog = useDialog()

const newName = ref('')
const creating = ref(false)
const editingId = ref(0)
const editingName = ref('')

// 卡片数按 groupId 现算，不额外请求。找不到分组的卡片归「未分组」，不在这里管。
const countOf = (id: number) => panel.sites.filter((s) => s.groupId === id).length

const looseCount = computed(() => {
  const ids = new Set(panel.groups.map((g) => g.id))
  return panel.sites.filter((s) => !ids.has(s.groupId)).length
})

async function create() {
  const name = newName.value.trim()
  if (!name) return
  creating.value = true
  try {
    await panel.createGroup(name)
    await panel.reloadGroups()
    newName.value = ''
    emit('changed')
  } catch (e: any) {
    message.error(e?.message ?? t('home.newGroupFailed'))
  } finally {
    creating.value = false
  }
}

function startRename(g: Group) {
  editingId.value = g.id
  editingName.value = g.name
}

async function commitRename() {
  const g = panel.groups.find((x) => x.id === editingId.value)
  const name = editingName.value.trim()
  editingId.value = 0
  if (!g || !name || name === g.name) return
  try {
    await panel.updateGroup(g.id, { name })
    await panel.reloadGroups()
    emit('changed')
  } catch (e: any) {
    message.error(e?.message ?? t('common.renameFailed'))
  }
}

function remove(g: Group) {
  const n = countOf(g.id)
  dialog.warning({
    title: t('home.deleteGroup'),
    content: n
      ? t('groups.deleteWithCards', { name: g.name, n })
      : t('groups.deleteEmpty', { name: g.name }),
    positiveText: t('home.deleteGroup'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await panel.deleteGroup(g.id, false)
        await panel.loadAll()
        emit('changed')
      } catch (e: any) {
        message.error(e?.message ?? t('common.deleteFailed'))
      }
    },
  })
}

// v-model 必须直接绑 store 里那个数组：拖拽结果要写回它本身，
// persistGroupOrder 读的也是它。绑到 filter/map 出来的副本就会拖完不落库（见 CLAUDE.md）。
async function onDragEnd() {
  try {
    await panel.persistGroupOrder()
    emit('changed')
  } catch (e: any) {
    message.error(e?.message ?? t('common.sortFailed'))
    await panel.reloadGroups()
  }
}
</script>

<template>
  <SettingsSection :title="t('groups.title')">
    <div class="flex gap-2">
      <n-input
        v-model:value="newName"
        :placeholder="t('groups.newName')"
        size="small"
        @keydown.enter.prevent="create"
      />
      <n-button size="small" type="primary" :loading="creating" @click="create">
        {{ t('common.create') }}
      </n-button>
    </div>

    <p v-if="!panel.groups.length" class="text-sm opacity-60">{{ t('groups.emptyHint') }}</p>

    <VueDraggable
      v-else
      v-model="panel.groups"
      :animation="160"
      handle=".tp-group-handle"
      ghost-class="tp-ghost"
      class="space-y-1.5"
      @end="onDragEnd"
    >
      <div
        v-for="g in panel.groups"
        :key="g.id"
        class="flex items-center gap-2 rounded-lg border border-black/5 dark:border-white/10 bg-black/[0.03] dark:bg-white/[0.04] px-3 py-2"
      >
        <Icon icon="mdi:drag" class="tp-group-handle cursor-move opacity-40 shrink-0" />

        <n-input
          v-if="editingId === g.id"
          v-model:value="editingName"
          size="tiny"
          autofocus
          class="flex-1"
          @blur="commitRename"
          @keydown.enter.prevent="commitRename"
          @keydown.esc="editingId = 0"
        />
        <template v-else>
          <span class="text-sm truncate flex-1">{{ g.name }}</span>
          <span class="text-xs opacity-45 shrink-0">{{ t('groups.count', { n: countOf(g.id) }) }}</span>
        </template>

        <n-button size="tiny" quaternary :title="t('common.rename')" @click="startRename(g)">
          <Icon icon="mdi:rename-outline" />
        </n-button>
        <n-button size="tiny" quaternary :title="t('common.delete')" @click="remove(g)">
          <Icon icon="mdi:trash-can-outline" />
        </n-button>
      </div>
    </VueDraggable>

    <!-- 未分组是 grouped 里拼出来的虚拟分组，数据库里没有这一行，不能改名/删除/排序 -->
    <div
      v-if="looseCount"
      class="flex items-center gap-2 rounded-lg border border-dashed border-black/10 dark:border-white/10 px-3 py-2 opacity-60"
    >
      <Icon icon="mdi:folder-question-outline" class="shrink-0" />
      <span class="text-sm flex-1">{{ t('common.ungrouped') }}</span>
      <span class="text-xs">{{ t('groups.count', { n: looseCount }) }}</span>
    </div>

    <p class="text-xs opacity-45">{{ t('groups.dragHint') }}</p>
  </SettingsSection>
</template>
