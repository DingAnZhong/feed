<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/index'

const router = useRouter()
const content = ref('')
const loading = ref(false)
const toastContainer = ref(null)

const MAX_LENGTH = 500
const MAX_IMAGES = 9
const MAX_IMAGE_SIZE = 5 * 1024 * 1024 // 5MB

// 图片相关状态
const imageFiles = ref([]) // 选择的文件对象 [{ file, preview, name }]
const isDragging = ref(false)

function showToast(message, type = 'info') {
  const el = document.createElement('div')
  el.className = `toast ${type}`
  el.textContent = message
  toastContainer.value.appendChild(el)
  setTimeout(() => el.remove(), 3000)
}

// 处理文件选择
function handleFileSelect(event) {
  const files = Array.from(event.target.files || [])
  addFiles(files)
  // 清空 input 以便重复选择同一文件
  event.target.value = ''
}

// 处理拖拽
function handleDragOver(e) {
  e.preventDefault()
  isDragging.value = true
}

function handleDragLeave(e) {
  e.preventDefault()
  isDragging.value = false
}

function handleDrop(e) {
  e.preventDefault()
  isDragging.value = false
  const files = Array.from(e.dataTransfer.files)
  addFiles(files)
}

// 添加文件到图片列表
function addFiles(files) {
  const remaining = MAX_IMAGES - imageFiles.value.length
  if (remaining <= 0) {
    showToast(`最多只能上传 ${MAX_IMAGES} 张图片`, 'error')
    return
  }

  const toAdd = files.slice(0, remaining)
  for (const file of toAdd) {
    // 检查是否为图片
    if (!file.type.startsWith('image/')) {
      showToast(`"${file.name}" 不是图片文件`, 'error')
      continue
    }
    // 检查文件大小
    if (file.size > MAX_IMAGE_SIZE) {
      showToast(`"${file.name}" 超过 5MB 限制`, 'error')
      continue
    }
    // 检查重复
    const exists = imageFiles.value.some(item => item.name === file.name && item.size === file.size)
    if (exists) {
      showToast(`"${file.name}" 已选择`, 'error')
      continue
    }

    const preview = URL.createObjectURL(file)
    imageFiles.value.push({ file, preview, name: file.name, size: file.size })
  }
}

// 删除图片
function removeImage(index) {
  URL.revokeObjectURL(imageFiles.value[index].preview)
  imageFiles.value.splice(index, 1)
}

// 模拟上传：将文件转为 base64 URL 作为临时 media_url
// 实际项目中应替换为真正的图片上传接口
function uploadImages() {
  return Promise.all(
    imageFiles.value.map(item => {
      return new Promise((resolve, reject) => {
        const reader = new FileReader()
        reader.onload = (e) => {
          resolve(e.target.result)
        }
        reader.onerror = () => reject(new Error('图片读取失败'))
        reader.readAsDataURL(item.file)
      })
    })
  )
}

async function handlePublish() {
  const text = content.value.trim()
  if (!text) {
    showToast('内容不能为空', 'error')
    return
  }
  if (text.length > MAX_LENGTH) {
    showToast(`内容不能超过 ${MAX_LENGTH} 字`, 'error')
    return
  }

  loading.value = true
  try {
    // 上传图片
    let mediaUrls = []
    if (imageFiles.value.length > 0) {
      mediaUrls = await uploadImages()
    }

    await api.publishPost(text, mediaUrls)
    showToast('发布成功！', 'success')
    content.value = ''
    // 释放所有 preview URL
    imageFiles.value.forEach(item => URL.revokeObjectURL(item.preview))
    imageFiles.value = []
    setTimeout(() => router.push('/feed'), 800)
  } catch (err) {
    showToast(err.message, 'error')
  } finally {
    loading.value = false
  }
}

function handleKeydown(e) {
  if (e.ctrlKey && e.key === 'Enter') {
    handlePublish()
  }
}
</script>

<template>
  <div class="publish-page">
    <div class="publish-container">
      <div ref="toastContainer" class="toast-container"></div>

      <div class="publish-card animate-fade-in">
        <h2>✏️ 发布动态</h2>
        <p class="publish-desc">分享你的想法到 Feed 流</p>

        <textarea
          v-model="content"
          placeholder="有什么新鲜事？"
          rows="6"
          class="publish-textarea"
          @keydown="handleKeydown"
        ></textarea>

        <!-- 图片上传区域 -->
        <div class="image-upload-section">
          <div
            class="drop-zone"
            :class="{ dragging: isDragging }"
            @dragover="handleDragOver"
            @dragleave="handleDragLeave"
            @drop="handleDrop"
            @click="fileInput.click()"
          >
            <input
              ref="fileInput"
              type="file"
              multiple
              accept="image/*"
              class="file-input"
              @change="handleFileSelect"
            />
            <div v-if="imageFiles.length === 0" class="drop-zone-content">
              <span class="upload-icon">📷</span>
              <p class="drop-text">点击或拖拽图片到此区域</p>
              <p class="drop-hint">支持 JPG/PNG/GIF，最多 9 张，单张不超过 5MB</p>
            </div>

            <!-- 已选图片预览 -->
            <div v-else class="image-preview-grid">
              <div
                v-for="(img, idx) in imageFiles"
                :key="idx"
                class="image-preview-item"
              >
                <img :src="img.preview" :alt="img.name" class="preview-img" />
                <button class="remove-btn" @click.stop="removeImage(idx)">
                  ✕
                </button>
              </div>
              <!-- 添加更多按钮（未达到上限时） -->
              <div
                v-if="imageFiles.length < MAX_IMAGES"
                class="add-more-btn"
                @click="fileInput.click()"
              >
                <span class="add-icon">＋</span>
                <span class="add-text">添加图片</span>
              </div>
            </div>
          </div>
          <div v-if="imageFiles.length > 0" class="image-count">
            已选择 {{ imageFiles.length }} / {{ MAX_IMAGES }} 张图片
          </div>
        </div>

        <div class="publish-footer">
          <div class="char-count">
            <span :class="{ 'over-limit': content.length > MAX_LENGTH }">
              {{ content.length }}
            </span>
            <span class="char-max">/{{ MAX_LENGTH }}</span>
          </div>
          <div class="publish-hint">
            <kbd>Ctrl</kbd> + <kbd>Enter</kbd> 快捷发布
          </div>
          <button
            class="btn-publish"
            :disabled="loading || !content.trim()"
            @click="handlePublish"
          >
            <span v-if="loading" class="spinner-small"></span>
            <span v-else>发布</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.publish-page {
  max-width: 640px;
  margin: 0 auto;
  padding: 24px 16px;
}

.publish-container {
  min-height: calc(100vh - 104px);
  display: flex;
  align-items: flex-start;
}

.publish-card {
  background: var(--bg-card);
  border-radius: var(--radius);
  padding: 28px;
  width: 100%;
  box-shadow: var(--shadow-sm);
}

.publish-card h2 {
  font-size: 20px;
  font-weight: 700;
  margin-bottom: 4px;
}

.publish-desc {
  font-size: 14px;
  color: var(--text-secondary);
  margin-bottom: 20px;
}

.publish-textarea {
  width: 100%;
  border: 2px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 14px;
  font-size: 16px;
  line-height: 1.6;
  color: var(--text-primary);
  resize: vertical;
  outline: none;
  transition: border-color 0.2s;
}

.publish-textarea:focus {
  border-color: var(--accent);
}

.publish-textarea::placeholder {
  color: var(--text-tertiary);
}

.publish-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--border-light);
}

.char-count {
  font-size: 13px;
  color: var(--text-tertiary);
}

.char-count .over-limit {
  color: #ef4444;
  font-weight: 700;
}

.char-max {
  margin-left: 2px;
}

.publish-hint {
  font-size: 12px;
  color: var(--text-tertiary);
}

kbd {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: 3px;
  padding: 1px 5px;
  font-size: 11px;
  font-family: var(--font);
}

.btn-publish {
  padding: 10px 28px;
  border: none;
  border-radius: 20px;
  background: linear-gradient(135deg, #e94560, #764ba2);
  color: #fff;
  font-size: 15px;
  font-weight: 600;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 6px;
}

.btn-publish:hover:not(:disabled) {
  opacity: 0.9;
  transform: translateY(-1px);
  box-shadow: 0 4px 16px rgba(233, 69, 96, 0.4);
}

.btn-publish:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.spinner-small {
  width: 16px;
  height: 16px;
  border: 2px solid rgba(255,255,255,0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

/* 图片上传区域 */
.image-upload-section {
  margin-top: 16px;
}

.drop-zone {
  border: 2px dashed var(--border);
  border-radius: var(--radius-sm);
  padding: 20px;
  text-align: center;
  cursor: pointer;
  transition: all 0.2s;
  background: var(--bg-secondary);
  min-height: 120px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.drop-zone.dragging {
  border-color: var(--accent);
  background: rgba(233, 69, 96, 0.05);
}

.file-input {
  display: none;
}

.drop-zone-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  color: var(--text-tertiary);
  pointer-events: none;
}

.upload-icon {
  font-size: 32px;
  margin-bottom: 4px;
}

.drop-text {
  font-size: 14px;
  color: var(--text-secondary);
  margin: 0;
}

.drop-hint {
  font-size: 12px;
  color: var(--text-tertiary);
  margin: 0;
}

.image-preview-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(90px, 1fr));
  gap: 8px;
  width: 100%;
}

.image-preview-item {
  position: relative;
  width: 100%;
  padding-top: 100%;
  border-radius: 8px;
  overflow: hidden;
  background: var(--bg-tertiary);
}

.preview-img {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.remove-btn {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  border: none;
  background: rgba(0, 0, 0, 0.6);
  color: #fff;
  font-size: 12px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
  line-height: 1;
}

.remove-btn:hover {
  background: rgba(239, 68, 68, 0.9);
  transform: scale(1.1);
}

.add-more-btn {
  position: relative;
  width: 100%;
  padding-top: 100%;
  border-radius: 8px;
  border: 2px dashed var(--border);
  background: var(--bg-tertiary);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  cursor: pointer;
  transition: all 0.2s;
  color: var(--text-tertiary);
}

.add-more-btn:hover {
  border-color: var(--accent);
  color: var(--accent);
  background: rgba(233, 69, 96, 0.03);
}

.add-icon {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  font-size: 28px;
  font-weight: 300;
  line-height: 1;
}

.add-text {
  font-size: 11px;
}

.image-count {
  font-size: 12px;
  color: var(--text-tertiary);
  margin-top: 8px;
  text-align: right;
}
</style>
