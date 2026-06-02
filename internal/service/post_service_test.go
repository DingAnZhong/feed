package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPublishPost_ContentValidation 测试内容校验逻辑
func TestPublishPost_ContentValidation(t *testing.T) {
	// 初始化测试环境
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}

	ctx := context.Background()

	// 测试空内容
	_, err := PublishPost(ctx, 1, "", nil)
	assert.ErrorIs(t, err, ErrContentEmpty, "空内容应该返回 ErrContentEmpty")

	// 测试超长内容
	longContent := string(make([]rune, 501)) // 501 个字符
	_, err = PublishPost(ctx, 1, longContent, nil)
	assert.ErrorIs(t, err, ErrContentTooLong, "超长内容应该返回 ErrContentTooLong")

	// 测试刚好 500 字符（边界值）
	exactly500 := string(make([]rune, 500))
	_, err = PublishPost(ctx, 1, exactly500, nil)
	assert.NotErrorIs(t, err, ErrContentTooLong, "500字符内容不应该返回 ErrContentTooLong")
}

// TestErrContentEmpty 测试空内容错误
func TestErrContentEmpty(t *testing.T) {
	assert.Equal(t, "帖子内容不能为空", ErrContentEmpty.Error())
}

// TestErrContentTooLong 测试内容过长错误
func TestErrContentTooLong(t *testing.T) {
	assert.Equal(t, "帖子内容太长", ErrContentTooLong.Error())
}

// TestPostStatusConstants 测试帖子状态常量
func TestPostStatusConstants(t *testing.T) {
	assert.Equal(t, 0, PostStatusNormal)
	assert.Equal(t, 1, PostStatusReviewing)
	assert.Equal(t, 2, PostStatusRejected)
}

// TestPublishPost_PostIDGeneration 测试 postID 生成
func TestPublishPost_PostIDGeneration(t *testing.T) {
	// 初始化测试环境
	if !setupTest(t) {
		t.Skip("数据库未初始化，跳过测试")
	}

	ctx := context.Background()
	content := "test content for post ID generation"

	// 发帖后得到的 postID 应该是有效的
	// 实际测试中，由于我们有初始化的 DB 和 Redis，应该成功
	postID, err := PublishPost(ctx, 1, content, nil)

	// 成功时应该返回有效的 postID
	assert.NoError(t, err, "应该成功生成 postID")
	assert.Greater(t, postID, int64(0), "postID 应该大于 0")
}
