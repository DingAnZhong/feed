package service

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

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
		CreatedAt: now,
	}
	err = repository.CreatePost(ctx, post)
	if err != nil {
		logger.Error("repository.CreatePost(ctx, post)", zap.Error(err))
		return 0, fmt.Errorf("publishpost createpost failed%w", err)
	}
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
