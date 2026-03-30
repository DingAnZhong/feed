package api

import (
	"github.com/DingAnZhong/feed/pkg/middleware"
	"github.com/gin-gonic/gin"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()

	r.Use(middleware.RateLimitMiddleware())
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"msg": "pong"})
	})

	v1 := r.Group("/api/v1")
	v1.Use(middleware.AuthMiddleware())
	{
		v1.POST("/user/follow", FollowHandler)
		v1.POST("/post/publish", PublishPostHandler)
		v1.GET("/feed/timeline", FetchFeedHandler)
	}

	return r
}
