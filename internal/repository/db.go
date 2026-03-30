package repository

import (
	"fmt"
	"time"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/pkg/config"
	"github.com/DingAnZhong/feed/pkg/logger"
	"go.uber.org/zap"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// DB 全局数据库实例
var DB *gorm.DB

// InitDB 初始化 MySQL 数据库连接池
func InitDB(cfg *config.MySQLConfig) error {
	var err error

	zglogger := logger.NewZapGormLogger(zap.L(), gormlogger.Info, 200*time.Millisecond)
	DB, err = gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
		Logger:                 zglogger,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return fmt.Errorf("连接 MySQL 失败: %v", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("获取底层 sql.DB 失败: %v", err)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Hour)

	err = DB.AutoMigrate(
		&model.User{},
		&model.Post{},
		&model.Relation{},
	)
	if err != nil {
		return fmt.Errorf("自动迁移数据库表结构失败: %v", err)
	}

	logger.Info("MySQL 数据库初始化成功！")
	return nil
}
