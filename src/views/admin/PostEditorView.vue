<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { NAlert, NButton, NInput, NPopconfirm, NSelect, useMessage } from 'naive-ui'
import type { InputInst } from 'naive-ui'
import {
  PhArrowSquareOut,
  PhCheckCircle,
  PhFloppyDisk,
  PhTrash,
  PhXCircle,
} from '@phosphor-icons/vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import {
  checkSlug,
  createPost,
  deletePost,
  getAdminPost,
  listTags,
  publishPost,
  unpublishPost,
  updatePost,
} from '@/api/posts'
import type { PostInput, PostStatus } from '@/types/blog'
import RichTextEditor from '@/components/editor/RichTextEditor.vue'
import { countWords, readingMinutes, slugify } from '@/utils/text'

const route = useRoute()
const router = useRouter()
const message = useMessage()

const loading = ref(true)
const saving = ref(false)
const publishing = ref(false)
const status = ref<PostStatus>('draft')
const postId = ref(0)
const dirty = ref(false)
const tagOptions = ref<{ label: string; value: string }[]>([])

const form = reactive({
  title: '',
  slug: '',
  summary: '',
  coverUrl: '',
  contentHtml: '',
  contentJson: '{}',
  tagNames: [] as string[],
  visibility: 'public' as 'public' | 'private',
  metaTitle: '',
  metaDescription: '',
})

/** The slug is auto-derived from the title only until the author edits it. */
const slugTouched = ref(false)
const slugState = ref<'idle' | 'checking' | 'ok' | 'taken'>('idle')

/**
 * Outcome of the last save/publish, kept on screen.
 *
 * A toast is gone in a few seconds, which is easy to miss in the middle of a
 * long article — and a *refused* save (empty title, taken slug) used to look
 * exactly like a successful one: the draft stayed on screen, nothing reached
 * the server, and the article was gone after the next navigation.
 */
type EditorAlert = {
  type: 'info' | 'success' | 'warning' | 'error'
  text: string
  /** "Look at the result" shortcut, e.g. the live post after publishing. */
  link?: { href: string; label: string }
  action?: { label: string; run: () => void }
  at: number
}

const alert = ref<EditorAlert | null>(null)
const titleInput = ref<InputInst | null>(null)
const titleEmpty = ref(false)

const isNew = computed(() => route.name === 'admin-post-new')
const heading = computed(() => (isNew.value ? '写新文章' : form.title || '编辑文章'))
const isPublished = computed(() => status.value === 'published')

const words = computed(() => countWords(form.contentHtml))
const minutes = computed(() => readingMinutes(form.contentHtml))

const slugHint = computed(() => {
  if (slugState.value === 'checking') return '正在检查…'
  if (slugState.value === 'taken') return '这个地址已被占用'
  if (slugState.value === 'ok') return '可以使用'
  return '留空会根据标题自动生成'
})

let slugTimer = 0

function scheduleSlugCheck() {
  window.clearTimeout(slugTimer)
  const candidate = form.slug.trim()
  if (!candidate) {
    slugState.value = 'idle'
    return
  }
  slugState.value = 'checking'
  slugTimer = window.setTimeout(async () => {
    try {
      const result = await checkSlug(candidate, postId.value || undefined)
      // Only trust a reply that actually carries a verdict: an unexpected body
      // (undefined `available`) would be read as "slug taken" and block saving
      // outright, which is the one failure mode this check must never cause.
      if (!result.slug) {
        slugState.value = 'idle'
        return
      }
      slugState.value = result.available ? 'ok' : 'taken'
    } catch {
      // A failed probe must never block saving — the server re-checks anyway.
      slugState.value = 'idle'
    }
  }, 400)
}

function onSlugInput() {
  slugTouched.value = true
  scheduleSlugCheck()
}

watch(
  () => form.title,
  (title) => {
    if (slugTouched.value || !isNew.value) return
    form.slug = slugify(title)
    scheduleSlugCheck()
  },
)

/**
 * Set while the form is being filled programmatically (a load, or the server's
 * echo after a save). The dirty flag must not be raised by our own writes.
 */
let applyingForm = false

// Any edit at all means there is something to lose on navigation.
// Sync flush: the flag above is only meaningful at mutation time, but a queued
// watcher would run after the caller has already reset it.
watch(
  form,
  () => {
    if (applyingForm) return
    dirty.value = true
    if (form.title.trim()) titleEmpty.value = false
    scheduleDraftSave()
  },
  { flush: 'sync' },
)

/* -------------------------------------------------------------------------- */
/* Local draft mirror                                                         */
/* -------------------------------------------------------------------------- */

type DraftForm = {
  title: string
  slug: string
  summary: string
  coverUrl: string
  contentHtml: string
  contentJson: string
  tagNames: string[]
  visibility: 'public' | 'private'
  metaTitle: string
  metaDescription: string
}

type StoredDraft = { savedAt: number; form: DraftForm }

/**
 * A copy of the form in localStorage, written while the author types.
 *
 * The server copy only exists once a save has been accepted. Without this
 * mirror, closing the tab, reloading, or leaving a draft that the server never
 * received meant the whole article was gone with no way back.
 */
const DRAFT_NEW_KEY = 'bs-blog.editor-draft:new'
const draftSavedAt = ref(0)
let draftTimer = 0

const draftLabel = computed(() => {
  if (!dirty.value) return ''
  return draftSavedAt.value ? '草稿已自动保存在本地' : '尚未保存'
})

function draftKey(id: number): string {
  return id > 0 ? `bs-blog.editor-draft:post-${id}` : DRAFT_NEW_KEY
}

function draftForm(): DraftForm {
  return {
    title: form.title,
    slug: form.slug,
    summary: form.summary,
    coverUrl: form.coverUrl,
    contentHtml: form.contentHtml,
    contentJson: form.contentJson,
    tagNames: [...form.tagNames],
    visibility: form.visibility,
    metaTitle: form.metaTitle,
    metaDescription: form.metaDescription,
  }
}

/** Nothing typed yet: no draft to keep, and no stale one to leave behind. */
function draftIsEmpty(): boolean {
  return !form.title.trim() && !form.contentHtml.trim() && !form.summary.trim()
}

function persistDraft() {
  window.clearTimeout(draftTimer)
  if (applyingForm) return
  try {
    if (draftIsEmpty()) {
      window.localStorage.removeItem(DRAFT_NEW_KEY)
      if (postId.value > 0) window.localStorage.removeItem(draftKey(postId.value))
      draftSavedAt.value = 0
      return
    }
    const snapshot: StoredDraft = { savedAt: Date.now(), form: draftForm() }
    window.localStorage.setItem(draftKey(postId.value), JSON.stringify(snapshot))
    draftSavedAt.value = snapshot.savedAt
  } catch {
    // Quota exceeded or storage blocked: the mirror must never break editing,
    // the server copy stays authoritative.
  }
}

function scheduleDraftSave() {
  window.clearTimeout(draftTimer)
  draftTimer = window.setTimeout(persistDraft, 1200)
}

function readDraft(id: number): StoredDraft | null {
  try {
    const raw = window.localStorage.getItem(draftKey(id))
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<StoredDraft>
    if (!parsed || typeof parsed !== 'object' || !parsed.form) return null
    if (typeof parsed.savedAt !== 'number') return null
    return parsed as StoredDraft
  } catch {
    return null
  }
}

/** Drop the mirror: the server now holds this article. */
function clearDraft() {
  window.clearTimeout(draftTimer)
  try {
    window.localStorage.removeItem(DRAFT_NEW_KEY)
    if (postId.value > 0) window.localStorage.removeItem(draftKey(postId.value))
  } catch {
    // Ignore: an unremovable draft is harmless, it is overwritten by the next
    // edit and ignored once the article has been saved.
  }
  draftSavedAt.value = 0
}

function applyDraft(draft: StoredDraft) {
  applyingForm = true
  Object.assign(form, draft.form)
  applyingForm = false
  slugTouched.value = true
  dirty.value = true
}

/** Offer the local copy back, so an interrupted article is not lost. */
function restoreDraft(id: number) {
  const draft = readDraft(id)
  if (!draft) return
  applyDraft(draft)
  draftSavedAt.value = draft.savedAt
  const minutes = Math.max(1, Math.round((Date.now() - draft.savedAt) / 60000))
  alert.value = {
    type: 'info',
    text: `已恢复 ${minutes} 分钟前自动保存在本地的草稿，它还没有写入服务器。`,
    action: { label: '丢弃这份草稿', run: discardDraft },
    at: Date.now(),
  }
}

function discardDraft() {
  clearDraft()
  alert.value = null
  void load()
}

function payload(): PostInput {
  return {
    title: form.title.trim(),
    slug: form.slug.trim() || undefined,
    summary: form.summary.trim(),
    coverUrl: form.coverUrl.trim(),
    contentHtml: form.contentHtml,
    contentJson: form.contentJson,
    tagNames: form.tagNames,
    visibility: form.visibility,
    metaTitle: form.metaTitle.trim(),
    metaDescription: form.metaDescription.trim(),
  }
}

async function save(options: { silent?: boolean } = {}): Promise<boolean> {
  if (!form.title.trim()) {
    titleEmpty.value = true
    alert.value = {
      type: 'warning',
      text: '标题不能为空，这篇文章还没有保存。填上标题后点“保存”或“发布”即可。',
      at: Date.now(),
    }
    message.warning('标题不能为空')
    titleInput.value?.focus()
    return false
  }
  if (slugState.value === 'taken') {
    alert.value = {
      type: 'warning',
      text: '这个地址已被占用，这篇文章还没有保存。换一个地址再保存即可。',
      at: Date.now(),
    }
    message.warning('这个地址已被占用，换一个再保存')
    return false
  }

  saving.value = true
  try {
    if (isNew.value) {
      const created = await createPost(payload())
      applyingForm = true
      postId.value = created.id
      status.value = created.status
      form.slug = created.slug
      slugState.value = 'ok'
      applyingForm = false
      dirty.value = false
      // The server now holds this article, so the local mirror is stale.
      clearDraft()
      // Replace so "back" leaves the editor instead of returning to a blank form.
      await router.replace({ name: 'admin-post-edit', params: { id: String(created.id) } })
      if (!options.silent) message.success('草稿已保存')
    } else {
      const updated = await updatePost(postId.value, payload())
      applyingForm = true
      status.value = updated.status
      form.slug = updated.slug
      slugState.value = 'ok'
      applyingForm = false
      dirty.value = false
      clearDraft()
      if (!options.silent) message.success('已保存')
    }
    if (options.silent) {
      // togglePublish reports the publish outcome right after this.
      alert.value = null
    } else {
      alert.value = {
        type: 'success',
        text: isPublished.value
          ? '已保存到服务器，发布状态不变。'
          : '已保存到服务器。草稿不会显示在前台，点“发布”才会出现在页面上。',
        at: Date.now(),
      }
    }
    return true
  } catch (caught) {
    const text = caught instanceof Error ? caught.message : '保存失败'
    alert.value = {
      type: 'error',
      text: `保存失败，文章没有写入服务器：${text}`,
      at: Date.now(),
    }
    message.error(text)
    return false
  } finally {
    saving.value = false
  }
}

/** Absolute in-app path of the public page for a slug. */
function publicHref(slug: string): string {
  return router.resolve({ name: 'post', params: { slug } }).href
}

async function togglePublish() {
  if (isNew.value || dirty.value) {
    const ok = await save({ silent: true })
    if (!ok) return
  }
  publishing.value = true
  try {
    const updated = isPublished.value
      ? await unpublishPost(postId.value)
      : await publishPost(postId.value)
    status.value = updated.status
    dirty.value = false
    clearDraft()
    // Report the status the server actually stored. Reading isPublished here
    // would be a read-after-write of the line above, inverting the message.
    if (updated.status === 'published') {
      message.success('已发布')
      alert.value = {
        type: 'success',
        text: '已发布，前台现在可以看到这篇文章了。',
        link: { href: publicHref(updated.slug || form.slug), label: '查看文章' },
        at: Date.now(),
      }
    } else {
      message.success('已转为草稿')
      alert.value = {
        type: 'info',
        text: '已转为草稿，前台不再显示这篇文章。',
        at: Date.now(),
      }
    }
  } catch (caught) {
    const text = caught instanceof Error ? caught.message : '操作失败'
    alert.value = {
      type: 'error',
      text: `发布失败，文章前台还看不到：${text}`,
      at: Date.now(),
    }
    message.error(text)
  } finally {
    publishing.value = false
  }
}

async function remove() {
  if (isNew.value) {
    void router.push({ name: 'admin-posts' })
    return
  }
  try {
    await deletePost(postId.value)
    clearDraft()
    alert.value = null
    message.success('已删除')
    void router.push({ name: 'admin-posts' })
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '删除失败')
  }
}

/**
 * Open the public page of this article.
 *
 * The public route only serves *published* posts, so previewing a draft used to
 * open a "not found" page — indistinguishable, from the author's side, from the
 * article having vanished. Say what is actually going on instead.
 */
function preview() {
  if (isNew.value || postId.value <= 0) {
    message.warning('先保存一次才有可预览的地址')
    return
  }
  if (!isPublished.value) {
    message.warning('文章还是草稿，发布之后才能在页面上打开')
    return
  }
  window.open(publicHref(form.slug), '_blank')
}

/** Blank the form; used when the editor is pointed at a brand-new post. */
function resetForm() {
  applyingForm = true
  form.title = ''
  form.slug = ''
  form.summary = ''
  form.coverUrl = ''
  form.contentHtml = ''
  form.contentJson = '{}'
  form.tagNames = []
  form.visibility = 'public'
  form.metaTitle = ''
  form.metaDescription = ''
  slugTouched.value = false
  slugState.value = 'idle'
  applyingForm = false
  dirty.value = false
}

async function load() {
  loading.value = true
  try {
    if (isNew.value) {
      resetForm()
      postId.value = 0
      status.value = 'draft'
      // An article that was interrupted before its first save lives only here.
      restoreDraft(0)
      return
    }
    const id = Number(route.params.id)
    if (!Number.isFinite(id) || id <= 0) {
      message.error('无效的文章 ID')
      void router.replace({ name: 'admin-posts' })
      return
    }
    const post = await getAdminPost(id)
    applyingForm = true
    postId.value = post.id
    status.value = post.status
    form.title = post.title
    form.slug = post.slug
    form.summary = post.summary
    form.coverUrl = post.coverUrl
    form.contentHtml = post.contentHtml
    form.contentJson = post.contentJson || '{}'
    form.tagNames = post.tags.map((tag) => tag.name)
    form.visibility = post.visibility
    form.metaTitle = post.metaTitle ?? ''
    form.metaDescription = post.metaDescription ?? ''
    slugTouched.value = true
    slugState.value = 'ok'
    applyingForm = false
    // Loading is not editing.
    dirty.value = false
    // A newer local copy means the last save never reached the server.
    restoreDraft(post.id)
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '加载失败')
  } finally {
    loading.value = false
  }
}

function onBeforeUnload(event: BeforeUnloadEvent) {
  if (!dirty.value) return
  event.preventDefault()
}

onBeforeRouteLeave(() => {
  if (!dirty.value) return true
  return window.confirm('有未保存的修改，确定离开吗?')
})

// The router reuses this component for /admin/posts/new and
// /admin/posts/:id/edit, so onMounted alone never re-runs when the editor is
// pointed at another post: switching between them used to leave the previous
// post's body on screen, or a blank form in front of one that does exist.
watch(
  () => `${String(route.name)}:${String(route.params.id ?? '')}`,
  () => {
    // save() replaces the URL with the post we already hold; reloading there
    // would only make the editor flicker.
    if (!isNew.value && Number(route.params.id) === postId.value) return
    void load()
  },
)

onMounted(async () => {
  window.addEventListener('beforeunload', onBeforeUnload)
  try {
    const result = await listTags()
    tagOptions.value = result.tags.map((tag) => ({ label: tag.name, value: tag.name }))
  } catch {
    // Tag suggestions are optional; free-form tags still work.
  }
  await load()
})

onBeforeUnmount(() => {
  window.clearTimeout(slugTimer)
  window.clearTimeout(draftTimer)
  // Keep the last unsaved state so returning to the editor can restore it.
  if (dirty.value) persistDraft()
  window.removeEventListener('beforeunload', onBeforeUnload)
})
</script>

<template>
  <div class="editor">
    <header class="editor__bar">
      <div class="editor__ident">
        <NInput
          ref="titleInput"
          v-model:value="form.title"
          class="editor__title"
          placeholder="标题"
          size="large"
          :status="titleEmpty ? 'error' : undefined"
        />
        <span class="editor__status" :class="{ 'is-live': isPublished }">
          {{ isPublished ? '已发布' : '草稿' }}
        </span>
        <span v-if="draftLabel" class="editor__draft">{{ draftLabel }}</span>
      </div>

      <div class="editor__actions">
        <span class="editor__count">{{ words }} 字 · 约 {{ minutes }} 分钟</span>

        <NButton size="small" secondary @click="preview">
          <template #icon><PhArrowSquareOut :size="16" /></template>
          预览
        </NButton>

        <NButton size="small" secondary :loading="saving" @click="save()">
          <template #icon><PhFloppyDisk :size="16" /></template>
          保存
        </NButton>

        <NButton
          size="small"
          :type="isPublished ? 'warning' : 'primary'"
          :loading="publishing"
          @click="togglePublish"
        >
          <template #icon>
            <component :is="isPublished ? PhXCircle : PhCheckCircle" :size="16" />
          </template>
          {{ isPublished ? '转草稿' : '发布' }}
        </NButton>

        <NPopconfirm @positive-click="remove">
          <template #trigger>
            <NButton size="small" quaternary type="error" :title="isNew ? '放弃' : '删除'">
              <template #icon><PhTrash :size="16" /></template>
            </NButton>
          </template>
          {{ isNew ? '放弃这篇草稿?' : '删除后无法恢复，确定吗?' }}
        </NPopconfirm>
      </div>
    </header>

    <NAlert
      v-if="alert"
      class="editor__alert"
      :type="alert.type"
      closable
      @close="alert = null"
    >
      <span class="editor__alert-text">{{ alert.text }}</span>
      <a
        v-if="alert.link"
        class="editor__alert-link"
        :href="alert.link.href"
        target="_blank"
        rel="noopener"
      >
        {{ alert.link.label }}
      </a>
      <NButton v-if="alert.action" size="tiny" secondary @click="alert.action.run()">
        {{ alert.action.label }}
      </NButton>
    </NAlert>

    <div v-if="loading" class="editor__loading">正在加载…</div>

    <div v-else class="editor__body">
      <div class="editor__main">
        <RichTextEditor
          v-model="form.contentHtml"
          min-height="540px"
          placeholder="从这里开始写…"
          @update:json="(json) => (form.contentJson = JSON.stringify(json))"
        />
      </div>

      <aside class="editor__side">
        <section class="panel">
          <h2 class="panel__title">发布</h2>

          <label class="field">
            <span class="field__label">可见性</span>
            <NSelect
              v-model:value="form.visibility"
              :options="[
                { label: '公开', value: 'public' },
                { label: '仅自己', value: 'private' },
              ]"
            />
          </label>

          <label class="field">
            <span class="field__label">地址</span>
            <NInput
              v-model:value="form.slug"
              placeholder="post-slug"
              @input="onSlugInput"
            />
            <span class="field__hint" :class="`is-${slugState}`">{{ slugHint }}</span>
          </label>

          <label class="field">
            <span class="field__label">标签</span>
            <NSelect
              v-model:value="form.tagNames"
              multiple
              filterable
              tag
              :options="tagOptions"
              placeholder="回车创建新标签"
            />
          </label>

          <label class="field">
            <span class="field__label">摘要</span>
            <NInput
              v-model:value="form.summary"
              type="textarea"
              :autosize="{ minRows: 3, maxRows: 6 }"
              placeholder="留空会自动从正文截取"
            />
          </label>
        </section>

        <section class="panel">
          <h2 class="panel__title">封面</h2>
          <label class="field">
            <span class="field__label">图片地址</span>
            <NInput v-model:value="form.coverUrl" placeholder="/uploads/… 或外部链接" />
          </label>
          <div v-if="form.coverUrl" class="cover">
            <img :src="form.coverUrl" alt="封面预览" />
          </div>
          <p class="panel__hint">在「媒体」页上传图片后，把地址复制到这里。</p>
        </section>

        <section class="panel">
          <h2 class="panel__title">SEO</h2>
          <label class="field">
            <span class="field__label">页面标题</span>
            <NInput v-model:value="form.metaTitle" placeholder="留空则用文章标题" />
          </label>
          <label class="field">
            <span class="field__label">页面描述</span>
            <NInput
              v-model:value="form.metaDescription"
              type="textarea"
              :autosize="{ minRows: 2, maxRows: 4 }"
              placeholder="搜索结果的摘要文字"
            />
          </label>
        </section>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.editor {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.editor__bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-md);
  padding: var(--space-md);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
}

.editor__ident {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  flex: 1;
  min-width: 240px;
}

.editor__title {
  max-width: 520px;
}

.editor__status {
  flex-shrink: 0;
  padding: 0.2em 0.6em;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-full);
  color: var(--text-muted);
  font-size: var(--text-2xs);
  letter-spacing: var(--tracking-wide);
}

.editor__status.is-live {
  border-color: var(--success);
  color: var(--success);
}

/* "saved locally, not on the server yet" — the difference that used to be
   invisible while writing. */
.editor__draft {
  flex-shrink: 0;
  color: var(--warning, var(--text-muted));
  font-size: var(--text-2xs);
  letter-spacing: var(--tracking-wide);
}

.editor__alert {
  margin-top: var(--space-md);
}

.editor__alert-text {
  margin-right: var(--space-sm);
}

.editor__alert-link {
  margin-right: var(--space-sm);
  color: inherit;
  font-weight: 600;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.editor__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-sm);
}

.editor__count {
  color: var(--text-muted);
  font-size: var(--text-2xs);
  font-variant-numeric: tabular-nums;
}

.editor__loading {
  padding: var(--space-3xl);
  color: var(--text-muted);
  text-align: center;
}

.editor__body {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: var(--space-md);
  align-items: start;
}

.editor__main {
  min-width: 0;
}

.editor__side {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  position: sticky;
  top: calc(var(--header-h) + var(--space-lg));
}

.panel {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  padding: var(--space-md);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
}

.panel__title {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-base);
}

.panel__hint {
  margin: 0;
  color: var(--text-muted);
  font-size: var(--text-2xs);
  line-height: var(--leading-normal);
}

.field__hint.is-taken {
  color: var(--danger);
}

.field__hint.is-ok {
  color: var(--success);
}

.cover {
  overflow: hidden;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  aspect-ratio: 16 / 9;
  background: var(--bg-inset);
}

.cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

@media (max-width: 1100px) {
  .editor__body {
    grid-template-columns: minmax(0, 1fr);
  }

  .editor__side {
    position: static;
  }
}
</style>
