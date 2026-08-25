<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { api } from '@/api/http'
import { t } from '@/i18n'
import { useSiteStore } from '@/stores/site'
import { useUserStore } from '@/stores/user'
import type { UpdateCheck } from '@/api/types'
import SettingsSection from './SettingsSection.vue'
import BrandMark from '@/components/BrandMark.vue'

const repo = 'https://github.com/lovetimmy1314/timmypanel'
const author = 'timmylau1'
const email = 'timmyliulove2@gmail.com'
// AGPL 第 13 条：作为网络服务提供时要让使用者拿得到源码，所以「关于」里
// 同时给出仓库地址和许可证，别把这两行删掉。
const license = `${repo}/blob/main/LICENSE`

// 命令写死，不从检测接口回的字段拼——防投毒（决策 037）。和 README 升级段一致。
const updateCommand = [
  'curl -fsSLO https://raw.githubusercontent.com/lovetimmy1314/timmypanel/main/deploy/update.sh',
  'chmod +x update.sh',
  './update.sh',
].join('\n')

const site = useSiteStore()
const userStore = useUserStore()
const message = useMessage()

const isAdmin = computed(() => userStore.user?.role === 'admin')
const version = computed(() => site.config.version || 'dev')

const checking = ref(false)
const copying = ref(false)
const result = ref<UpdateCheck | null>(null)

const alertType = computed(() => {
  switch (result.value?.status) {
    case 'upToDate':
      return 'success' as const
    case 'updateAvailable':
      return 'warning' as const
    case 'dev':
      return 'info' as const
    default:
      return 'error' as const
  }
})

const alertText = computed(() => {
  const r = result.value
  if (!r) return ''
  switch (r.status) {
    case 'upToDate':
      return t('about.statusUpToDate', { current: r.current })
    case 'updateAvailable':
      return t('about.statusUpdate', { current: r.current, latest: r.latest })
    case 'dev':
      return t('about.statusDev')
    default:
      return t('about.statusUnavailable')
  }
})

const showUpgrade = computed(
  () => isAdmin.value && result.value?.status === 'updateAvailable',
)

async function checkUpdate() {
  checking.value = true
  try {
    result.value = await api.get<UpdateCheck>('/update/check')
  } catch (e: any) {
    result.value = {
      current: version.value,
      latest: '',
      hasUpdate: false,
      channel: 'dev',
      status: 'upstreamUnavailable',
      checkedAt: 0,
      inDocker: false,
    }
    message.error(e?.message ?? t('about.checkFailed'))
  } finally {
    checking.value = false
  }
}

async function copyCommand() {
  copying.value = true
  try {
    await navigator.clipboard.writeText(updateCommand)
    message.success(t('about.copied'))
  } catch {
    message.warning(t('about.copyFailed'))
  } finally {
    copying.value = false
  }
}
</script>

<template>
  <div class="space-y-3">
    <SettingsSection :title="t('about.title')">
      <div class="flex items-start gap-3">
        <BrandMark :size="44" class="rounded-xl shadow-card" />
        <div class="space-y-2 text-sm leading-relaxed">
          <p>{{ t('about.intro') }}</p>
          <p>{{ t('about.author') }}<span class="font-medium">{{ author }}</span></p>
          <p>
            {{ t('about.email') }}
            <a
              :href="`mailto:${email}`"
              class="hover:underline break-all"
              style="color: var(--tp-brand)"
            >
              {{ email }}
            </a>
          </p>
          <p>
            {{ t('about.repo') }}
            <a
              :href="repo"
              target="_blank"
              rel="noopener noreferrer"
              class="hover:underline break-all"
              style="color: var(--tp-brand)"
            >
              {{ repo }}
            </a>
          </p>
          <p>
            {{ t('about.license') }}
            <a
              :href="license"
              target="_blank"
              rel="noopener noreferrer"
              class="hover:underline"
              style="color: var(--tp-brand)"
            >
              AGPL-3.0
            </a>
          </p>
          <p class="opacity-60 text-xs">{{ t('about.offline') }}</p>
        </div>
      </div>
    </SettingsSection>

    <SettingsSection :title="t('about.versionTitle')">
      <p class="text-sm">
        {{ t('about.currentVersion') }}
        <span class="font-medium font-mono">{{ version }}</span>
      </p>
      <div class="flex flex-wrap items-center gap-2">
        <n-button size="small" type="primary" :loading="checking" @click="checkUpdate">
          <Icon icon="mdi:update" class="mr-1" />{{ t('about.check') }}
        </n-button>
        <a
          v-if="result?.releaseUrl"
          :href="result.releaseUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="text-xs hover:underline"
          style="color: var(--tp-brand)"
        >
          <Icon icon="mdi:open-in-new" class="inline-block align-text-bottom mr-0.5" />
          {{ t('about.releaseNotes') }}
        </a>
      </div>
      <n-alert v-if="result" :type="alertType" :bordered="false">{{ alertText }}</n-alert>
    </SettingsSection>

    <SettingsSection v-if="showUpgrade" :title="t('about.upgradeTitle')">
      <template v-if="result?.inDocker">
        <p class="text-sm opacity-70">{{ t('about.upgradeHint') }}</p>
        <pre class="text-xs leading-relaxed overflow-x-auto rounded-lg px-3 py-2 font-mono bg-black/[0.04] dark:bg-white/[0.06]">{{ updateCommand }}</pre>
        <n-button size="small" :loading="copying" @click="copyCommand">
          <Icon icon="mdi:content-copy" class="mr-1" />{{ t('about.copyCommand') }}
        </n-button>
      </template>
      <p v-else class="text-sm opacity-70">{{ t('about.binaryHint') }}</p>
    </SettingsSection>
  </div>
</template>
