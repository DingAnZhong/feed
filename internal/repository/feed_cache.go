package repository

import (
	"context"
	"fmt"
	"strconv"

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
// followerIDs: 粉丝的 ID 列表 (可能有一千个)
// postID: 帖子 ID
// timestamp: 帖子发布的时间戳 (用作 ZSet 的 Score)
func PushToTimeline(ctx context.Context, followerIDs []int64, postID int64, timestamp int64) error {
	if len(followerIDs) == 0 {
		return nil
	}

	pipe := RDB.Pipeline()
	for _, followerID := range followerIDs {
		key := fmt.Sprintf("%s%d", feedTimelineKeyPrefix, followerID)
		pushScript.Run(ctx, pipe, []string{key}, timestamp, postID, maxTimelineLength)
	}
	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.Warn("PushToTimeLine failed", zap.Error(err))
		return fmt.Errorf("PushToTimeLine failed:%w", err)
	}

	return nil
}

// GetTimeline 从用户的收件箱拉取 Feed 流 (游标分页)
// userID: 当前登录用户
// latestTime: 游标（上一次拉取的最后一条帖子的时间戳），如果是第一次拉取，传系统的当前时间戳
// limit: 本次拉取多少条 (通常是 10-20 条)
func GetTimeline(ctx context.Context, userID int64, latestTime int64, limit int) ([]int64, error) {
	key := fmt.Sprintf("%s%d", feedTimelineKeyPrefix, userID)
	var postIDs []int64

	opt := &redis.ZRangeArgs{
		Key:     key,
		Start:   fmt.Sprintf("(%d", latestTime),
		Stop:    "-inf",
		ByScore: true,
		Rev:     true,
		Offset:  0,
		Count:   int64(limit),
	}
	result, err := RDB.ZRangeArgs(ctx, *opt).Result()
	if err != nil {
		logger.Warn("GetTimeLine failed")
		return postIDs, fmt.Errorf("GetTimeLine failed:%w", err)
	}
	for _, re := range result {
		reInt, err := strconv.ParseInt(re, 10, 64)
		if err != nil {
			logger.Warn("GetTimeLine ParseInt failed")
			return postIDs, fmt.Errorf("GetTimeLine ParseInt failed:%w", err)
		}
		postIDs = append(postIDs, reInt)
	}
	return postIDs, nil
}
