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

	// 主动缓存新帖子
	CacheSetPost(post)

	return nil
}

// GetPostsByIDs 根据一组 ID 批量查询帖子详情
// 使用多级缓存：先查各帖子的 L1/L2 缓存，miss 的部分查 DB，回填缓存
func GetPostsByIDs(ctx context.Context, postIDs []int64) ([]*model.Post, error) {
	var posts []*model.Post

	if len(postIDs) == 0 {
		return posts, nil
	}

	// 如果 DB 未初始化，返回空结果
	if DB == nil {
		return posts, nil
	}

	// 第一阶段：从多级缓存获取已有数据
	cachedPosts := make(map[int64]*model.Post)
	missingIDs := make([]int64, 0, len(postIDs))

	for _, postID := range postIDs {
		if post, err := CacheGetPost(ctx, postID); err == nil && post != nil {
			cachedPosts[postID] = post
		} else {
			missingIDs = append(missingIDs, postID)
		}
	}

	// 如果全部命中缓存，直接返回
	if len(missingIDs) == 0 {
		// 按 postIDs 顺序组装结果
		for _, postID := range postIDs {
			if p := cachedPosts[postID]; p != nil {
				posts = append(posts, p)
			}
		}
		return posts, nil
	}

	// 第二阶段：查询缺失的帖子
	var missingPosts []*model.Post
	idPlaceholders := make([]string, len(missingIDs))
	for i, id := range missingIDs {
		idPlaceholders[i] = fmt.Sprintf("%d", id)
	}
	orderClause := fmt.Sprintf("FIELD(id, %s)", strings.Join(idPlaceholders, ","))

	err := DB.WithContext(ctx).Where("id IN ?", missingIDs).Order(orderClause).Find(&missingPosts).Error
	if err != nil {
		return posts, fmt.Errorf("GetPostsByIDs failed:%w", err)
	}

	// 回填缓存并合并结果
	cachedPostMap := make(map[int64]*model.Post, len(missingPosts))
	for _, p := range missingPosts {
		cachedPostMap[p.ID] = p
		CacheSetPost(p)
		posts = append(posts, p)
	}

	// 将缓存的帖子也按顺序加入结果
	for _, postID := range postIDs {
		if _, exists := cachedPostMap[postID]; !exists {
			if p := cachedPosts[postID]; p != nil {
				posts = append(posts, p)
			}
		}
	}

	// 使用 FIELD 保证返回顺序与 postIDs 一致
	if len(posts) > 0 && len(missingPosts) > 0 {
		// 需要按 FIELD 顺序重新排序
		postMap := make(map[int64]*model.Post, len(posts))
		for _, p := range posts {
			postMap[p.ID] = p
		}
		sorted := make([]*model.Post, 0, len(posts))
		for _, postID := range postIDs {
			if p := postMap[postID]; p != nil {
				sorted = append(sorted, p)
			}
		}
		posts = sorted
	}

	return posts, nil
}

// GetPostsByUserID 根据用户 ID 查询该用户最近 N 篇帖子（直接查数据库，不走缓存）
// 避免与 CacheGetUserPosts 形成循环依赖
func GetPostsByUserID(ctx context.Context, userID int64, limit int) ([]*model.Post, error) {
	var posts []*model.Post
	err := DB.WithContext(ctx).Where("user_id = ?", userID).Order("id DESC").Limit(limit).Find(&posts).Error
	if err != nil {
		logger.Warn("GetPostsByUserID failed", zap.Error(err))
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

	// 使帖子缓存失效
	CacheInvalidatePost(postID)

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
