package logger

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ZapGormLogger 适配器
type ZapGormLogger struct {
	zapLogger     *zap.Logger // 明确依赖一个 Zap 实例
	LogLevel      gormlogger.LogLevel
	SlowThreshold time.Duration
}

// NewZapGormLogger
func NewZapGormLogger(zapLogger *zap.Logger, level gormlogger.LogLevel, slowThreshold time.Duration) *ZapGormLogger {
	return &ZapGormLogger{
		zapLogger:     zapLogger,
		LogLevel:      level,
		SlowThreshold: slowThreshold,
	}
}

func (l *ZapGormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

func (l *ZapGormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Info {
		l.zapLogger.Sugar().Infof(msg, data...)
	}
}

func (l *ZapGormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Warn {
		l.zapLogger.Sugar().Warnf(msg, data...)
	}
}

func (l *ZapGormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Error {
		l.zapLogger.Sugar().Errorf(msg, data...)
	}
}

func (l *ZapGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		l.zapLogger.Error("SQL 执行失败",
			zap.Error(err),
			zap.Duration("cost", elapsed),
			zap.Int64("rows", rows),
			zap.String("sql", sql),
		)
		return
	}

	if l.SlowThreshold != 0 && elapsed > l.SlowThreshold && l.LogLevel >= gormlogger.Warn {
		l.zapLogger.Warn("检测到慢查询 SQL",
			zap.Duration("cost", elapsed),
			zap.Int64("rows", rows),
			zap.String("sql", sql),
		)
		return
	}

	if l.LogLevel == gormlogger.Info {
		l.zapLogger.Debug("SQL Trace",
			zap.Duration("cost", elapsed),
			zap.Int64("rows", rows),
			zap.String("sql", sql),
		)
	}
}
