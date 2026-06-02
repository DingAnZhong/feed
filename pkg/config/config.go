package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

var Conf = new(Config)

type Config struct {
	App   *AppConfig   `mapstructure:"app"`
	Auth  *AuthConfig  `mapstructure:"auth"`
	MySQL *MySQLConfig `mapstructure:"mysql"`
	Redis *RedisConfig `mapstructure:"redis"`
	Kafka *KafkaConfig `mapstructure:"kafka"`
	Log   *LogConfig   `mapstructure:"log"`
}

type AppConfig struct {
	Name                      string         `mapstructure:"name"`
	Port                      int            `mapstructure:"port"`
	Env                       string         `mapstructure:"env"`
	// 大 V 用户粉丝阈值：粉丝数超过此值的用户采用推拉结合模式
	HugeUserFollowerThreshold int            `mapstructure:"huge_user_follower_threshold"`
	// 推拉结合模式配置
	PullMode                  *PullModeConfig `mapstructure:"pull_mode"`
	LocalCache                *LocalCacheConfig `mapstructure:"local_cache"`
}

type AuthConfig struct {
	JwtSecret      string `mapstructure:"jwt_secret"`
	TokenTTL       string `mapstructure:"token_ttl"`
	RefreshTokenTTL string `mapstructure:"refresh_token_ttl"`
}

type LocalCacheConfig struct {
	Enabled           bool `mapstructure:"enabled"`
	TTLSeconds        int  `mapstructure:"ttl_seconds"`
	MaxItems          int  `mapstructure:"max_items"`
	EmptyCacheTTL     int  `mapstructure:"empty_cache_ttl_seconds"`
}

// PullModeConfig 推拉结合模式配置
type PullModeConfig struct {
	Enabled           bool `mapstructure:"enabled"`
	// 大 V 粉丝阈值：粉丝数超过此值的用户采用推拉结合模式
	HugeUserThreshold int `mapstructure:"huge_user_threshold"`
	// 热门帖子拉模式阈值：点赞数超过此值的帖子采用拉模式
	PopularPostThreshold int `mapstructure:"popular_post_threshold"`
}

type MySQLConfig struct {
	DSN          string `mapstructure:"dsn"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	PassWord string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type KafkaConfig struct {
	Brokers   []string `mapstructure:"brokers"`
	TopicFeed string   `mapstructure:"topic_feed"`
}

type LogConfig struct {
	Level      string `mapstructure:"level"`
	Mode       string `mapstructure:"mode"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

// TokenTTLSeconds 返回 access_token 的 TTL 秒数
func (a *AuthConfig) TokenTTLSeconds() int {
	if ttl, err := time.ParseDuration(a.TokenTTL); err == nil {
		return int(ttl.Seconds())
	}
	return 7200 // 默认 2h
}

// RefreshTokenTTLSeconds 返回 refresh_token 的 TTL 秒数
func (a *AuthConfig) RefreshTokenTTLSeconds() int {
	if ttl, err := time.ParseDuration(a.RefreshTokenTTL); err == nil {
		return int(ttl.Seconds())
	}
	return 604800 // 默认 7d
}

// IsLocalCacheEnabled 是否启用本地缓存
func (a *AppConfig) IsLocalCacheEnabled() bool {
	return a.LocalCache != nil && a.LocalCache.Enabled
}

// LocalCacheTTL 返回本地缓存 TTL 秒数
func (a *AppConfig) LocalCacheTTL() int {
	if a.LocalCache == nil {
		return 30
	}
	return a.LocalCache.TTLSeconds
}

func InitConfig(filePath string) error {
	viper.SetConfigFile(filePath)

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	if err := viper.Unmarshal(Conf); err != nil {
		return fmt.Errorf("解析配置文件失败: %v", err)
	}

	viper.WatchConfig()

	return nil
}
