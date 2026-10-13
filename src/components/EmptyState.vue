<script setup lang="ts">
withDefaults(
  defineProps<{
    title: string
    description?: string
  }>(),
  { description: '' },
)
</script>

<template>
  <div class="empty">
    <span class="empty__rule" aria-hidden="true" />
    <p class="empty__title">{{ title }}</p>
    <p v-if="description" class="empty__description">{{ description }}</p>
    <div v-if="$slots.default" class="empty__actions">
      <slot />
    </div>
  </div>
</template>

<style scoped>
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-sm);
  padding: var(--space-3xl) var(--space-lg);
  text-align: center;
}

.empty__rule {
  width: 48px;
  height: 2px;
  margin-bottom: var(--space-sm);
  background: linear-gradient(90deg, transparent, var(--accent), transparent);
}

.empty__title {
  margin: 0;
  color: var(--text-primary);
  font-family: var(--font-display);
  font-size: var(--text-lg);
}

.empty__description {
  max-width: var(--measure-narrow);
  margin: 0;
  color: var(--text-muted);
  font-size: var(--text-sm);
}

.empty__actions {
  margin-top: var(--space-md);
}
</style>
