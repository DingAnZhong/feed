<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'

const router = useRouter()
const mode = ref('login') // 'login' | 'register'
const userId = ref('')
const nickname = ref('')
const loading = ref(false)
const error = ref('')

async function handleLogin() {
  if (!userId.value.trim()) {
    error.value = '请输入用户 ID'
    return
  }
  if (!/^\d+$/.test(userId.value.trim())) {
    error.value = '用户 ID 必须是数字'
    return
  }
  loading.value = true
  error.value = ''
  try {
    const id = userId.value.trim()
    const nk = nickname.value.trim() || `用户${id}`
    localStorage.setItem('feed_user_id', id)
    localStorage.setItem('feed_nickname', nk)
    router.push('/feed')
  } catch (err) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

async function handleRegister() {
  if (!userId.value.trim()) {
    error.value = '请输入用户 ID'
    return
  }
  if (!/^\d+$/.test(userId.value.trim())) {
    error.value = '用户 ID 必须是数字'
    return
  }
  if (!nickname.value.trim()) {
    error.value = '请输入昵称'
    return
  }
  loading.value = true
  error.value = ''
  try {
    const id = parseInt(userId.value.trim(), 10)
    const nk = nickname.value.trim()
    await axios.post('/web/api/v1/user/register', { user_id: id, nickname: nk })
    localStorage.setItem('feed_user_id', String(id))
    localStorage.setItem('feed_nickname', nk)
    router.push('/feed')
  } catch (err) {
    error.value = err.response?.data?.msg || '注册失败'
  } finally {
    loading.value = false
  }
}

function switchMode() {
  mode.value = mode.value === 'login' ? 'register' : 'login'
  error.value = ''
  nickname.value = ''
}
</script>

<template>
  <div class="login-page">
    <div class="login-card animate-fade-in">
      <div class="login-header">
        <div class="logo-circle">📡</div>
        <h1>Feed 流系统</h1>
        <p class="subtitle">千万级分布式 Feed 流管理平台</p>
      </div>
      <form class="login-form" @submit.prevent="mode === 'login' ? handleLogin() : handleRegister()">
        <div class="form-group">
          <label for="userId">用户 ID</label>
          <input
            id="userId"
            v-model="userId"
            type="text"
            placeholder="请输入用户 ID（数字）"
            autocomplete="off"
            autofocus
          />
        </div>
        <div class="form-group">
          <label for="nickname">昵称 <span v-if="mode === 'login'" class="optional">选填</span><span v-else class="required">必填</span></label>
          <input
            id="nickname"
            v-model="nickname"
            type="text"
            placeholder="请输入昵称"
            autocomplete="off"
          />
        </div>
        <p v-if="error" class="error-msg">{{ error }}</p>
        <button type="submit" class="btn-submit" :disabled="loading">
          <span v-if="loading" class="spinner-small"></span>
          <span v-else>{{ mode === 'login' ? '进入系统' : '注册并进入' }}</span>
        </button>
      </form>
      <div class="login-hint">
        <p v-if="mode === 'login'">支持用户 ID: 1 ~ 100（与压测数据一致）</p>
        <p v-else>新用户可在此注册账号</p>
      </div>
      <div class="switch-mode" @click="switchMode">
        {{ mode === 'login' ? '没有账号？点击注册' : '已有账号？点击登录' }}
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-page {
  min-height: calc(100vh - 56px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.login-card {
  background: var(--bg-card);
  border-radius: 20px;
  padding: 48px 40px;
  width: 100%;
  max-width: 420px;
  box-shadow: 0 20px 60px rgba(0,0,0,0.15);
}

.login-header {
  text-align: center;
  margin-bottom: 36px;
}

.logo-circle {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  background: linear-gradient(135deg, #e94560, #764ba2);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 32px;
  margin: 0 auto 16px;
}

.login-header h1 {
  font-size: 24px;
  font-weight: 700;
  color: var(--text-primary);
  margin-bottom: 4px;
}

.subtitle {
  font-size: 14px;
  color: var(--text-secondary);
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
}

.optional {
  font-weight: 400;
  color: var(--text-tertiary);
}

.required {
  font-weight: 400;
  color: var(--accent);
}

.switch-mode {
  text-align: center;
  margin-top: 16px;
  font-size: 13px;
  color: var(--accent);
  cursor: pointer;
  transition: color 0.2s;
}

.switch-mode:hover {
  color: var(--text-primary);
  text-decoration: underline;
}

.form-group input {
  height: 44px;
  padding: 0 14px;
  border: 2px solid var(--border);
  border-radius: var(--radius-sm);
  font-size: 15px;
  color: var(--text-primary);
  transition: border-color 0.2s;
  outline: none;
}

.form-group input:focus {
  border-color: var(--accent);
}

.error-msg {
  color: #ef4444;
  font-size: 13px;
  text-align: center;
  background: #fef2f2;
  padding: 8px 12px;
  border-radius: 6px;
}

.btn-submit {
  height: 48px;
  border: none;
  border-radius: var(--radius-sm);
  background: linear-gradient(135deg, #e94560, #764ba2);
  color: #fff;
  font-size: 16px;
  font-weight: 600;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.btn-submit:hover:not(:disabled) {
  opacity: 0.9;
  transform: translateY(-1px);
  box-shadow: 0 4px 16px rgba(233, 69, 96, 0.4);
}

.btn-submit:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.spinner-small {
  width: 18px;
  height: 18px;
  border: 2px solid rgba(255,255,255,0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

.login-hint {
  text-align: center;
  margin-top: 20px;
  font-size: 12px;
  color: var(--text-tertiary);
}
</style>
