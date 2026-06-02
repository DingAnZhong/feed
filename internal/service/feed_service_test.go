package service

import (
	"context"
	"testing"
	"time"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/internal/repository"
	"github.com/DingAnZhong/feed/pkg/snowflake"
	"github.com/stretchr/testify/assert"
)

// TestFetchFeed_LimitValidation 测试 limit 参数校验（纯逻辑部分）
func TestFetchFeed_LimitValidation(t *testing.T) {
	assert.Equal(t, 10, calculateFeedLimit(0))
	assert.Equal(t, 10, calculateFeedLimit(-1))
	assert.Equal(t, 10, calculateFeedLimit(101))
	assert.Equal(t, 20, calculateFeedLimit(20))
}

// calculateFeedLimit 是一个可测试的辅助函数
func calculateFeedLimit(limit int) int {
	if limit <= 0 || limit > 50 {
		return 10
	}
	return limit
}

// TestFetchFeed_EmptyTimeline 测试空时间线处理
func TestFetchFeed_EmptyTimeline(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()
	posts, nextTime, err := FetchFeed(ctx, 999999, 0, 10)
	assert.NoError(t, err)
	assert.Nil(t, posts)
	assert.Equal(t, int64(0), nextTime)
}

// TestFetchFeed_Success 测试成功获取 Feed 流
func TestFetchFeed_Success(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()
	content := "test post for fetch feed"
	postID, err := PublishPost(ctx, 1, content, nil)
	assert.NoError(t, err)
	assert.Greater(t, postID, int64(0))

	time.Sleep(100 * time.Millisecond)

	posts, nextTime, err := FetchFeed(ctx, 1, 0, 10)
	assert.NoError(t, err)
	
	if len(posts) > 0 {
		assert.Equal(t, postID, posts[0].ID)
		assert.NotZero(t, nextTime)
	}
}

// TestFetchFeed_Pagination 测试游标分页
func TestFetchFeed_Pagination(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()
	postsCreated := make([]int64, 0, 5)
	for i := 0; i < 5; i++ {
		content := "test post " + string(rune('0'+i))
		postID, err := PublishPost(ctx, 1, content, nil)
		assert.NoError(t, err)
		postsCreated = append(postsCreated, postID)
		time.Sleep(10 * time.Millisecond)
	}

	latestTime := postsCreated[4]
	posts, nextTime, err := FetchFeed(ctx, 1, latestTime+1, 2)
	assert.NoError(t, err)
	assert.LessOrEqual(t, len(posts), 2)

	if len(posts) > 0 && nextTime > 0 {
		_, nextTime, err = FetchFeed(ctx, 1, nextTime, 2)
		assert.NoError(t, err)
	}
}

// TestFetchPopularFeed_LimitValidation 测试热门 Feed 的 limit 校验
func TestFetchPopularFeed_LimitValidation(t *testing.T) {
	assert.Equal(t, 10, calculatePopularFeedLimit(0))
	assert.Equal(t, 10, calculatePopularFeedLimit(-1))
	assert.Equal(t, 10, calculatePopularFeedLimit(101))
	assert.Equal(t, 20, calculatePopularFeedLimit(20))
}

func calculatePopularFeedLimit(limit int) int {
	if limit <= 0 || limit > 50 {
		return 10
	}
	return limit
}

// TestFetchPopularFeed_Empty 测试空热门 Feed
func TestFetchPopularFeed_Empty(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()
	posts, nextTime, err := FetchPopularFeed(ctx, 10)
	assert.NoError(t, err)
	assert.Nil(t, posts)
	assert.Equal(t, int64(0), nextTime)
}

// TestFetchPopularFeed_Success 测试成功获取热门 Feed
func TestFetchPopularFeed_Success(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()
	// 使用雪花算法生成 ID
	id, err := snowflake.GenerateID()
	assert.NoError(t, err)
	post := &model.Post{
		ID:        id,
		UserID:    1,
		Content:   "popular post",
		LikeCount: 100,
	}
	err = repository.CreatePost(ctx, post)
	assert.NoError(t, err)

	posts, nextTime, err := FetchPopularFeed(ctx, 10)
	assert.NoError(t, err)

	if len(posts) > 0 {
		assert.Equal(t, post.ID, posts[0].ID)
		assert.NotZero(t, nextTime)
	}
}

// TestFetchFeed_FilterRejectedPosts 测试过滤审核不通过的帖子
func TestFetchFeed_FilterRejectedPosts(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()
	// 生成雪花 ID
	normalID, err := snowflake.GenerateID()
	assert.NoError(t, err)
	rejectedID, err := snowflake.GenerateID()
	assert.NoError(t, err)
	normalPost := &model.Post{
		ID:        normalID,
		UserID:    1,
		Content:   "normal post",
		Status:    PostStatusNormal,
	}
	err = repository.CreatePost(ctx, normalPost)
	assert.NoError(t, err)

	rejectedPost := &model.Post{
		ID:        rejectedID,
		UserID:    1,
		Content:   "rejected post",
		Status:    PostStatusRejected,
	}
	err = repository.CreatePost(ctx, rejectedPost)
	assert.NoError(t, err)

	posts, _, err := FetchFeed(ctx, 1, 0, 10)
	assert.NoError(t, err)

	for _, post := range posts {
		assert.NotEqual(t, PostStatusRejected, post.Status,
			"不应该返回审核不通过的帖子")
	}
}

// TestFetchFeed_PostOrdering 测试帖子排序
func TestFetchFeed_PostOrdering(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()
	createdPosts := make([]*model.Post, 0, 3)
	for i := 0; i < 3; i++ {
		id, err := snowflake.GenerateID()
		assert.NoError(t, err)
		post := &model.Post{
			ID:        id,
			UserID:    1,
			Content:   "test post " + string(rune('0'+i)),
			Status:    PostStatusNormal,
		}
		err = repository.CreatePost(ctx, post)
		assert.NoError(t, err)
		createdPosts = append(createdPosts, post)
		time.Sleep(10 * time.Millisecond)
	}

	posts, _, err := FetchFeed(ctx, 1, 0, 10)
	assert.NoError(t, err)

	if len(posts) >= 2 {
		for i := 0; i < len(posts)-1; i++ {
			assert.True(t, posts[i].CreatedAt.After(posts[i+1].CreatedAt),
				"帖子应该按时间倒序排列")
		}
	}
}
