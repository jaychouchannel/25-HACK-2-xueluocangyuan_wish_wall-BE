// internal/router/router.go
package router

import (
	"github.com/NCUHOME-Y/25-HACK-2-xueluocangyuan_wish_wall-BE/internal/app/handler"
	"github.com/NCUHOME-Y/25-HACK-2-xueluocangyuan_wish_wall-BE/internal/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupRouter 是你唯一的路由配置
func SetupRouter(db *gorm.DB) *gin.Engine {
	// 1. 使用 gin.New()，100% 手动控制中间件
	r := gin.New()

	// 2. 注册“全局”中间件 (CORS, Logger, Recovery)
	// (这会应用到 *所有* 路由)
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.LoggerMiddleware())
	r.Use(middleware.RecoveryMiddleware())

	// 3. 创建 /api 根路由组

	api := r.Group("/api")
	{
		// --- 3.1 公开路由 (Public Routes) ---
		// (不需要 Token)

		// 认证与用户
		api.POST("/register", func(c *gin.Context) { handler.Register(c, db) })
		api.POST("/login", func(c *gin.Context) { handler.Login(c, db) })

		// 应用状态
		api.GET("/app-state", handler.GetAppState)

		api.POST("/test-ai", func(c *gin.Context) {
			handler.TestAI(c)
		})

		// 愿望墙 (公开)
		// 获取公开愿望列表
		api.GET("/wishes/public", func(c *gin.Context) {
			handler.GetPublicWishes(c, db)
		})

		// --- 3.2 受保护路由 (Protected Routes) ---
		// (所有在这个组里的，都必须先通过 JWTAuthMiddleware)

		auth := api.Group("/")
		auth.Use(middleware.JWTAuthMiddleware()) // (我们合规的 code: 3 版本)
		{
			// === 认证与用户 (User) ===
			auth.GET("/user/me", func(c *gin.Context) { handler.GetUserMe(c, db) })
			auth.PUT("/user", func(c *gin.Context) { handler.UpdateUser(c, db) })
			auth.POST("/user/complete-review", func(c *gin.Context) {
				handler.CompleteV2Review(c, db)
			})

			// === 愿望墙 (Wishes) ===
			// POST /api/wishes - 发布新愿望
			auth.POST("/wishes", func(c *gin.Context) {
				handler.CreateWish(c, db)
			})

			// GET /api/wishes/me - 获取个人星河列表
			auth.GET("/wishes/me", func(c *gin.Context) {
				handler.GetMyWishes(c, db)
			})

			// DELETE /api/wishes/:id - 删除指定ID的愿望
			auth.DELETE("/wishes/:id", func(c *gin.Context) {
				handler.DeleteWish(c, db)
			})

			// === 互动 (Interactions) ===

			// POST /api/wishes/:id/like - 点赞/取消点赞
			auth.POST("/wishes/:id/like", func(c *gin.Context) {
				handler.LikeWish(c, db)
			})

			// POST /api/wishes/:id/comment - 评论愿望
			auth.POST("/wishes/:id/comment", func(c *gin.Context) {
				handler.CreateComment(c, db)
			})

			// GET /api/wishes/:id/interactions - 查看评论和点赞列表

			auth.GET("/wishes/:id/interactions", func(c *gin.Context) {
				handler.GetInteractions(c, db)
			})
		}
	}

	return r
}