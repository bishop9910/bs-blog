import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useAuth } from '@/stores/auth'
import {
  abortRouteTransition,
  beginRouteTransition,
  cancelRouteTransition,
  markRouteCommitted,
} from '@/composables/usePageTransition'

declare module 'vue-router' {
  interface RouteMeta {
    /** Requires an authenticated admin session. */
    requiresAuth?: boolean
    /** Marks the admin shell (different chrome, no grain/cursor). */
    admin?: boolean
    title?: string
    /**
     * Position of a public page in the site's own reading order — the header's
     * nav list, with detail pages placed just after the list they belong to.
     * The route transition reads it to decide which way a page should travel,
     * so going "back to the archive" does not arrive looking like going
     * forward to it.
     */
    order?: number
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    component: () => import('@/layouts/PublicLayout.vue'),
    children: [
      {
        path: '',
        name: 'home',
        component: () => import('@/views/HomeView.vue'),
        meta: { title: '首页', order: 0 },
      },
      {
        path: 'posts',
        name: 'posts',
        component: () => import('@/views/PostListView.vue'),
        meta: { title: '全部文章', order: 1 },
      },
      {
        path: 'post/:slug',
        name: 'post',
        component: () => import('@/views/PostDetailView.vue'),
        meta: { title: '文章', order: 2 },
      },
      {
        path: 'archive',
        name: 'archive',
        component: () => import('@/views/ArchiveView.vue'),
        meta: { title: '归档', order: 3 },
      },
      {
        path: 'tags',
        name: 'tags',
        component: () => import('@/views/TagsView.vue'),
        meta: { title: '标签', order: 4 },
      },
      {
        path: 'tag/:slug',
        name: 'tag',
        component: () => import('@/views/PostListView.vue'),
        meta: { title: '标签', order: 5 },
      },
      {
        path: 'calendar',
        name: 'calendar',
        component: () => import('@/views/CalendarView.vue'),
        meta: { title: '日历', order: 6 },
      },
      {
        path: 'about',
        name: 'about',
        component: () => import('@/views/AboutView.vue'),
        meta: { title: '关于', order: 7 },
      },
    ],
  },
  {
    path: '/admin/login',
    name: 'admin-login',
    component: () => import('@/views/admin/LoginView.vue'),
    meta: { title: '登录' },
  },
  {
    path: '/admin',
    component: () => import('@/layouts/AdminLayout.vue'),
    meta: { requiresAuth: true, admin: true },
    children: [
      { path: '', redirect: { name: 'admin-posts' } },
      {
        path: 'posts',
        name: 'admin-posts',
        component: () => import('@/views/admin/PostListView.vue'),
        meta: { requiresAuth: true, admin: true, title: '文章管理' },
      },
      {
        path: 'posts/new',
        name: 'admin-post-new',
        component: () => import('@/views/admin/PostEditorView.vue'),
        meta: { requiresAuth: true, admin: true, title: '新建文章' },
      },
      {
        path: 'posts/:id/edit',
        name: 'admin-post-edit',
        component: () => import('@/views/admin/PostEditorView.vue'),
        meta: { requiresAuth: true, admin: true, title: '编辑文章' },
      },
      {
        path: 'media',
        name: 'admin-media',
        component: () => import('@/views/admin/MediaView.vue'),
        meta: { requiresAuth: true, admin: true, title: '媒体库' },
      },
      {
        path: 'music',
        name: 'admin-music',
        component: () => import('@/views/admin/MusicView.vue'),
        meta: { requiresAuth: true, admin: true, title: '音乐库' },
      },
      {
        path: 'settings',
        name: 'admin-settings',
        component: () => import('@/views/admin/SettingsView.vue'),
        meta: { requiresAuth: true, admin: true, title: '站点设置' },
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('@/views/NotFoundView.vue'),
    meta: { title: '页面不存在' },
  },
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  scrollBehavior(to, _from, savedPosition) {
    if (savedPosition) return savedPosition
    if (to.hash) return { el: to.hash, behavior: 'smooth', top: 80 }
    // Keep the transition quiet between the admin screens.
    return to.meta.admin ? { top: 0 } : { top: 0, behavior: 'instant' as ScrollBehavior }
  },
})

const SITE_NAME = 'bs-blog'

router.beforeEach(async (to) => {
  document.title = to.meta.title ? `${to.meta.title} · ${SITE_NAME}` : SITE_NAME

  if (!to.meta.requiresAuth) return true

  const auth = useAuth()
  // `restore` validates the persisted token once per session.
  const ok = await auth.restore()

  if (ok) return true
  return { name: 'admin-login', query: { redirect: to.fullPath } }
})

/**
 * The curtain closes here — at click time, before the lazy chunk is even
 * requested, and a full route change before the next page starts working. This
 * guard waits for it, so the chunk fetch, the mount and the page's own data
 * request all happen behind a covered screen instead of during the animation.
 *
 * Registered after the auth guard on purpose: when that one redirects, the
 * remaining guards are skipped, so a visit that never lands never flashes a
 * blade across the page.
 */
router.beforeEach(async (to, from) => {
  await beginRouteTransition(to, from)
})

router.afterEach((_to, _from, failure) => {
  if (failure) {
    cancelRouteTransition()
    return
  }
  // The route is committed; the page behind the blade gets a frame to lay out,
  // and then reports for itself whether it has content yet.
  void markRouteCommitted()
})

router.onError(() => {
  abortRouteTransition()
})

export default router
