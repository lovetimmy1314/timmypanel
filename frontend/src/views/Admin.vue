<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { onMounted, ref } from 'vue'
import { useDialog, useMessage } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { api } from '@/api/http'
import { useUserStore } from '@/stores/user'
import { dateLocale, t } from '@/i18n'
import type { AdminUser } from '@/api/types'

const message = useMessage()
const dialog = useDialog()
const userStore = useUserStore()

const users = ref<AdminUser[]>([])
const loading = ref(false)
const showEditor = ref(false)
const target = ref<AdminUser | null>(null)
const form = ref({
  username: '',
  password: '',
  password2: '',
  nickname: '',
  role: 'user' as 'admin' | 'user',
  disabled: false,
})

async function load() {
  loading.value = true
  try {
    users.value = (await api.get<{ items: AdminUser[] }>('/admin/users')).items ?? []
  } catch (e: any) {
    message.error(e?.message ?? t('common.loadFailed'))
  } finally {
    loading.value = false
  }
}
onMounted(load)

function openCreate() {
  target.value = null
  form.value = { username: '', password: '', password2: '', nickname: '', role: 'user', disabled: false }
  showEditor.value = true
}

function openEdit(u: AdminUser) {
  target.value = u
  form.value = {
    username: u.username,
    password: '',
    password2: '',
    nickname: u.nickname,
    role: u.role,
    disabled: u.disabled,
  }
  showEditor.value = true
}

async function save() {
  if (!target.value || form.value.password) {
    if (form.value.password !== form.value.password2) {
      message.warning(t('account.mismatch'))
      return
    }
  }
  try {
    if (target.value) await api.put(`/admin/users/${target.value.id}`, form.value)
    else await api.post('/admin/users', form.value)
    showEditor.value = false
    message.success(t('common.saved'))
    await load()
  } catch (e: any) {
    message.error(e?.message ?? t('common.saveFailed'))
  }
}

function remove(u: AdminUser) {
  dialog.error({
    title: t('admin.deleteTitle'),
    content: t('admin.deleteTip', { name: u.username, cards: u.siteCount, groups: u.groupCount }),
    positiveText: t('admin.deleteConfirm'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await api.del(`/admin/users/${u.id}`)
        message.success(t('common.deleted'))
        await load()
      } catch (e: any) {
        message.error(e?.message ?? t('common.deleteFailed'))
      }
    },
  })
}

const fmt = (s: string) => new Date(s).toLocaleDateString(dateLocale.value)
</script>

<template>
  <div class="tp-hero-bg min-h-screen p-4 md:p-8">
    <div class="max-w-4xl mx-auto">
      <div class="flex items-center gap-3 mb-5">
        <n-button quaternary circle @click="$router.push('/')">
          <Icon icon="mdi:arrow-left" class="text-lg text-white" />
        </n-button>
        <h1 class="text-white text-lg font-semibold">{{ t('admin.title') }}</h1>
        <n-button class="ml-auto" type="primary" size="small" @click="openCreate">
          <Icon icon="mdi:account-plus-outline" class="mr-1" />{{ t('admin.new') }}
        </n-button>
      </div>

      <n-spin :show="loading">
        <div class="tp-glass rounded-xl overflow-hidden">
          <div
            v-for="u in users"
            :key="u.id"
            class="flex items-center gap-3 px-4 py-3 border-b border-white/10 last:border-0"
          >
            <div
              class="w-9 h-9 rounded-full bg-white/15 flex items-center justify-center text-white font-medium shrink-0"
            >
              {{ (u.nickname || u.username).charAt(0).toUpperCase() }}
            </div>
            <div class="min-w-0 flex-1">
              <div class="text-white text-sm flex items-center gap-2">
                <span class="truncate">{{ u.nickname || u.username }}</span>
                <n-tag v-if="u.role === 'admin'" size="tiny" type="warning">{{ t('admin.roleAdmin') }}</n-tag>
                <n-tag v-if="u.disabled" size="tiny" type="error">{{ t('admin.disabled') }}</n-tag>
                <n-tag v-if="u.id === userStore.user?.id" size="tiny" type="success">{{ t('admin.you') }}</n-tag>
              </div>
              <div class="text-white/50 text-xs truncate">
                {{ u.username }} ·
                {{ t('admin.meta', { cards: u.siteCount, groups: u.groupCount, date: fmt(u.createdAt) }) }}
              </div>
            </div>
            <n-button size="tiny" quaternary @click="openEdit(u)">
              <Icon icon="mdi:pencil-outline" class="text-white/70" />
            </n-button>
            <n-button
              size="tiny"
              quaternary
              :disabled="u.id === userStore.user?.id"
              @click="remove(u)"
            >
              <Icon icon="mdi:trash-can-outline" class="text-white/70" />
            </n-button>
          </div>
        </div>
      </n-spin>

      <p class="text-white/40 text-xs mt-4 leading-relaxed">{{ t('admin.footer') }}</p>
    </div>

    <n-modal
      v-model:show="showEditor"
      preset="card"
      class="max-w-sm"
      :title="target ? t('admin.editTitle', { name: target.username }) : t('admin.new')"
    >
      <n-form label-placement="top" size="small">
        <n-form-item :label="t('admin.username')">
          <n-input v-model:value="form.username" :disabled="!!target" :placeholder="t('admin.usernameHint')" />
        </n-form-item>
        <n-form-item :label="t('admin.nickname')">
          <n-input v-model:value="form.nickname" :placeholder="t('admin.nicknameHint')" />
        </n-form-item>
        <n-form-item :label="target ? t('admin.resetPwd') : t('admin.newPwd')">
          <n-input v-model:value="form.password" type="password" show-password-on="click" />
        </n-form-item>
        <n-form-item v-if="!target || form.password" :label="t('admin.pwd2')">
          <n-input v-model:value="form.password2" type="password" show-password-on="click" />
        </n-form-item>
        <n-form-item :label="t('admin.role')">
          <n-radio-group v-model:value="form.role" size="small">
            <n-radio-button value="user">{{ t('admin.roleUser') }}</n-radio-button>
            <n-radio-button value="admin">{{ t('admin.roleAdmin') }}</n-radio-button>
          </n-radio-group>
        </n-form-item>
        <n-checkbox v-model:checked="form.disabled">{{ t('admin.disableAccount') }}</n-checkbox>
      </n-form>
      <template #footer>
        <div class="flex justify-end gap-2">
          <n-button @click="showEditor = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" @click="save">{{ t('common.save') }}</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>
