package router

import (
	"gacha-simulator/internal/admin"
	"gacha-simulator/internal/auth"
	"gacha-simulator/internal/gacha"
	"gacha-simulator/internal/notification"

	"github.com/gin-gonic/gin"
)

func SetupManagementRoutes(public *gin.RouterGroup, protected *gin.RouterGroup, adminOnly *gin.RouterGroup) {
	if public != nil {
		public.POST("/register", auth.RegisterHandler)
		public.POST("/login", auth.LoginHandler)
		public.GET("/notifications", notification.SSEHandler)

		public.POST("/auth/email/send-code", auth.SendEmailCodeHandler)
		public.POST("/auth/email/login", auth.EmailLoginHandler)
		public.GET("/auth/github/login", auth.GithubLoginHandler)
		public.GET("/auth/github/callback", auth.GithubCallbackHandler)
		public.POST("/auth/github/callback", auth.GithubCallbackHandler)
	}

	if protected != nil {
		protected.PUT("/user/profile", auth.UpdateProfileHandler)
		protected.POST("/user/logout", auth.LogoutHandler)
		protected.POST("/user/divination", gacha.DivinationHandler)
		protected.GET("/user/divination", gacha.GetDivinationHandler)
		protected.GET("/user/me", auth.GetMeHandler)
	}

	if adminOnly != nil {
		adminOnly.GET("/characters", admin.GetAdminCharactersHandler)
		adminOnly.POST("/character", admin.CreateCharacterHandler)
		adminOnly.POST("/pool/push", admin.PushCharacterToPoolHandler)
		adminOnly.POST("/pool/load-presets", admin.LoadPresetsHandler)
		adminOnly.PUT("/pool/config", admin.UpdatePoolConfigHandler)
	}
}

func SetupGameRoutes(public *gin.RouterGroup, protected *gin.RouterGroup) {
	if public != nil {
		public.GET("/pool/info", admin.GetPoolInfoHandler)
	}
	if protected != nil {
		protected.POST("/gacha/draw", gacha.DrawHandler)
		protected.GET("/gacha/inventory", gacha.GetUserInventoryHandler)
		protected.GET("/gacha/history", gacha.GetGachaHistoryHandler)
		protected.GET("/gacha/stats", gacha.GetGachaStatsHandler)
		protected.DELETE("/gacha/history", gacha.ClearHistoryHandler)
		protected.POST("/gacha/simulate", gacha.SimulateGachaHandler)
	}
}

func SetupManagementEngine() *gin.Engine {
	r := gin.Default()

	r.StaticFile("/", "./web/index.html")
	r.Static("/web", "./web")

	public := r.Group("/api")
	protected := r.Group("/api")
	protected.Use(auth.AuthMiddleware())
	adminOnly := protected.Group("/admin")
	adminOnly.Use(auth.AdminRequired())

	SetupManagementRoutes(public, protected, adminOnly)
	return r
}

func SetupGameEngine() *gin.Engine {
	r := gin.Default()

	public := r.Group("/api")
	protected := r.Group("/api")
	protected.Use(auth.AuthMiddleware())

	SetupGameRoutes(public, protected)
	return r
}

func SetupRouter() *gin.Engine {
	r := gin.Default()

	// Static web interface
	r.StaticFile("/", "./web/index.html")
	r.Static("/web", "./web")

	public := r.Group("/api")
	protected := r.Group("/api")
	protected.Use(auth.AuthMiddleware())
	adminOnly := protected.Group("/admin")
	adminOnly.Use(auth.AdminRequired())

	SetupManagementRoutes(public, protected, adminOnly)
	SetupGameRoutes(public, protected)
	return r
}
