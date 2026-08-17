// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

import { ref } from 'vue'
import { addAPIProvider, addCollection, disableCache, iconLoaded, type IconifyJSON } from '@iconify/vue'
import { uiIcons } from './ui-icons'

// 图标离线化（决策 019）。@iconify/vue 默认是**运行时**去 api.iconify.design
// 取图形数据的，后果有两个：断网 / 内网部署第一次打开一个图标都没有；
// 每个访问者的浏览器都会和第三方通信，把「这个站用了哪些图标名」连同 IP 一起送出去。
//
// 所以这里改成全部本地供货：
//   1. 界面自己用的那几十个图标（ui-icons.ts，构建时从 mdi 里抽的子集）第一帧就注册好；
//   2. 用户给卡片挑的图标名可能是 mdi 里的任何一个，整份 mdi 有 3MB，
//      按需异步加载（loadIconSet），不拖累首屏；
//   3. 把 API provider 的 resources 清空 —— 这是**真正**保证不发外部请求的那一步，
//      光有本地图标集不够：没命中的名字照样会去打 API。
addCollection(uiIcons)
addAPIProvider('', { resources: [] })
// 图标不再来自网络，localStorage 里那份缓存没有意义，留着只会让旧数据盖住新图标。
disableCache('all')

let pending: Promise<void> | null = null

/**
 * 整份图标集是否已就位。用它把 <Icon> 的渲染**挡到加载完之后**：
 * 图标名没命中时 @iconify/vue 会去问 API，而 API 已经被清空，那一次失败之后
 * 它不会因为后来 addCollection 就自己重画，卡片上会永久空一块。
 */
export const iconSetReady = ref(false)

/**
 * 按需加载整份 mdi 图标集。卡片用「图标库」类型时才需要，
 * 重复调用只会加载一次。加载失败不抛出：没图标是退化，不该让调用方崩掉。
 */
export function loadIconSet(): Promise<void> {
  if (!pending) {
    // 用 ?raw 而不是直接 import JSON：3MB 的 JSON 走 TS 的 resolveJsonModule 会让
    // vue-tsc 去推一个巨型字面量类型，类型检查直接卡住；字符串 + JSON.parse
    // 既绕开这点，运行时解析也比等价的 JS 对象字面量快。
    pending = import('@iconify-json/mdi/icons.json?raw')
      .then((m) => {
        addCollection(JSON.parse(m.default) as IconifyJSON)
        iconSetReady.value = true
      })
      .catch((e) => {
        console.error('加载图标集失败', e)
      })
  }
  return pending
}

/** 这个图标名现在能不能画出来。要先 await loadIconSet()，否则只认得界面子集。 */
export function hasOfflineIcon(name: string): boolean {
  return iconLoaded(name)
}
