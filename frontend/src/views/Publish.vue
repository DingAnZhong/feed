<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '../api/index'

const router = useRouter()
const content = ref('')
const loading = ref(false)
const toastContainer = ref(null)

const MAX_LENGTH = 500

function showToast(message, type = 'info') {
  const el = document.createElement('div')
  el.className = `toast ${type}`
  el.textContent = message
  toastContainer.value.appendChild(el)
  setTimeout(() => el.remove(), 3000)
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
    await api.publishPost(text, [])
    showToast('发布成功！', 'success')
    content.value = ''
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
</style>
