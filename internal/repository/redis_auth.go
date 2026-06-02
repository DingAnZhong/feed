package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/DingAnZhong/feed/pkg/config"
)

const (
	// RefreshTokenKeyPrefix refresh token 的 Redis key 前缀
	RefreshTokenKeyPrefix = "auth:refresh_token:"
)

// GenerateRefreshTokenKey 生成 refresh token 的 Redis key
func GenerateRefreshTokenKey(userID int64) string {
	return fmt.Sprintf("%s%d", RefreshTokenKeyPrefix, userID)
}

// SaveRefreshToken 保存 refresh token 到 Redis
// TTL 由配置决定
func SaveRefreshToken(ctx context.Context, userID int64, refreshToken string) error {
	authConf := config.Conf.Auth
	key := GenerateRefreshTokenKey(userID)
	
	return RDB.Set(ctx, key, refreshToken, time.Duration(authConf.RefreshTokenTTLSeconds())*time.Second).Err()
}

// GetRefreshToken 获取 refresh token
// 返回存储的 token 字符串，如果不存在返回空字符串
func GetRefreshToken(ctx context.Context, userID int64) (string, error) {
	key := GenerateRefreshTokenKey(userID)
	return RDB.Get(ctx, key).Result()
}

// DeleteRefreshToken 删除 refresh token（用于登出或吊销）
func DeleteRefreshToken(ctx context.Context, userID int64) error {
	key := GenerateRefreshTokenKey(userID)
	return RDB.Del(ctx, key).Err()
}

// RefreshTokenExists 检查 refresh token 是否存在
func RefreshTokenExists(ctx context.Context, userID int64) (bool, error) {
	key := GenerateRefreshTokenKey(userID)
	exists, err := RDB.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}
