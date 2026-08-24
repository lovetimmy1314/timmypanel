// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

import { onUnmounted, ref } from 'vue'

// 悬浮小组件（天气、万年历）只在桌面端出现。断点和 tailwind 的 lg 对齐。
const DESKTOP_QUERY = '(min-width: 1024px)'

// useIsDesktop 返回一个跟着窗口宽度变化的响应式布尔值。
//
// **用它做 v-if，不要用 CSS 把组件藏起来**：`hidden lg:block` 只是不画，
// 组件照样挂载、定时器照样跑、接口照样打——天气那个是真出站请求，
// 在手机上白烧流量和上游配额。
export function useIsDesktop() {
  const media = window.matchMedia(DESKTOP_QUERY)
  const isDesktop = ref(media.matches)
  const sync = () => (isDesktop.value = media.matches)
  media.addEventListener('change', sync)
  onUnmounted(() => media.removeEventListener('change', sync))
  return isDesktop
}
