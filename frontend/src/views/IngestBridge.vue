<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from '@/api/http'
import { t } from '@/i18n'

// 决策 027：目标站 CSP 的 connect-src 管不到页面导航和 postMessage，
// 所以书签在直连 fetch 被拦时 window.open 打开本页，把令牌/网址/标题/图标
// postMessage 过来，由本页在面板自己的源下同源提交 /api/v1/ingest。
// 本页是免登录公开路由（鉴权只靠负载里的 ingest 令牌，威胁模型同 ingest 端点）。
// 安全口径：接受任意源的 postMessage（令牌即凭证，同决策 026），
// 但回传结果的 targetOrigin 锁定为来消息那一帧的 event.origin，绝不外发。

type Status = 'waiting' | 'submitting' | 'ok' | 'updated' | 'fail'

const status = ref<Status>('waiting')
const detail = ref('')
const canClose = ref(false)
let closeTimer: number | undefined

const statusText = computed(() => {
  switch (status.value) {
    case 'submitting':
      return t('ingestBridge.submitting')
    case 'ok':
      return t('ingestBridge.ok')
    case 'updated':
      return t('ingestBridge.updated')
    case 'fail':
      return t('ingestBridge.fail')
    default:
      return t('ingestBridge.waiting')
  }
})

async function handle(ev: MessageEvent) {
  const d = ev.data
  if (!d || d.type !== 'tp-ingest-payload') return
  if (typeof d.token !== 'string' || !d.token) return
  if (typeof d.url !== 'string' || !d.url) return
  // 只处理第一份有效负载，之后的消息一律忽略，避免重复提交。
  window.removeEventListener('message', onMessage)

  const source = ev.source as Window | null
  const origin = ev.origin
  const reply = (msg: Record<string, unknown>) => {
    try {
      source?.postMessage(msg, origin)
    } catch {
      // opener 已被用户关掉，没有接收方，忽略
    }
  }

  status.value = 'submitting'
  try {
    const fd = new FormData()
    fd.append('token', d.token)
    fd.append('url', d.url)
    fd.append('title', typeof d.title === 'string' ? d.title : '')
    if (d.icon instanceof Blob) fd.append('icon', d.icon, 'favicon')
    else if (typeof d.iconUrl === 'string' && d.iconUrl) fd.append('iconUrl', d.iconUrl)
    // silent401：本页没有会话，401 是令牌问题，不能触发全局的跳登录。
    const r = await api.upload<{ created?: boolean; next?: string; remaining?: number }>(
      '/ingest',
      fd,
      { silent401: true },
    )
    status.value = r.created ? 'ok' : 'updated'
    reply({
      type: 'tp-ingest-result',
      ok: true,
      created: !!r.created,
      next: r.next || '',
      remaining: r.remaining ?? 0,
    })
  } catch (err) {
    status.value = 'fail'
    detail.value = err instanceof Error ? err.message : String(err)
    reply({ type: 'tp-ingest-result', ok: false, error: detail.value })
  }
  canClose.value = true
  // 书签靠回传结果做事（跳 next / 弹提示），这个窗口只是进度展示，
  // 闪一下就好；脚本 window.open 出来的窗口允许自闭。
  closeTimer = window.setTimeout(() => window.close(), 1000)
}

function onMessage(ev: MessageEvent) {
  void handle(ev)
}

onMounted(() => {
  if (!window.opener) {
    status.value = 'fail'
    detail.value = t('ingestBridge.noOpener')
    return
  }
  window.addEventListener('message', onMessage)
  // 握手：告诉书签「可以发负载了」。此刻还不知道 opener 的源，只能 '*'；
  // 这条消息不含任何敏感数据，真正的负载是书签锁着面板源发过来的。
  window.opener.postMessage({ type: 'tp-ingest-ready' }, '*')
})

onBeforeUnmount(() => {
  window.removeEventListener('message', onMessage)
  if (closeTimer !== undefined) window.clearTimeout(closeTimer)
})
</script>

<template>
  <!-- 小弹窗，风格跟登录页一致：品牌紫深色渐变底 + 白字 -->
  <div class="bridge">
    <div class="card">
      <p class="title">{{ t('ingestBridge.title') }}</p>
      <p class="status">{{ statusText }}</p>
      <p v-if="detail" class="detail">{{ detail }}</p>
      <p v-if="canClose" class="hint">{{ t('ingestBridge.closing') }}</p>
    </div>
  </div>
</template>

<style scoped>
.bridge {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #2a2456 0%, #534ab7 100%);
  color: #fff;
}

.card {
  max-width: 320px;
  padding: 24px;
  text-align: center;
}

.title {
  font-size: 13px;
  opacity: 0.7;
  margin-bottom: 10px;
}

.status {
  font-size: 18px;
  font-weight: 600;
}

.detail {
  margin-top: 8px;
  font-size: 12px;
  opacity: 0.7;
  word-break: break-all;
}

.hint {
  margin-top: 12px;
  font-size: 12px;
  opacity: 0.5;
}
</style>
