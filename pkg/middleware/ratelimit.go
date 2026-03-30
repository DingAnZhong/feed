package middleware

import (
	"github.com/DingAnZhong/feed/internal/repository"
	"github.com/DingAnZhong/feed/pkg/logger"
	"github.com/DingAnZhong/feed/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis_rate/v10"
	"go.uber.org/zap"
)

// RateLimitMiddleware 简易 API 限流中间件
func RateLimitMiddleware() gin.HandlerFunc {
	limiter := redis_rate.NewLimiter(repository.RDB)

	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		res, err := limiter.Allow(c.Request.Context(), "rate_limit:"+clientIP, redis_rate.PerSecond(10))
		if err != nil {
			logger.Warn("限流器执行异常, 降级放行", zap.Error(err))
			c.Next()
			return
		}

		if res.Allowed == 0 {
			response.Response(c, 429, "请求过于频繁，请稍后再试", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}
