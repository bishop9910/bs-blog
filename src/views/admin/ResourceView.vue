<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import {
  NButton,
  NInput,
  NModal,
  NPopconfirm,
  NProgress,
  NSpace,
  NSwitch,
  NTag,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import {
  PhArrowSquareOut,
  PhCopy,
  PhPencilSimple,
  PhTrash,
  PhUploadSimple,
} from '@phosphor-icons/vue'
import { useRouter } from 'vue-router'
import {
  deleteResource,
  listResources,
  resourceDownloadUrl,
  updateResource,
  uploadResource,
} from '@/api/resources'
import type { ResourceFile } from '@/types/resource'
import { useLimits } from '@/composables/useLimits'
import { describeExtensions, describeSizeLimit } from '@/utils/limits'
import { formatBytes, formatCount, formatDate } from '@/utils/format'
import DataTable from '@/components/admin/DataTable.vue'

const PAGE_SIZE = 10

const router = useRouter()
const message = useMessage()

const { limits, load: loadLimits } = useLimits()
const acceptAttr = computed(() =>
  limits.value.resources.allowedExtensions.map((ext) => `.${ext}`).join(','),
)
const dropHint = computed(() => {
  const types = describeExtensions(limits.value.resources.allowedExtensions)
  const size = describeSizeLimit(limits.value.resources.maxUploadBytes)
  return types ? `支持 ${types}，${size}` : size
})

const rows = ref<ResourceFile[]>([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const keyword = ref('')

const fileInput = ref<HTMLInputElement | null>(null)
const dragOver = ref(false)
const pending = ref<File | null>(null)
const uploading = ref(false)
const progress = ref(0)

// Metadata for the file waiting in the drop zone. Filled in after a pick, never
// before: the title defaults to the filename so the fast path is one click.
const draft = reactive({
  title: '',
  description: '',
  coverUrl: '',
  visible: true,
})

const editing = ref<ResourceFile | null>(null)
const savingEdit = ref(false)
const editForm = reactive({
  title: '',
  description: '',
  coverUrl: '',
  visible: true,
})

async function load() {
  loading.value = true
  try {
    const result = await listResources({
      page: page.value,
      pageSize: PAGE_SIZE,
      keyword: keyword.value.trim() || undefined,
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

function search() {
  page.value = 1
  void load()
}

/** "project-v2.zip" -> "project-v2" — the extension is noise in a title. */
function titleFromFilename(name: string): string {
  return name.replace(/\.[^./\\]+$/, '')
}

function pickFiles(files: FileList | null) {
  if (!files || files.length === 0) return
  const file = files[0]
  if (!file) return
  if (files.length > 1) message.info('一次上传一个文件，其余文件请逐个添加')

  // Reject locally what the server would reject anyway: pushing 200 MB through
  // the network only to read "单文件不超过 32 MB" wastes the transfer.
  if (file.size > limits.value.resources.maxUploadBytes) {
    message.warning(`${file.name} 超过${describeSizeLimit(limits.value.resources.maxUploadBytes)}`)
    return
  }

  pending.value = file
  draft.title = titleFromFilename(file.name)
  draft.description = ''
  draft.coverUrl = ''
  draft.visible = true
  progress.value = 0
}

function onDrop(event: DragEvent) {
  dragOver.value = false
  pickFiles(event.dataTransfer?.files ?? null)
}

function clearPending() {
  pending.value = null
  progress.value = 0
  draft.title = ''
  draft.description = ''
  draft.coverUrl = ''
  draft.visible = true
  if (fileInput.value) fileInput.value.value = ''
}

async function submitUpload() {
  const file = pending.value
  if (!file) return

  uploading.value = true
  progress.value = 0
  try {
    await uploadResource(
      file,
      {
        title: draft.title.trim(),
        description: draft.description.trim(),
        coverUrl: draft.coverUrl.trim(),
        visible: draft.visible,
      },
      (percent) => {
        progress.value = percent
      },
    )
    message.success('已上传')
    clearPending()
    page.value = 1
    await load()
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '上传失败')
  } finally {
    uploading.value = false
    progress.value = 0
  }
}

async function copyLink(item: ResourceFile) {
  const absolute = new URL(resourceDownloadUrl(item.id), window.location.origin).href
  try {
    await navigator.clipboard.writeText(absolute)
    message.success('下载地址已复制')
  } catch {
    message.warning('浏览器拒绝了剪贴板访问，请手动复制')
  }
}

/** One-click publish/hide, the same shortcut the post list offers. */
async function toggleVisible(item: ResourceFile) {
  try {
    const updated = await updateResource(item.id, {
      title: item.title,
      description: item.description,
      coverUrl: item.coverUrl,
      visible: !item.visible,
    })
    replaceRow(updated)
    message.success(updated.visible ? '已在资源页公开' : '已从资源页隐藏')
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '操作失败')
  }
}

function replaceRow(updated: ResourceFile) {
  const index = rows.value.findIndex((row) => row.id === updated.id)
  if (index >= 0) rows.value[index] = updated
}

function openEdit(item: ResourceFile) {
  editing.value = item
  editForm.title = item.title
  editForm.description = item.description
  editForm.coverUrl = item.coverUrl
  editForm.visible = item.visible
}

async function saveEdit() {
  const item = editing.value
  if (!item) return

  savingEdit.value = true
  try {
    const updated = await updateResource(item.id, {
      title: editForm.title.trim(),
      description: editForm.description.trim(),
      coverUrl: editForm.coverUrl.trim(),
      visible: editForm.visible,
    })
    replaceRow(updated)
    editing.value = null
    message.success('已保存')
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '保存失败')
  } finally {
    savingEdit.value = false
  }
}

async function remove(item: ResourceFile) {
  try {
    await deleteResource(item.id)
    message.success('已删除')
    // Stepping back a page when the last row disappears avoids an empty view.
    if (rows.value.length === 1 && page.value > 1) page.value -= 1
    await load()
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '删除失败')
  }
}

const columns: DataTableColumns<ResourceFile> = [
  {
    title: '文件',
    key: 'title',
    minWidth: 260,
    render: (item) =>
      h('div', { class: 'cell-title' }, [
        h('span', { class: 'cell-title__link cell-title__link--static' }, item.title || item.filename),
        h('span', { class: 'cell-title__slug' }, item.filename),
      ]),
  },
  {
    title: '大小',
    key: 'size',
    width: 100,
    render: (item) => h('span', { class: 'cell-num' }, formatBytes(item.size)),
  },
  {
    title: '下载',
    key: 'downloadCount',
    width: 90,
    render: (item) => h('span', { class: 'cell-num' }, formatCount(item.downloadCount)),
  },
  {
    title: '状态',
    key: 'visible',
    width: 100,
    render: (item) =>
      h(
        NTag,
        { size: 'small', type: item.visible ? 'success' : 'default', bordered: false },
        { default: () => (item.visible ? '公开' : '已隐藏') },
      ),
  },
  {
    title: '更新',
    key: 'updatedAt',
    width: 120,
    render: (item) => h('span', { class: 'cell-muted' }, formatDate(item.updatedAt)),
  },
  {
    title: '操作',
    key: 'actions',
    width: 200,
    align: 'right',
    render: (item) =>
      h(NSpace, { size: 4, justify: 'end' }, {
        default: () => [
          h(
            NButton,
            { size: 'tiny', quaternary: true, title: '复制下载地址', onClick: () => void copyLink(item) },
            { icon: () => h(PhCopy, { size: 15 }) },
          ),
          h(
            NButton,
            {
              size: 'tiny',
              quaternary: true,
              type: item.visible ? 'warning' : 'primary',
              onClick: () => void toggleVisible(item),
            },
            { default: () => (item.visible ? '隐藏' : '公开') },
          ),
          h(
            NButton,
            { size: 'tiny', quaternary: true, title: '编辑', onClick: () => openEdit(item) },
            { icon: () => h(PhPencilSimple, { size: 15 }) },
          ),
          h(
            NPopconfirm,
            { onPositiveClick: () => remove(item) },
            {
              trigger: () =>
                h(
                  NButton,
                  { size: 'tiny', quaternary: true, type: 'error', title: '删除' },
                  { icon: () => h(PhTrash, { size: 15 }) },
                ),
              default: () => '文件与记录都会被删除，确定吗?',
            },
          ),
        ],
      }),
  },
]

onMounted(() => {
  void load()
  void loadLimits()
})
</script>

<template>
  <div class="resources">
    <header class="toolbar">
      <div class="toolbar__search">
        <NInput
          v-model:value="keyword"
          clearable
          placeholder="按标题或文件名搜索"
          @keydown.enter="search"
          @clear="search"
        />
        <NButton @click="search">搜索</NButton>
      </div>
      <span class="toolbar__count">共 {{ total }} 个文件</span>
      <NButton quaternary @click="router.push({ name: 'resources' })">
        <template #icon><PhArrowSquareOut :size="16" /></template>
        资源页
      </NButton>
    </header>

    <section class="upload">
      <div
        class="drop"
        :class="{ 'is-over': dragOver, 'is-busy': uploading }"
        @dragover.prevent="dragOver = true"
        @dragleave.prevent="dragOver = false"
        @drop.prevent="onDrop"
      >
        <input
          ref="fileInput"
          class="drop__input"
          type="file"
          :accept="acceptAttr"
          @change="pickFiles(($event.target as HTMLInputElement).files)"
        />
        <template v-if="!pending">
          <PhUploadSimple :size="28" weight="bold" />
          <p class="drop__text">
            把文件拖到这里，或<button class="drop__pick" type="button" @click="fileInput?.click()">
              选择文件
            </button>
          </p>
          <p class="drop__hint">{{ dropHint }}</p>
        </template>
        <template v-else>
          <p class="drop__file">{{ pending.name }}</p>
          <p class="drop__hint">{{ formatBytes(pending.size) }}</p>
          <NProgress
            v-if="uploading"
            class="drop__progress"
            type="line"
            :percentage="progress"
            :show-indicator="false"
            processing
          />
        </template>
      </div>

      <div v-if="pending" class="meta">
        <label class="field">
          <span class="field__label">标题</span>
          <NInput v-model:value="draft.title" placeholder="留空则使用文件名" maxlength="160" />
        </label>
        <label class="field">
          <span class="field__label">简介</span>
          <NInput
            v-model:value="draft.description"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 4 }"
            placeholder="这个项目是什么、怎么用（可选）"
            maxlength="2000"
          />
        </label>
        <label class="field">
          <span class="field__label">封面地址</span>
          <NInput v-model:value="draft.coverUrl" placeholder="可选，媒体库图片的地址" maxlength="512" />
        </label>
        <div class="meta__row">
          <label class="switch">
            <NSwitch v-model:value="draft.visible" />
            <span>在资源页公开</span>
          </label>
          <div class="meta__actions">
            <NButton quaternary :disabled="uploading" @click="clearPending">取消</NButton>
            <NButton type="primary" :loading="uploading" @click="submitUpload">上传</NButton>
          </div>
        </div>
      </div>
    </section>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :page="page"
      :page-size="PAGE_SIZE"
      :total="total"
      empty-text="还没有上传任何文件"
      @update:page="
        (value) => {
          page = value
          load()
        }
      "
    />

    <NModal
      :show="editing !== null"
      preset="card"
      title="编辑资源"
      class="edit-modal"
      :bordered="false"
      @update:show="(value) => { if (!value) editing = null }"
    >
      <div class="edit">
        <label class="field">
          <span class="field__label">标题</span>
          <NInput v-model:value="editForm.title" maxlength="160" />
        </label>
        <label class="field">
          <span class="field__label">简介</span>
          <NInput
            v-model:value="editForm.description"
            type="textarea"
            :autosize="{ minRows: 2, maxRows: 6 }"
            maxlength="2000"
          />
        </label>
        <label class="field">
          <span class="field__label">封面地址</span>
          <NInput v-model:value="editForm.coverUrl" maxlength="512" />
        </label>
        <label class="switch">
          <NSwitch v-model:value="editForm.visible" />
          <span>在资源页公开</span>
        </label>
        <p class="edit__hint">文件名和大小由上传时的文件决定，不能在这里修改。</p>
      </div>
      <template #footer>
        <div class="edit__footer">
          <NButton quaternary @click="editing = null">取消</NButton>
          <NButton type="primary" :loading="savingEdit" @click="saveEdit">保存</NButton>
        </div>
      </template>
    </NModal>
  </div>
</template>

<style scoped>
.resources {
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

.toolbar__search {
  display: flex;
  gap: var(--space-sm);
  min-width: 280px;
}

.toolbar__count {
  margin-left: auto;
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.upload {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.drop {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-xs);
  padding: var(--space-2xl) var(--space-lg);
  border: 1px dashed var(--border-strong);
  border-radius: var(--radius-lg);
  background: var(--bg-subtle);
  color: var(--text-muted);
  text-align: center;
  transition:
    border-color var(--dur-fast) var(--ease-out),
    background var(--dur-fast) var(--ease-out);
}

.drop.is-over {
  border-color: var(--accent);
  background: var(--accent-subtle);
  color: var(--accent);
}

.drop.is-busy {
  pointer-events: none;
  opacity: 0.8;
}

.drop__input {
  display: none;
}

.drop__text {
  margin: 0;
  color: var(--text-secondary);
  font-size: var(--text-sm);
}

.drop__pick {
  padding: 0;
  border: none;
  background: none;
  color: var(--accent);
  font: inherit;
  text-decoration: underline;
  cursor: pointer;
}

.drop__hint {
  margin: 0;
  font-size: var(--text-2xs);
}

.drop__file {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-mono);
  font-size: var(--text-sm);
  word-break: break-all;
}

.drop__progress {
  width: min(320px, 100%);
  margin-top: var(--space-sm);
}

.meta {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  padding: var(--space-lg);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
}

.meta__row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-md);
}

.meta__actions {
  display: flex;
  gap: var(--space-sm);
}

.switch {
  display: flex;
  align-items: center;
  gap: var(--space-sm);
  color: var(--text-secondary);
  font-size: var(--text-sm);
  cursor: pointer;
}

.edit {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.edit__hint {
  margin: 0;
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.edit__footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-sm);
}
</style>
