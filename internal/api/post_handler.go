package api

import (
	"github.com/DingAnZhong/feed/internal/repository"
	"github.com/DingAnZhong/feed/internal/service"
	"github.com/DingAnZhong/feed/pkg/logger"
	"github.com/DingAnZhong/feed/pkg/middleware"
	"github.com/DingAnZhong/feed/pkg/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type PublishPostReq struct {
	Content   string   `json:"content" binding:"required"`
	MediaUrls []string `json:"media_urls"`
}

func PublishPostHandler(c *gin.Context) {
	userID := c.GetInt64(middleware.ContextUserIDKey)

	var req PublishPostReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Response(c, 400, "参数解析失败", err.Error())
		return
	}

	postID, err := service.PublishPost(c.Request.Context(), userID, req.Content, req.MediaUrls)
	if err != nil {
		// 判断是否因敏感词导致发布失败
		if err.Error() == "post contains sensitive word: "+err.Error() {
			response.Response(c, 0, "发布失败：帖子包含敏感词，已进入审核", postID)
		} else {
			response.Response(c, 500, "发布帖子失败，请稍后重试", nil)
		}
		return
	}
	response.Response(c, 0, "发布帖子成功", postID)
}

// AdminPostPendingReq 待审核帖子列表请求
type AdminPostPendingReq struct {
	Page  int `json:"page" form:"page" default:"1"`
	Limit int `json:"limit" form:"limit" default:"20"`
}

// AdminPostActionReq 审核操作请求
type AdminPostActionReq struct {
	PostID int64 `json:"post_id" binding:"required"`
	Action string `json:"action" binding:"required,oneof=approve reject"`
}

// AdminPostPendingHandler 获取待审核帖子列表
func AdminPostPendingHandler(c *gin.Context) {
	var req AdminPostPendingReq
	if err := c.ShouldBindQuery(&req); err != nil {
		response.Response(c, 400, "参数解析失败", err.Error())
		return
	}

	// 分页参数校验
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Limit < 1 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	offset := (req.Page - 1) * req.Limit

	// 查询待审核帖子（status = 1 审核中）
	posts, total, err := repository.GetPendingPosts(c.Request.Context(), offset, req.Limit)
	if err != nil {
		logger.Error("AdminPostPendingHandler failed", zap.Error(err))
		response.Response(c, 500, "查询失败", nil)
		return
	}

	response.Response(c, 0, "查询成功", gin.H{
		"list":  posts,
		"total": total,
		"page":  req.Page,
		"limit": req.Limit,
	})
}

// AdminPostApproveHandler 审核通过帖子
func AdminPostApproveHandler(c *gin.Context) {
	var req AdminPostActionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Response(c, 400, "参数解析失败", err.Error())
		return
	}

	err := repository.UpdatePostStatus(c.Request.Context(), req.PostID, 0)
	if err != nil {
		logger.Error("AdminPostApproveHandler failed", zap.Error(err))
		response.Response(c, 500, "审核失败", nil)
		return
	}

	response.Response(c, 0, "审核通过", nil)
}

// AdminPostRejectHandler 审核不通过帖子
func AdminPostRejectHandler(c *gin.Context) {
	var req AdminPostActionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Response(c, 400, "参数解析失败", err.Error())
		return
	}

	err := repository.UpdatePostStatus(c.Request.Context(), req.PostID, 2)
	if err != nil {
		logger.Error("AdminPostRejectHandler failed", zap.Error(err))
		response.Response(c, 500, "审核失败", nil)
		return
	}

	response.Response(c, 0, "审核不通过", nil)
}
