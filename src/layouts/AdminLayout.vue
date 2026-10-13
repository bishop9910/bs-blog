<script setup lang="ts">
import { computed, h, ref } from 'vue'
import {
  PhArrowSquareOut,
  PhArticle,
  PhGear,
  PhImages,
  PhPlus,
  PhSignOut,
} from '@phosphor-icons/vue'
import { NButton, NDropdown, NMenu } from 'naive-ui'
import type { DropdownOption, MenuOption } from 'naive-ui'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from '@/stores/auth'
import ThemeToggle from '@/components/ThemeToggle.vue'

const route = useRoute()
const router = useRouter()
const { user, signOut } = useAuth()

const collapsed = ref(false)

const menuOptions = computed<MenuOption[]>(() => [
  {
    label: '文章',
    key: 'admin-posts',
    icon: () => h(PhArticle, { size: 18 }),
  },
  {
    label: '媒体',
    key: 'admin-media',
    icon: () => h(PhImages, { size: 18 }),
  },
  {
    label: '站点设置',
    key: 'admin-settings',
    icon: () => h(PhGear, { size: 18 }),
  },
])

const activeKey = computed(() => String(route.name ?? 'admin-posts'))

const pageTitle = computed(() => String(route.meta.title ?? '管理后台'))

const userOptions: DropdownOption[] = [
  { label: '查看站点', key: 'view-site', icon: () => h(PhArrowSquareOut, { size: 16 }) },
  { type: 'divider', key: 'd1' },
  { label: '退出登录', key: 'sign-out', icon: () => h(PhSignOut, { size: 16 }) },
]

function onMenuSelect(key: string) {
  void router.push({ name: key })
}

async function onUserSelect(key: string) {
  if (key === 'sign-out') {
    await signOut()
    void router.push({ name: 'admin-login' })
    return
  }
  if (key === 'view-site') {
    window.open(router.resolve({ name: 'home' }).href, '_blank')
  }
}
</script>

<template>
  <div class="admin" :class="{ 'is-collapsed': collapsed }">
    <aside class="admin__side">
      <RouterLink class="admin__brand" :to="{ name: 'admin-posts' }">
        <span class="admin__mark" aria-hidden="true" />
        <span v-show="!collapsed" class="admin__brand-name">bs-blog</span>
      </RouterLink>

      <NMenu
        :value="activeKey"
        :options="menuOptions"
        :collapsed="collapsed"
        :collapsed-width="64"
        :collapsed-icon-size="20"
        :indent="18"
        @update:value="onMenuSelect"
      />

      <button
        class="admin__collapse"
        type="button"
        :aria-label="collapsed ? '展开侧栏' : '收起侧栏'"
        @click="collapsed = !collapsed"
      >
        {{ collapsed ? '»' : '«' }}
      </button>
    </aside>

    <div class="admin__main">
      <header class="admin__header">
        <h1 class="admin__title">{{ pageTitle }}</h1>

        <div class="admin__actions">
          <NButton
            size="small"
            secondary
            type="primary"
            @click="router.push({ name: 'admin-post-new' })"
          >
            <template #icon>
              <PhPlus :size="16" weight="bold" />
            </template>
            写文章
          </NButton>

          <ThemeToggle />

          <NDropdown :options="userOptions" trigger="click" @select="onUserSelect">
            <button class="admin__user" type="button">
              <span class="admin__avatar">{{ (user?.nickname || user?.username || 'b')[0] }}</span>
              <span class="admin__user-name">{{ user?.nickname || user?.username || '未登录' }}</span>
            </button>
          </NDropdown>
        </div>
      </header>

      <main class="admin__content">
        <RouterView v-slot="{ Component }">
          <Transition name="route-fade" mode="out-in">
            <component :is="Component" />
          </Transition>
        </RouterView>
      </main>
    </div>
  </div>
</template>

<style scoped>
.admin {
  display: flex;
  min-height: 100vh;
  background: var(--bg-base);
}

.admin__side {
  position: sticky;
  top: 0;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  width: 220px;
  height: 100vh;
  border-right: 1px solid var(--border-subtle);
  background: var(--bg-subtle);
  transition: width var(--dur-normal) var(--ease-out);
}

.admin.is-collapsed .admin__side {
  width: 64px;
}

.admin__brand {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  height: var(--header-h);
  padding: 0 var(--space-md);
  color: var(--text-primary);
  text-decoration: none;
}

.admin__mark {
  flex-shrink: 0;
  width: 10px;
  height: 10px;
  border-radius: 3px;
  background: var(--accent);
  box-shadow: 0 0 12px var(--glow-accent);
}

.admin__brand-name {
  font-family: var(--font-display);
  font-size: var(--text-base);
  white-space: nowrap;
}

.admin__collapse {
  margin: auto var(--space-md) var(--space-md);
  height: 32px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
}

.admin__collapse:hover {
  border-color: var(--border-accent);
  color: var(--accent);
}

.admin__main {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.admin__header {
  position: sticky;
  top: 0;
  z-index: var(--z-sticky);
  display: flex;
  align-items: center;
  gap: var(--space-md);
  height: var(--header-h);
  padding: 0 var(--space-lg);
  border-bottom: 1px solid var(--border-subtle);
  background: color-mix(in srgb, var(--bg-base) 82%, transparent);
  backdrop-filter: blur(14px) saturate(1.4);
}

.admin__title {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-lg);
}

.admin__actions {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  margin-left: auto;
}

.admin__user {
  display: inline-flex;
  align-items: center;
  gap: var(--space-sm);
  min-height: var(--hit-min);
  padding: 0 var(--space-sm) 0 var(--space-2xs);
  border: 1px solid transparent;
  border-radius: var(--radius-full);
  background: transparent;
  color: var(--text-secondary);
  font: inherit;
  font-size: var(--text-sm);
  cursor: pointer;
}

.admin__user:hover {
  border-color: var(--border-subtle);
  background: var(--bg-hover);
}

.admin__avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: var(--radius-full);
  background: var(--accent-subtle);
  color: var(--accent);
  font-size: var(--text-sm);
  text-transform: uppercase;
}

.admin__content {
  flex: 1;
  min-width: 0;
  padding: var(--space-lg);
}

@media (max-width: 800px) {
  .admin__side {
    width: 64px;
  }

  .admin__brand-name,
  .admin__user-name {
    display: none;
  }

  .admin__content {
    padding: var(--space-md);
  }
}
</style>
