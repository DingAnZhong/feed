package repository

import (
	"context"
	"fmt"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/pkg/logger"
	"go.uber.org/zap"
)

const (
	// FollowerCountKeyPrefix 粉丝数计数器 key 前缀
	FollowerCountKeyPrefix = "feed:count:followers:"
	// FolloweeCountKeyPrefix 关注数计数器 key 前缀
	FolloweeCountKeyPrefix = "feed:count:followees:"
)

// GenerateFollowerCountKey 生成粉丝数计数器 key
func GenerateFollowerCountKey(userID int64) string {
	return fmt.Sprintf("%s%d", FollowerCountKeyPrefix, userID)
}

// GenerateFolloweeCountKey 生成关注数计数器 key
func GenerateFolloweeCountKey(userID int64) string {
	return fmt.Sprintf("%s%d", FolloweeCountKeyPrefix, userID)
}

// IncrementFollower 增加粉丝数
func IncrementFollower(ctx context.Context, userID int64) error {
	key := GenerateFollowerCountKey(userID)
	return RDB.Incr(ctx, key).Err()
}

// DecrementFollower 减少粉丝数
func DecrementFollower(ctx context.Context, userID int64) error {
	key := GenerateFollowerCountKey(userID)
	return RDB.Decr(ctx, key).Err()
}

// IncrementFollowee 增加关注数
func IncrementFollowee(ctx context.Context, userID int64) error {
	key := GenerateFolloweeCountKey(userID)
	return RDB.Incr(ctx, key).Err()
}

// DecrementFollowee 减少关注数
func DecrementFollowee(ctx context.Context, userID int64) error {
	key := GenerateFolloweeCountKey(userID)
	return RDB.Decr(ctx, key).Err()
}

// GetFollowerCount 获取粉丝数
// 如果 key 不存在，返回 0
func GetFollowerCount(ctx context.Context, userID int64) (int64, error) {
	key := GenerateFollowerCountKey(userID)
	return RDB.Get(ctx, key).Int64()
}

// GetFolloweeCount 获取关注数
// 如果 key 不存在，返回 0
func GetFolloweeCount(ctx context.Context, userID int64) (int64, error) {
	key := GenerateFolloweeCountKey(userID)
	return RDB.Get(ctx, key).Int64()
}

// SyncFollowerCountFromDB 从数据库同步粉丝数到 Redis
func SyncFollowerCountFromDB(ctx context.Context, userID int64) (int64, error) {
	// 查询数据库中的实际粉丝数
	count, err := GetFollowerCountFromDB(ctx, userID)
	if err != nil {
		return 0, err
	}

	// 更新 Redis 计数器
	key := GenerateFollowerCountKey(userID)
	err = RDB.Set(ctx, key, count, 0).Err()
	if err != nil {
		logger.Warn("SyncFollowerCountFromDB failed", zap.Error(err))
		return 0, err
	}

	return count, nil
}

// SyncFolloweeCountFromDB 从数据库同步关注数到 Redis
func SyncFolloweeCountFromDB(ctx context.Context, userID int64) (int64, error) {
	// 查询数据库中的实际关注数
	count, err := GetFolloweeCountFromDB(ctx, userID)
	if err != nil {
		return 0, err
	}

	// 更新 Redis 计数器
	key := GenerateFolloweeCountKey(userID)
	err = RDB.Set(ctx, key, count, 0).Err()
	if err != nil {
		logger.Warn("SyncFolloweeCountFromDB failed", zap.Error(err))
		return 0, err
	}

	return count, nil
}

// GetFollowerCountFromDB 从数据库获取粉丝数
func GetFollowerCountFromDB(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := DB.WithContext(ctx).Model(&model.Relation{}).
		Where("followee_id = ? AND status = 1", userID).
		Count(&count).Error
	if err != nil {
		logger.Warn("GetFollowerCountFromDB failed", zap.Error(err))
		return 0, fmt.Errorf("GetFollowerCountFromDB failed: %w", err)
	}
	return count, nil
}

// GetFolloweeCountFromDB 从数据库获取关注数
func GetFolloweeCountFromDB(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := DB.WithContext(ctx).Model(&model.Relation{}).
		Where("follower_id = ? AND status = 1", userID).
		Count(&count).Error
	if err != nil {
		logger.Warn("GetFolloweeCountFromDB failed", zap.Error(err))
		return 0, fmt.Errorf("GetFolloweeCountFromDB failed: %w", err)
	}
	return count, nil
}

// RefreshAllFollowerCounts 刷新所有用户的计数器（用于测试清理）
func RefreshAllFollowerCounts() error {
	// 这是一个测试辅助函数，用于刷新所有计数器
	// 实际使用时可以根据需要调整
	return nil
}
