<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, onMounted, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { api } from '@/api/http'
import { t } from '@/i18n'
import SettingsSection from './SettingsSection.vue'
import SettingsRow from './SettingsRow.vue'
import type { IngestTokenCreated, IngestTokenStatus } from '@/api/types'

// 决策 026：后端直连抓不动的站点（Cloudflare 按 TLS 指纹拦机器人），
// 让用户浏览器在自己已过反爬的会话里抓图标，POST 回 /api/v1/ingest。
// 这个面板负责生成/吊销令牌，并把令牌拼进 bookmarklet 给用户拖进收藏夹栏。

const message = useMessage()

const loading = ref(true)
const exists = ref(false)
const createdAt = ref('')
const busy = ref(false)
// 令牌原文只存在于「刚生成」这一刻的内存里，刷新或离开面板就再也拿不回来
// （后端只存 SHA-256）—— 所以生成后要立刻把书签展示出来让用户拖走。
const freshToken = ref('')

onMounted(async () => {
  try {
    const res = await api.get<IngestTokenStatus>('/ingest/token')
    exists.value = res.exists
    createdAt.value = res.createdAt ?? ''
  } finally {
    loading.value = false
  }
})

// bookmarklet 在目标页面的上下文里执行：同源抓图标没有 CORS 问题，
// 跨站 POST 回面板是 simple request（FormData），不需要目标站配合。
// 打分时 apple-touch-icon 和大尺寸优先，口径和后端 iconRelScore 一致。
//
// 决策 027：先试直连 fetch（宽松站点静默完成）；目标站 CSP 的 connect-src
// 不含面板域时 fetch 在发请求前抛 TypeError，这时降级为 window.open 打开
// 面板的 /ingest-bridge 桥接页，用 postMessage 交接（connect-src 管不到
// 页面导航和 postMessage）。向弹窗发消息的 targetOrigin 锁面板源（令牌
// 不泄露给别的窗口）；收回传时校验 event.origin === 面板源（防恶意页面
// 伪造 next 做跳转钓鱼）。弹窗 URL 带时间戳参数，强制复用同名旧窗口时
// 也重新加载，保证 tp-ingest-ready 握手一定会再发一次。
const bookmarklet = computed(() => {
  if (!freshToken.value) return ''
  const panel = location.origin
  const code = `(async()=>{
const P=${JSON.stringify(panel)},T=${JSON.stringify(freshToken.value)};
const ls=[...document.querySelectorAll('link[rel*="icon"]')];
const sc=l=>{let s=1;const r=(l.rel||'').toLowerCase();if(r.includes('apple-touch-icon'))s=5;else if(r.includes('shortcut'))s=2;const m=((l.sizes&&l.sizes.value)||'').match(/(\\d+)x/);if(m&&+m[1]>=64)s+=3;return s};
const cs=[...new Set(ls.map(l=>({u:l.href,s:sc(l)})).filter(x=>x.u&&!x.u.startsWith('data:')).sort((a,b)=>b.s-a.s).map(x=>x.u).concat(location.origin+'/favicon.ico'))];
let icon=null;
for(const u of cs){try{const r=await fetch(u);if(!r.ok)continue;const b=await r.blob();if(!b.size||b.size>1048576)continue;if(b.type.startsWith('text/html'))continue;icon=b;break}catch(e){}}
const done=d=>{if(d.next){location.href=d.next;return}alert(d.created?${JSON.stringify(t('bookmarklet.alertOk'))}:${JSON.stringify(t('bookmarklet.alertUpdated'))})};
const fail=m=>alert(${JSON.stringify(t('bookmarklet.alertFail'))}+' '+m);
try{
const fd=new FormData();fd.append('token',T);fd.append('url',location.href);fd.append('title',document.title||'');
if(icon)fd.append('icon',icon,'favicon');else if(cs[0])fd.append('iconUrl',cs[0]);
const r=await fetch(P+'/api/v1/ingest',{method:'POST',body:fd});
const d=await r.json().catch(()=>({}));
if(!r.ok){fail(d.error||r.status);return}
done(d);
}catch(e){
const w=window.open(P+'/ingest-bridge?r='+Date.now(),'tp-ingest','width=420,height=260');
if(!w){alert(${JSON.stringify(t('bookmarklet.alertPopup'))});return}
const pl={type:'tp-ingest-payload',token:T,url:location.href,title:document.title||''};
if(icon)pl.icon=icon;else if(cs[0])pl.iconUrl=cs[0];
const onMsg=ev=>{
if(ev.origin!==P)return;
const d=ev.data||{};
if(d.type==='tp-ingest-ready'){w.postMessage(pl,P);return}
if(d.type!=='tp-ingest-result')return;
window.removeEventListener('message',onMsg);
if(!d.ok){fail(d.error||'');return}
done(d);
};
window.addEventListener('message',onMsg);
}
})()`
  return `javascript:${encodeURIComponent(code).replace(/'/g, '%27')}`
})

async function generate() {
  busy.value = true
  try {
    const res = await api.post<IngestTokenCreated>('/ingest/token')
    freshToken.value = res.token
    exists.value = true
    createdAt.value = res.createdAt
    message.success(t('bookmarklet.generated'))
  } finally {
    busy.value = false
  }
}

async function revoke() {
  busy.value = true
  try {
    await api.del('/ingest/token')
    exists.value = false
    freshToken.value = ''
    message.success(t('bookmarklet.revoked'))
  } finally {
    busy.value = false
  }
}

async function copyCode() {
  try {
    await navigator.clipboard.writeText(bookmarklet.value)
    message.success(t('bookmarklet.copied'))
  } catch {
    message.warning(t('bookmarklet.copyFailed'))
  }
}
</script>

<template>
  <div class="space-y-3">
    <SettingsSection :title="t('bookmarklet.what')">
      <p class="text-sm opacity-70">{{ t('bookmarklet.whatHint') }}</p>
    </SettingsSection>

    <SettingsSection :title="t('bookmarklet.manage')">
      <div v-if="loading" class="text-sm opacity-50">{{ t('common.loading') }}</div>
      <template v-else>
        <SettingsRow
          v-if="exists"
          :label="t('bookmarklet.exists')"
          :hint="t('bookmarklet.existsHint', { time: createdAt.slice(0, 16).replace('T', ' ') })"
        >
          <n-button size="small" secondary :loading="busy" @click="revoke">
            {{ t('bookmarklet.revoke') }}
          </n-button>
        </SettingsRow>
        <SettingsRow
          :label="exists ? t('bookmarklet.regenerate') : t('bookmarklet.generate')"
          :hint="exists ? t('bookmarklet.regenerateHint') : t('bookmarklet.generateHint')"
        >
          <n-button size="small" type="primary" :loading="busy" @click="generate">
            {{ exists ? t('bookmarklet.regenerate') : t('bookmarklet.generate') }}
          </n-button>
        </SettingsRow>
      </template>
    </SettingsSection>

    <!-- 令牌只完整出现这一次：必须马上把书签拖走或复制走 -->
    <SettingsSection v-if="freshToken" :title="t('bookmarklet.install')">
      <p class="text-sm opacity-70">{{ t('bookmarklet.dragHint') }}</p>
      <div class="py-1">
        <a
          :href="bookmarklet"
          class="inline-flex items-center gap-1.5 rounded-lg bg-sky-500/15 text-sky-500 px-3 py-1.5 text-sm font-medium cursor-grab select-none"
          @click.prevent
        >
          <Icon icon="mdi:bookmark-plus-outline" />
          {{ t('bookmarklet.linkText') }}
        </a>
      </div>
      <SettingsRow :label="t('bookmarklet.manualCopy')">
        <n-button size="tiny" @click="copyCode">{{ t('bookmarklet.copy') }}</n-button>
      </SettingsRow>
      <p class="text-xs opacity-45">{{ t('bookmarklet.cspNote') }}</p>
    </SettingsSection>
  </div>
</template>
