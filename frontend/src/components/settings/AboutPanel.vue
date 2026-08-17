<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import SettingsSection from './SettingsSection.vue'
import BrandMark from '@/components/BrandMark.vue'
import { t } from '@/i18n'

const repo = 'https://github.com/lovetimmy1314/timmypanel'
const author = 'timmylau1'
const email = 'timmyliulove2@gmail.com'
// AGPL 第 13 条：作为网络服务提供时要让使用者拿得到源码，所以「关于」里
// 同时给出仓库地址和许可证，别把这两行删掉。
const license = `${repo}/blob/main/LICENSE`
</script>

<template>
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
</template>
