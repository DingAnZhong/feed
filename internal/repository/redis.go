package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/DingAnZhong/feed/pkg/config"
	"github.com/DingAnZhong/feed/pkg/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RDB 全局 Redis 客户端实例
var RDB *redis.Client

func InitRedis(cfg *config.RedisConfig) error {

	RDB = redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.PassWord,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := RDB.Ping(ctx).Result()
	if err != nil {
		logger.Error("Redis connect failed", zap.Error(err))
		return fmt.Errorf("redis connect failed:%w", err)
	}
	logger.Info("Redis 连接池初始化成功！")

	return nil
}
