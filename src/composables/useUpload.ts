import { ref } from 'vue'
import { useMessage } from 'naive-ui'
import { uploadMedia } from '@/api/media'
import type { MediaAsset } from '@/types/blog'

/**
 * Shared upload helper for the editor, the media library and the cover picker.
 * Uploads are serialised: a second file while one is in flight would otherwise
 * fight over the same progress indicator and the same server write.
 */
export function useUpload() {
  const message = useMessage()
  const uploading = ref(false)
  const progress = ref(0)

  async function upload(file: File): Promise<MediaAsset | null> {
    if (uploading.value) {
      message.warning('已有文件正在上传，请稍候')
      return null
    }
    uploading.value = true
    progress.value = 0
    try {
      return await uploadMedia(file, (percent) => {
        progress.value = percent
      })
    } catch (error) {
      message.error(error instanceof Error ? error.message : '上传失败')
      return null
    } finally {
      uploading.value = false
      progress.value = 0
    }
  }

  return { upload, uploading, progress }
}
