package repository

import (
	"context"
	"testing"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/pkg/config"
	"github.com/DingAnZhong/feed/pkg/logger"
	"github.com/DingAnZhong/feed/pkg/snowflake"
	"github.com/stretchr/testify/assert"
)

// setupRepoTest 初始化 Repository 测试环境
func setupRepoTest(t *testing.T) bool {
	// 初始化 logger
	err := logger.Init(&config.LogConfig{
		Level:      "error",
		Mode:       "dev",
		Filename:   "log/test.log",
		MaxSize:    10,
		MaxBackups: 5,
		MaxAge:     7,
	})
	if err != nil {
		t.Logf("Logger 初始化失败: %v", err)
		return false
	}

	// 初始化雪花算法
	err = snowflake.InitSnowflake(1)
	if err != nil {
		t.Logf("雪花算法初始化失败: %v", err)
		return false
	}

	// 初始化 DB 连接
	if DB == nil {
		err := InitDB(&config.MySQLConfig{
			DSN:          "root:123456@tcp(127.0.0.1:13306)/feed_db?charset=utf8mb4&parseTime=True&loc=Local",
			MaxOpenConns: 100,
			MaxIdleConns: 10,
		})
		if err != nil {
			t.Logf("DB 初始化失败: %v", err)
			return false
		}
	}

	// 初始化 Redis 连接
	if RDB == nil {
		err := InitRedis(&config.RedisConfig{
			Addr:     "127.0.0.1:6379",
			PassWord: "",
			DB:       0,
			PoolSize: 20,
		})
		if err != nil {
			t.Logf("Redis 初始化失败: %v", err)
			return false
		}
	}

	return true
}

// TestGetPopularPosts 测试获取热门帖子
func TestGetPopularPosts(t *testing.T) {
	// 检查数据库是否已初始化
	if !setupRepoTest(t) {
		t.Skip("数据库未初始化，跳过集成测试")
	}

	ctx := context.Background()

	// 创建测试帖子
	posts := make([]*model.Post, 0, 3)
	for i := 0; i < 3; i++ {
		id, err := snowflake.GenerateID()
		assert.NoError(t, err)
		post := &model.Post{
			ID:        id,
			UserID:    1,
			Content:   "test content",
			LikeCount: (3 - i) * 10, // 确保降序
		}
		err = CreatePost(ctx, post)
		assert.NoError(t, err)
		posts = append(posts, post)
	}

	// 获取热门帖子
	retrievedPosts, err := GetPopularPosts(ctx, 2)
	assert.NoError(t, err)
	assert.Len(t, retrievedPosts, 2)

	// 验证按点赞数降序排列
	if len(retrievedPosts) >= 2 {
		assert.GreaterOrEqual(t, retrievedPosts[0].LikeCount, retrievedPosts[1].LikeCount)
	}

	// 清理
	for _, post := range posts {
		_ = DB.WithContext(ctx).Delete(post)
	}
}
