<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, onMounted, ref } from 'vue'
import { useMessage } from 'naive-ui'
import { Icon } from '@iconify/vue'
import { api } from '@/api/http'
import { useSiteStore } from '@/stores/site'
import { t } from '@/i18n'
import GalleryPanel from './GalleryPanel.vue'
import SettingsSection from './SettingsSection.vue'
import SettingsRow from './SettingsRow.vue'
import type { SiteConfig } from '@/api/types'

const emit = defineEmits<{ changed: [] }>()

const message = useMessage()
const site = useSiteStore()

const draft = ref<SiteConfig>({ siteTitle: '', siteIcon: '', loginBackground: '' })
const loading = ref(false)
const saving = ref(false)
// 两个字段都从图库选，用同一个弹层，picking 记住这次在选哪个。
const picking = ref<'' | 'icon' | 'login'>('')

async function load() {
  loading.value = true
  try {
    draft.value = await api.get<SiteConfig>('/admin/site')
  } catch (e: any) {
    message.error(e?.message ?? t('site.loadFailed'))
  } finally {
    loading.value = false
  }
}
onMounted(load)

// 预览用。后端已经把值限制成 /uploads/ 路径或 http(s)，这里再抹掉引号即可。
const loginBgStyle = computed(() =>
  draft.value.loginBackground
    ? { backgroundImage: `url("${draft.value.loginBackground.replace(/["\\]/g, '')}")` }
    : undefined,
)

function onPick(path: string) {
  if (picking.value === 'icon') draft.value.siteIcon = path
  else if (picking.value === 'login') draft.value.loginBackground = path
  picking.value = ''
}

async function save() {
  saving.value = true
  try {
    draft.value = await api.put<SiteConfig>('/admin/site', draft.value)
    // 保存后立刻刷新公开配置：标题和 favicon 当场就变，不用重进页面。
    await site.load()
    message.success(t('common.saved'))
    emit('changed')
  } catch (e: any) {
    message.error(e?.message ?? t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <n-spin :show="loading">
    <div class="space-y-3">
      <n-alert type="info" :bordered="false">{{ t('site.notice') }}</n-alert>

      <SettingsSection :title="t('site.section')">
        <SettingsRow :label="t('site.title')" stack>
          <n-input v-model:value="draft.siteTitle" size="small" placeholder="Timmypanel" />
        </SettingsRow>

        <SettingsRow :label="t('site.icon')" :hint="t('site.iconHint')" stack>
          <div class="flex items-center gap-2">
            <img
              v-if="draft.siteIcon"
              :src="draft.siteIcon"
              class="w-8 h-8 rounded object-contain bg-black/5 dark:bg-white/10"
              alt=""
            />
            <n-input
              v-model:value="draft.siteIcon"
              size="small"
              :placeholder="t('site.iconPlaceholder')"
              class="flex-1"
            />
            <n-button size="small" @click="picking = 'icon'">
              <Icon icon="mdi:image-multiple-outline" />
            </n-button>
            <n-button v-if="draft.siteIcon" size="small" quaternary @click="draft.siteIcon = ''">
              {{ t('common.clear') }}
            </n-button>
          </div>
        </SettingsRow>
      </SettingsSection>

      <SettingsSection :title="t('site.loginBg')">
        <SettingsRow :hint="t('site.loginBgHint')" stack>
          <div class="flex items-center gap-2">
            <n-input
              v-model:value="draft.loginBackground"
              size="small"
              :placeholder="t('site.loginBgPlaceholder')"
              class="flex-1"
            />
            <n-button size="small" @click="picking = 'login'">
              <Icon icon="mdi:image-multiple-outline" />
            </n-button>
            <n-button v-if="draft.loginBackground" size="small" quaternary @click="draft.loginBackground = ''">
              {{ t('common.clear') }}
            </n-button>
          </div>
        </SettingsRow>
        <div v-if="draft.loginBackground" class="h-20 rounded-lg bg-cover bg-center" :style="loginBgStyle" />
      </SettingsSection>

      <div class="sticky bottom-0 py-2 flex justify-end bg-gradient-to-t from-black/[0.06] dark:from-white/[0.06] to-transparent backdrop-blur-sm">
        <n-button type="primary" :loading="saving" @click="save">{{ t('common.save') }}</n-button>
      </div>
    </div>
  </n-spin>

  <n-modal
    :show="picking !== ''"
    preset="card"
    :title="t('site.pickTitle')"
    class="max-w-xl"
    :bordered="false"
    @update:show="picking = ''"
  >
    <GalleryPanel selectable @select="onPick" />
  </n-modal>
</template>
