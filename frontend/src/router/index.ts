// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

import { createRouter, createWebHistory } from 'vue-router'
import { setUnauthorizedHandler } from '@/api/http'
import { useUserStore } from '@/stores/user'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: () => import('@/views/Home.vue'), meta: { auth: true } },
    { path: '/admin', name: 'admin', component: () => import('@/views/Admin.vue'), meta: { auth: true, admin: true } },
    { path: '/login', name: 'login', component: () => import('@/views/Login.vue') },
    // 决策 027：书签桥接页是免登录公开路由——鉴权只靠 postMessage 负载里的
    // ingest 令牌，威胁模型同 /api/v1/ingest 端点，不能加 meta.auth。
    { path: '/ingest-bridge', name: 'ingest-bridge', component: () => import('@/views/IngestBridge.vue') },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  const store = useUserStore()
  // 首次进入时问一次 /auth/me —— 有有效会话 Cookie 就直接进首页，
  // 这就是“不用每次都重新登录”的实现。
  if (!store.loaded) await store.fetchMe()

  if (to.meta.auth && !store.user) {
    return { name: 'login', query: to.fullPath === '/' ? {} : { redirect: to.fullPath } }
  }
  if (to.meta.admin && store.user?.role !== 'admin') return { name: 'home' }
  if (to.name === 'login' && store.user) return { name: 'home' }
  return true
})

// 会话在使用中过期时，统一跳回登录页。
setUnauthorizedHandler(() => {
  const store = useUserStore()
  store.user = null
  if (router.currentRoute.value.name !== 'login') {
    router.replace({ name: 'login' })
  }
})

export default router
