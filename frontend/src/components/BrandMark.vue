<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
// 站点主标（九宫格 T），源文件在仓库根的 brand/mark.svg。
// 这里是**内联**而不是 <img src="/brand/mark.svg">：public/ 下的文件会被 vite 拷进
// backend/internal/web/dist/ 根目录，而那个目录只跟踪 index.html 一个文件（决策 014），
// 多出来的文件既是 git 噪声、又在「只跑 go build 没跑 npm build」时缺失。
// 内联还顺带让 mono 变体能吃到父级的 currentColor（外部引用的 SVG 拿不到宿主文字色）。
withDefaults(
  defineProps<{
    /** 边长（px），图标是正方形 */
    size?: number
    /** color = 品牌紫底白格；mono = 跟随父级文字色的单色版 */
    variant?: 'color' | 'mono'
  }>(),
  { size: 32, variant: 'color' },
)
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 128 128"
    role="img"
    aria-label="Timmypanel"
    class="shrink-0"
    :class="variant === 'mono' ? 'tp-mark-mono' : 'tp-mark-color'"
  >
    <title>Timmypanel</title>
    <rect v-if="variant === 'color'" class="tp-mark-bg" x="0" y="0" width="128" height="128" rx="29" />
    <!-- 亮起的五格构成字母 T，暗格代表尚未收录的站点；右中一格是点缀色。 -->
    <rect class="tp-mark-on" x="13" y="13" width="26" height="26" rx="6.5" />
    <rect class="tp-mark-on" x="51" y="13" width="26" height="26" rx="6.5" />
    <rect class="tp-mark-on" x="89" y="13" width="26" height="26" rx="6.5" />
    <rect class="tp-mark-off" x="13" y="51" width="26" height="26" rx="6.5" />
    <rect class="tp-mark-on" x="51" y="51" width="26" height="26" rx="6.5" />
    <rect class="tp-mark-ac" x="89" y="51" width="26" height="26" rx="6.5" />
    <rect class="tp-mark-off" x="13" y="89" width="26" height="26" rx="6.5" />
    <rect class="tp-mark-on" x="51" y="89" width="26" height="26" rx="6.5" />
    <rect class="tp-mark-off" x="89" y="89" width="26" height="26" rx="6.5" />
  </svg>
</template>

<style scoped>
/* 品牌色取自 brand/README.md 的色值表，深浅色切换由 style.css 里的 --tp-brand-* 负责。 */
.tp-mark-color .tp-mark-bg {
  fill: var(--tp-brand);
}
.tp-mark-color .tp-mark-on {
  fill: #fff;
}
.tp-mark-color .tp-mark-off {
  fill: #fff;
  fill-opacity: 0.24;
}
.tp-mark-color .tp-mark-ac {
  fill: var(--tp-brand-accent-soft);
  fill-opacity: 0.7;
}

/* 单色版整块跟随父级文字色，暗格靠透明度拉开层次。 */
.tp-mark-mono rect {
  fill: currentColor;
}
.tp-mark-mono .tp-mark-off,
.tp-mark-mono .tp-mark-ac {
  fill-opacity: 0.25;
}
</style>
