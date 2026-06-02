package repository

import (
	"context"
	"fmt"
	"strings"

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

	// 如果 DB 未初始化，返回空结果
	if DB == nil {
		return posts, nil
	}

	// 使用 FIELD 保证返回顺序与 postIDs 一致
	idPlaceholders := make([]string, len(postIDs))
	for i, id := range postIDs {
		idPlaceholders[i] = fmt.Sprintf("%d", id)
	}
	orderClause := fmt.Sprintf("FIELD(id, %s)", strings.Join(idPlaceholders, ","))

	err := DB.WithContext(ctx).Where("id IN ?", postIDs).Order(orderClause).Find(&posts).Error
	if err != nil {
		return posts, fmt.Errorf("GetPostsByIDs failed:%w", err)
	}
	return posts, nil
}

// GetPostsByUserID 根据用户 ID 查询该用户最近 N 篇帖子
func GetPostsByUserID(ctx context.Context, userID int64, limit int) ([]*model.Post, error) {
	var posts []*model.Post

	// 如果 DB 未初始化，返回空结果
	if DB == nil {
		return posts, nil
	}

	err := DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("id DESC").
		Limit(limit).
		Find(&posts).Error
	if err != nil {
		return nil, fmt.Errorf("GetPostsByUserID failed: %w", err)
	}
	return posts, nil
}

// UpdatePostStatus 更新帖子审核状态
func UpdatePostStatus(ctx context.Context, postID int64, status int) error {
	// 如果 DB 未初始化，返回错误
	if DB == nil {
		return fmt.Errorf("DB not initialized")
	}

	err := DB.WithContext(ctx).
		Model(&model.Post{}).
		Where("id = ?", postID).
		Update("status", status).Error
	if err != nil {
		logger.Warn("UpdatePostStatus failed", zap.Error(err), zap.Int64("post_id", postID), zap.Int("status", status))
		return fmt.Errorf("UpdatePostStatus failed: %w", err)
	}
	return nil
}

// GetPendingPosts 获取待审核帖子列表
func GetPendingPosts(ctx context.Context, offset, limit int) ([]*model.Post, int64, error) {
	var posts []*model.Post
	var total int64

	// 如果 DB 未初始化，返回空结果
	if DB == nil {
		return posts, 0, nil
	}

	// 查询总数
	err := DB.WithContext(ctx).
		Model(&model.Post{}).
		Where("status = ?", 1).
		Count(&total).Error
	if err != nil {
		return nil, 0, fmt.Errorf("GetPendingPosts count failed: %w", err)
	}

	// 查询列表
	err = DB.WithContext(ctx).
		Where("status = ?", 1).
		Order("id DESC").
		Offset(offset).
		Limit(limit).
		Find(&posts).Error
	if err != nil {
		return nil, 0, fmt.Errorf("GetPendingPosts find failed: %w", err)
	}

	return posts, total, nil
}

// GetUserPost 根据 ID 获取帖子（用于测试）
func GetUserPost(ctx context.Context, postID int64) (*model.Post, error) {
	// 如果 DB 未初始化，返回空结果
	if DB == nil {
		return nil, nil
	}

	var post model.Post
	err := DB.WithContext(ctx).Where("id = ?", postID).First(&post).Error
	return &post, err
}
