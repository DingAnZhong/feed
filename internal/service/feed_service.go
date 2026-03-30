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

	postIDs, err := repository.GetTimeline(ctx, userID, latestTime, limit)
	if err != nil {
		logger.Warn("gettimeline failed", zap.Error(err))
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

	postMap := make(map[int64]*model.Post)
	for _, post := range posts {
		postMap[int64(post.ID)] = post
	}
	sortedPosts := make([]*model.Post, 0, len(postIDs))
	for _, postID := range postIDs {
		if post := postMap[int64(postID)]; post != nil {
			sortedPosts = append(sortedPosts, post)
		}
	}
	var nextTime int64 = 0
	nextTime = sortedPosts[len(sortedPosts)-1].CreatedAt.UnixMilli()
	return sortedPosts, nextTime, nil
}
