package config

import (
	"fmt"

	"github.com/spf13/viper"
)

var Conf = new(Config)

type Config struct {
	App   *AppConfig   `mapstructure:"app"`
	MySQL *MySQLConfig `mapstructure:"mysql"`
	Redis *RedisConfig `mapstructure:"redis"`
	Kafka *KafkaConfig `mapstructure:"kafka"`
	Log   *LogConfig   `mapstructure:"log"`
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Port int    `mapstructure:"port"`
	Env  string `mapstructure:"env"`
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
