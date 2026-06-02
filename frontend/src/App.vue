<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const userId = computed(() => localStorage.getItem('feed_user_id') || '')
const nickname = computed(() => localStorage.getItem('feed_nickname') || '')

function handleLogout() {
  localStorage.removeItem('feed_user_id')
  localStorage.removeItem('feed_nickname')
  router.push('/login')
}
</script>

<template>
  <div id="app-root">
    <nav class="navbar">
      <div class="nav-inner">
        <RouterLink to="/feed" class="logo">📡 Feed 流系统</RouterLink>
        <div class="nav-links">
          <RouterLink to="/feed" class="nav-link">📖 时间线</RouterLink>
          <RouterLink to="/publish" class="nav-link">✏️ 发布</RouterLink>
        </div>
        <div v-if="userId" class="user-info">
          <span class="user-avatar">{{ nickname.charAt(0) || 'U' }}</span>
          <span class="user-name">{{ nickname || userId }}</span>
          <button class="btn-logout" @click="handleLogout">退出</button>
        </div>
        <RouterLink v-else to="/login" class="nav-link btn-login">登录</RouterLink>
      </div>
    </nav>
    <main class="main-content">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
#app-root {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.navbar {
  background: #1a1a2e;
  color: #eaeaea;
  padding: 0 24px;
  position: sticky;
  top: 0;
  z-index: 100;
  box-shadow: 0 2px 12px rgba(0,0,0,0.3);
}

.nav-inner {
  max-width: 960px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  height: 56px;
  gap: 24px;
}

.logo {
  font-size: 18px;
  font-weight: 700;
  color: #e94560;
  text-decoration: none;
  white-space: nowrap;
}

.nav-links {
  display: flex;
  gap: 8px;
  flex: 1;
}

.nav-link {
  color: #b0b0cc;
  text-decoration: none;
  padding: 6px 14px;
  border-radius: 6px;
  font-size: 14px;
  transition: all 0.2s;
}

.nav-link:hover, .nav-link.router-link-exact-active {
  background: rgba(233, 69, 96, 0.15);
  color: #e94560;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.user-avatar {
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background: #e94560;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 700;
  color: #fff;
}

.user-name {
  font-size: 13px;
  color: #c0c0dd;
}

.btn-logout {
  background: none;
  border: 1px solid rgba(255,255,255,0.15);
  color: #999;
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-logout:hover {
  border-color: #e94560;
  color: #e94560;
}

.btn-login {
  border: 1px solid #e94560;
  color: #e94560;
  border-radius: 6px;
}

.btn-login:hover {
  background: rgba(233, 69, 96, 0.15);
}

.main-content {
  flex: 1;
  background: #f0f2f5;
}
</style>
