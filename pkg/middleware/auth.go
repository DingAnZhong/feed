package middleware

import (
	"strconv"

	"github.com/DingAnZhong/feed/pkg/response"
	"github.com/gin-gonic/gin"
)

// ContextUserIDKey 是存放在 Gin Context 中的键名
const ContextUserIDKey = "userID"

// AuthMiddleware 模拟用户鉴权中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.GetHeader("X-User-ID")

		if userIDStr == "" {
			response.Response(c, 200, "未登录或缺少 X-User-ID 请求头", "")
			c.Abort() // 熔断
			return
		}

		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			response.Response(c, 200, "将字符串格式的 ID 转换为 int64 失败", "")
			c.Abort()
			return
		}
		c.Set(ContextUserIDKey, userID)
		c.Next()
	}
}
