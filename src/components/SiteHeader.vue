<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { PhList, PhLockSimple, PhX } from '@phosphor-icons/vue'
import { useRoute } from 'vue-router'
import { useSite } from '@/composables/useSite'
import AuthorAvatar from './AuthorAvatar.vue'
import ThemeToggle from './ThemeToggle.vue'

const { site } = useSite()
const route = useRoute()

const scrolled = ref(false)
const menuOpen = ref(false)

const NAV = [
  { name: 'home', label: '首页' },
  { name: 'posts', label: '文章' },
  { name: 'archive', label: '归档' },
  { name: 'tags', label: '标签' },
  { name: 'calendar', label: '日历' },
  { name: 'about', label: '关于' },
] as const

/** Tag pages belong to the article section, so "文章" stays lit there. */
function isActive(name: string): boolean {
  const current = String(route.name ?? '')
  if (name === 'posts') return current === 'posts' || current === 'tag'
  return current === name
}

function onScroll() {
  scrolled.value = window.scrollY > 12
}

onMounted(() => {
  onScroll()
  window.addEventListener('scroll', onScroll, { passive: true })
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', onScroll)
})

watch(
  () => route.fullPath,
  () => {
    menuOpen.value = false
  },
)
</script>

<template>
  <header class="header" :class="{ 'is-scrolled': scrolled }">
    <div class="header__inner">
      <RouterLink class="brand" :to="{ name: 'home' }">
        <AuthorAvatar
          v-if="site?.avatarUrl"
          class="brand__avatar"
          :src="site.avatarUrl"
          :name="site.author"
          :size="26"
        />
        <span v-else class="brand__mark" aria-hidden="true" />
        <span class="brand__name">{{ site?.title ?? 'bs-blog' }}</span>
      </RouterLink>

      <nav class="nav" aria-label="主导航">
        <RouterLink
          v-for="item in NAV"
          :key="item.name"
          class="nav__link"
          :class="{ 'is-active': isActive(item.name) }"
          :to="{ name: item.name }"
        >
          {{ item.label }}
        </RouterLink>
      </nav>

      <div class="header__actions">
        <ThemeToggle />
        <RouterLink
          class="icon-btn"
          :to="{ name: 'admin-posts' }"
          title="管理后台"
          aria-label="管理后台"
        >
          <PhLockSimple :size="18" weight="bold" />
        </RouterLink>
        <button
          class="icon-btn header__burger"
          type="button"
          :aria-expanded="menuOpen"
          aria-label="切换菜单"
          @click="menuOpen = !menuOpen"
        >
          <component :is="menuOpen ? PhX : PhList" :size="20" weight="bold" />
        </button>
      </div>
    </div>

    <Transition name="drawer">
      <nav v-if="menuOpen" class="drawer" aria-label="移动导航">
        <RouterLink
          v-for="item in NAV"
          :key="item.name"
          class="drawer__link"
          :class="{ 'is-active': isActive(item.name) }"
          :to="{ name: item.name }"
        >
          {{ item.label }}
        </RouterLink>
      </nav>
    </Transition>
  </header>
</template>

<style scoped>
.header {
  position: sticky;
  top: 0;
  z-index: var(--z-sticky);
  border-bottom: 1px solid transparent;
  transition:
    background var(--dur-normal) var(--ease-out),
    border-color var(--dur-normal) var(--ease-out),
    backdrop-filter var(--dur-normal) var(--ease-out);
}

.header.is-scrolled {
  border-bottom-color: var(--border-subtle);
  background: color-mix(in srgb, var(--bg-base) 78%, transparent);
  backdrop-filter: blur(14px) saturate(1.4);
}

.header__inner {
  display: flex;
  align-items: center;
  gap: var(--space-lg);
  max-width: var(--container-xl);
  height: var(--header-h);
  margin: 0 auto;
  padding: 0 var(--space-lg);
}

.brand {
  display: inline-flex;
  align-items: center;
  gap: var(--space-sm);
  color: var(--text-primary);
  text-decoration: none;
}

.brand__mark {
  width: 10px;
  height: 10px;
  border-radius: 3px;
  background: var(--accent);
  box-shadow: 0 0 12px var(--glow-accent);
  transition: transform var(--dur-normal) var(--ease-spring);
}

.brand:hover .brand__mark {
  transform: rotate(45deg) scale(1.2);
}

.brand__avatar {
  border-color: var(--accent-border);
  transition: transform var(--dur-normal) var(--ease-spring);
}

.brand:hover .brand__avatar {
  transform: scale(1.08);
}

.brand__name {
  font-family: var(--font-display);
  font-size: var(--text-lg);
  letter-spacing: var(--tracking-tight);
}

.nav {
  display: flex;
  align-items: center;
  gap: var(--space-2xs);
  margin-left: auto;
}

.nav__link {
  position: relative;
  padding: var(--space-sm) var(--space-md);
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  font-size: var(--text-sm);
  text-decoration: none;
  transition:
    color var(--dur-fast) var(--ease-out),
    background var(--dur-fast) var(--ease-out);
}

.nav__link:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.nav__link.is-active {
  color: var(--accent);
}

.nav__link.is-active::after {
  content: '';
  position: absolute;
  left: 50%;
  bottom: 2px;
  width: 16px;
  height: 2px;
  border-radius: 1px;
  background: var(--accent);
  transform: translateX(-50%);
}

.header__actions {
  display: flex;
  align-items: center;
  gap: var(--space-2xs);
}

.icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: var(--hit-min);
  height: var(--hit-min);
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition:
    color var(--dur-fast) var(--ease-out),
    background var(--dur-fast) var(--ease-out);
}

.icon-btn:hover {
  color: var(--accent);
  background: var(--bg-hover);
}

.icon-btn:active {
  transform: scale(0.94);
}

.header__burger {
  display: none;
}

.drawer {
  display: none;
  flex-direction: column;
  padding: var(--space-sm) var(--space-lg) var(--space-lg);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-elevated);
}

.drawer__link {
  padding: var(--space-md) var(--space-sm);
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  font-size: var(--text-base);
  text-decoration: none;
}

.drawer__link.is-active {
  color: var(--accent);
  background: var(--accent-subtle);
}

.drawer-enter-active,
.drawer-leave-active {
  transition:
    opacity var(--dur-fast) var(--ease-out),
    transform var(--dur-fast) var(--ease-out);
}

.drawer-enter-from,
.drawer-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}

@media (max-width: 800px) {
  .header__inner {
    gap: var(--space-md);
    padding: 0 var(--space-md);
  }

  .nav {
    display: none;
  }

  .header__burger {
    display: inline-flex;
  }

  .drawer {
    display: flex;
  }
}
</style>
