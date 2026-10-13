<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { NButton, NForm, NFormItem, NInput, useMessage } from 'naive-ui'
import { PhLockKey, PhWarningCircle } from '@phosphor-icons/vue'
import { useRoute, useRouter } from 'vue-router'
import { setupState } from '@/api/auth'
import { useAuth } from '@/stores/auth'
import ShaderBackdrop from '@/components/visual/ShaderBackdrop.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const { signIn, claim } = useAuth()

/** 'loading' while we ask the server whether an owner exists yet. */
const mode = ref<'loading' | 'login' | 'setup'>('loading')
const submitting = ref(false)
const errorText = ref('')

const form = reactive({
  username: '',
  password: '',
  confirm: '',
})

const isSetup = () => mode.value === 'setup'

function redirectTarget() {
  const raw = route.query.redirect
  return typeof raw === 'string' && raw.startsWith('/') ? raw : ''
}

async function submit() {
  errorText.value = ''

  if (!form.username.trim()) {
    errorText.value = '请输入用户名'
    return
  }
  if (!form.password) {
    errorText.value = '请输入密码'
    return
  }
  if (isSetup() && form.password !== form.confirm) {
    errorText.value = '两次输入的密码不一致'
    return
  }

  submitting.value = true
  try {
    if (isSetup()) {
      // Must go through the store: the raw API call returns a token but never
      // stores it, so the auth guard would bounce us straight back here.
      await claim(form.username.trim(), form.password)
      message.success('创建完成，已自动登录')
    } else {
      await signIn(form.username.trim(), form.password)
    }
    await router.replace(redirectTarget() || { name: 'admin-posts' })
  } catch (caught) {
    errorText.value = caught instanceof Error ? caught.message : '登录失败'
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  try {
    const state = await setupState()
    mode.value = state.initialised ? 'login' : 'setup'
  } catch {
    // If the API is unreachable, showing the login form is the least confusing
    // fallback: the error surfaces when the user actually submits.
    mode.value = 'login'
  }
})
</script>

<template>
  <div class="login">
    <ShaderBackdrop class="login__backdrop" :intensity="0.9" />

    <div class="login__aside">
      <RouterLink class="login__brand" :to="{ name: 'home' }">
        <span class="login__mark" aria-hidden="true" />
        bs-blog
      </RouterLink>

      <div class="login__pitch">
        <p class="eyebrow">{{ isSetup() ? '首次使用' : '仅限本人' }}</p>
        <h1 class="login__title">
          {{ isSetup() ? '先给这个博客上把锁' : '欢迎回来' }}
        </h1>
        <p class="login__text">
          {{
            isSetup()
              ? '创建一个管理员账号。之后写文章、发文章、传图片，都要从这里进来。'
              : '登录后可以写作、发布、管理媒体与站点信息。'
          }}
        </p>
      </div>

      <ThemeToggle />
    </div>

    <div class="login__panel">
      <div class="login__card">
        <p v-if="mode === 'loading'" class="login__loading">正在检查站点状态…</p>

        <NForm v-else :show-label="true" @submit.prevent="submit">
          <NFormItem label="用户名" path="username">
            <NInput
              v-model:value="form.username"
              placeholder="怎么称呼你"
              :input-props="{ autocomplete: 'username', autocapitalize: 'off' }"
              @keydown.enter="submit"
            />
          </NFormItem>

          <NFormItem label="密码" path="password">
            <NInput
              v-model:value="form.password"
              type="password"
              show-password-on="click"
              placeholder="至少 8 位"
              :input-props="{
                autocomplete: isSetup() ? 'new-password' : 'current-password',
              }"
              @keydown.enter="submit"
            />
          </NFormItem>

          <NFormItem v-if="isSetup()" label="再输一次" path="confirm">
            <NInput
              v-model:value="form.confirm"
              type="password"
              show-password-on="click"
              placeholder="确认密码"
              :input-props="{ autocomplete: 'new-password' }"
              @keydown.enter="submit"
            />
          </NFormItem>

          <p v-if="errorText" class="login__error">
            <PhWarningCircle :size="16" weight="bold" />
            {{ errorText }}
          </p>

          <NButton
            class="login__submit"
            type="primary"
            block
            size="large"
            :loading="submitting"
            attr-type="submit"
            @click="submit"
          >
            <template #icon>
              <PhLockKey :size="18" weight="bold" />
            </template>
            {{ isSetup() ? '创建并进入' : '登录' }}
          </NButton>
        </NForm>

        <RouterLink class="login__back" :to="{ name: 'home' }">← 回到博客</RouterLink>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login {
  position: relative;
  display: grid;
  grid-template-columns: 1.1fr 1fr;
  min-height: 100vh;
  background: var(--bg-base);
}

.login__backdrop {
  position: absolute;
  inset: 0;
  z-index: 0;
  opacity: 0.85;
}

.login__aside {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  gap: var(--space-xl);
  padding: var(--space-2xl) var(--space-3xl);
}

.login__brand {
  display: inline-flex;
  align-items: center;
  gap: var(--space-sm);
  align-self: flex-start;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-lg);
  text-decoration: none;
}

.login__mark {
  width: 10px;
  height: 10px;
  border-radius: 3px;
  background: var(--accent);
  box-shadow: 0 0 12px var(--glow-accent);
}

.login__pitch {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
  max-width: 34ch;
  margin: auto 0;
}

.login__title {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-4xl);
  font-weight: 600;
  letter-spacing: var(--tracking-tight);
  line-height: var(--leading-tight);
}

.login__text {
  margin: 0;
  color: var(--text-secondary);
  line-height: var(--leading-relaxed);
}

.login__panel {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-2xl);
  background: color-mix(in srgb, var(--bg-elevated) 82%, transparent);
  backdrop-filter: blur(18px);
  border-left: 1px solid var(--border-subtle);
}

.login__card {
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
  width: 100%;
  max-width: 360px;
}

.login__loading {
  color: var(--text-muted);
  font-size: var(--text-sm);
}

.login__error {
  display: flex;
  align-items: center;
  gap: var(--space-xs);
  margin: 0 0 var(--space-md);
  color: var(--danger);
  font-size: var(--text-sm);
}

.login__submit {
  margin-top: var(--space-sm);
}

.login__back {
  color: var(--text-muted);
  font-size: var(--text-xs);
  text-decoration: none;
}

.login__back:hover {
  color: var(--accent);
}

@media (max-width: 900px) {
  .login {
    grid-template-columns: minmax(0, 1fr);
  }

  .login__aside {
    padding: var(--space-xl) var(--space-lg) 0;
  }

  .login__pitch {
    margin: var(--space-xl) 0 0;
  }

  .login__panel {
    border-left: none;
    padding: var(--space-xl) var(--space-lg) var(--space-3xl);
  }
}
</style>
