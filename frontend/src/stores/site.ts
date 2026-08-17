// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@/api/http'
import type { PublicSiteConfig } from '@/api/types'

const DEFAULTS: PublicSiteConfig = {
  allowRegister: false,
  siteTitle: 'Timmypanel',
  siteIcon: '',
  loginBackground: '',
}

// 品牌图标，和 frontend/index.html 里那份 <link rel="icon"> 必须是同一张图（改一处要改两处）。
// 清空站点图标后要回到这里，不能让上一张 favicon 一直挂在 <link> 上。
// 色值里的 # 写成 %23：data URI 里的 # 会被当成片段起点，截断后面的内容。
const DEFAULT_FAVICON =
  "data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 128 128'>" +
  "<rect fill='%23534AB7' width='128' height='128' rx='20'/><g fill='%23fff'>" +
  "<rect x='13' y='16' width='26' height='26' rx='5'/><rect x='51' y='16' width='26' height='26' rx='5'/>" +
  "<rect x='89' y='16' width='26' height='26' rx='5'/><rect x='51' y='51' width='26' height='26' rx='5'/>" +
  "<rect x='51' y='86' width='26' height='26' rx='5'/></g></svg>"

// site store 是实例级配置（整站一份，未登录也能读）。用户各自的界面设置在 panel store，
// 两者不要混：这里的东西登录页就要用，那里的要登录后才拿得到。
export const useSiteStore = defineStore('site', () => {
  const config = ref<PublicSiteConfig>({ ...DEFAULTS })
  const loaded = ref(false)

  // 标题和 favicon 是 document 上的全局状态，只能这样命令式地改。
  function apply() {
    document.title = config.value.siteTitle || DEFAULTS.siteTitle
    let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    if (!link) {
      link = document.createElement('link')
      link.rel = 'icon'
      document.head.appendChild(link)
    }
    link.href = config.value.siteIcon || DEFAULT_FAVICON
    // apple-touch-icon 跟着站点图标走（配置只收本站上传的 PNG/JPG，系统收藏时可用）；
    // 没配就回到品牌位图。data URI 不行，iOS 不认。
    const touch = document.querySelector<HTMLLinkElement>('link[rel="apple-touch-icon"]')
    if (touch) touch.href = config.value.siteIcon || '/icons/apple-touch-icon.png'
  }

  async function load() {
    try {
      config.value = { ...DEFAULTS, ...(await api.get<PublicSiteConfig>('/auth/config')) }
      apply()
    } catch {
      // 拿不到就用默认值，别让登录页因为这个白屏。
    } finally {
      loaded.value = true
    }
  }

  return { config, loaded, load, apply }
})
