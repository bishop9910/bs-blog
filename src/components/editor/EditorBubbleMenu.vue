<script setup lang="ts">
import { BubbleMenu } from '@tiptap/vue-3/menus'
import type { Editor } from '@tiptap/vue-3'
import {
  PhTextBolder,
  PhCode,
  PhHighlighter,
  PhTextItalic,
  PhLinkBreak,
  PhLinkSimple,
  PhTextStrikethrough,
  PhTextUnderline,
} from '@phosphor-icons/vue'

const props = defineProps<{ editor: Editor }>()

function isActive(name: string) {
  return props.editor.isActive(name)
}

function toggleLink() {
  const previous = (props.editor.getAttributes('link').href as string | undefined) ?? ''
  const href = window.prompt('链接地址', previous)
  if (href === null) return
  const chain = props.editor.chain().focus().extendMarkRange('link')
  if (href.trim() === '') {
    chain.unsetLink().run()
    return
  }
  chain.setLink({ href: href.trim() }).run()
}
</script>

<template>
  <BubbleMenu
    :editor="editor"
    :options="{ placement: 'top', offset: 10 }"
    class="bubble-menu"
  >
    <button
      class="bubble-menu__btn"
      :class="{ 'is-active': isActive('bold') }"
      title="加粗"
      @click="editor.chain().focus().toggleBold().run()"
    >
      <PhTextBolder :size="16" weight="bold" />
    </button>
    <button
      class="bubble-menu__btn"
      :class="{ 'is-active': isActive('italic') }"
      title="斜体"
      @click="editor.chain().focus().toggleItalic().run()"
    >
      <PhTextItalic :size="16" weight="bold" />
    </button>
    <button
      class="bubble-menu__btn"
      :class="{ 'is-active': isActive('underline') }"
      title="下划线"
      @click="editor.chain().focus().toggleUnderline().run()"
    >
      <PhTextUnderline :size="16" weight="bold" />
    </button>
    <button
      class="bubble-menu__btn"
      :class="{ 'is-active': isActive('strike') }"
      title="删除线"
      @click="editor.chain().focus().toggleStrike().run()"
    >
      <PhTextStrikethrough :size="16" weight="bold" />
    </button>
    <button
      class="bubble-menu__btn"
      :class="{ 'is-active': isActive('code') }"
      title="行内代码"
      @click="editor.chain().focus().toggleCode().run()"
    >
      <PhCode :size="16" weight="bold" />
    </button>
    <button
      class="bubble-menu__btn"
      :class="{ 'is-active': isActive('highlight') }"
      title="高亮"
      @click="editor.chain().focus().toggleHighlight().run()"
    >
      <PhHighlighter :size="16" weight="bold" />
    </button>
    <span class="bubble-menu__sep" />
    <button
      class="bubble-menu__btn"
      :class="{ 'is-active': isActive('link') }"
      title="链接"
      @click="toggleLink"
    >
      <PhLinkSimple :size="16" weight="bold" />
    </button>
    <button
      v-if="isActive('link')"
      class="bubble-menu__btn"
      title="移除链接"
      @click="editor.chain().focus().unsetLink().run()"
    >
      <PhLinkBreak :size="16" weight="bold" />
    </button>
  </BubbleMenu>
</template>
