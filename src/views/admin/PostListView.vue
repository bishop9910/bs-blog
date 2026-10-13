<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import { NButton, NInput, NPopconfirm, NSpace, NTag, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { PhArrowSquareOut, PhPencilSimple, PhTrash } from '@phosphor-icons/vue'
import { useRouter } from 'vue-router'
import {
  deletePost,
  listAdminPosts,
  publishPost,
  unpublishPost,
} from '@/api/posts'
import type { PostSummary, PostStatus } from '@/types/blog'
import DataTable from '@/components/admin/DataTable.vue'
import { formatDate } from '@/utils/format'

const router = useRouter()
const message = useMessage()

const FILTERS: { label: string; value: PostStatus | 'all' }[] = [
  { label: '全部', value: 'all' },
  { label: '草稿', value: 'draft' },
  { label: '已发布', value: 'published' },
]

const rows = ref<PostSummary[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const status = ref<PostStatus | 'all'>('all')
const keyword = ref('')
const busyId = ref(0)

async function load() {
  loading.value = true
  try {
    const result = await listAdminPosts({
      page: page.value,
      pageSize: pageSize.value,
      status: status.value,
      keyword: keyword.value.trim() || undefined,
      sort: 'updated',
      order: 'desc',
    })
    rows.value = result.items
    total.value = result.total
    page.value = result.page
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '加载失败')
    rows.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function applyFilter(value: PostStatus | 'all') {
  status.value = value
  page.value = 1
  void load()
}

function search() {
  page.value = 1
  void load()
}

async function togglePublish(post: PostSummary) {
  busyId.value = post.id
  try {
    const updated =
      post.status === 'published' ? await unpublishPost(post.id) : await publishPost(post.id)
    post.status = updated.status
    message.success(updated.status === 'published' ? '已发布' : '已转为草稿')
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '操作失败')
  } finally {
    busyId.value = 0
  }
}

async function remove(post: PostSummary) {
  busyId.value = post.id
  try {
    await deletePost(post.id)
    message.success('已删除')
    // Stepping back a page when the last row disappears avoids an empty view.
    if (rows.value.length === 1 && page.value > 1) page.value -= 1
    await load()
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '删除失败')
  } finally {
    busyId.value = 0
  }
}

const columns: DataTableColumns<PostSummary> = [
  {
    title: '标题',
    key: 'title',
    minWidth: 260,
    render: (post) =>
      h('div', { class: 'cell-title' }, [
        h(
          'button',
          {
            class: 'cell-title__link',
            type: 'button',
            onClick: () => router.push({ name: 'admin-post-edit', params: { id: String(post.id) } }),
          },
          post.title,
        ),
        h('span', { class: 'cell-title__slug' }, `/post/${post.slug}`),
      ]),
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render: (post) =>
      h(
        NTag,
        {
          size: 'small',
          type: post.status === 'published' ? 'success' : 'default',
          bordered: false,
        },
        { default: () => (post.status === 'published' ? '已发布' : '草稿') },
      ),
  },
  {
    title: '标签',
    key: 'tags',
    width: 180,
    render: (post) =>
      post.tags.length === 0
        ? h('span', { class: 'cell-muted' }, '—')
        : h(
            NSpace,
            { size: 4 },
            { default: () => post.tags.map((tag) => h(NTag, { size: 'tiny', bordered: false }, { default: () => tag.name })) },
          ),
  },
  {
    title: '字数',
    key: 'wordCount',
    width: 90,
    render: (post) => h('span', { class: 'cell-num' }, String(post.wordCount)),
  },
  {
    title: '浏览',
    key: 'viewCount',
    width: 90,
    render: (post) => h('span', { class: 'cell-num' }, String(post.viewCount)),
  },
  {
    title: '更新',
    key: 'updatedAt',
    width: 120,
    render: (post) => h('span', { class: 'cell-muted' }, formatDate(post.updatedAt)),
  },
  {
    title: '操作',
    key: 'actions',
    width: 200,
    align: 'right',
    render: (post) =>
      h(NSpace, { size: 4, justify: 'end' }, {
        default: () => [
          h(
            NButton,
            {
              size: 'tiny',
              quaternary: true,
              title: '编辑',
              onClick: () =>
                router.push({ name: 'admin-post-edit', params: { id: String(post.id) } }),
            },
            { icon: () => h(PhPencilSimple, { size: 15 }) },
          ),
          h(
            NButton,
            {
              size: 'tiny',
              quaternary: true,
              type: post.status === 'published' ? 'warning' : 'primary',
              loading: busyId.value === post.id,
              onClick: () => void togglePublish(post),
            },
            { default: () => (post.status === 'published' ? '转草稿' : '发布') },
          ),
          h(
            NButton,
            {
              size: 'tiny',
              quaternary: true,
              title: '在站点中打开',
              onClick: () =>
                window.open(
                  router.resolve({ name: 'post', params: { slug: post.slug } }).href,
                  '_blank',
                ),
            },
            { icon: () => h(PhArrowSquareOut, { size: 15 }) },
          ),
          h(
            NPopconfirm,
            { onPositiveClick: () => remove(post) },
            {
              trigger: () =>
                h(
                  NButton,
                  { size: 'tiny', quaternary: true, type: 'error', title: '删除' },
                  { icon: () => h(PhTrash, { size: 15 }) },
                ),
              default: () => '删除后无法恢复，确定吗?',
            },
          ),
        ],
      }),
  },
]

onMounted(load)
</script>

<template>
  <div class="admin-posts">
    <header class="toolbar">
      <div class="toolbar__filters">
        <button
          v-for="item in FILTERS"
          :key="item.value"
          class="chip"
          :class="{ 'is-active': status === item.value }"
          type="button"
          @click="applyFilter(item.value)"
        >
          {{ item.label }}
        </button>
      </div>

      <div class="toolbar__search">
        <NInput
          v-model:value="keyword"
          clearable
          placeholder="搜索标题、摘要或正文"
          @keydown.enter="search"
          @clear="search"
        />
        <NButton @click="search">搜索</NButton>
      </div>
    </header>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :page="page"
      :page-size="pageSize"
      :total="total"
      @update:page="
        (value) => {
          page = value
          load()
        }
      "
    />
  </div>
</template>

<style scoped>
.admin-posts {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-md);
  padding: var(--space-md);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
}

.toolbar__filters {
  display: flex;
  gap: var(--space-xs);
}

.chip {
  padding: 0.35em 0.9em;
  border: 1px solid transparent;
  border-radius: var(--radius-full);
  background: transparent;
  color: var(--text-secondary);
  font: inherit;
  font-size: var(--text-xs);
  cursor: pointer;
}

.chip:hover {
  color: var(--accent);
}

.chip.is-active {
  border-color: var(--accent);
  background: var(--accent-subtle);
  color: var(--accent);
}

.toolbar__search {
  display: flex;
  gap: var(--space-sm);
  margin-left: auto;
  min-width: 280px;
}
</style>
