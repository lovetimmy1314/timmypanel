<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { useUserStore } from '@/stores/user'
import { dateLocale, t } from '@/i18n'
import type { SessionInfo } from '@/api/types'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ 'update:show': [boolean] }>()

const userStore = useUserStore()
const message = useMessage()

const oldPwd = ref('')
const newPwd = ref('')
const newPwd2 = ref('')
const sessions = ref<SessionInfo[]>([])

watch(
  () => props.show,
  (open) => {
    if (open) loadSessions()
  },
)

async function loadSessions() {
  try {
    sessions.value = (await userStore.listSessions()).items ?? []
  } catch {
    sessions.value = []
  }
}

async function changePassword() {
  if (newPwd.value.length < 8) {
    message.warning(t('account.tooShort'))
    return
  }
  if (new TextEncoder().encode(newPwd.value).length > 72) {
    message.warning(t('account.tooLong'))
    return
  }
  if (newPwd.value !== newPwd2.value) {
    message.warning(t('account.mismatch'))
    return
  }
  try {
    await userStore.changePassword(oldPwd.value, newPwd.value, newPwd2.value)
    oldPwd.value = newPwd.value = newPwd2.value = ''
    message.success(t('account.changed'))
    await loadSessions()
  } catch (e: any) {
    message.error(e?.message ?? t('account.changeFailed'))
  }
}

async function revoke(id: number) {
  await userStore.revokeSession(id)
  message.success(t('account.revoked'))
  await loadSessions()
}

const fmt = (s: string) => new Date(s).toLocaleString(dateLocale.value, { hour12: false })
const shortUA = (ua: string) => {
  const m = ua.match(/(Edg|Chrome|Firefox|Safari)\/[\d.]+/)
  const os = /Windows/.test(ua)
    ? 'Windows'
    : /Android/.test(ua)
      ? 'Android'
      : /iPhone|iPad/.test(ua)
        ? 'iOS'
        : /Mac/.test(ua)
          ? 'macOS'
          : /Linux/.test(ua)
            ? 'Linux'
            : ''
  return [os, m?.[0]].filter(Boolean).join(' · ') || ua.slice(0, 40) || t('account.unknownDevice')
}

const isAdmin = computed(() => userStore.user?.role === 'admin')
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :title="t('account.title')"
    class="max-w-lg"
    :bordered="false"
    @update:show="emit('update:show', $event)"
  >
    <n-form label-placement="top" size="small">
      <n-form-item :label="t('account.oldPwd')">
        <n-input v-model:value="oldPwd" type="password" show-password-on="click" />
      </n-form-item>
      <n-form-item :label="t('account.newPwd')">
        <n-input v-model:value="newPwd" type="password" show-password-on="click" />
      </n-form-item>
      <n-form-item :label="t('account.newPwd2')">
        <n-input v-model:value="newPwd2" type="password" show-password-on="click" />
      </n-form-item>
      <n-button size="small" type="primary" @click="changePassword">{{ t('account.change') }}</n-button>
    </n-form>

    <n-divider class="!my-3" />
    <div class="flex items-center justify-between mb-2">
      <span class="text-sm font-medium">{{ t('account.devices') }}</span>
      <n-button size="tiny" quaternary @click="loadSessions">
        <Icon icon="mdi:refresh" />
      </n-button>
    </div>
    <div
      v-for="s in sessions"
      :key="s.id"
      class="flex items-center gap-2 py-2 border-b border-black/5 dark:border-white/5 last:border-0"
    >
      <div class="flex-1 min-w-0">
        <div class="text-sm truncate">
          {{ shortUA(s.userAgent) }}
          <n-tag v-if="s.current" size="tiny" type="success" class="ml-1">{{ t('account.current') }}</n-tag>
        </div>
        <div class="text-xs opacity-50">
          {{ s.ip }} · {{ t('account.lastUsed', { time: fmt(s.lastUsedAt) }) }}
        </div>
      </div>
      <n-button v-if="!s.current" size="tiny" quaternary @click="revoke(s.id)">
        {{ t('account.revoke') }}
      </n-button>
    </div>

    <template v-if="isAdmin">
      <n-divider class="!my-3" />
      <n-button size="small" block @click="$router.push('/admin')">
        <Icon icon="mdi:account-cog-outline" class="mr-1" />{{ t('account.admin') }}
      </n-button>
    </template>
  </n-modal>
</template>
