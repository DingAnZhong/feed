package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CreateUser 创建新用户
func CreateUser(ctx context.Context, user *model.User) error {
	err := DB.WithContext(ctx).Create(user).Error
	if err != nil {
		return fmt.Errorf("new user create failed:%w", err)
	}
	return nil
}

// GetUserByID 根据 ID 获取用户信息（直接查数据库，不走缓存）
// 避免与 CacheGetUser 形成循环依赖
// L1 本地缓存 → L2 Redis → L3 数据库
func GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	// 直接查询数据库
	var user model.User
	err := DB.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Info("no such user", zap.Int64("user_id", id))
			return nil, nil
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	return &user, nil
}

// FollowUser 关注/取消关注操作
// status: 1 表示关注，0 表示取消关注
func FollowUser(ctx context.Context, followerID, followeeID int64, status int8) error {
	// 判断是否是关注操作（status=1）
	isFollowing := status == 1

	// 检查当前状态
	var relation model.Relation
	err := DB.WithContext(ctx).Where("follower_id = ? AND followee_id = ?", followerID, followeeID).First(&relation).Error
	
	if err == nil {
		// 用户已存在关系，判断是否是状态变更
		logger.Info("FollowUser relation exists",
			zap.Int64("follower_id", followerID),
			zap.Int64("followee_id", followeeID),
			zap.Int8("old_status", relation.Status),
			zap.Int8("new_status", status))
		if relation.Status == status {
			// 状态未变，直接返回
			logger.Info("FollowUser status unchanged, return")
			return nil
		}

		// 状态变更：从关注变为取关 或 从取关变为关注
		if relation.Status == 1 && status == 0 {
			// 取关：减少双方计数器
			logger.Info("FollowUser unfollowing")
			if err := DecrementFollowee(ctx, followerID); err != nil {
				logger.Warn("Decrease followee count failed", zap.Error(err))
			}
			if err := DecrementFollower(ctx, followeeID); err != nil {
				logger.Warn("Decrease follower count failed", zap.Error(err))
			}
		} else if relation.Status == 0 && status == 1 {
			// 关注：增加双方计数器
			logger.Info("FollowUser following")
			if err := IncrementFollowee(ctx, followerID); err != nil {
				logger.Warn("Increase followee count failed", zap.Error(err))
			}
			if err := IncrementFollower(ctx, followeeID); err != nil {
				logger.Warn("Increase follower count failed", zap.Error(err))
			}
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) && isFollowing {
		// 新增关注关系：增加双方计数器
		logger.Info("FollowUser new relationship")
		if err := IncrementFollowee(ctx, followerID); err != nil {
			logger.Warn("Increase followee count failed", zap.Error(err))
		}
		if err := IncrementFollower(ctx, followeeID); err != nil {
			logger.Warn("Increase follower count failed", zap.Error(err))
		}
	}

	// 执行数据库操作
	// 使用 ON CONFLICT DO UPDATE 来处理存在则更新，不存在则插入
	// 这是 MySQL 的语法
	err = DB.WithContext(ctx).Exec(`
		INSERT INTO relations (follower_id, followee_id, status, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE
			status = VALUES(status),
			updated_at = VALUES(updated_at)
	`, followerID, followeeID, status).Error
	if err != nil {
		logger.Warn("关注功能异常", zap.Error(err))
		return fmt.Errorf("关注功能异常:%w", err)
	}
	return nil
}

// IsFollowing 检查 followerID 是否已关注 followeeID
func IsFollowing(ctx context.Context, followerID, followeeID int64) bool {
	var count int64
	err := DB.WithContext(ctx).Model(&model.Relation{}).
		Where("follower_id = ? AND followee_id = ? AND status = 1", followerID, followeeID).
		Count(&count).Error
	if err != nil {
		logger.Warn("IsFollowing failed", zap.Error(err),
			zap.Int64("follower_id", followerID),
			zap.Int64("followee_id", followeeID),
		)
		return false
	}
	return count > 0
}

// GetFollowerIDs 获取某个大V的所有粉丝 ID（分批查询，防止大V粉丝过多时 OOM）
func GetFollowerIDs(ctx context.Context, followeeID int64) ([]int64, error) {
	const batchSize = 500
	var allFollowerIDs []int64
	offset := 0

	for {
		var batch []int64
		err := DB.WithContext(ctx).Model(&model.Relation{}).
			Where("followee_id = ? AND status = 1", followeeID).
			Order("follower_id ASC").
			Offset(offset).
			Limit(batchSize).
			Pluck("follower_id", &batch).Error
		if err != nil {
			logger.Warn("GetFollowerIDs failed", zap.Error(err))
			return allFollowerIDs, fmt.Errorf("GetFollowerIDs failed:%w", err)
		}
		if len(batch) == 0 {
			break
		}
		allFollowerIDs = append(allFollowerIDs, batch...)
		offset += batchSize
		if len(batch) < batchSize {
			break
		}
	}
	return allFollowerIDs, nil
}

// GetUserFollowStats 获取用户的关注数和粉丝数
// 注意：此函数直接查询数据库，不使用缓存，避免与 CacheGetFollowStats 形成循环依赖
func GetUserFollowStats(ctx context.Context, userID int64) (followingCount, followerCount int64, err error) {
	// 查询关注数
	err = DB.WithContext(ctx).Model(&model.Relation{}).
		Where("follower_id = ? AND status = 1", userID).
		Count(&followingCount).Error
	if err != nil {
		return 0, 0, fmt.Errorf("GetUserFollowStats get following failed: %w", err)
	}

	// 查询粉丝数
	err = DB.WithContext(ctx).Model(&model.Relation{}).
		Where("followee_id = ? AND status = 1", userID).
		Count(&followerCount).Error
	if err != nil {
		return 0, 0, fmt.Errorf("GetUserFollowStats get follower failed: %w", err)
	}

	return followingCount, followerCount, nil
}
