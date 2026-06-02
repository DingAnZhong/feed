package api

import (
	"fmt"

	"github.com/DingAnZhong/feed/internal/repository"
	"github.com/DingAnZhong/feed/internal/service"
	"github.com/DingAnZhong/feed/pkg/auth"
	"github.com/DingAnZhong/feed/pkg/middleware"
	"github.com/DingAnZhong/feed/pkg/response"
	"github.com/gin-gonic/gin"
)

type FollowReq struct {
	FolloweeID int64 `json:"followee_id" binding:"required"`
	ActionType int   `json:"action_type" binding:"required,oneof=1 2"` // 1-关注, 2-取关
}

type RegisterReq struct {
	UserID   int64  `json:"user_id" binding:"required,min=1"`
	Nickname string `json:"nickname" binding:"required,max=64"`
}

// RegisterHandler 用户注册接口（公开接口，无需鉴权）
// POST /web/api/v1/user/register
func RegisterHandler(c *gin.Context) {
	var req RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Response(c, 400, "参数解析失败或不合法", err.Error())
		return
	}
	if req.Nickname == "" {
		req.Nickname = fmt.Sprintf("用户%d", req.UserID)
	}

	err := service.RegisterUser(c.Request.Context(), req.UserID, req.Nickname)
	if err != nil {
		if err.Error() == "用户已存在" {
			response.Response(c, 409, "用户已存在，请直接登录", err.Error())
			return
		}
		response.Response(c, 500, "注册失败，请稍后重试", err.Error())
		return
	}
	response.Response(c, 0, "注册成功", gin.H{"user_id": req.UserID, "nickname": req.Nickname})
}

type LoginReq struct {
	UserID   int64  `json:"user_id" binding:"required"`
	Nickname string `json:"nickname" binding:"required,max=64"`
}

type RefreshTokenReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LoginHandler 用户登录接口（模拟鉴权 → JWT）
// POST /web/api/v1/user/login
func LoginHandler(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Response(c, 400, "参数解析失败或不合法", err.Error())
		return
	}
	if req.Nickname == "" {
		req.Nickname = fmt.Sprintf("用户%d", req.UserID)
	}

	// 注册或获取用户
	err := service.RegisterUser(c.Request.Context(), req.UserID, req.Nickname)
	if err != nil && err.Error() != "用户已存在" {
		response.Response(c, 500, "登录失败，请稍后重试", err.Error())
		return
	}

	// 生成 JWT tokens
	accessToken, refreshToken, err := auth.GenerateToken(req.UserID)
	if err != nil {
		response.Response(c, 500, "生成认证令牌失败", err.Error())
		return
	}

	// 保存 refresh token 到 Redis
	ctx := c.Request.Context()
	if err := repository.SaveRefreshToken(ctx, req.UserID, refreshToken); err != nil {
		response.Response(c, 500, "保存认证令牌失败", err.Error())
		return
	}

	response.Response(c, 0, "登录成功", gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
	})
}

// RefreshHandler 刷新 access token
// POST /web/api/v1/user/refresh
func RefreshHandler(c *gin.Context) {
	var req RefreshTokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Response(c, 400, "参数解析失败或不合法", err.Error())
		return
	}

	// 解析 refresh token
	claims, err := auth.ParseToken(req.RefreshToken)
	if err != nil {
		response.Response(c, 401, "Token 无效", err.Error())
		return
	}

	// 验证 Redis 中的 refresh token 是否匹配
	ctx := c.Request.Context()
	storedToken, err := repository.GetRefreshToken(ctx, claims.UserID)
	if err != nil {
		response.Response(c, 500, "Token 验证失败", err.Error())
		return
	}

	if storedToken != req.RefreshToken {
		response.Response(c, 401, "Token 无效或已吊销", nil)
		return
	}

	// 生成新的 tokens
	newAccessToken, newRefreshToken, err := auth.GenerateToken(claims.UserID)
	if err != nil {
		response.Response(c, 500, "生成新 Token 失败", err.Error())
		return
	}

	// 更新 Redis 中的 refresh token
	if err := repository.SaveRefreshToken(ctx, claims.UserID, newRefreshToken); err != nil {
		response.Response(c, 500, "保存新 Token 失败", err.Error())
		return
	}

	response.Response(c, 0, "Token 刷新成功", gin.H{
		"access_token":  newAccessToken,
		"refresh_token": newRefreshToken,
	})
}

// LogoutHandler 用户登出（吊销 refresh token）
// POST /web/api/v1/user/logout
func LogoutHandler(c *gin.Context) {
	userID := c.GetInt64(middleware.ContextUserIDKey)

	ctx := c.Request.Context()
	if err := repository.DeleteRefreshToken(ctx, userID); err != nil {
		response.Response(c, 500, "登出失败", err.Error())
		return
	}

	response.Response(c, 0, "登出成功", nil)
}

// FollowHandler 关注/取关接口（使用 JWT 鉴权）
func FollowHandler(c *gin.Context) {
	followerID := c.GetInt64(middleware.ContextUserIDKey)

	var req FollowReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Response(c, 400, "参数解析失败或不合法", err.Error())
		return
	}

	err := service.FollowAction(c.Request.Context(), followerID, req.FolloweeID, req.ActionType)
	if err != nil {
		response.Response(c, 500, "操作失败，请稍后重试", err.Error())
		return
	}
	response.Response(c, 0, "操作成功", nil)
}

// FollowStatusHandler 检查关注状态（使用 JWT 鉴权）
func FollowStatusHandler(c *gin.Context) {
	followerID := c.GetInt64(middleware.ContextUserIDKey)
	followeeIDStr := c.Query("followee_id")
	if followeeIDStr == "" {
		response.Response(c, 400, "缺少 followee_id 参数", "")
		return
	}

	var followeeID int64
	fmt.Sscanf(followeeIDStr, "%d", &followeeID)
	if followeeID <= 0 {
		response.Response(c, 400, "无效的 followee_id", "")
		return
	}

	following := service.IsFollowing(c.Request.Context(), followerID, followeeID)
	response.Response(c, 0, "查询成功", gin.H{"is_following": following})
}

// GetUserInfoHandler 获取用户信息（使用 JWT 鉴权）
func GetUserInfoHandler(c *gin.Context) {
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		response.Response(c, 400, "缺少 user_id 参数", "")
		return
	}

	var userID int64
	fmt.Sscanf(userIDStr, "%d", &userID)
	if userID <= 0 {
		response.Response(c, 400, "无效的 user_id", "")
		return
	}

	user, err := service.GetUserInfo(c.Request.Context(), userID)
	if err != nil || user == nil {
		response.Response(c, 404, "用户不存在", nil)
		return
	}

	// 获取关注数和粉丝数
	followingCount, followerCount, err := service.GetUserFollowStats(c.Request.Context(), userID)
	if err != nil {
		response.Response(c, 500, "查询失败", err.Error())
		return
	}

	response.Response(c, 0, "查询成功", gin.H{
		"user_id":         user.ID,
		"nickname":        user.Nickname,
		"follower_count":  followerCount,
		"following_count": followingCount,
	})
}

// UserCountHandler 获取用户关注/粉丝计数（独立计数查询接口）
func UserCountHandler(c *gin.Context) {
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		response.Response(c, 400, "缺少 user_id 参数", "")
		return
	}

	var userID int64
	fmt.Sscanf(userIDStr, "%d", &userID)
	if userID <= 0 {
		response.Response(c, 400, "无效的 user_id", "")
		return
	}

	followingCount, followerCount, err := service.GetUserFollowStats(c.Request.Context(), userID)
	if err != nil {
		response.Response(c, 500, "查询失败", err.Error())
		return
	}

	response.Response(c, 0, "查询成功", gin.H{
		"user_id":         userID,
		"follower_count":  followerCount,
		"following_count": followingCount,
	})
}

// SyncUserCountHandler 从数据库同步用户关注/粉丝计数到 Redis
func SyncUserCountHandler(c *gin.Context) {
	userIDStr := c.Query("user_id")
	if userIDStr == "" {
		response.Response(c, 400, "缺少 user_id 参数", "")
		return
	}

	var userID int64
	fmt.Sscanf(userIDStr, "%d", &userID)
	if userID <= 0 {
		response.Response(c, 400, "无效的 user_id", "")
		return
	}

	err := service.SyncUserCountFromDB(c.Request.Context(), userID)
	if err != nil {
		response.Response(c, 500, "同步失败", err.Error())
		return
	}

	response.Response(c, 0, "同步成功", nil)
}
