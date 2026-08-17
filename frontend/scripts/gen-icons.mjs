// 从 @iconify-json/mdi 里抽出**界面自己用到**的那些图标，生成 src/icons/ui-icons.ts。
//
// 为什么要抽子集：整份 mdi 是 3MB（约 7500 个图标），首屏全量加载不值得。
// 界面上写死的这几十个图标必须第一帧就在，剩下的（用户给卡片挑的图标名）
// 由 src/icons/index.ts 按需异步加载整份 mdi。
//
// 名字直接从源码里扫，避免手工维护一份清单跟代码长歪。
// package.json 的 predev / prebuild 会自动跑它，一般不用手动执行。
import { readFileSync, readdirSync, writeFileSync, statSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join, relative } from 'node:path'
import { getIcons } from '@iconify/utils'

const here = dirname(fileURLToPath(import.meta.url))
const root = join(here, '..')
const srcDir = join(root, 'src')
const outFile = join(srcDir, 'icons', 'ui-icons.ts')

/** 递归收集 src 下的 .vue / .ts，跳过生成物本身。 */
function collectFiles(dir) {
  const out = []
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    if (statSync(full).isDirectory()) out.push(...collectFiles(full))
    else if (/\.(vue|ts)$/.test(entry) && full !== outFile) out.push(full)
  }
  return out
}

const names = new Set()
for (const file of collectFiles(srcDir)) {
  const text = readFileSync(file, 'utf8')
  for (const m of text.matchAll(/\bmdi:[a-z0-9]+(?:-[a-z0-9]+)*\b/g)) {
    names.add(m[0].slice('mdi:'.length))
  }
}

const full = JSON.parse(readFileSync(join(root, 'node_modules/@iconify-json/mdi/icons.json'), 'utf8'))
const subset = getIcons(full, [...names].sort())
if (!subset) throw new Error('抽取图标子集失败：mdi 图标集读不出来')

const missing = [...names].filter((n) => !subset.icons[n] && !subset.aliases?.[n])
if (missing.length) {
  // 拼错的图标名在这里就该暴露，不要等到界面上出现空方块。
  throw new Error(`这些图标名不在 mdi 图标集里：${missing.join(', ')}`)
}

const banner = `// 由 scripts/gen-icons.mjs 生成，**别手改**。改了图标名之后跑 npm run icons。
// 这里只有界面自己用到的 ${Object.keys(subset.icons).length} 个图标；
// 用户给卡片挑的图标名走 src/icons/index.ts 的按需加载。
import type { IconifyJSON } from '@iconify/vue'

export const uiIcons: IconifyJSON = `

writeFileSync(outFile, banner + JSON.stringify(subset, null, 2) + '\n', 'utf8')
console.log(`已写出 ${relative(root, outFile)}（${Object.keys(subset.icons).length} 个图标）`)
