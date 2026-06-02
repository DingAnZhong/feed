package repository

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/pkg/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// 定义好 Redis Key 的前缀，规范化管理
const (
	feedTimelineKeyPrefix = "feed:timeline:" // 例如: feed:timeline:2001
	maxTimelineLength     = 1000             // 每个用户的收件箱最大保留 1000 条，防止 Redis 内存撑爆
)

// 将 Lua 脚本定义为常量
const pushFeedLua = `
local timeline_key = KEYS[1]
local post_score = ARGV[1]
local post_id = ARGV[2]
local max_length = tonumber(ARGV[3])

redis.call('ZADD', timeline_key, post_score, post_id)
local current_length = redis.call('ZCARD', timeline_key)
if current_length > max_length then
	local remove_count = current_length - max_length
	redis.call('ZREMRANGEBYRANK', timeline_key, 0, remove_count - 1)
end
return 1
`

// 预编译 Lua 脚本，提升执行效率
var pushScript = redis.NewScript(pushFeedLua)

// PushToTimeline 将帖子推送到一批粉丝的收件箱中 (写扩散 Push)
// 使用并发推送，每个粉丝独立上下文，单条失败不阻断其他粉丝
func PushToTimeline(ctx context.Context, followerIDs []int64, postID int64, timestamp int64) error {
	if len(followerIDs) == 0 {
		return nil
	}

	const (
		workerCount = 16 // 并发 worker 数量
	)

	type result struct {
		followerID int64
		err        error
	}

	jobCh := make(chan int64, len(followerIDs))
	resultCh := make(chan result, len(followerIDs))

	// 启动固定数量的 worker
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for followerID := range jobCh {
				key := fmt.Sprintf("%s%d", feedTimelineKeyPrefix, followerID)
				err := pushScript.Run(ctx, RDB, []string{key}, timestamp, postID, maxTimelineLength).Err()
				resultCh <- result{followerID: followerID, err: err}
			}
		}()
	}

	// 分发任务
	for _, fid := range followerIDs {
		jobCh <- fid
	}
	close(jobCh)

	// 等待所有 worker 完成
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 收集结果
	var lastErr error
	for res := range resultCh {
		if res.err != nil {
			logger.Warn("单条粉丝推送失败", zap.Error(res.err),
				zap.Int64("follower_id", res.followerID),
				zap.Int64("post_id", postID),
			)
			lastErr = fmt.Errorf("push to follower %d failed: %w", res.followerID, res.err)
		}
	}
	return lastErr
}

// PushToSelfTimeline 将帖子推送到发帖者自己的收件箱
func PushToSelfTimeline(ctx context.Context, userID int64, postID int64, timestamp int64) error {
	key := fmt.Sprintf("%s%d", feedTimelineKeyPrefix, userID)
	err := pushScript.Run(ctx, RDB, []string{key}, timestamp, postID, maxTimelineLength).Err()
	if err != nil {
		return fmt.Errorf("push to self timeline failed: %w", err)
	}
	return nil
}

// GetTimeline 从用户的收件箱拉取 Feed 流 (游标分页)
// userID: 当前登录用户
// latestTime: 游标（上一次拉取的最后一条帖子的时间戳），首次拉取传 0 表示拉全部最新
// limit: 本次拉取多少条 (通常是 10-20 条)
// 返回按时间倒序排列的帖子 ID 列表（最新的在前）
func GetTimeline(ctx context.Context, userID int64, latestTime int64, limit int) ([]int64, error) {
	key := fmt.Sprintf("%s%d", feedTimelineKeyPrefix, userID)
	var postIDs []int64

	// 如果 Redis 未初始化，返回空结果
	if RDB == nil {
		return postIDs, nil
	}

	// 使用 ZRevRangeByScore 获取时间倒序的帖子（最新的在前）
	// ZRevRangeByScore 从最大分数开始（最新的帖子）
	var result []string
	var err error

	if latestTime > 0 {
		// 有游标：获取分数严格小于 latestTime 的帖子（更旧的）
		result, err = RDB.ZRevRangeByScore(ctx, key, &redis.ZRangeBy{
			Max:   strconv.FormatInt(latestTime, 10),
			Min:   "-inf",
			Count: int64(limit),
		}).Result()
	} else {
		// 首次拉取：获取最新的 limit 条
		result, err = RDB.ZRevRangeByScore(ctx, key, &redis.ZRangeBy{
			Max:   "+inf",
			Min:   "-inf",
			Count: int64(limit),
		}).Result()
	}
	if err != nil {
		logger.Warn("GetTimeline failed")
		return postIDs, fmt.Errorf("GetTimeline failed:%w", err)
	}

	// ZRevRangeByScore 已经按时间倒序返回（最新的在前），直接解析 ID
	for _, re := range result {
		reInt, err := strconv.ParseInt(re, 10, 64)
		if err != nil {
			logger.Warn("GetTimeline ParseInt failed")
			return postIDs, fmt.Errorf("GetTimeline ParseInt failed:%w", err)
		}
		postIDs = append(postIDs, reInt)
	}
	// 调试日志：输出获取到的帖子 ID 顺序
	logger.Debug("GetTimeline return postIDs", zap.Int64s("post_ids", postIDs))
	return postIDs, nil
}

// GetPopularPosts 获取热门帖子（按点赞数降序）
func GetPopularPosts(ctx context.Context, limit int) ([]*model.Post, error) {
	var posts []*model.Post

	// 如果 DB 未初始化，返回空结果
	if DB == nil {
		return posts, nil
	}

	err := DB.WithContext(ctx).Order("like_count DESC, id DESC").Limit(limit).Find(&posts).Error
	if err != nil {
		logger.Warn("GetPopularPosts failed", zap.Error(err))
		return nil, fmt.Errorf("GetPopularPosts failed:%w", err)
	}
	return posts, nil
}

// GetTimelineFromPopularPosts 从热门帖子池拉取Feed流（拉模式）
// 用于大 V 用户的补充 Feed 来源
func GetTimelineFromPopularPosts(ctx context.Context, userID int64, latestTime int64, limit int) ([]*model.Post, error) {
	var posts []*model.Post

	// 如果 DB 未初始化，返回空结果
	if DB == nil {
		return posts, nil
	}

	// 查询热门帖子（点赞数高的）
	query := DB.WithContext(ctx).Order("like_count DESC, id DESC")

	// 游标过滤：如果 latestTime > 0，只获取比游标旧的帖子
	if latestTime > 0 {
		query = query.Where("created_at < ?", time.UnixMilli(latestTime))
	}

	err := query.Limit(limit).Find(&posts).Error
	if err != nil {
		logger.Warn("GetTimelineFromPopularPosts failed", zap.Error(err))
		return nil, fmt.Errorf("GetTimelineFromPopularPosts failed:%w", err)
	}

	return posts, nil
}
