<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, onMounted, watchEffect } from 'vue'
import { darkTheme, type GlobalThemeOverrides } from 'naive-ui'
import { useSiteStore } from '@/stores/site'
import { useThemeStore } from '@/stores/theme'
import { naiveDateLocale, naiveLocale } from '@/i18n'

const site = useSiteStore()
const themeStore = useThemeStore()

onMounted(() => {
  // 实例配置决定标题和 favicon，登录与否都要读一次。
  site.load()
})

const isDark = computed(() => themeStore.isDark)
const theme = computed(() => (isDark.value ? darkTheme : null))

// 把**最终生效**的明暗盖在 <html> 上，style.css 里的 --tp-* 靠它切换。
// 只写 @media (prefers-color-scheme) 不够：用户在设置里选了 light/dark 时，
// 系统偏好和界面主题会相反，配色就会挑错那一套。
watchEffect(() => {
  document.documentElement.dataset.theme = isDark.value ? 'dark' : 'light'
})

// 主色从 naive 默认的绿换成品牌紫（brand/README.md 的色值表）。
// 深色模式要用调亮的那一档，否则紫色压在深底上对比度不够。
const themeOverrides = computed<GlobalThemeOverrides>(() => {
  const c = isDark.value
    ? { primary: '#7F77DD', hover: '#948DE6', pressed: '#6A62C8' }
    : { primary: '#534AB7', hover: '#6A61C9', pressed: '#423A9B' }
  return {
    common: {
      primaryColor: c.primary,
      primaryColorHover: c.hover,
      primaryColorPressed: c.pressed,
      // suppl 是填充态（如 loading 的按钮）用的，跟 hover 保持一致即可。
      primaryColorSuppl: c.hover,
      infoColor: c.primary,
      infoColorHover: c.hover,
      infoColorPressed: c.pressed,
      infoColorSuppl: c.hover,
    },
  }
})
</script>

<template>
  <n-config-provider
    :theme="theme"
    :theme-overrides="themeOverrides"
    :locale="naiveLocale"
    :date-locale="naiveDateLocale"
  >
    <n-message-provider :max="3">
      <n-dialog-provider>
        <router-view />
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
