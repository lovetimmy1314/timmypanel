// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

import { toRaw } from 'vue'

// structuredClone 不接受 Proxy：从 store 里取出来的对象都是 reactive 代理，
// 直接 structuredClone(panel.settings) 会抛 DataCloneError，组件的 setup 当场中断、
// 整个面板渲染成空白（浏览器控制台才看得到）。先 toRaw 拿到底下那个普通对象再克隆。
// toRaw 只脱一层，但底层对象的嵌套属性本来就是普通对象，够用。
export function deepClone<T>(value: T): T {
  return structuredClone(toRaw(value))
}
