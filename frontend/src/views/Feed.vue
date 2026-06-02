<script setup>
import { ref, onMounted } from 'vue'
import { api } from '../api/index'

const posts = ref([])
const refreshing = ref(false)
const loadingMore = ref(false)
const nextTime = ref(0)
const isEnd = ref(false)
const hasLoaded = ref(false)
const toastContainer = ref(null)
const currentUserId = ref(getUserId())

// 当前 Tab: 'timeline' 或 'popular'
const activeTab = ref('timeline')
const tabLoading = ref(false)
const tabNextTime = ref({ timeline: 0, popular: 0 })
const tabIsEnd = ref({ timeline: false, popular: false })
const tabHasLoaded = ref({ timeline: false, popular: false })

// 关注状态缓存: { userId: isFollowing }
const followStatusCache = ref({})
// 用户信息缓存: { userId: { nickname, user_id } }
const userInfoCache = ref({})

function getUserId() {
  return localStorage.getItem('feed_user_id')
}

function showToast(message, type = 'info') {
  const el = document.createElement('div')
  el.className = `toast ${type}`
  el.textContent = message
  toastContainer.value.appendChild(el)
  setTimeout(() => el.remove(), 3000)
}

function formatTime(timestamp) {
  const date = new Date(timestamp)
  const now = new Date()
  const diff = now - date
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return `${Math.floor(diff / 60000)} 分钟前`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)} 小时前`
  if (diff < 604800000) return `${Math.floor(diff / 86400000)} 天前`
  return date.toLocaleDateString('zh-CN')
}

function getTimestamp(post) {
  if (typeof post.create_time === 'number') return post.create_time
  if (post.create_time) return new Date(post.create_time).getTime()
  return Date.now()
}

// 获取用户信息（带缓存）
async function getUserInfo(userId) {
  if (userInfoCache.value[userId]) return userInfoCache.value[userId]
  try {
    const info = await api.getUserInfo(userId)
    userInfoCache.value[userId] = info
    return info
  } catch {
    return { user_id: userId, nickname: `用户${userId}` }
  }
}

// 获取关注状态（带缓存）
async function getFollowStatus(userId) {
  if (followStatusCache.value[userId] !== undefined) return followStatusCache.value[userId]
  try {
    const result = await api.checkFollowStatus(userId)
    followStatusCache.value[userId] = result.is_following
    return result.is_following
  } catch {
    return false
  }
}

// 关注/取关操作
async function toggleFollow(post) {
  const currentUserId = getUserId()
  const userId = post.user_id

  // 不能关注自己
  if (currentUserId && String(userId) === String(currentUserId)) {
    showToast('不能关注自己哦', 'error')
    return
  }

  const isFollowing = followStatusCache.value[userId]
  const actionType = isFollowing ? 2 : 1 // 1-关注, 2-取关

  try {
    post.loadingFollow = true
    await api.followAction(userId, actionType)
    followStatusCache.value[userId] = !isFollowing
    showToast(isFollowing ? '已取关' : '关注成功')
  } catch (err) {
    showToast(err.message, 'error')
  } finally {
    post.loadingFollow = false
  }
}

// 预加载帖子列表中的用户信息和关注状态
async function preloadUserInfos() {
  const userIds = [...new Set(posts.value.map(p => p.user_id))]
  for (const userId of userIds) {
    const [info, following] = await Promise.all([
      getUserInfo(userId),
      getFollowStatus(userId),
    ])
    userInfoCache.value[userId] = info
    followStatusCache.value[userId] = following

    // 更新帖子中的作者信息
    posts.value.forEach(p => {
      if (p.user_id === userId) {
        p.author = {
          user_id: p.user_id,
          nickname: info.nickname || `用户${p.user_id}`,
          avatar: '',
        }
      }
    })
  }
}

async function loadFeed(type = null) {
  if (type === null) type = activeTab.value
  const isRefresh = type === activeTab.value && !tabHasLoaded.value[type]

  // 刷新用 refreshing，加载更多用 loadingMore
  if (type === activeTab.value) {
    if (isRefresh) {
      refreshing.value = true
    } else {
      loadingMore.value = true
    }
  } else {
    tabLoading.value = true
  }

  try {
    const result = await api.fetchFeed(tabNextTime.value[type], 10, type)
    const newPosts = (result?.posts || []).map(p => ({
      post_id: p.id,
      user_id: p.user_id,
      author: {
        user_id: p.user_id,
        nickname: `用户${p.user_id}`,
        avatar: '',
      },
      content: p.content,
      media_urls: p.media_urls || [],
      create_time: getTimestamp(p),
      like_count: p.like_count || 0,
      comment_count: p.comment_count || 0,
      loadingFollow: false,
    }))

    if (isRefresh || type !== activeTab.value) {
      posts.value = newPosts
    } else {
      posts.value = [...posts.value, ...newPosts]
    }
    tabNextTime.value[type] = result?.next_time || 0
    tabIsEnd.value[type] = result?.is_end || false
    tabHasLoaded.value[type] = true

    // 预加载用户信息和关注状态
    await preloadUserInfos()
  } catch (err) {
    showToast(err.message, 'error')
  } finally {
    if (type === activeTab.value) {
      refreshing.value = false
      loadingMore.value = false
    } else {
      tabLoading.value = false
    }
  }
}

function switchTab(type) {
  if (type === activeTab.value) return
  activeTab.value = type
  posts.value = []
  // 重置该 tab 状态
  tabNextTime.value[type] = 0
  tabIsEnd.value[type] = false
  tabHasLoaded.value[type] = false
  loadFeed(type)
}

onMounted(() => {
  loadFeed()
})
</script>

<template>
  <div class="feed-page">
    <div class="feed-container">
      <div ref="toastContainer" class="toast-container"></div>

      <div class="feed-header">
        <div class="tab-bar">
          <button
            class="tab-btn"
            :class="{ active: activeTab === 'timeline' }"
            @click="switchTab('timeline')"
          >
            📌 关注
          </button>
          <button
            class="tab-btn"
            :class="{ active: activeTab === 'popular' }"
            @click="switchTab('popular')"
          >
            🔥 推荐
          </button>
        </div>
        <button class="btn-refresh" @click="loadFeed()" :disabled="refreshing || tabLoading">
          <span v-if="refreshing || tabLoading" class="spinner"></span>
          <span v-else>🔄 刷新</span>
        </button>
      </div>

      <div class="post-list">
        <div
          v-for="post in posts"
          :key="post.post_id"
          class="post-card animate-fade-in"
        >
          <div class="post-avatar">
            {{ post.author?.nickname?.charAt(0) || 'U' }}
          </div>
          <div class="post-body">
            <div class="post-meta">
              <span class="post-author">{{ post.author?.nickname || '未知用户' }}</span>
              <span class="post-time">{{ formatTime(getTimestamp(post)) }}</span>
              <button
                v-if="!followStatusCache[post.user_id] && (currentUserId === null || String(post.user_id) !== String(currentUserId))"
                class="btn-follow"
                @click="toggleFollow(post)"
                :disabled="post.loadingFollow"
              >
                <span v-if="post.loadingFollow">⏳</span>
                <span v-else>+ 关注</span>
              </button>
            </div>
            <p class="post-content">{{ post.content }}</p>

            <div v-if="post.media_urls?.length" class="post-media">
              <img
                v-for="(url, idx) in post.media_urls.slice(0, 3)"
                :key="idx"
                :src="url"
                alt=""
                class="media-thumb"
                @error="$event.target.style.display='none'"
              />
            </div>

            <div class="post-actions">
              <span class="action-item">❤️ {{ post.like_count || 0 }}</span>
              <span class="action-item">💬 {{ post.comment_count || 0 }}</span>
            </div>
          </div>
        </div>

        <div v-if="loadingMore" class="load-more">
          <div class="spinner"></div>
        </div>

        <div v-if="!loadingMore && tabIsEnd[activeTab] && tabHasLoaded[activeTab]" class="end-hint">
          — 没有更多了 —
        </div>

        <button
          v-if="!tabIsEnd[activeTab] && !loadingMore && posts.length > 0"
          class="btn-load-more"
          @click="loadFeed()"
        >
          加载更多
        </button>

        <div v-if="!refreshing && !loadingMore && !tabLoading && tabHasLoaded[activeTab] && posts.length === 0" class="empty-state">
          <div class="icon">{{ activeTab === 'timeline' ? '📭' : '🔥' }}</div>
          <p>{{ activeTab === 'timeline' ? '时间线还是空的，快去发布第一条动态吧！' : '还没有热门帖子' }}</p>
        </div>

        <div v-if="tabLoading" class="empty-state">
          <div class="spinner"></div>
          <p style="margin-top: 12px">加载中...</p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.feed-page {
  max-width: 640px;
  margin: 0 auto;
  padding: 24px 16px;
}

.feed-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}

.feed-header h2 {
  font-size: 22px;
  font-weight: 700;
}

.tab-bar {
  display: flex;
  gap: 4px;
  background: var(--bg-card);
  border-radius: 20px;
  padding: 3px;
  border: 1px solid var(--border);
}

.tab-btn {
  padding: 6px 18px;
  border: none;
  border-radius: 17px;
  background: transparent;
  font-size: 13px;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
  font-weight: 500;
}

.tab-btn.active {
  background: var(--accent);
  color: #fff;
}

.tab-btn:not(.active):hover {
  background: var(--border-light);
}

.btn-refresh {
  padding: 6px 16px;
  border: 1px solid var(--border);
  border-radius: 20px;
  background: var(--bg-card);
  font-size: 13px;
  color: var(--text-secondary);
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 4px;
}

.btn-refresh:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--accent);
}

.btn-refresh:disabled {
  opacity: 0.5;
}

.post-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.post-card {
  background: var(--bg-card);
  border-radius: var(--radius);
  padding: 16px;
  box-shadow: var(--shadow-sm);
  display: flex;
  gap: 12px;
}

.post-avatar {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: linear-gradient(135deg, #e94560, #764ba2);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  font-weight: 700;
  color: #fff;
  flex-shrink: 0;
}

.post-body {
  flex: 1;
  min-width: 0;
}

.post-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.post-author {
  font-weight: 600;
  font-size: 14px;
}

.post-time {
  font-size: 12px;
  color: var(--text-tertiary);
}

.btn-follow {
  font-size: 12px;
  padding: 2px 10px;
  border-radius: 12px;
  border: 1px solid var(--accent);
  background: transparent;
  color: var(--accent);
  cursor: pointer;
  transition: all 0.2s;
  font-weight: 500;
  line-height: 1.6;
}

.btn-follow:hover:not(:disabled) {
  background: var(--accent);
  color: #fff;
}

.btn-follow.btn-following {
  border-color: var(--text-tertiary);
  color: var(--text-tertiary);
  cursor: default;
}

.btn-follow.btn-following:hover {
  background: transparent;
  color: #ef4444;
  border-color: #ef4444;
}

.post-content {
  font-size: 15px;
  line-height: 1.6;
  color: var(--text-primary);
  word-break: break-word;
  white-space: pre-wrap;
}

.post-media {
  display: flex;
  gap: 6px;
  margin-top: 10px;
  flex-wrap: wrap;
}

.media-thumb {
  width: 80px;
  height: 80px;
  border-radius: 8px;
  object-fit: cover;
}

.post-actions {
  display: flex;
  gap: 16px;
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px solid var(--border-light);
}

.action-item {
  font-size: 13px;
  color: var(--text-secondary);
}

.load-more {
  text-align: center;
  padding: 16px;
}

.end-hint {
  text-align: center;
  padding: 20px;
  font-size: 13px;
  color: var(--text-tertiary);
}

.btn-load-more {
  display: block;
  width: 100%;
  padding: 12px;
  border: none;
  border-radius: var(--radius-sm);
  background: var(--bg-card);
  font-size: 14px;
  color: var(--accent);
  font-weight: 600;
  margin-top: 8px;
  transition: all 0.2s;
  box-shadow: var(--shadow-sm);
}

.btn-load-more:hover {
  box-shadow: var(--shadow-md);
}
</style>
