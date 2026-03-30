package api

import (
	"github.com/DingAnZhong/feed/internal/service"
	"github.com/DingAnZhong/feed/pkg/middleware"
	"github.com/DingAnZhong/feed/pkg/response"
	"github.com/gin-gonic/gin"
)

type FollowReq struct {
	FolloweeID int64 `json:"followee_id" binding:"required"`
	ActionType int   `json:"action_type" binding:"required,oneof=1 2"` // 1-关注, 2-取关
}

func FollowHandler(c *gin.Context) {
	followerID := c.GetInt64(middleware.ContextUserIDKey)

	var req FollowReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Response(c, 400, "参数解析失败或不合法", err.Error())
		return
	}

	err := service.FollowAction(c.Request.Context(), followerID, req.FolloweeID, req.ActionType)
	if err != nil {
		response.Response(c, 200, "操作失败，请稍后重试", nil)
		return
	}
	response.Response(c, 200, "操作成功", nil)
}
