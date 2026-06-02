package service

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/DingAnZhong/feed/internal/filter"
	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/internal/mq"
	"github.com/DingAnZhong/feed/internal/repository"
	"github.com/DingAnZhong/feed/pkg/logger"
	"github.com/DingAnZhong/feed/pkg/snowflake"
	"go.uber.org/zap"
)

var (
	ErrContentEmpty   = errors.New("帖子内容不能为空")
	ErrContentTooLong = errors.New("帖子内容太长")
)

// Post status constants
const (
	PostStatusNormal    = 0 // 正常
	PostStatusReviewing = 1 // 审核中
	PostStatusRejected  = 2 // 审核不通过
)

// PublishPost 处理用户发帖的核心业务逻辑
// 返回生成的 postID 和 error
func PublishPost(ctx context.Context, userID int64, content string, mediaUrls []string) (int64, error) {
	if content == "" {
		return 0, ErrContentEmpty
	}
	if utf8.RuneCountInString(content) > 500 {
		return 0, ErrContentTooLong
	}
	postID, err := snowflake.GenerateID()
	if err != nil {
		return 0, fmt.Errorf("postID generate failed:%w", err)
	}
	now := time.Now()

	post := &model.Post{
		ID:        postID,
		UserID:    userID,
		Content:   content,
		MediaUrls: mediaUrls,
		Status:    PostStatusNormal, // 默认为正常状态
		CreatedAt: now,
	}
	err = repository.CreatePost(ctx, post)
	if err != nil {
		logger.Error("repository.CreatePost(ctx, post)", zap.Error(err))
		return 0, fmt.Errorf("publish post create post failed: %w", err)
	}
	
	// 检查是否包含敏感词
	sensitiveWord := filter.DetectSensitiveWord(content)
	if sensitiveWord != "" {
		// 命中敏感词，标记为审核不通过
		logger.Info("post contains sensitive word",
			zap.Int64("user_id", userID),
			zap.Int64("post_id", postID),
			zap.String("sensitive_word", sensitiveWord),
		)
		repository.UpdatePostStatus(ctx, postID, PostStatusRejected)
		// 审核不通过的帖子不发送 Kafka 事件
		return postID, fmt.Errorf("post contains sensitive word: %s", sensitiveWord)
	}

	// 未命中敏感词，发送 Kafka 事件进行推送到粉丝
	event := &model.PostPublishEvent{
		PostID:    postID,
		UserID:    userID,
		Timestamp: now.UnixMilli(),
	}

	err = mq.SendPostPublishEvent(event)
	if err != nil {
		logger.Error("Kafka 投递失败，转入异步补偿", zap.Error(err))
	}

	return postID, nil
}
