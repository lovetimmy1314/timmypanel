<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
// 设置面板里的一块「卡片」。右侧内容区由若干块拼成，标题 + 若干行控件。
defineProps<{ title?: string }>()
</script>

<template>
  <section
    class="rounded-xl border border-black/5 dark:border-white/[0.08] bg-black/[0.02] dark:bg-white/[0.03] px-4 py-3.5"
  >
    <header v-if="title || $slots.extra" class="flex items-center justify-between gap-2 mb-3">
      <h3 class="text-[13px] font-medium tracking-wide opacity-60">{{ title }}</h3>
      <slot name="extra" />
    </header>
    <div class="space-y-3">
      <slot />
    </div>
  </section>
</template>
