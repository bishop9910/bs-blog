<script setup lang="ts">
/**
 * Application shell: theme bridging to naive-ui + global providers.
 *
 * Note on `n-global-style`: intentionally NOT used. It writes body colours from
 * naive-ui's own theme variables, which would fight the design tokens in
 * tokens.css. Our `base.css` owns the document surface instead.
 */
import { computed } from 'vue'
import {
  NConfigProvider,
  NDialogProvider,
  NLoadingBarProvider,
  NMessageProvider,
  NNotificationProvider,
  dateZhCN,
  darkTheme,
  zhCN,
} from 'naive-ui'
import { useTheme } from '@/composables/useTheme'
import { darkOverrides, lightOverrides } from '@/styles/naive-theme'

const { effective } = useTheme()

const theme = computed(() => (effective.value === 'dark' ? darkTheme : null))
const overrides = computed(() => (effective.value === 'dark' ? darkOverrides : lightOverrides))
</script>

<template>
  <n-config-provider
    :theme="theme"
    :theme-overrides="overrides"
    :locale="zhCN"
    :date-locale="dateZhCN"
    inline-theme-disabled
  >
    <n-loading-bar-provider>
      <n-message-provider :max="3" placement="top">
        <n-dialog-provider>
          <n-notification-provider :max="3">
            <RouterView v-slot="{ Component }">
              <component :is="Component" />
            </RouterView>
          </n-notification-provider>
        </n-dialog-provider>
      </n-message-provider>
    </n-loading-bar-provider>
  </n-config-provider>
</template>
