package api

import (
	"github.com/DingAnZhong/feed/internal/service"
	"github.com/DingAnZhong/feed/pkg/middleware"
	"github.com/DingAnZhong/feed/pkg/response"
	"github.com/gin-gonic/gin"
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
		response.Response(c, 200, "发布帖子失败，请稍后重试", nil)
		return
	}
	response.Response(c, 200, "发布帖子成功", postID)
}
