package repository

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/pkg/logger"
	"go.uber.org/zap"
)

// LocalCacheConfig 本地缓存配置
type LocalCacheConfig struct {
	// 默认 TTL
	DefaultTTL time.Duration
	// 最大缓存项数，0 表示无限制
	MaxEntries int
	// 是否启用自动清理
	EnableCleanup bool
	// 清理间隔
	CleanupInterval time.Duration
	// 缓存前缀，避免不同环境混淆
	KeyPrefix string
}

// LocalCache 进程内本地缓存（L1 层）
// 使用 sync.RWMutex 实现并发安全
type LocalCache struct {
	mu    sync.RWMutex
	items map[string]*cacheEntry
	cfg   *LocalCacheConfig

	// 缓存统计信息
	stats struct {
		hits     int64
		misses   int64
		invalid  int64
		overflows int64
	}
}

type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

// CacheStats 缓存统计信息
type CacheStats struct {
	Hits      int64 `json:"hits"`       // 命中次数
	Misses    int64 `json:"misses"`     // 未命中次数
	Invalid   int64 `json:"invalid"`    // 过期/无效次数
	Overflows int64 `json:"overflows"`  // 溢出丢弃次数
	Items     int   `json:"items"`      // 当前缓存项数
}

// IsExpired 检查缓存条目是否过期
func (e *cacheEntry) IsExpired() bool {
	return time.Now().After(e.expiresAt)
}

// NewLocalCache 创建一个新的本地缓存
func NewLocalCache(cfg *LocalCacheConfig) *LocalCache {
	if cfg == nil {
		cfg = &LocalCacheConfig{
			DefaultTTL:      30 * time.Second,
			MaxEntries:      0, // 无限制
			EnableCleanup:   true,
			CleanupInterval: 1 * time.Minute,
			KeyPrefix:       "feed:",
		}
	}
	if cfg.DefaultTTL == 0 {
		cfg.DefaultTTL = 30 * time.Second
	}
	if cfg.CleanupInterval == 0 {
		cfg.CleanupInterval = 1 * time.Minute
	}

	lc := &LocalCache{
		items: make(map[string]*cacheEntry),
		cfg:   cfg,
	}

	// 启动自动清理
	if cfg.EnableCleanup {
		go lc.startCleanup()
	}

	return lc
}

// Put 向缓存中写入数据
func (lc *LocalCache) Put(key string, value []byte, ttl time.Duration) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// 检查是否超过最大条数限制
	if lc.cfg.MaxEntries > 0 && len(lc.items) >= lc.cfg.MaxEntries {
		lc.stats.overflows++
		// 简单策略：删除最旧的条目
		var oldestKey string
		var oldestTime time.Time
		for k, v := range lc.items {
			if oldestTime.IsZero() || v.expiresAt.Before(oldestTime) {
				oldestTime = v.expiresAt
				oldestKey = k
			}
		}
		if oldestKey != "" {
			delete(lc.items, oldestKey)
		}
	}

	if ttl == 0 {
		ttl = lc.cfg.DefaultTTL
	}

	lc.items[lc.cfg.KeyPrefix+key] = &cacheEntry{
		data:      value,
		expiresAt: time.Now().Add(ttl),
	}
}

// Get 从缓存中读取数据
func (lc *LocalCache) Get(key string) ([]byte, bool) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	entry, ok := lc.items[lc.cfg.KeyPrefix+key]
	if !ok {
		lc.stats.misses++
		return nil, false
	}
	if entry.IsExpired() {
		lc.mu.RUnlock()
		lc.mu.Lock()
		delete(lc.items, lc.cfg.KeyPrefix+key)
		lc.stats.invalid++
		lc.mu.Unlock()
		lc.mu.RLock()
		return nil, false
	}
	lc.stats.hits++
	return entry.data, true
}

// Delete 从缓存中删除数据
func (lc *LocalCache) Delete(key string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	delete(lc.items, lc.cfg.KeyPrefix+key)
}

// Clear 清空缓存
func (lc *LocalCache) Clear() {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.items = make(map[string]*cacheEntry)
}

// Size 返回缓存中的条目数
func (lc *LocalCache) Size() int {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return len(lc.items)
}

// Stats 获取缓存统计信息
func (lc *LocalCache) Stats() *CacheStats {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return &CacheStats{
		Hits:      lc.stats.hits,
		Misses:    lc.stats.misses,
		Invalid:   lc.stats.invalid,
		Overflows: lc.stats.overflows,
		Items:     len(lc.items),
	}
}

// ResetStats 重置统计信息（用于测试）
func (lc *LocalCache) ResetStats() {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.stats = struct {
		hits     int64
		misses   int64
		invalid  int64
		overflows int64
	}{}
}

// startCleanup 定期清理过期条目
func (lc *LocalCache) startCleanup() {
	ticker := time.NewTicker(lc.cfg.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		lc.cleanup()
	}
}

// cleanup 清理过期条目
func (lc *LocalCache) cleanup() {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	now := time.Now()
	for key, entry := range lc.items {
		if now.After(entry.expiresAt) {
			delete(lc.items, key)
			lc.stats.invalid++
		}
	}
}

// --- 全局本地缓存实例 ---
// 默认配置
var (
	defaultLocalCacheCfg = &LocalCacheConfig{
		DefaultTTL:      30 * time.Second,
		MaxEntries:      10000, // 最多缓存 10000 项
		EnableCleanup:   true,
		CleanupInterval: 1 * time.Minute,
		KeyPrefix:       "feed:",
	}
	localCache = NewLocalCache(defaultLocalCacheCfg)
)

// PostCacheTTL 帖子缓存的 TTL
const PostCacheTTL = 30 * time.Second

// FollowStatsCacheTTL 关注统计缓存的 TTL
const FollowStatsCacheTTL = 10 * time.Second

// PopularPostsCacheTTL 热门帖子缓存的 TTL
const PopularPostsCacheTTL = 1 * time.Minute

// UserPostsCacheTTL 用户帖子列表缓存的 TTL
const UserPostsCacheTTL = 30 * time.Second

// --- 帖子缓存 (L1 本地缓存 + L2 Redis 降级) ---

func cachePostKey(postID int64) string {
	return "cache:post:" + itos(postID)
}

// CacheGetPost 从多级缓存获取帖子详情
// 先查 L1 本地缓存，miss 后查 L2 Redis，再 miss 查 DB 并回填两级缓存
func CacheGetPost(ctx context.Context, postID int64) (*model.Post, error) {
	// L1: 本地缓存
	key := cachePostKey(postID)
	if data, ok := localCache.Get(key); ok {
		var post model.Post
		if err := json.Unmarshal(data, &post); err == nil {
			logger.Debug("local cache hit for post", zap.Int64("post_id", postID))
			return &post, nil
		}
	}

	// L2: Redis
	if RDB != nil {
		redisKey := "redis:post:" + itos(postID)
		data, err := RDB.Get(ctx, redisKey).Bytes()
		if err == nil {
			var post model.Post
			if err := json.Unmarshal(data, &post); err == nil {
				logger.Debug("redis cache hit for post", zap.Int64("post_id", postID))
				// 回填 L1
				encoded, _ := json.Marshal(post)
				localCache.Put(key, encoded, PostCacheTTL)
				return &post, nil
			}
		}
	}

	// L3: 数据库
	var post model.Post
	err := DB.WithContext(ctx).Where("id = ?", postID).First(&post).Error
	if err != nil {
		return nil, err
	}

	// 回填 L2 Redis
	if RDB != nil {
		encoded, _ := json.Marshal(post)
		RDB.Set(ctx, "redis:post:"+itos(postID), encoded, 60*time.Second)
	}

	// 回填 L1 本地缓存
	encoded, _ := json.Marshal(post)
	localCache.Put(key, encoded, PostCacheTTL)

	return &post, nil
}

// CacheInvalidatePost 使某个帖子的缓存失效（两级同时清除）
func CacheInvalidatePost(postID int64) {
	key := cachePostKey(postID)
	localCache.Delete(key)
	if RDB != nil {
		RDB.Del(context.Background(), "redis:post:"+itos(postID))
	}
}

// CacheSetPost 将帖子写入多级缓存（用于主动缓存预热）
func CacheSetPost(post *model.Post) {
	encoded, _ := json.Marshal(post)
	key := cachePostKey(post.ID)
	localCache.Put(key, encoded, PostCacheTTL)
	if RDB != nil {
		RDB.Set(context.Background(), "redis:post:"+itos(post.ID), encoded, 60*time.Second)
	}
}

// --- 用户信息缓存 (L1 本地缓存 + L2 Redis 降级) ---

// CacheGetUser 从多级缓存获取用户信息
func CacheGetUser(ctx context.Context, userID int64) (*model.User, error) {
	key := "cache:user:" + itos(userID)
	if data, ok := localCache.Get(key); ok {
		var user model.User
		if err := json.Unmarshal(data, &user); err == nil {
			logger.Debug("local cache hit for user", zap.Int64("user_id", userID))
			return &user, nil
		}
	}

	if RDB != nil {
		data, err := RDB.Get(ctx, "redis:user:"+itos(userID)).Bytes()
		if err == nil {
			var user model.User
			if err := json.Unmarshal(data, &user); err == nil {
				encoded, _ := json.Marshal(user)
				localCache.Put(key, encoded, PostCacheTTL)
				return &user, nil
			}
		}
	}

	// DB fallback
	user, err := GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}

	encoded, _ := json.Marshal(user)
	if RDB != nil {
		RDB.Set(ctx, "redis:user:"+itos(userID), encoded, 60*time.Second)
	}
	localCache.Put(key, encoded, PostCacheTTL)

	return user, nil
}

// CacheInvalidateUser 使某个用户的缓存失效
func CacheInvalidateUser(userID int64) {
	localCache.Delete("cache:user:" + itos(userID))
	if RDB != nil {
		RDB.Del(context.Background(), "redis:user:"+itos(userID))
	}
}

// CacheSetUser 主动设置用户缓存
func CacheSetUser(user *model.User) {
	encoded, _ := json.Marshal(user)
	key := "cache:user:" + itos(user.ID)
	localCache.Put(key, encoded, PostCacheTTL)
	if RDB != nil {
		RDB.Set(context.Background(), "redis:user:"+itos(user.ID), encoded, 60*time.Second)
	}
}

// --- 关注统计缓存 (L1 本地缓存 + L2 Redis 降级) ---

func cacheFollowStatsKey(userID int64) string {
	return "cache:follow:stats:" + itos(userID)
}

// FollowStats 关注/粉丝统计
type FollowStats struct {
	FollowingCount int64 `json:"following_count"`
	FollowerCount  int64 `json:"follower_count"`
}

// CacheGetFollowStats 从多级缓存获取关注/粉丝统计
func CacheGetFollowStats(ctx context.Context, userID int64) (*FollowStats, error) {
	key := cacheFollowStatsKey(userID)

	// L1: 本地缓存
	if data, ok := localCache.Get(key); ok {
		var stats FollowStats
		if err := json.Unmarshal(data, &stats); err == nil {
			logger.Debug("local cache hit for follow stats", zap.Int64("user_id", userID))
			return &stats, nil
		}
	}

	// L2: Redis
	if RDB != nil {
		data, err := RDB.Get(ctx, "redis:follow:stats:"+itos(userID)).Bytes()
		if err == nil {
			var stats FollowStats
			if err := json.Unmarshal(data, &stats); err == nil {
				encoded, _ := json.Marshal(stats)
				localCache.Put(key, encoded, FollowStatsCacheTTL)
				return &stats, nil
			}
		}
	}

	// L3: DB 降级
	followingCount, followerCount, err := GetUserFollowStats(ctx, userID)
	if err != nil {
		return nil, err
	}

	stats := &FollowStats{
		FollowingCount: followingCount,
		FollowerCount:  followerCount,
	}

	// 回填 L2
	if RDB != nil {
		encoded, _ := json.Marshal(stats)
		RDB.Set(ctx, "redis:follow:stats:"+itos(userID), encoded, 30*time.Second)
	}
	// 回填 L1
	encoded, _ := json.Marshal(stats)
	localCache.Put(key, encoded, FollowStatsCacheTTL)

	return stats, nil
}

// CacheInvalidateFollowStats 使关注统计缓存失效
func CacheInvalidateFollowStats(userID int64) {
	key := cacheFollowStatsKey(userID)
	localCache.Delete(key)
	if RDB != nil {
		RDB.Del(context.Background(), "redis:follow:stats:"+itos(userID))
	}
}

// CacheSetFollowStats 主动设置关注统计缓存
func CacheSetFollowStats(userID int64, followingCount, followerCount int64) {
	stats := &FollowStats{
		FollowingCount: followingCount,
		FollowerCount:  followerCount,
	}
	key := cacheFollowStatsKey(userID)
	encoded, _ := json.Marshal(stats)
	localCache.Put(key, encoded, FollowStatsCacheTTL)
	if RDB != nil {
		RDB.Set(context.Background(), "redis:follow:stats:"+itos(userID), encoded, 30*time.Second)
	}
}

// --- 热门帖子缓存 (L1 本地缓存 + L2 Redis 降级) ---

const cachePopularPostsKey = "cache:popular:posts"
const cachePopularPostsTTL = 1 * time.Minute

// CacheGetPopularPosts 从多级缓存获取热门帖子
func CacheGetPopularPosts(ctx context.Context, limit int) ([]*model.Post, error) {
	// L1: 本地缓存
	if data, ok := localCache.Get(cachePopularPostsKey); ok {
		var posts []*model.Post
		if err := json.Unmarshal(data, &posts); err == nil {
			logger.Debug("local cache hit for popular posts")
			return posts, nil
		}
	}

	// L2: Redis
	if RDB != nil {
		data, err := RDB.Get(ctx, "redis:popular:posts").Bytes()
		if err == nil {
			var posts []*model.Post
			if err := json.Unmarshal(data, &posts); err == nil {
				encoded, _ := json.Marshal(posts)
				localCache.Put(cachePopularPostsKey, encoded, cachePopularPostsTTL)
				return posts, nil
			}
		}
	}

	// L3: DB
	posts, err := GetPopularPosts(ctx, limit)
	if err != nil {
		return nil, err
	}

	// 回填
	encoded, _ := json.Marshal(posts)
	if RDB != nil {
		RDB.Set(ctx, "redis:popular:posts", encoded, 45*time.Second)
	}
	localCache.Put(cachePopularPostsKey, encoded, cachePopularPostsTTL)

	return posts, nil
}

// CacheInvalidatePopularPosts 使热门帖子缓存失效
func CacheInvalidatePopularPosts() {
	localCache.Delete(cachePopularPostsKey)
	if RDB != nil {
		RDB.Del(context.Background(), "redis:popular:posts")
	}
}

// --- 用户帖子列表缓存 (L1 本地缓存 + L2 Redis 降级) ---

// CacheGetUserPosts 从多级缓存获取用户帖子列表
func CacheGetUserPosts(ctx context.Context, userID int64, limit int) ([]*model.Post, error) {
	key := "cache:userposts:" + itos(userID)
	if data, ok := localCache.Get(key); ok {
		var posts []*model.Post
		if err := json.Unmarshal(data, &posts); err == nil {
			logger.Debug("local cache hit for user posts", zap.Int64("user_id", userID))
			return posts, nil
		}
	}

	if RDB != nil {
		data, err := RDB.Get(ctx, "redis:userposts:"+itos(userID)).Bytes()
		if err == nil {
			var posts []*model.Post
			if err := json.Unmarshal(data, &posts); err == nil {
				encoded, _ := json.Marshal(posts)
				localCache.Put(key, encoded, PostCacheTTL)
				return posts, nil
			}
		}
	}

	posts, err := GetPostsByUserID(ctx, userID, limit)
	if err != nil {
		return nil, err
	}

	encoded, _ := json.Marshal(posts)
	if RDB != nil {
		RDB.Set(ctx, "redis:userposts:"+itos(userID), encoded, 45*time.Second)
	}
	localCache.Put(key, encoded, PostCacheTTL)

	return posts, nil
}

// CacheInvalidateUserPosts 使用户帖子列表缓存失效
func CacheInvalidateUserPosts(userID int64) {
	localCache.Delete("cache:userposts:" + itos(userID))
	if RDB != nil {
		RDB.Del(context.Background(), "redis:userposts:"+itos(userID))
	}
}

// itos 高效 int64 -> string
func itos(n int64) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for n < 0 || n > 0 {
		if n < 0 {
			buf = append(buf, byte('-'))
			n = -n
		}
		d := n % 10
		buf = append(buf, byte('0'+d))
		n /= 10
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

// ClearLocalCache 清空本地缓存（测试用）
func ClearLocalCache() {
	localCache.Clear()
}

// GetLocalCacheStats 获取全局本地缓存统计信息
func GetLocalCacheStats() *CacheStats {
	return localCache.Stats()
}

// ResetLocalCacheStats 重置全局本地缓存统计信息
func ResetLocalCacheStats() {
	localCache.ResetStats()
}

// InitializeLocalCache 初始化全局本地缓存（可在配置加载后调用）
func InitializeLocalCache(cfg *LocalCacheConfig) {
	localCache = NewLocalCache(cfg)
}