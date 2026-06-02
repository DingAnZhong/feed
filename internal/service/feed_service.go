package service

import (
	"context"
	"fmt"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/internal/repository"
	"github.com/DingAnZhong/feed/pkg/config"
	"github.com/DingAnZhong/feed/pkg/logger"
	"go.uber.org/zap"
)

// FetchFeed 拉取用户的 Feed 流 (推拉结合模式)
// userID: 当前登录的用户
// latestTime: 游标（上一次拉取最后一条的时间戳），首次拉取传 0 或未来极大的时间戳
// limit: 拉取条数
// 返回值: 帖子列表，下一次请求的游标，错误
func FetchFeed(ctx context.Context, userID int64, latestTime int64, limit int) ([]*model.Post, int64, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	// 检查是否启用推拉结合模式
	pullModeEnabled := false
	if config.Conf.App != nil && config.Conf.App.PullMode != nil && config.Conf.App.PullMode.Enabled {
		pullModeEnabled = true
	}

	// 获取用户关注的大 V 列表（粉丝数超过阈值的作者）
	// 这里简化处理：直接获取 timeline 中的帖子，再补充热门帖子
	var posts []*model.Post
	var postIDs []int64

	// 1. 从 timeline 拉取（推模式部分）
	postIDs, err := repository.GetTimeline(ctx, userID, latestTime, limit)
	if err != nil {
		logger.Warn("gettimeline failed", zap.Error(err))
		return nil, 0, fmt.Errorf("gettimeline failed:%w", err)
	}
	logger.Debug("FetchFeed got postIDs from GetTimeline", zap.Int64s("post_ids", postIDs))

	// 2. 如果启用拉模式，补充热门帖子
	if pullModeEnabled && len(postIDs) < limit {
		// 获取已有的 post IDs
		existingIDs := make(map[int64]bool)
		for _, id := range postIDs {
			existingIDs[id] = true
		}

		// 从热门帖子中补充
		popularLimit := limit - len(postIDs)
		popularPosts, popErr := repository.CacheGetPopularPosts(ctx, popularLimit*2)
		if popErr == nil && len(popularPosts) > 0 {
			// 过滤已存在的帖子
			newPostIDs := make([]int64, 0, len(popularPosts))
			for _, post := range popularPosts {
				if !existingIDs[post.ID] {
					newPostIDs = append(newPostIDs, post.ID)
				}
			}

			// 组合帖子
			postsByIDs, getErr := repository.GetPostsByIDs(ctx, append(postIDs, newPostIDs...))
			if getErr == nil && len(postsByIDs) > 0 {
				posts = postsByIDs
			}
		}
	}

	if len(postIDs) == 0 && len(posts) == 0 {
		return nil, 0, nil
	}

	// 如果还没有获取帖子，通过 IDs 获取
	if len(posts) == 0 && len(postIDs) > 0 {
		posts, err = repository.GetPostsByIDs(ctx, postIDs)
		if err != nil {
			logger.Warn("getpostbyIDs failed", zap.Error(err))
			return nil, 0, fmt.Errorf("getpostbyIDs failed:%w", err)
		}
		ids := make([]int64, len(posts))
		for i, p := range posts {
			ids[i] = p.ID
		}
		logger.Debug("FetchFeed got posts from GetPostsByIDs", zap.Int64s("post_ids", ids))
	}

	if len(posts) == 0 {
		return nil, 0, nil
	}

	// 过滤掉审核不通过的帖子（status = 2）
	validPosts := make([]*model.Post, 0, len(posts))
	for _, post := range posts {
		if post.Status != PostStatusRejected {
			validPosts = append(validPosts, post)
		}
	}
	if len(validPosts) == 0 {
		return nil, 0, nil
	}
	ids1 := make([]int64, len(validPosts))
	for i, p := range validPosts {
		ids1[i] = p.ID
	}
	logger.Debug("FetchFeed validPosts before sort", zap.Int64s("post_ids", ids1))

	// 按 originalPostIDs 顺序排序（如果使用了混合模式）
	originalPostIDs := postIDs
	if len(posts) > 0 && len(postIDs) > 0 {
		postMap := make(map[int64]*model.Post, len(validPosts))
		for _, post := range validPosts {
			postMap[int64(post.ID)] = post
		}
		sortedPosts := make([]*model.Post, 0, len(originalPostIDs))
		for _, postID := range originalPostIDs {
			if post := postMap[int64(postID)]; post != nil {
				sortedPosts = append(sortedPosts, post)
			}
		}
		if len(sortedPosts) > 0 {
			posts = sortedPosts
		}
	}
	ids2 := make([]int64, len(posts))
	for i, p := range posts {
		ids2[i] = p.ID
	}
	logger.Debug("FetchFeed final posts", zap.Int64s("post_ids", ids2))

	var nextTime int64 = 0
	if len(posts) > 0 {
		nextTime = posts[len(posts)-1].CreatedAt.UnixMilli()
	}
	return posts, nextTime, nil
}

// FetchPopularFeed 获取热门帖子（按点赞数降序，不限关注关系）
// 使用多级缓存：L1 本地缓存 + L2 Redis + L3 DB 降级
func FetchPopularFeed(ctx context.Context, limit int) ([]*model.Post, int64, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	// 使用多级缓存获取热门帖子
	posts, err := repository.CacheGetPopularPosts(ctx, limit)
	if err != nil {
		logger.Warn("fetch popular feed failed", zap.Error(err))
		return nil, 0, fmt.Errorf("fetch popular feed failed: %w", err)
	}
	if len(posts) == 0 {
		return nil, 0, nil
	}

	// 过滤掉审核不通过的帖子
	validPosts := make([]*model.Post, 0, len(posts))
	for _, post := range posts {
		if post.Status != PostStatusRejected {
			validPosts = append(validPosts, post)
		}
	}
	if len(validPosts) == 0 {
		return nil, 0, nil
	}

	var nextTime int64 = 0
	if len(validPosts) > 0 {
		nextTime = validPosts[len(validPosts)-1].CreatedAt.UnixMilli()
	}
	return validPosts, nextTime, nil
}
