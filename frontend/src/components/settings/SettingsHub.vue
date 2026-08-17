<script setup lang="ts">
// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com
import { computed, ref, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { useUserStore } from '@/stores/user'
import { t } from '@/i18n'
import AppearancePanel from './AppearancePanel.vue'
import BackfillPanel from './BackfillPanel.vue'
import BookmarkletPanel from './BookmarkletPanel.vue'
import GroupManager from './GroupManager.vue'
import BackupPanel from './BackupPanel.vue'
import GalleryPanel from './GalleryPanel.vue'
import SiteConfigPanel from './SiteConfigPanel.vue'
import SearchPanel from './SearchPanel.vue'
import AboutPanel from './AboutPanel.vue'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ 'update:show': [boolean]; changed: [] }>()

type PanelKey =
  | 'appearance'
  | 'search'
  | 'groups'
  | 'backfill'
  | 'bookmarklet'
  | 'gallery'
  | 'backup'
  | 'site'
  | 'about'

const userStore = useUserStore()
const isAdmin = computed(() => userStore.user?.role === 'admin')

const active = ref<PanelKey>('appearance')

watch(
  () => props.show,
  (open) => {
    if (open) active.value = 'appearance'
  },
)

const entries = computed(() =>
  (
    [
      { key: 'appearance' as const, title: t('settings.nav.appearance'), desc: t('settings.nav.appearanceDesc'), icon: 'mdi:palette-outline' },
      { key: 'search' as const, title: t('settings.nav.search'), desc: t('settings.nav.searchDesc'), icon: 'mdi:magnify' },
      { key: 'groups' as const, title: t('settings.nav.groups'), desc: t('settings.nav.groupsDesc'), icon: 'mdi:folder-cog-outline' },
      { key: 'backfill' as const, title: t('settings.nav.backfill'), desc: t('settings.nav.backfillDesc'), icon: 'mdi:auto-fix' },
      { key: 'bookmarklet' as const, title: t('settings.nav.bookmarklet'), desc: t('settings.nav.bookmarkletDesc'), icon: 'mdi:bookmark-plus-outline' },
      { key: 'gallery' as const, title: t('settings.nav.gallery'), desc: t('settings.nav.galleryDesc'), icon: 'mdi:image-multiple-outline' },
      { key: 'backup' as const, title: t('settings.nav.backup'), desc: t('settings.nav.backupDesc'), icon: 'mdi:cloud-sync-outline' },
      { key: 'site' as const, title: t('settings.nav.site'), desc: t('settings.nav.siteDesc'), icon: 'mdi:earth', admin: true },
      { key: 'about' as const, title: t('settings.nav.about'), desc: t('settings.nav.aboutDesc'), icon: 'mdi:information-outline' },
    ] as const
  ).filter((item) => !('admin' in item && item.admin) || isAdmin.value),
)

const current = computed(() => entries.value.find((e) => e.key === active.value) ?? entries.value[0])
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :title="t('settings.title')"
    class="tp-settings-modal"
    :bordered="false"
    :content-style="{ padding: '0 20px 20px' }"
    @update:show="emit('update:show', $event)"
  >
    <div class="flex flex-col sm:flex-row gap-3 sm:gap-4 h-[min(560px,68vh)]">
      <!-- 侧栏：窄屏下横着滚，宽屏下是一列 -->
      <nav
        class="shrink-0 sm:w-44 flex sm:block gap-1.5 sm:gap-0 sm:space-y-1 overflow-x-auto sm:overflow-x-hidden sm:overflow-y-auto rounded-xl border border-black/5 dark:border-white/[0.08] bg-black/[0.02] dark:bg-white/[0.03] p-2"
      >
        <button
          v-for="item in entries"
          :key="item.key"
          type="button"
          class="shrink-0 sm:w-full flex items-center gap-2 rounded-lg px-3 py-2 text-left text-sm transition-colors whitespace-nowrap"
          :class="
            active === item.key
              ? 'bg-sky-500/15 text-sky-500 font-medium'
              : 'opacity-70 hover:opacity-100 hover:bg-black/[0.04] dark:hover:bg-white/[0.06]'
          "
          @click="active = item.key"
        >
          <Icon :icon="item.icon" class="text-base shrink-0" />
          <span class="truncate">{{ item.title }}</span>
        </button>
      </nav>

      <!-- 内容区：自己滚，侧栏不动 -->
      <div class="flex-1 min-w-0 overflow-y-auto pr-0.5">
        <div class="mb-3">
          <div class="text-base font-medium">{{ current.title }}</div>
          <div class="text-xs opacity-50 mt-0.5">{{ current.desc }}</div>
        </div>

        <AppearancePanel v-if="active === 'appearance'" @changed="emit('changed')" />
        <SearchPanel v-else-if="active === 'search'" @changed="emit('changed')" />
        <GroupManager v-else-if="active === 'groups'" @changed="emit('changed')" />
        <BackfillPanel v-else-if="active === 'backfill'" @changed="emit('changed')" />
        <BookmarkletPanel v-else-if="active === 'bookmarklet'" />
        <GalleryPanel v-else-if="active === 'gallery'" @changed="emit('changed')" />
        <BackupPanel v-else-if="active === 'backup'" @changed="emit('changed')" />
        <SiteConfigPanel v-else-if="active === 'site'" @changed="emit('changed')" />
        <AboutPanel v-else />
      </div>
    </div>
  </n-modal>
</template>

<style>
/* 两栏布局比原来的单列宽不少，窄屏再退回接近满宽。 */
.tp-settings-modal {
  width: min(880px, 94vw);
}
</style>
