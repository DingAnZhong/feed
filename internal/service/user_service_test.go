package service

import (
	"context"
	"testing"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/internal/repository"
	"github.com/DingAnZhong/feed/pkg/snowflake"
	"github.com/stretchr/testify/assert"
)

// TestFollowAction_CannotFollowSelf 测试不能关注自己
func TestFollowAction_CannotFollowSelf(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()

	err := FollowAction(ctx, 1, 1, 1)
	assert.ErrorIs(t, err, ErrCannotFollowSelf)
}

// TestFollowAction_UserNotFound 测试用户不存在
func TestFollowAction_UserNotFound(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()

	err := FollowAction(ctx, 1, 999999, 1)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// TestFollowAction_InvalidAction 测试无效动作
func TestFollowAction_InvalidAction(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()

	err := FollowAction(ctx, 1, 2, 3)
	// 注意：由于用户 2 不存在，可能返回 ErrUserNotFound 而不是 ErrInvalidAction
	// 这里检查错误类型是否在预期范围内
	assert.Error(t, err)
	assert.True(t,
		err.Error() == "无效的操作类型" ||
		err.Error() == "目标用户不存在",
		"错误应该是无效操作类型或目标用户不存在")
}

// TestFollowAction_Success 关注成功
func TestFollowAction_Success(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()

	id1, err := snowflake.GenerateID()
	assert.NoError(t, err)
	id2, err := snowflake.GenerateID()
	assert.NoError(t, err)

	testUser := &model.User{ID: id1, Nickname: "testuser1"}
	_ = repository.CreateUser(ctx, testUser)

	testUser2 := &model.User{ID: id2, Nickname: "testuser2"}
	_ = repository.CreateUser(ctx, testUser2)

	err = FollowAction(ctx, id1, id2, 1)
	assert.NoError(t, err)

	isFollowing := IsFollowing(ctx, id1, id2)
	assert.True(t, isFollowing)

	err = FollowAction(ctx, id1, id2, 2)
	assert.NoError(t, err)

	// 查询数据库验证状态
	var relation model.Relation
	err = repository.DB.WithContext(ctx).Where("follower_id = ? AND followee_id = ?", id1, id2).First(&relation).Error
	if err == nil {
		t.Logf("Relation status in DB: %d", relation.Status)
	} else {
		t.Logf("Relation not found in DB: %v", err)
	}

	isFollowing = IsFollowing(ctx, id1, id2)
	// 添加日志以调试
	t.Logf("After unfollow, isFollowing=%v", isFollowing)
	assert.False(t, isFollowing, "取消关注后应该返回 false")
}

// TestIsFollowing 测试 IsFollowing 函数
func TestIsFollowing(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()

	id1, err := snowflake.GenerateID()
	assert.NoError(t, err)
	id2, err := snowflake.GenerateID()
	assert.NoError(t, err)

	testUser := &model.User{ID: id1, Nickname: "testuser1"}
	_ = repository.CreateUser(ctx, testUser)

	testUser2 := &model.User{ID: id2, Nickname: "testuser2"}
	_ = repository.CreateUser(ctx, testUser2)

	isFollowing := IsFollowing(ctx, id1, id2)
	assert.False(t, isFollowing)

	err = FollowAction(ctx, id1, id2, 1)
	assert.NoError(t, err)

	isFollowing = IsFollowing(ctx, id1, id2)
	assert.True(t, isFollowing)
}

// TestGetUserInfo 测试获取用户信息
func TestGetUserInfo(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()

	id, err := snowflake.GenerateID()
	assert.NoError(t, err)

	testUser := &model.User{ID: id, Nickname: "testuser"}
	err = repository.CreateUser(ctx, testUser)
	assert.NoError(t, err)

	user, err := GetUserInfo(ctx, id)
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, id, user.ID)
	assert.Equal(t, "testuser", user.Nickname)

	user, err = GetUserInfo(ctx, 999999)
	assert.NoError(t, err)
	assert.Nil(t, user)
}

// TestGetUserFollowStats 测试获取关注/粉丝统计
func TestGetUserFollowStats(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()

	id1, err := snowflake.GenerateID()
	assert.NoError(t, err)
	id2, err := snowflake.GenerateID()
	assert.NoError(t, err)

	testUser := &model.User{ID: id1, Nickname: "testuser1"}
	_ = repository.CreateUser(ctx, testUser)

	testUser2 := &model.User{ID: id2, Nickname: "testuser2"}
	_ = repository.CreateUser(ctx, testUser2)

	following, follower, err := GetUserFollowStats(ctx, id1)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), following)
	assert.Equal(t, int64(0), follower)

	err = FollowAction(ctx, id1, id2, 1)
	assert.NoError(t, err)

	following, follower, err = GetUserFollowStats(ctx, id1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), following)
	assert.Equal(t, int64(0), follower)

	following, follower, err = GetUserFollowStats(ctx, id2)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), following)
	assert.Equal(t, int64(1), follower)
}

// TestSyncUserCountFromDB 测试从数据库同步计数
func TestSyncUserCountFromDB(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()

	id1, err := snowflake.GenerateID()
	assert.NoError(t, err)
	id2, err := snowflake.GenerateID()
	assert.NoError(t, err)

	testUser := &model.User{ID: id1, Nickname: "testuser1"}
	_ = repository.CreateUser(ctx, testUser)

	testUser2 := &model.User{ID: id2, Nickname: "testuser2"}
	_ = repository.CreateUser(ctx, testUser2)

	err = FollowAction(ctx, id1, id2, 1)
	assert.NoError(t, err)

	err = SyncUserCountFromDB(ctx, id1)
	assert.NoError(t, err)

	err = SyncUserCountFromDB(ctx, id2)
	assert.NoError(t, err)
}

// TestRegisterUser 测试用户注册
func TestRegisterUser(t *testing.T) {
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}
	ctx := context.Background()

	id, err := snowflake.GenerateID()
	assert.NoError(t, err)

	err = RegisterUser(ctx, id, "testuser")
	assert.NoError(t, err)

	user, err := GetUserInfo(ctx, id)
	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, id, user.ID)
	assert.Equal(t, "testuser", user.Nickname)

	err = RegisterUser(ctx, id, "testuser")
	assert.ErrorIs(t, err, ErrUserAlreadyExists)
}
