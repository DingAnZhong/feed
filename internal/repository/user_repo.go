package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateUser 创建新用户
func CreateUser(ctx context.Context, user *model.User) error {
	err := DB.WithContext(ctx).Create(user).Error
	if err != nil {
		return fmt.Errorf("new user create failed:%w", err)
	}
	return nil
}

// GetUserByID 根据 ID 获取用户信息
func GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	var user model.User

	err := DB.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Info("no such user")
			return nil, nil
		}
		logger.Warn("GetUserByID error", zap.Error(err))
		return nil, fmt.Errorf("查询错误，并不是没数据:%w", err)
	}
	return &user, nil
}

// FollowUser 关注/取消关注操作
// status: 1 表示关注，0 表示取消关注
func FollowUser(ctx context.Context, followerID, followeeID int64, status int8) error {

	relation := model.Relation{
		FollowerID: followerID,
		FolloweeID: followeeID,
		Status:     status,
	}
	err := DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "follower_id"}, {Name: "followee_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"status"}),
	}).Create(&relation).Error
	if err != nil {
		logger.Warn("关注功能异常", zap.Error(err))
		return fmt.Errorf("关注功能异常:%w", err)
	}
	return nil
}

// GetFollowerIDs 获取某个大V的所有粉丝 ID
func GetFollowerIDs(ctx context.Context, followeeID int64) ([]int64, error) {
	var followerIDs []int64

	err := DB.WithContext(ctx).Model(&model.Relation{}).
		Where("followee_id = ? AND status = 1", followeeID).
		Pluck("follower_id", &followerIDs).Error
	if err != nil {
		logger.Warn("GetFollowerIDs failed", zap.Error(err))
		return followerIDs, fmt.Errorf("GetFollowerIDs failed:%w", err)
	}
	return followerIDs, nil
}
