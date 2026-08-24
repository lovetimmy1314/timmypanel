<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
// 会动的天气图标。自绘 SVG 而不是用 mdi：mdi 是一张静态路径，转不起来也下不了雨。
// 动画一律 transform/opacity，且整体受 prefers-reduced-motion 管（见文件末尾）。
import { computed } from 'vue'
import { weatherGroup, type WeatherGroup } from '@/utils/weather'

const props = withDefaults(defineProps<{ code: number; isDay?: boolean; size?: number }>(), {
  isDay: true,
  size: 28,
})

const group = computed<WeatherGroup>(() => weatherGroup(props.code))
// 晴和少云要分昼夜：白天画太阳，晚上画月亮。其余几类昼夜长得一样。
const withSun = computed(() => group.value === 'clear' || group.value === 'partly')
const hasCloud = computed(() => group.value !== 'clear')
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 32 32"
    fill="none"
    class="tp-wicon"
    aria-hidden="true"
  >
    <!-- 太阳：光芒整组慢转，本体轻微呼吸 -->
    <g v-if="withSun && isDay" :class="group === 'partly' ? 'tp-wicon-off' : ''">
      <g class="tp-wicon-spin" style="transform-origin: 13px 13px">
        <g stroke="#f5a623" stroke-width="1.8" stroke-linecap="round">
          <line x1="13" y1="2.5" x2="13" y2="5" />
          <line x1="13" y1="21" x2="13" y2="23.5" />
          <line x1="2.5" y1="13" x2="5" y2="13" />
          <line x1="21" y1="13" x2="23.5" y2="13" />
          <line x1="5.6" y1="5.6" x2="7.4" y2="7.4" />
          <line x1="18.6" y1="18.6" x2="20.4" y2="20.4" />
          <line x1="5.6" y1="20.4" x2="7.4" y2="18.6" />
          <line x1="18.6" y1="7.4" x2="20.4" y2="5.6" />
        </g>
      </g>
      <circle cx="13" cy="13" r="5.4" fill="#f7b733" class="tp-wicon-breathe" style="transform-origin: 13px 13px" />
    </g>

    <!-- 月亮：夜间的晴/少云 -->
    <g v-else-if="withSun" :class="group === 'partly' ? 'tp-wicon-off' : ''">
      <path
        d="M18.5 13.2a7 7 0 1 1-7.7-7 5.6 5.6 0 0 0 7.7 7z"
        fill="#cbd5f5"
        class="tp-wicon-breathe"
        style="transform-origin: 13px 13px"
      />
    </g>

    <!-- 云：所有非晴天类都有，横向缓慢漂 -->
    <g v-if="hasCloud" class="tp-wicon-drift">
      <path
        d="M10.2 24.5a5.2 5.2 0 0 1-.5-10.4 7.2 7.2 0 0 1 13.8 1.4 4.5 4.5 0 0 1-1 8.9z"
        fill="currentColor"
        :fill-opacity="group === 'cloudy' || group === 'fog' ? 0.55 : 0.45"
      />
    </g>

    <!-- 雾：云下面三条平移的横线 -->
    <g v-if="group === 'fog'" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-opacity="0.6">
      <line x1="7" y1="27" x2="19" y2="27" class="tp-wicon-fog" style="animation-delay: 0s" />
      <line x1="11" y1="30" x2="24" y2="30" class="tp-wicon-fog" style="animation-delay: 0.9s" />
    </g>

    <!-- 雨 / 毛毛雨：雨滴错开落 -->
    <g v-else-if="group === 'drizzle' || group === 'rain'" fill="#5aa9e6">
      <rect
        v-for="(x, i) in [10, 15, 20]"
        :key="x"
        :x="x"
        y="23.6"
        :width="group === 'rain' ? 1.8 : 1.4"
        :height="group === 'rain' ? 4.5 : 3"
        rx="0.9"
        class="tp-wicon-rain"
        :style="{ animationDelay: `${i * 0.28}s` }"
      />
    </g>

    <!-- 雪：雪花边落边转 -->
    <g v-else-if="group === 'snow'" fill="#dbeeff">
      <circle
        v-for="(x, i) in [10.5, 16, 21.5]"
        :key="x"
        :cx="x"
        cy="25.6"
        r="1.5"
        class="tp-wicon-snow"
        :style="{ animationDelay: `${i * 0.4}s` }"
      />
    </g>

    <!-- 雷：闪电忽明忽暗 -->
    <g v-else-if="group === 'thunder'">
      <path d="M17.4 23l-4.6 5h3l-.9 2.8 4.5-5.3h-3z" fill="#f7c948" class="tp-wicon-flash" />
    </g>
  </svg>
</template>

<style>
/* 不加 scoped：这些类名要挂到 v-for 生成的元素和内联 style 上，
   而且整套只有这一个组件在用，没有冲突风险。 */
.tp-wicon {
  overflow: hidden;
  color: var(--tp-fg-soft);
}

/* 少云时太阳/月亮缩到左上角，给云让出位置 */
.tp-wicon-off {
  transform: translate(-2px, -3px) scale(0.72);
  transform-origin: 13px 13px;
}

@keyframes tp-wicon-spin {
  to {
    transform: rotate(360deg);
  }
}
@keyframes tp-wicon-breathe {
  0%,
  100% {
    transform: scale(1);
    opacity: 0.95;
  }
  50% {
    transform: scale(1.08);
    opacity: 1;
  }
}
@keyframes tp-wicon-drift {
  0%,
  100% {
    transform: translateX(0);
  }
  50% {
    transform: translateX(1.6px);
  }
}
@keyframes tp-wicon-rain {
  0% {
    transform: translateY(-3px);
    opacity: 0;
  }
  25% {
    opacity: 1;
  }
  100% {
    transform: translateY(7px);
    opacity: 0;
  }
}
@keyframes tp-wicon-snow {
  0% {
    transform: translateY(-3px) scale(0.7);
    opacity: 0;
  }
  30% {
    opacity: 1;
  }
  100% {
    transform: translateY(6px) scale(1);
    opacity: 0;
  }
}
@keyframes tp-wicon-fog {
  0%,
  100% {
    transform: translateX(-2px);
    opacity: 0.35;
  }
  50% {
    transform: translateX(2px);
    opacity: 0.75;
  }
}
@keyframes tp-wicon-flash {
  0%,
  55%,
  100% {
    opacity: 0.35;
  }
  60%,
  70% {
    opacity: 1;
  }
  65% {
    opacity: 0.5;
  }
}

.tp-wicon-spin {
  animation: tp-wicon-spin 22s linear infinite;
}
.tp-wicon-breathe {
  animation: tp-wicon-breathe 3.6s ease-in-out infinite;
}
.tp-wicon-drift {
  animation: tp-wicon-drift 5s ease-in-out infinite;
}
.tp-wicon-rain {
  animation: tp-wicon-rain 1.3s linear infinite;
}
.tp-wicon-snow {
  animation: tp-wicon-snow 2.6s linear infinite;
}
.tp-wicon-fog {
  animation: tp-wicon-fog 4s ease-in-out infinite;
}
.tp-wicon-flash {
  animation: tp-wicon-flash 2.4s ease-in-out infinite;
}

/* 系统开了「减弱动态效果」就全停下。图标本身照常显示，只是不动。 */
@media (prefers-reduced-motion: reduce) {
  .tp-wicon-spin,
  .tp-wicon-breathe,
  .tp-wicon-drift,
  .tp-wicon-rain,
  .tp-wicon-snow,
  .tp-wicon-fog,
  .tp-wicon-flash {
    animation: none;
  }
}
</style>
