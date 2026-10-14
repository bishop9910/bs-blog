<script setup lang="ts">
import { computed } from 'vue'
import { NDataTable } from 'naive-ui'
import type { DataTableColumns, PaginationProps } from 'naive-ui'

/**
 * A thin, server-paginated wrapper around NDataTable. It exists so every admin
 * table gets the same empty state, the same page-size handling, and — because
 * the cell renderers below are produced outside this component's template —
 * one global stylesheet for the shared cell classes.
 */
const props = withDefaults(
  defineProps<{
    columns: DataTableColumns<any>
    rows: any[]
    loading?: boolean
    page?: number
    pageSize?: number
    total?: number
    emptyText?: string
  }>(),
  {
    loading: false,
    page: 1,
    pageSize: 10,
    total: 0,
    emptyText: '还没有数据',
  },
)

const emit = defineEmits<{ (event: 'update:page', value: number): void }>()

const pagination = computed<PaginationProps | false>(() => {
  if (props.total <= props.pageSize) return false
  return {
    page: props.page,
    pageSize: props.pageSize,
    itemCount: props.total,
    showSizePicker: false,
    prefix: ({ itemCount }) => `共 ${itemCount} 条`,
    onUpdatePage: (value: number) => emit('update:page', value),
  }
})

const rowKey = (row: { id?: number }) => row.id ?? 0
</script>

<template>
  <div class="admin-table">
    <!--
      Deliberately no flex-height. It makes the table body flex to fill its
      parent's height, which only works when that parent has a definite one.
      .admin-table instead sizes to its content inside a document-scrolling
      page, so flex-height collapsed the body to zero: the header still
      rendered, the rows did not, and because the collapsed body also swallowed
      hit testing the rows could not even be clicked.
    -->
    <NDataTable
      remote
      :columns="columns"
      :data="rows"
      :loading="loading"
      :pagination="pagination"
      :row-key="rowKey"
      :bordered="false"
      :single-line="false"
      size="small"
    />
    <p v-if="!loading && rows.length === 0" class="admin-table__empty">{{ emptyText }}</p>
  </div>
</template>

<style scoped>
.admin-table {
  position: relative;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: var(--bg-card);
  overflow: hidden;
}

.admin-table__empty {
  position: absolute;
  inset: 56px 0 auto;
  margin: 0;
  color: var(--text-muted);
  font-size: var(--text-sm);
  text-align: center;
}
</style>

<!--
  Cell renderers are built by the calling view and executed inside NDataTable,
  so they never receive this component's scope id. These rules must be global.
-->
<style>
.admin-table .cell-title {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.admin-table .cell-title__link {
  padding: 0;
  border: none;
  background: none;
  color: var(--text-primary);
  font: inherit;
  font-size: var(--text-sm);
  text-align: left;
  cursor: pointer;
}

.admin-table .cell-title__link:hover {
  color: var(--accent);
  text-decoration: underline;
}

.admin-table .cell-title__slug {
  color: var(--text-muted);
  font-family: var(--font-mono);
  font-size: var(--text-2xs);
}

.admin-table .cell-muted {
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.admin-table .cell-num {
  color: var(--text-secondary);
  font-size: var(--text-xs);
  font-variant-numeric: tabular-nums;
}
</style>
