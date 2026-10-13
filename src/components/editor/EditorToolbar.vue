<script setup lang="ts">
import { computed } from 'vue'
import type { Editor } from '@tiptap/vue-3'
import {
  PhArrowClockwise,
  PhArrowCounterClockwise,
  PhTextBolder,
  PhCheckSquare,
  PhCode,
  PhCodeBlock,
  PhEraser,
  PhHighlighter,
  PhImage,
  PhTextItalic,
  PhLinkBreak,
  PhLinkSimple,
  PhListBullets,
  PhListNumbers,
  PhMinus,
  PhQuotes,
  PhTextStrikethrough,
  PhTable,
  PhTextAlignCenter,
  PhTextAlignJustify,
  PhTextAlignLeft,
  PhTextAlignRight,
  PhTextUnderline,
} from '@phosphor-icons/vue'
import { CODE_LANGUAGE_OPTIONS } from '@/composables/useHighlight'

const props = defineProps<{
  editor: Editor | undefined
  uploading: boolean
  progress: number
}>()

const emit = defineEmits<{ (event: 'insert-image'): void }>()

const HEADING_LEVELS = [1, 2, 3, 4] as const

function active(name: string, attrs?: Record<string, unknown>) {
  return props.editor?.isActive(name, attrs) ?? false
}

/** Runs a chain only when the editor is ready, so the toolbar needs no guards. */
function run(build: (chain: ReturnType<Editor['chain']>) => void) {
  const editor = props.editor
  if (!editor) return
  build(editor.chain().focus())
}

const blockType = computed({
  get: () => {
    const editor = props.editor
    if (!editor) return 'paragraph'
    for (const level of HEADING_LEVELS) {
      if (editor.isActive('heading', { level })) return `h${level}`
    }
    return 'paragraph'
  },
  set: (value: string) => {
    if (value === 'paragraph') {
      run((chain) => chain.setParagraph().run())
      return
    }
    const level = Number(value.slice(1))
    if (level === 1 || level === 2 || level === 3 || level === 4) {
      run((chain) => chain.setHeading({ level }).run())
    }
  },
})

const codeLanguage = computed({
  get: () => (props.editor?.getAttributes('codeBlock').language as string | undefined) ?? '',
  set: (value: string) => {
    run((chain) => chain.updateAttributes('codeBlock', { language: value || null }).run())
  },
})

function toggleLink() {
  const editor = props.editor
  if (!editor) return
  const previous = (editor.getAttributes('link').href as string | undefined) ?? ''
  const href = window.prompt('链接地址', previous)
  if (href === null) return
  if (href.trim() === '') {
    run((chain) => chain.extendMarkRange('link').unsetLink().run())
    return
  }
  run((chain) => chain.extendMarkRange('link').setLink({ href: href.trim() }).run())
}
</script>

<template>
  <div class="rtb" role="toolbar" aria-label="编辑工具栏">
    <div class="rtb__group">
      <button
        class="rtb__btn"
        title="撤销"
        :disabled="!editor?.can().undo()"
        @click="run((chain) => chain.undo().run())"
      >
        <PhArrowCounterClockwise :size="16" />
      </button>
      <button
        class="rtb__btn"
        title="重做"
        :disabled="!editor?.can().redo()"
        @click="run((chain) => chain.redo().run())"
      >
        <PhArrowClockwise :size="16" />
      </button>
    </div>

    <span class="rtb__sep" />

    <div class="rtb__group">
      <select v-model="blockType" class="rtb__select" title="段落样式">
        <option value="paragraph">正文</option>
        <option value="h1">标题 1</option>
        <option value="h2">标题 2</option>
        <option value="h3">标题 3</option>
        <option value="h4">标题 4</option>
      </select>
    </div>

    <span class="rtb__sep" />

    <div class="rtb__group">
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('bold') }"
        title="加粗"
        @click="run((chain) => chain.toggleBold().run())"
      >
        <PhTextBolder :size="16" weight="bold" />
      </button>
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('italic') }"
        title="斜体"
        @click="run((chain) => chain.toggleItalic().run())"
      >
        <PhTextItalic :size="16" weight="bold" />
      </button>
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('underline') }"
        title="下划线"
        @click="run((chain) => chain.toggleUnderline().run())"
      >
        <PhTextUnderline :size="16" weight="bold" />
      </button>
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('strike') }"
        title="删除线"
        @click="run((chain) => chain.toggleStrike().run())"
      >
        <PhTextStrikethrough :size="16" weight="bold" />
      </button>
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('code') }"
        title="行内代码"
        @click="run((chain) => chain.toggleCode().run())"
      >
        <PhCode :size="16" weight="bold" />
      </button>
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('highlight') }"
        title="高亮"
        @click="run((chain) => chain.toggleHighlight().run())"
      >
        <PhHighlighter :size="16" weight="bold" />
      </button>
    </div>

    <span class="rtb__sep" />

    <div class="rtb__group">
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('textAlign', { textAlign: 'left' }) }"
        title="左对齐"
        @click="run((chain) => chain.setTextAlign('left').run())"
      >
        <PhTextAlignLeft :size="16" />
      </button>
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('textAlign', { textAlign: 'center' }) }"
        title="居中"
        @click="run((chain) => chain.setTextAlign('center').run())"
      >
        <PhTextAlignCenter :size="16" />
      </button>
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('textAlign', { textAlign: 'right' }) }"
        title="右对齐"
        @click="run((chain) => chain.setTextAlign('right').run())"
      >
        <PhTextAlignRight :size="16" />
      </button>
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('textAlign', { textAlign: 'justify' }) }"
        title="两端对齐"
        @click="run((chain) => chain.setTextAlign('justify').run())"
      >
        <PhTextAlignJustify :size="16" />
      </button>
    </div>

    <span class="rtb__sep" />

    <div class="rtb__group">
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('bulletList') }"
        title="无序列表"
        @click="run((chain) => chain.toggleBulletList().run())"
      >
        <PhListBullets :size="16" />
      </button>
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('orderedList') }"
        title="有序列表"
        @click="run((chain) => chain.toggleOrderedList().run())"
      >
        <PhListNumbers :size="16" />
      </button>
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('taskList') }"
        title="任务列表"
        @click="run((chain) => chain.toggleTaskList().run())"
      >
        <PhCheckSquare :size="16" />
      </button>
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('blockquote') }"
        title="引用"
        @click="run((chain) => chain.toggleBlockquote().run())"
      >
        <PhQuotes :size="16" />
      </button>
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('codeBlock') }"
        title="代码块"
        @click="run((chain) => chain.toggleCodeBlock().run())"
      >
        <PhCodeBlock :size="16" />
      </button>
      <button
        class="rtb__btn"
        title="分隔线"
        @click="run((chain) => chain.setHorizontalRule().run())"
      >
        <PhMinus :size="16" weight="bold" />
      </button>
      <select
        v-if="active('codeBlock')"
        v-model="codeLanguage"
        class="rtb__select rtb__select--lang"
        title="代码语言"
      >
        <option v-for="option in CODE_LANGUAGE_OPTIONS" :key="option.value" :value="option.value">
          {{ option.label }}
        </option>
      </select>
    </div>

    <span class="rtb__sep" />

    <div class="rtb__group">
      <button
        class="rtb__btn"
        :class="{ 'is-active': active('link') }"
        title="插入链接"
        @click="toggleLink"
      >
        <PhLinkSimple :size="16" />
      </button>
      <button
        v-if="active('link')"
        class="rtb__btn"
        title="移除链接"
        @click="run((chain) => chain.unsetLink().run())"
      >
        <PhLinkBreak :size="16" />
      </button>
      <button
        class="rtb__btn"
        title="插入图片"
        :disabled="uploading"
        @click="emit('insert-image')"
      >
        <PhImage :size="16" />
      </button>
      <button
        class="rtb__btn"
        title="插入表格"
        @click="run((chain) => chain.insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run())"
      >
        <PhTable :size="16" />
      </button>
      <button
        v-if="active('table')"
        class="rtb__btn rtb__btn--danger"
        title="删除表格"
        @click="run((chain) => chain.deleteTable().run())"
      >
        <PhTable :size="16" weight="fill" />
      </button>
    </div>

    <span class="rtb__sep" />

    <div class="rtb__group">
      <button
        class="rtb__btn"
        title="清除格式"
        @click="run((chain) => chain.unsetAllMarks().clearNodes().run())"
      >
        <PhEraser :size="16" />
      </button>
    </div>
  </div>
</template>
