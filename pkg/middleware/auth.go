package middleware

import (
	"strings"

	"github.com/DingAnZhong/feed/pkg/auth"
	"github.com/DingAnZhong/feed/pkg/response"
	"github.com/gin-gonic/gin"
)

// ContextUserIDKey 是存放在 Gin Context 中的键名
const ContextUserIDKey = "userID"

// AuthMiddleware JWT 鉴权中间件
// 从 Authorization: Bearer <token> 头中解析用户 ID
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Response(c, 401, "未提供认证信息", nil)
			c.Abort()
			return
		}

		// 检查是否为 Bearer Token
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			response.Response(c, 401, "认证格式错误，应为: Authorization: Bearer <token>", nil)
			c.Abort()
			return
		}

		tokenString := parts[1]
		if tokenString == "" {
			response.Response(c, 401, "Token 为空", nil)
			c.Abort()
			return
		}

		// 解析 JWT token
		claims, err := auth.ParseToken(tokenString)
		if err != nil {
			response.Response(c, 401, "Token 无效或已过期", err.Error())
			c.Abort()
			return
		}

		// 检查 token 是否过期
		isExpired, err := auth.IsTokenExpired(tokenString)
		if err != nil {
			response.Response(c, 401, "Token 验证失败", err.Error())
			c.Abort()
			return
		}
		if isExpired {
			response.Response(c, 401, "Token 已过期，请重新登录", nil)
			c.Abort()
			return
		}

		// 将 user_id 存入 context
		c.Set(ContextUserIDKey, claims.UserID)
		c.Next()
	}
}

// SkipAuthMiddleware 跳过鉴权的中间件（用于公开接口）
// 仅从 JWT 中提取 user_id（如果存在），不强制要求认证
func SkipAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.Next()
			return
		}

		tokenString := parts[1]
		if tokenString == "" {
			c.Next()
			return
		}

		// 尝试解析 JWT token（不强制成功）
		if claims, err := auth.ParseToken(tokenString); err == nil {
			// 检查是否过期
			if isExpired, _ := auth.IsTokenExpired(tokenString); !isExpired {
				c.Set(ContextUserIDKey, claims.UserID)
			}
		}
		c.Next()
	}
}

// RefreshTokenAuthMiddleware 用于 refresh token 的鉴权
// 只检查 token 是否有效，不检查是否为 access token
func RefreshTokenAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Response(c, 401, "未提供认证信息", nil)
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			response.Response(c, 401, "认证格式错误", nil)
			c.Abort()
			return
		}

		tokenString := parts[1]
		if tokenString == "" {
			response.Response(c, 401, "Token 为空", nil)
			c.Abort()
			return
		}

		// 解析 JWT token
		claims, err := auth.ParseToken(tokenString)
		if err != nil {
			response.Response(c, 401, "Token 无效", err.Error())
			c.Abort()
			return
		}

		// 检查是否过期
		isExpired, err := auth.IsTokenExpired(tokenString)
		if err != nil {
			response.Response(c, 401, "Token 验证失败", err.Error())
			c.Abort()
			return
		}
		if isExpired {
			response.Response(c, 401, "Token 已过期", nil)
			c.Abort()
			return
		}

		c.Set(ContextUserIDKey, claims.UserID)
		c.Next()
	}
}
