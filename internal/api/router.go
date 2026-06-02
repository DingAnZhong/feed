package api

import (
	"embed"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/DingAnZhong/feed/pkg/middleware"
	"github.com/gin-gonic/gin"
)

//go:embed frontend-dist
var dist embed.FS

func SetupRouter() *gin.Engine {
	r := gin.Default()

	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.RateLimitMiddleware())

	// 公开 API（不需要鉴权）
	r.POST("/web/api/v1/user/register", RegisterHandler)
	r.POST("/web/api/v1/user/login", LoginHandler)
	r.POST("/web/api/v1/user/refresh", RefreshHandler)
	r.POST("/web/api/v1/user/logout", LogoutHandler)

	// API 路由 (鉴权)
	v1 := r.Group("/web/api/v1")
	v1.Use(middleware.AuthMiddleware())
	{
		v1.POST("/user/follow", FollowHandler)
		v1.GET("/user/follow/status", FollowStatusHandler)
		v1.GET("/user/info", GetUserInfoHandler)
		v1.GET("/user/count", UserCountHandler)
		v1.POST("/user/sync-count", SyncUserCountHandler)
		v1.POST("/post/publish", PublishPostHandler)
		v1.GET("/feed/timeline", FetchFeedHandler)

		// Admin 路由（需要管理员权限）
		v1.GET("/admin/posts/pending", AdminPostPendingHandler)
		v1.POST("/admin/post/approve", AdminPostApproveHandler)
		v1.POST("/admin/post/reject", AdminPostRejectHandler)
	}

	// ping 端点
	r.GET("/web/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"msg": "pong"})
	})

	// 前端 SPA: 根路径返回 index.html
	r.GET("/", func(c *gin.Context) {
		data, _ := dist.ReadFile("frontend-dist/index.html")
		c.Data(200, "text/html; charset=utf-8", data)
	})

	// 前端静态资源处理 (JS / CSS / 图片等) — 支持多级路径
	r.GET("/static/*filepath", serveStaticFile)

	// SPA fallback: 所有未匹配的 GET 请求都返回 index.html
	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method == "GET" {
			data, _ := dist.ReadFile("frontend-dist/index.html")
			c.Data(200, "text/html; charset=utf-8", data)
			return
		}
		c.AbortWithStatus(404)
	})

	return r
}

// serveStaticFile 根据文件路径从 embed FS 中读取文件并返回
func serveStaticFile(c *gin.Context) {
	path := c.Param("filepath")
	if path == "" {
		c.AbortWithStatus(404)
		return
	}
	// *filepath 会包含开头的 /，去掉它
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	fullPath := "frontend-dist/" + path

	// 防止目录遍历攻击
	if strings.Contains(path, "..") {
		c.AbortWithStatus(404)
		return
	}

	data, err := dist.ReadFile(fullPath)
	if err != nil {
		c.AbortWithStatus(404)
		return
	}

	// 自动识别 Content-Type
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	// 没有扩展名的文件 (如 index.html 在根路径)
	if filepath.Ext(path) == "" && strings.HasSuffix(path, ".html") {
		contentType = "text/html; charset=utf-8"
	}

	// 禁止浏览器缓存静态资源，确保更新后客户端能获取最新文件
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	c.Data(http.StatusOK, contentType, data)
}
