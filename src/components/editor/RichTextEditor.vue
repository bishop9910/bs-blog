<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { EditorContent, useEditor, type Editor } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import { CharacterCount, Placeholder } from '@tiptap/extensions'
import { TaskItem, TaskList } from '@tiptap/extension-list'
import { TableKit } from '@tiptap/extension-table'
import TextAlign from '@tiptap/extension-text-align'
import Highlight from '@tiptap/extension-highlight'
import Image from '@tiptap/extension-image'
import FileHandler from '@tiptap/extension-file-handler'
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import { lowlight } from '@/composables/useHighlight'
import { useUpload } from '@/composables/useUpload'
import EditorToolbar from './EditorToolbar.vue'
import EditorBubbleMenu from './EditorBubbleMenu.vue'
import '@/styles/editor.css'

const props = withDefaults(
  defineProps<{
    modelValue: string
    placeholder?: string
    minHeight?: string
  }>(),
  {
    placeholder: '写点什么…',
    minHeight: '460px',
  },
)

const emit = defineEmits<{
  (event: 'update:modelValue', value: string): void
  (event: 'update:json', value: Record<string, unknown>): void
  (event: 'update:stats', value: { words: number; characters: number }): void
}>()

const { upload, uploading, progress } = useUpload()
const fileInput = ref<HTMLInputElement | null>(null)
const focused = ref(false)
const stats = ref({ words: 0, characters: 0 })

/**
 * Extension callbacks (onUpdate, FileHandler's onDrop/onPaste) hand back
 * @tiptap/core's Editor, while useEditor gives us vue-3's subclass. We cannot
 * name the core type directly because @tiptap/core is only a transitive peer,
 * so the helpers take just the members they use — both classes are assignable
 * to that subset.
 */
type EditorLike = Pick<Editor, 'storage' | 'chain'>

function syncStats(instance: EditorLike) {
  const counter = instance.storage.characterCount as
    | { words: () => number; characters: () => number }
    | undefined
  stats.value = {
    words: counter?.words() ?? 0,
    characters: counter?.characters() ?? 0,
  }
  emit('update:stats', stats.value)
}

/** Upload then insert, one file at a time so progress stays meaningful. */
async function insertImages(instance: EditorLike, files: File[]) {
  for (const file of files) {
    if (!file.type.startsWith('image/')) continue
    const asset = await upload(file)
    if (!asset) continue
    instance.chain().focus().setImage({ src: asset.url, alt: asset.filename }).run()
  }
}

const editor = useEditor({
  content: props.modelValue,
  extensions: [
    StarterKit.configure({
      // The lowlight variant replaces the plain code block entirely.
      codeBlock: false,
      heading: { levels: [1, 2, 3, 4] },
    }),
    Placeholder.configure({ placeholder: () => props.placeholder }),
    CharacterCount,
    TaskList,
    TaskItem.configure({ nested: true }),
    TableKit.configure({ table: { resizable: true } }),
    TextAlign.configure({ types: ['heading', 'paragraph'] }),
    Highlight.configure({ multicolor: false }),
    Image.configure({ allowBase64: false, inline: false }),
    CodeBlockLowlight.configure({ lowlight, defaultLanguage: 'plaintext' }),
    FileHandler.configure({
      allowedMimeTypes: ['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/avif'],
      onDrop: (current, files) => {
        void insertImages(current, files)
        return true
      },
      onPaste: (current, files) => {
        void insertImages(current, files)
        return true
      },
    }),
  ],
  editorProps: {
    attributes: {
      class: 'prose prose--editor',
      spellcheck: 'false',
    },
  },
  onUpdate: ({ editor: instance }) => {
    emit('update:modelValue', instance.getHTML())
    emit('update:json', instance.getJSON() as Record<string, unknown>)
    syncStats(instance)
  },
  onSelectionUpdate: ({ editor: instance }) => syncStats(instance),
  onFocus: () => {
    focused.value = true
  },
  onBlur: () => {
    focused.value = false
  },
})

// External writes (loading an article, clearing the form) push into the editor.
// Comparing first avoids a setContent/onUpdate cycle.
watch(
  () => props.modelValue,
  (value) => {
    const instance = editor.value
    if (!instance) return
    if (value === instance.getHTML()) return
    instance.commands.setContent(value || '')
    syncStats(instance)
  },
)

function pickImage() {
  fileInput.value?.click()
}

function onFilePicked(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = ''
  if (editor.value) void insertImages(editor.value, files)
}

onBeforeUnmount(() => {
  editor.value?.destroy()
})

defineExpose({
  focus: () => editor.value?.commands.focus(),
  editor,
})
</script>

<template>
  <div class="rte" :class="{ 'is-focused': focused }">
    <EditorToolbar
      :editor="editor"
      :uploading="uploading"
      :progress="progress"
      @insert-image="pickImage"
    />

    <div class="rte__body" :style="{ '--rte-min-height': minHeight }">
      <EditorContent :editor="editor" />
      <EditorBubbleMenu v-if="editor" :editor="editor" />
    </div>

    <div class="rte__status">
      <span class="rte__count">{{ stats.words }} 字</span>
      <span class="rte__count">{{ stats.characters }} 字符</span>
      <span v-if="uploading" class="rte__uploading">图片上传中 {{ progress }}%</span>
      <span v-else class="rte__hint">拖拽或粘贴图片可直接上传</span>
    </div>

    <input
      ref="fileInput"
      type="file"
      accept="image/*"
      multiple
      class="rte__file"
      @change="onFilePicked"
    />
  </div>
</template>
