<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { useUserStore } from '@/stores/user'
import { useSiteStore } from '@/stores/site'
import { ApiError } from '@/api/http'
import { locale, setLocale, t, type Locale } from '@/i18n'
import BrandMark from '@/components/BrandMark.vue'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const store = useUserStore()
// 实例配置由 App.vue 在挂载时统一拉一次，这里只读。
const site = useSiteStore()

// 配置里的背景是 /api/v1/auth/site-asset/login-bg 或 http(s) 外链，
// 两种都由后端归一化过；没配就交给 .tp-hero-bg 那套品牌渐变（style.css）。
const pageStyle = computed(() => {
  const bg = site.config.loginBackground
  if (!bg) return undefined
  return {
    backgroundImage: `url("${bg.replace(/["\\]/g, '')}")`,
    backgroundSize: 'cover',
    backgroundPosition: 'center',
  }
})

const username = ref('')
const password = ref('')
// 默认勾上“记住我”：自用导航站每次输密码太折腾，这也是需求里的登录记忆。
const remember = ref(true)
const loading = ref(false)

async function submit() {
  if (!username.value || !password.value) {
    message.warning(t('login.needBoth'))
    return
  }
  loading.value = true
  try {
    await store.login(username.value, password.value, remember.value)
    const redirect = route.query.redirect
    const safe =
      typeof redirect === 'string' &&
      redirect.startsWith('/') &&
      !redirect.startsWith('//') &&
      !redirect.startsWith('/\\')
    router.replace(safe ? redirect : '/')
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : t('login.failed'))
  } finally {
    loading.value = false
  }
}

// 登录页读不到 /settings，语言只能来自 localStorage 里那份镜像（见 src/i18n/index.ts）。
// 所以这里直接改本地值就够了；登录后服务端的设置会再盖一次。
const switchLocale = (next: Locale) => setLocale(next)
</script>

<template>
  <div
    class="min-h-screen w-full flex items-center justify-center p-4"
    :class="{ 'tp-hero-bg': !site.config.loginBackground }"
    :style="pageStyle"
  >
    <div class="tp-glass w-full max-w-sm rounded-2xl p-8 shadow-card">
      <div class="flex flex-col items-center mb-7">
        <!-- 管理员配了站点图标就用他那张，没配则回落到品牌主标。 -->
        <img
          v-if="site.config.siteIcon"
          :src="site.config.siteIcon"
          class="w-14 h-14 mb-3 rounded-2xl object-contain"
          alt=""
        />
        <BrandMark v-else :size="56" class="mb-3 rounded-2xl shadow-card" />
        <h1 class="text-white text-xl font-semibold tracking-wide">{{ site.config.siteTitle }}</h1>
        <p class="text-white/60 text-xs mt-1">{{ t('login.subtitle') }}</p>
      </div>

      <n-form @submit.prevent="submit">
        <n-input
          v-model:value="username"
          :placeholder="t('login.username')"
          size="large"
          class="mb-3"
          autocomplete="username"
          @keyup.enter="submit"
        >
          <template #prefix><Icon icon="mdi:account-outline" /></template>
        </n-input>
        <n-input
          v-model:value="password"
          type="password"
          show-password-on="click"
          :placeholder="t('login.password')"
          size="large"
          autocomplete="current-password"
          @keyup.enter="submit"
        >
          <template #prefix><Icon icon="mdi:lock-outline" /></template>
        </n-input>

        <div class="flex items-center justify-between mt-4 mb-5">
          <n-checkbox v-model:checked="remember">
            <span class="text-white/80 text-sm">{{ t('login.remember') }}</span>
          </n-checkbox>
        </div>

        <n-button type="primary" size="large" block :loading="loading" attr-type="submit" @click="submit">
          {{ t('login.submit') }}
        </n-button>
      </n-form>

      <p class="text-white/40 text-xs text-center mt-6 leading-relaxed">
        {{ t('login.forgot') }}
      </p>

      <!-- 语言开关也放在登录页：不然第一次来的人只能先登进去才看得懂界面 -->
      <div class="flex items-center justify-center gap-2 mt-4 text-xs">
        <button
          v-for="opt in [
            { key: 'zh' as const, label: '中文' },
            { key: 'en' as const, label: 'English' },
          ]"
          :key="opt.key"
          type="button"
          class="bg-transparent border-0 cursor-pointer px-1"
          :class="locale === opt.key ? 'text-white' : 'text-white/45 hover:text-white/70'"
          @click="switchLocale(opt.key)"
        >
          {{ opt.label }}
        </button>
      </div>
    </div>
  </div>
</template>
