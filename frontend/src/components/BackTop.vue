<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { onMounted, onUnmounted, ref } from 'vue'
import { Icon } from '@iconify/vue'
import { t } from '@/i18n'

// 页面滚动主体是 window（首页内容把 body 撑高），所以监听 window 的 scroll。
const visible = ref(false)

function onScroll() {
  visible.value = window.scrollY > 400
}

function toTop() {
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

onMounted(() => {
  onScroll()
  window.addEventListener('scroll', onScroll, { passive: true })
})
onUnmounted(() => window.removeEventListener('scroll', onScroll))
</script>

<template>
  <Transition name="tp-fade">
    <button
      v-if="visible"
      type="button"
      class="tp-fab tp-fab-br"
      :aria-label="t('home.backTop')"
      :title="t('home.backTop')"
      @click="toTop"
    >
      <Icon icon="mdi:arrow-up" class="text-xl" />
    </button>
  </Transition>
</template>
