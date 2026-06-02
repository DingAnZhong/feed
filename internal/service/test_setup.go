package service

import (
	"github.com/DingAnZhong/feed/internal/repository"
	"github.com/DingAnZhong/feed/pkg/config"
	"github.com/DingAnZhong/feed/pkg/logger"
	"github.com/DingAnZhong/feed/pkg/snowflake"
	"testing"
)

// setupTest 初始化测试环境
// 返回是否初始化成功
func setupTest(t *testing.T) bool {
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
	if repository.DB == nil {
		err := repository.InitDB(&config.MySQLConfig{
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
	if repository.RDB == nil {
		err := repository.InitRedis(&config.RedisConfig{
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

	// 测试前清理数据（仅清理可能影响测试的表）
	cleanTestData(t)

	return true
}

// cleanTestData 清理测试数据
func cleanTestData(t *testing.T) {
	// 清理帖子数据（仅清理测试 ID 范围）
	if err := repository.DB.Exec("DELETE FROM posts WHERE id >= 3000").Error; err != nil {
		t.Logf("清理 posts 数据失败: %v", err)
	}

	// 清理关系数据（仅清理测试用户）
	if err := repository.DB.Exec("DELETE FROM relations WHERE follower_id >= 100").Error; err != nil {
		t.Logf("清理 relations 数据失败: %v", err)
	}

	// 清理用户数据（仅清理测试用户）
	if err := repository.DB.Exec("DELETE FROM users WHERE id >= 100").Error; err != nil {
		t.Logf("清理 users 数据失败: %v", err)
	}
}
