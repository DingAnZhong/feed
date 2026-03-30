package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/DingAnZhong/feed/internal/repository"
	"github.com/DingAnZhong/feed/pkg/logger"
	"go.uber.org/zap"
)

// 定义业务层专属的错误，方便向外抛出并统一处理
var (
	ErrCannotFollowSelf = errors.New("不能关注自己")
	ErrUserNotFound     = errors.New("目标用户不存在")
	ErrInvalidAction    = errors.New("无效的操作类型")
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
