package repository

import (
	"context"
	"testing"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/pkg/snowflake"
	"github.com/stretchr/testify/assert"
)

// TestCreatePost 测试创建帖子
func TestCreatePost(t *testing.T) {
	// 检查数据库是否已初始化
	if DB == nil {
		t.Skip("DB 未初始化，跳过集成测试")
	}

	ctx := context.Background()

	// 生成雪花 ID
	id, err := snowflake.GenerateID()
	assert.NoError(t, err)

	post := &model.Post{
		ID:        id,
		UserID:    1,
		Content:   "test content",
		Status:    0,
		LikeCount: 0,
	}

	err = CreatePost(ctx, post)
	assert.NoError(t, err)
	assert.Greater(t, post.ID, int64(0))

	// 清理
	_ = DB.WithContext(ctx).Delete(post)
}

// TestGetPostsByIDs_EmptyList 测试空 ID 列表
func TestGetPostsByIDs_EmptyList(t *testing.T) {
	ctx := context.Background()

	posts, err := GetPostsByIDs(ctx, []int64{})
	assert.NoError(t, err)
	assert.Nil(t, posts)
}

// TestGetPostsByIDs_SinglePost 测试单个帖子查询
func TestGetPostsByIDs_SinglePost(t *testing.T) {
	// 检查数据库是否已初始化
	if DB == nil {
		t.Skip("DB 未初始化，跳过集成测试")
	}

	ctx := context.Background()

	// 生成雪花 ID
	id, err := snowflake.GenerateID()
	assert.NoError(t, err)

	// 创建测试帖子
	post := &model.Post{
		ID:        id,
		UserID:    1,
		Content:   "test content",
		Status:    0,
	}
	err = CreatePost(ctx, post)
	assert.NoError(t, err)
	defer DB.WithContext(ctx).Delete(post)

	// 查询帖子
	posts, err := GetPostsByIDs(ctx, []int64{post.ID})
	assert.NoError(t, err)
	assert.Len(t, posts, 1)
	assert.Equal(t, post.ID, posts[0].ID)
	assert.Equal(t, post.Content, posts[0].Content)
}

// TestGetPostsByIDs_MultiplePosts 测试批量查询
func TestGetPostsByIDs_MultiplePosts(t *testing.T) {
	// 检查数据库是否已初始化
	if DB == nil {
		t.Skip("DB 未初始化，跳过集成测试")
	}

	ctx := context.Background()

	// 创建多个测试帖子
	posts := make([]*model.Post, 0, 3)
	for i := 0; i < 3; i++ {
		id, err := snowflake.GenerateID()
		assert.NoError(t, err)
		post := &model.Post{
			ID:        id,
			UserID:    1,
			Content:   "test content " + string(rune('0'+i)),
			Status:    0,
		}
		err = CreatePost(ctx, post)
		assert.NoError(t, err)
		posts = append(posts, post)
	}
	defer func() {
		for _, post := range posts {
			_ = DB.WithContext(ctx).Delete(post)
		}
	}()

	// 按特定顺序查询帖子
	idOrder := []int64{posts[2].ID, posts[0].ID, posts[1].ID}
	retrievedPosts, err := GetPostsByIDs(ctx, idOrder)
	assert.NoError(t, err)
	assert.Len(t, retrievedPosts, 3)

	// 验证返回顺序与 ID 顺序一致
	assert.Equal(t, posts[2].ID, retrievedPosts[0].ID)
	assert.Equal(t, posts[0].ID, retrievedPosts[1].ID)
	assert.Equal(t, posts[1].ID, retrievedPosts[2].ID)
}

// TestGetPostsByIDs_NonExistentIDs 测试查询不存在的帖子 ID
func TestGetPostsByIDs_NonExistentIDs(t *testing.T) {
	ctx := context.Background()

	// 查询不存在的帖子
	posts, err := GetPostsByIDs(ctx, []int64{999999, 888888})
	assert.NoError(t, err)
	assert.Len(t, posts, 0)
}

// TestGetPostsByIDs_MixedIDs 测试混合存在和不存在的 ID
func TestGetPostsByIDs_MixedIDs(t *testing.T) {
	// 检查数据库是否已初始化
	if DB == nil {
		t.Skip("DB 未初始化，跳过集成测试")
	}

	ctx := context.Background()

	// 生成雪花 ID
	id, err := snowflake.GenerateID()
	assert.NoError(t, err)

	// 创建一个测试帖子
	post := &model.Post{
		ID:        id,
		UserID:    1,
		Content:   "test content",
		Status:    0,
	}
	err = CreatePost(ctx, post)
	assert.NoError(t, err)
	defer DB.WithContext(ctx).Delete(post)

	// 查询一个存在和一个不存在的 ID
	posts, err := GetPostsByIDs(ctx, []int64{post.ID, 999999})
	assert.NoError(t, err)
	assert.Len(t, posts, 1)
	assert.Equal(t, post.ID, posts[0].ID)
}

// TestGetPostsByUserID 测试按用户 ID 查询帖子
func TestGetPostsByUserID(t *testing.T) {
	// 检查数据库是否已初始化
	if DB == nil {
		t.Skip("DB 未初始化，跳过集成测试")
	}

	ctx := context.Background()

	userID := int64(100)

	// 创建多个测试帖子
	posts := make([]*model.Post, 0, 5)
	for i := 0; i < 5; i++ {
		id, err := snowflake.GenerateID()
		assert.NoError(t, err)
		post := &model.Post{
			ID:        id,
			UserID:    userID,
			Content:   "test content " + string(rune('0'+i)),
			Status:    0,
		}
		err = CreatePost(ctx, post)
		assert.NoError(t, err)
		posts = append(posts, post)
	}

	// 查询该用户的所有帖子
	retrievedPosts, err := GetPostsByUserID(ctx, userID, 10)
	assert.NoError(t, err)
	assert.Len(t, retrievedPosts, 5)

	// 验证按 ID 降序排列
	for i := 0; i < len(retrievedPosts)-1; i++ {
		assert.True(t, retrievedPosts[i].ID > retrievedPosts[i+1].ID, "帖子应该按 ID 降序排列")
	}

	// 限制返回数量
	retrievedPosts, err = GetPostsByUserID(ctx, userID, 3)
	assert.NoError(t, err)
	assert.Len(t, retrievedPosts, 3)

	// 清理
	_ = DB.WithContext(ctx).Where("user_id = ?", userID).Delete(&model.Post{})
}

// TestUpdatePostStatus 测试更新帖子状态
func TestUpdatePostStatus(t *testing.T) {
	// 检查数据库是否已初始化
	if DB == nil {
		t.Skip("DB 未初始化，跳过集成测试")
	}

	ctx := context.Background()

	// 生成雪花 ID
	id, err := snowflake.GenerateID()
	assert.NoError(t, err)

	// 创建测试帖子
	post := &model.Post{
		ID:        id,
		UserID:    1,
		Content:   "test content",
		Status:    0,
	}
	err = CreatePost(ctx, post)
	assert.NoError(t, err)
	defer DB.WithContext(ctx).Delete(post)

	// 更新状态为审核中
	err = UpdatePostStatus(ctx, post.ID, 1)
	assert.NoError(t, err)

	// 验证状态已更新
	retrievedPost, err := GetUserPost(ctx, post.ID)
	assert.NoError(t, err)
	assert.Equal(t, 1, retrievedPost.Status)

	// 更新状态为审核不通过
	err = UpdatePostStatus(ctx, post.ID, 2)
	assert.NoError(t, err)

	retrievedPost, err = GetUserPost(ctx, post.ID)
	assert.NoError(t, err)
	assert.Equal(t, 2, retrievedPost.Status)
}
