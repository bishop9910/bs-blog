<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { NButton, NInput, NPopconfirm, NSelect, useMessage } from 'naive-ui'
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

// Any edit at all means there is something to lose on navigation.
watch(form, () => {
  dirty.value = true
})

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
    message.warning('标题不能为空')
    return false
  }
  if (slugState.value === 'taken') {
    message.warning('这个地址已被占用，换一个再保存')
    return false
  }

  saving.value = true
  try {
    if (isNew.value) {
      const created = await createPost(payload())
      postId.value = created.id
      status.value = created.status
      form.slug = created.slug
      slugState.value = 'ok'
      dirty.value = false
      // Replace so "back" leaves the editor instead of returning to a blank form.
      await router.replace({ name: 'admin-post-edit', params: { id: String(created.id) } })
      if (!options.silent) message.success('草稿已保存')
    } else {
      const updated = await updatePost(postId.value, payload())
      status.value = updated.status
      form.slug = updated.slug
      slugState.value = 'ok'
      dirty.value = false
      if (!options.silent) message.success('已保存')
    }
    return true
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '保存失败')
    return false
  } finally {
    saving.value = false
  }
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
    message.success(isPublished.value ? '已转为草稿' : '已发布')
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '操作失败')
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
    message.success('已删除')
    void router.push({ name: 'admin-posts' })
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '删除失败')
  }
}

function preview() {
  if (!form.slug.trim()) {
    message.warning('先保存一次才有可预览的地址')
    return
  }
  window.open(router.resolve({ name: 'post', params: { slug: form.slug } }).href, '_blank')
}

async function load() {
  loading.value = true
  try {
    if (isNew.value) {
      postId.value = 0
      status.value = 'draft'
      return
    }
    const id = Number(route.params.id)
    if (!Number.isFinite(id) || id <= 0) {
      message.error('无效的文章 ID')
      void router.replace({ name: 'admin-posts' })
      return
    }
    const post = await getAdminPost(id)
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
    // Loading is not editing: clear the flag on the next tick.
    await Promise.resolve()
    dirty.value = false
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
  window.removeEventListener('beforeunload', onBeforeUnload)
})
</script>

<template>
  <div class="editor">
    <header class="editor__bar">
      <div class="editor__ident">
        <NInput
          v-model:value="form.title"
          class="editor__title"
          placeholder="标题"
          size="large"
        />
        <span class="editor__status" :class="{ 'is-live': isPublished }">
          {{ isPublished ? '已发布' : '草稿' }}
        </span>
      </div>

      <div class="editor__actions">
        <span class="editor__count">{{ words }} 字 · 约 {{ minutes }} 分钟</span>

        <NButton size="small" secondary :disabled="!form.slug" @click="preview">
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
