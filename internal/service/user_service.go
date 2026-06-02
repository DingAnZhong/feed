package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/internal/repository"
	"github.com/DingAnZhong/feed/pkg/logger"
	"go.uber.org/zap"
)

// 定义业务层专属的错误，方便向外抛出并统一处理
var (
	ErrCannotFollowSelf = errors.New("不能关注自己")
	ErrUserNotFound     = errors.New("目标用户不存在")
	ErrInvalidAction    = errors.New("无效的操作类型")
	ErrUserAlreadyExists = errors.New("用户已存在")
)

// FollowAction 处理关注与取消关注的业务逻辑
// followerID: 发起操作的用户 ID (当前登录用户)
// followeeID: 被操作的目标大V ID
// actionType: 动作类型 (API 层面通常传: 1-关注, 2-取消关注)
func FollowAction(ctx context.Context, followerID int64, followeeID int64, actionType int) error {
	if followeeID == followerID {
		logger.Warn("followerID 和 followeeID 相等,返回 ErrCannotFollowSelf")
		return ErrCannotFollowSelf
	}

	user, err := repository.GetUserByID(ctx, followeeID)
	if err != nil {
		return fmt.Errorf("repository.GetUserByID(ctx, followeeID):%w", err)
	}
	if user == nil {
		logger.Warn("查找不存在的用户")
		return ErrUserNotFound
	}

	var status int8
	switch actionType {
	case 1:
		status = 1
	case 2:
		status = 0
	default:
		return ErrInvalidAction
	}

	err = repository.FollowUser(ctx, followerID, followeeID, status)
	if err != nil {
		logger.Warn("关注数据持久化失败",
			zap.Int64("follower_id", followerID),
			zap.Int64("followee_id", followeeID),
			zap.Error(err),
		)
		return fmt.Errorf("FollowAction to DB failed:%w", err)
	}
	return nil
}

// IsFollowing 检查 followerID 是否已关注 followeeID
func IsFollowing(ctx context.Context, followerID int64, followeeID int64) bool {
	return repository.IsFollowing(ctx, followerID, followeeID)
}

// GetUserInfo 获取用户信息
func GetUserInfo(ctx context.Context, userID int64) (*model.User, error) {
	return repository.GetUserByID(ctx, userID)
}

// GetUserFollowStats 获取用户的关注数和粉丝数
func GetUserFollowStats(ctx context.Context, userID int64) (int64, int64, error) {
	return repository.GetUserFollowStats(ctx, userID)
}

// SyncUserCountFromDB 从数据库同步用户关注/粉丝计数到 Redis
func SyncUserCountFromDB(ctx context.Context, userID int64) error {
	// 同步关注数
	_, err := repository.SyncFolloweeCountFromDB(ctx, userID)
	if err != nil {
		return err
	}
	// 同步粉丝数
	_, err = repository.SyncFollowerCountFromDB(ctx, userID)
	if err != nil {
		return err
	}
	return nil
}

// RegisterUser 注册用户
func RegisterUser(ctx context.Context, userID int64, nickname string) error {
	user, err := repository.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("检查用户存在性失败: %w", err)
	}
	if user != nil {
		return ErrUserAlreadyExists
	}

	registeredUser := &model.User{
		ID:       userID,
		Nickname: nickname,
	}

	err = repository.CreateUser(ctx, registeredUser)
	if err != nil {
		logger.Warn("用户注册失败",
			zap.Int64("user_id", userID),
			zap.String("nickname", nickname),
			zap.Error(err),
		)
		return fmt.Errorf("注册失败: %w", err)
	}

	logger.Info("用户注册成功",
		zap.Int64("user_id", userID),
		zap.String("nickname", nickname),
	)
	return nil
}
