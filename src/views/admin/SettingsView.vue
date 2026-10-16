<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { NButton, NInput, NSelect, useMessage } from 'naive-ui'
import { PhFloppyDisk, PhImage, PhPlus, PhTrash } from '@phosphor-icons/vue'
import { updateSite } from '@/api/site'
import { changePassword } from '@/api/auth'
import { useSite } from '@/composables/useSite'
import MediaPicker from '@/components/admin/MediaPicker.vue'
import WallpaperManager from '@/components/admin/WallpaperManager.vue'
import type { SocialLink } from '@/types/blog'

const message = useMessage()
const { site, setSite } = useSite()

const ICON_OPTIONS = [
  { label: 'GitHub', value: 'github' },
  { label: 'X', value: 'x' },
  { label: '邮箱', value: 'email' },
  { label: 'RSS', value: 'rss' },
  { label: '其他链接', value: 'link' },
]

const loading = ref(true)
const saving = ref(false)
const changing = ref(false)

const form = reactive({
  title: '',
  tagline: '',
  description: '',
  author: '',
  avatarUrl: '',
  footerNote: '',
})

const socials = ref<SocialLink[]>([])
const wallpapers = ref<string[]>([])
const avatarPickerOpen = ref(false)

const password = reactive({
  oldPassword: '',
  newPassword: '',
  confirm: '',
})

async function save() {
  saving.value = true
  try {
    const updated = await updateSite({
      title: form.title.trim(),
      tagline: form.tagline.trim(),
      description: form.description.trim(),
      author: form.author.trim(),
      avatarUrl: form.avatarUrl.trim(),
      footerNote: form.footerNote.trim(),
      wallpapers: wallpapers.value,
      socials: socials.value
        .map((link) => ({
          icon: link.icon,
          label: link.label.trim(),
          url: link.url.trim(),
        }))
        .filter((link) => link.url.length > 0),
    })
    setSite(updated)
    applySite(updated)
    message.success('已保存')
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '保存失败')
  } finally {
    saving.value = false
  }
}

function applySite(meta: typeof site.value) {
  if (!meta) return
  form.title = meta.title
  form.tagline = meta.tagline
  form.description = meta.description
  form.author = meta.author
  form.avatarUrl = meta.avatarUrl
  form.footerNote = meta.footerNote
  wallpapers.value = [...(meta.wallpapers ?? [])]
  socials.value = meta.socials.map((link) => ({ ...link }))
}

function addSocial() {
  socials.value.push({ icon: 'link', label: '', url: '' })
}

function removeSocial(index: number) {
  socials.value.splice(index, 1)
}

async function submitPassword() {
  if (!password.oldPassword || !password.newPassword) {
    message.warning('请填写当前密码和新密码')
    return
  }
  if (password.newPassword !== password.confirm) {
    message.warning('两次输入的新密码不一致')
    return
  }
  changing.value = true
  try {
    await changePassword(password.oldPassword, password.newPassword)
    password.oldPassword = ''
    password.newPassword = ''
    password.confirm = ''
    message.success('密码已更新')
  } catch (caught) {
    message.error(caught instanceof Error ? caught.message : '修改失败')
  } finally {
    changing.value = false
  }
}

onMounted(async () => {
  try {
    const { load } = useSite()
    await load()
  } finally {
    applySite(site.value)
    loading.value = false
  }
})
</script>

<template>
  <div class="settings">
    <p v-if="loading" class="state">正在加载…</p>

    <template v-else>
      <section class="card">
        <header class="card__head">
          <h2 class="card__title">站点信息</h2>
          <p class="card__hint">这些内容会出现在首页、页脚和浏览器标题里。</p>
        </header>

        <div class="form">
          <label class="field">
            <span class="field__label">站点名称</span>
            <NInput v-model:value="form.title" placeholder="bs-blog" />
          </label>

          <label class="field">
            <span class="field__label">一句话介绍</span>
            <NInput v-model:value="form.tagline" placeholder="写点东西" />
          </label>

          <label class="field">
            <span class="field__label">站点描述</span>
            <NInput
              v-model:value="form.description"
              type="textarea"
              :autosize="{ minRows: 2, maxRows: 4 }"
              placeholder="出现在关于页和搜索结果里"
            />
          </label>

          <div class="form__row">
            <label class="field">
              <span class="field__label">作者</span>
              <NInput v-model:value="form.author" placeholder="怎么称呼你" />
            </label>

            <label class="field">
              <span class="field__label">头像地址</span>
              <NInput v-model:value="form.avatarUrl" placeholder="/uploads/… 或外部链接" />
              <NButton size="tiny" secondary class="field__action" @click="avatarPickerOpen = true">
                <template #icon><PhImage :size="14" /></template>
                从媒体库选择
              </NButton>
            </label>
          </div>

          <label class="field">
            <span class="field__label">页脚说明</span>
            <NInput v-model:value="form.footerNote" placeholder="留空则用站点描述" />
          </label>
        </div>
      </section>

      <section class="card">
        <header class="card__head">
          <h2 class="card__title">社交链接</h2>
          <p class="card__hint">
            只支持 http/https 链接，其他协议会被服务端丢弃。
          </p>
        </header>

        <div v-if="socials.length > 0" class="socials">
          <div v-for="(link, index) in socials" :key="index" class="socials__row">
            <NSelect v-model:value="link.icon" class="socials__icon" :options="ICON_OPTIONS" />
            <NInput v-model:value="link.label" placeholder="显示名称" />
            <NInput v-model:value="link.url" placeholder="https://…" />
            <NButton quaternary type="error" title="删除" @click="removeSocial(index)">
              <template #icon><PhTrash :size="16" /></template>
            </NButton>
          </div>
        </div>
        <p v-else class="card__hint">还没有添加任何链接。</p>

        <NButton secondary size="small" @click="addSocial">
          <template #icon><PhPlus :size="16" /></template>
          添加链接
        </NButton>
      </section>

      <section class="card">
        <header class="card__head">
          <h2 class="card__title">壁纸</h2>
          <p class="card__hint">
            加到这里的图片会变成整站背景，每次打开或刷新随机显示其中一张。留空则使用默认的极光背景。
          </p>
        </header>

        <WallpaperManager v-model="wallpapers" />
      </section>

      <div class="save-bar">
        <NButton type="primary" :loading="saving" @click="save">
          <template #icon><PhFloppyDisk :size="16" /></template>
          保存站点信息
        </NButton>
      </div>

      <section class="card">
        <header class="card__head">
          <h2 class="card__title">修改密码</h2>
          <p class="card__hint">改完之后其他设备上的登录会立即失效。</p>
        </header>

        <div class="form">
          <label class="field">
            <span class="field__label">当前密码</span>
            <NInput
              v-model:value="password.oldPassword"
              type="password"
              show-password-on="click"
              placeholder="当前使用的密码"
            />
          </label>

          <div class="form__row">
            <label class="field">
              <span class="field__label">新密码</span>
              <NInput
                v-model:value="password.newPassword"
                type="password"
                show-password-on="click"
                placeholder="至少 8 位"
              />
            </label>

            <label class="field">
              <span class="field__label">再输一次</span>
              <NInput
                v-model:value="password.confirm"
                type="password"
                show-password-on="click"
                placeholder="确认新密码"
              />
            </label>
          </div>

          <NButton secondary :loading="changing" @click="submitPassword">更新密码</NButton>
        </div>
      </section>

      <MediaPicker
        v-model:show="avatarPickerOpen"
        title="选择头像"
        hint="头像会显示在页头和关于页。"
        @select="form.avatarUrl = $event[0] ?? ''"
      />
    </template>
  </div>
</template>

<style scoped>
.settings {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  max-width: 860px;
}

.card {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  padding: var(--space-lg);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
}

.card__head {
  display: flex;
  flex-direction: column;
  gap: var(--space-2xs);
}

.card__title {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-lg);
}

.card__hint {
  margin: 0;
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.form {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}

.form__row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-md);
}

.field__action {
  align-self: flex-start;
  margin-top: var(--space-2xs);
}

.socials {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
}

.socials__row {
  display: grid;
  grid-template-columns: 120px minmax(0, 1fr) minmax(0, 1.4fr) 40px;
  gap: var(--space-sm);
  align-items: center;
}

.socials__icon {
  width: 120px;
}

.save-bar {
  display: flex;
  justify-content: flex-end;
}

.state {
  padding: var(--space-3xl);
  color: var(--text-muted);
  font-size: var(--text-sm);
  text-align: center;
}

@media (max-width: 720px) {
  .form__row {
    grid-template-columns: minmax(0, 1fr);
  }

  .socials__row {
    grid-template-columns: minmax(0, 1fr);
  }

  .socials__icon {
    width: 100%;
  }
}
</style>
