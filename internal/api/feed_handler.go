package api

import (
	"strconv"

	"github.com/DingAnZhong/feed/internal/model"
	"github.com/DingAnZhong/feed/internal/service"
	"github.com/DingAnZhong/feed/pkg/middleware"
	"github.com/DingAnZhong/feed/pkg/response"
	"github.com/gin-gonic/gin"
)

type FetchFeedData struct {
	Posts    []*model.Post `json:"posts"`
	NextTime int64         `json:"next_time"`
}

func FetchFeedHandler(c *gin.Context) {
	userID := c.GetInt64(middleware.ContextUserIDKey)

	latestTimeStr := c.DefaultQuery("latest_time", "0")
	limitStr := c.DefaultQuery("limit", "10")
	feedType := c.DefaultQuery("feed_type", "timeline") // timeline | popular

	latestTime, _ := strconv.ParseInt(latestTimeStr, 10, 64)
	limit, _ := strconv.Atoi(limitStr)

	var posts []*model.Post
	var err error
	var nextTime int64

	if feedType == "popular" {
		posts, nextTime, err = service.FetchPopularFeed(c.Request.Context(), limit)
	} else {
		posts, nextTime, err = service.FetchFeed(c.Request.Context(), userID, latestTime, limit)
	}
	if err != nil {
		response.Response(c, 500, "获取帖子失败", err.Error())
		return
	}
	response.Response(c, 0, "获取帖子成功", &FetchFeedData{
		Posts:    posts,
		NextTime: nextTime,
	})
}
