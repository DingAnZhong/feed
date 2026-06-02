// 开发环境 (Vite proxy): /api -> http://127.0.0.1:8080 (dev 时后端也是 /api)
// 生产环境 (Go embed): API 在 /web/api
const isProd = !import.meta.env.DEV
const API_PREFIX = isProd ? '/web/api' : ''

function getUserId() {
  return localStorage.getItem('feed_user_id')
}

async function request(url, options = {}) {
  const headers = {
    'Content-Type': 'application/json',
    ...options.headers,
  }
  const userId = getUserId()
  if (userId) {
    headers['X-User-ID'] = userId
  }

  const config = {
    ...options,
    headers,
  }

  try {
    const response = await fetch(API_PREFIX + url, config)
    const data = await response.json()
    if (data.code !== 0) {
      throw new Error(data.msg || '请求失败')
    }
    return data.data
  } catch (err) {
    throw err
  }
}

export const api = {
  publishPost(content, mediaUrls = []) {
    return request('/v1/post/publish', {
      method: 'POST',
      body: JSON.stringify({ content, media_urls: mediaUrls }),
    })
  },
  fetchFeed(latestTime = 0, limit = 10, feedType = 'timeline') {
    const params = new URLSearchParams({
      latest_time: String(latestTime),
      limit: String(limit),
      feed_type: feedType,
    })
    return request('/v1/feed/timeline?' + params.toString())
  },
  followAction(followeeId, actionType) {
    return request('/v1/user/follow', {
      method: 'POST',
      body: JSON.stringify({ followee_id: followeeId, action_type: actionType }),
    })
  },
  checkFollowStatus(followeeId) {
    return request(`/v1/user/follow/status?followee_id=${followeeId}`)
  },
  getUserInfo(userId) {
    return request(`/v1/user/info?user_id=${userId}`)
  },
}
