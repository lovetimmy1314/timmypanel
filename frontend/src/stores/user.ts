// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@/api/http'
import type { SessionInfo, User } from '@/api/types'

export const useUserStore = defineStore('user', () => {
  const user = ref<User | null>(null)
  const loaded = ref(false)

  // fetchMe 用来判断“登录记忆”是否还有效：带着 Cookie 问一次后端就知道。
  async function fetchMe() {
    try {
      const res = await api.get<{ user: User }>('/auth/me', { silent401: true })
      user.value = res.user
    } catch {
      user.value = null
    } finally {
      loaded.value = true
    }
    return user.value
  }

  async function login(username: string, password: string, remember: boolean) {
    const res = await api.post<{ user: User }>(
      '/auth/login',
      { username, password, remember },
      { silent401: true },
    )
    user.value = res.user
    loaded.value = true
    return res.user
  }

  async function logout() {
    try {
      await api.post('/auth/logout')
    } finally {
      user.value = null
    }
  }

  const changePassword = (oldPwd: string, newPwd: string, newPwd2: string) =>
    api.post('/auth/password', { old: oldPwd, new: newPwd, new2: newPwd2 }, { silent401: true })

  const listSessions = () => api.get<{ items: SessionInfo[] }>('/auth/sessions')

  const revokeSession = (id: number) => api.del(`/auth/sessions/${id}`)

  return { user, loaded, fetchMe, login, logout, changePassword, listSessions, revokeSession }
})
