<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
// 一行设置：左边标签、右边控件。stack 用于控件本身要占满宽度的情况（输入框、滑块、上传区）。
withDefaults(defineProps<{ label?: string; hint?: string; stack?: boolean; grow?: boolean }>(), {
  stack: false,
  grow: false,
})
</script>

<template>
  <div :class="stack ? 'space-y-1.5' : 'flex items-center gap-3'">
    <div v-if="label" :class="stack ? 'text-[13px] opacity-70' : 'text-[13px] opacity-70 shrink-0 w-24'">
      {{ label }}
    </div>
    <div :class="stack ? '' : grow ? 'flex-1 min-w-0' : 'min-w-0'">
      <slot />
      <p v-if="hint" class="text-xs opacity-45 mt-1">{{ hint }}</p>
    </div>
  </div>
</template>
