// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

import { defineStore } from 'pinia'
import { computed, onScopeDispose, ref } from 'vue'
import { usePanelStore } from '@/stores/panel'

// 明暗是跨组件共享的：App.vue 要拿它挑 naive 主题和 <html data-theme>，
// 顶栏要拿它画开关的图标，Home 要拿它决定遮罩往哪边压。所以放 store 而不是
// 各自算一遍——各自算的话 theme=auto 时几处对系统偏好的监听会不同步。
export const useThemeStore = defineStore('theme', () => {
  const panel = usePanelStore()

  const media = window.matchMedia('(prefers-color-scheme: dark)')
  const prefersDark = ref(media.matches)
  const onChange = (e: MediaQueryListEvent) => (prefersDark.value = e.matches)
  media.addEventListener('change', onChange)
  onScopeDispose(() => media.removeEventListener('change', onChange))

  // theme 为 auto 时跟随系统；导航站多数时候是深色更好看，但尊重用户选择。
  const isDark = computed(() => {
    const t = panel.settings.theme
    if (t === 'dark') return true
    if (t === 'light') return false
    return prefersDark.value
  })

  // 顶栏那个开关：从 auto 点一下，落到与当前**实际**效果相反的那一档，
  // 而不是原地在 auto 里打转。想回「跟随系统」在设置里选。
  const toggle = () => panel.patchSettings({ theme: isDark.value ? 'light' : 'dark' })

  return { isDark, prefersDark, toggle }
})
