package repository

import (
	"context"
	"fmt"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/pkg/logger"
	"go.uber.org/zap"
)

// CreatePost 存入一条新帖子
func CreatePost(ctx context.Context, post *model.Post) error {
	err := DB.WithContext(ctx).Create(post).Error
	if err != nil {
		logger.Warn("CreatePost failed", zap.Error(err))
		return fmt.Errorf("CreatePost failed:%w", err)
	}
	return nil
}

// GetPostsByIDs 根据一组 ID 批量查询帖子详情
func GetPostsByIDs(ctx context.Context, postIDs []int64) ([]*model.Post, error) {
	var posts []*model.Post

	if len(postIDs) == 0 {
		return posts, nil
	}

	err := DB.WithContext(ctx).Where("id IN ?", postIDs).Find(&posts).Error
	if err != nil {
		return posts, fmt.Errorf("GetPostsByIDs failed:%w", err)
	}
	return posts, nil
}
