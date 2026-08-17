/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts}'],
  // dark: 变体跟着 App.vue 写在 <html> 上的 data-theme 走，不用默认的 media 策略。
  // 默认策略跟的是**系统**偏好，用户在设置里选了和系统相反的主题时，
  // 设置面板里那一堆 dark:border-white/10 就会挑错，界面一半深一半浅。
  darkMode: ['selector', '[data-theme="dark"]'],
  theme: {
    extend: {
      boxShadow: {
        card: '0 4px 20px -6px rgba(0,0,0,0.35)',
      },
    },
  },
  plugins: [],
}
