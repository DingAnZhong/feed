package service

import (
	"context"
	"fmt"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/internal/repository"
	"github.com/DingAnZhong/feed/pkg/logger"
	"go.uber.org/zap"
)

// FetchFeed 拉取用户的 Feed 流 (收件箱模式)
// userID: 当前登录的用户
// latestTime: 游标（上一次拉取最后一条的时间戳），首次拉取传 0 或未来极大的时间戳
// limit: 拉取条数
// 返回值: 帖子列表，下一次请求的游标，错误
func FetchFeed(ctx context.Context, userID int64, latestTime int64, limit int) ([]*model.Post, int64, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	postIDs, err := repository.GetTimeline(ctx, userID, latestTime, limit)
	if err != nil {
		logger.Warn("gettimeline failed", zap.Error(err))
		return nil, 0, fmt.Errorf("gettimeline failed:%w", err)
	}
	if len(postIDs) == 0 {
		return nil, 0, nil
	}

	posts, err := repository.GetPostsByIDs(ctx, postIDs)
	if err != nil {
		logger.Warn("getpostbyIDs failed", zap.Error(err))
		return nil, 0, fmt.Errorf("getpostbyIDs failed:%w", err)
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

	postMap := make(map[int64]*model.Post, len(validPosts))
	for _, post := range validPosts {
		postMap[int64(post.ID)] = post
	}
	sortedPosts := make([]*model.Post, 0, len(postIDs))
	for _, postID := range postIDs {
		if post := postMap[int64(postID)]; post != nil {
			sortedPosts = append(sortedPosts, post)
		}
	}
	var nextTime int64 = 0
	if len(sortedPosts) > 0 {
		nextTime = sortedPosts[len(sortedPosts)-1].CreatedAt.UnixMilli()
	}
	return sortedPosts, nextTime, nil
}

// FetchPopularFeed 获取热门帖子（按点赞数降序，不限关注关系）
func FetchPopularFeed(ctx context.Context, limit int) ([]*model.Post, int64, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	posts, err := repository.GetPopularPosts(ctx, limit)
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
