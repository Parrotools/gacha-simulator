package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func initDB() {
	var err error
	DB, err = gorm.Open(sqlite.Open("gacha.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Connection to database error:%v", err)
	}
	err = DB.AutoMigrate(
		&User{},
		&Character{},
		&UserCharacter{},
		&GachaRecord{},
	)
	if err != nil {
		log.Fatalf("migration failed: %v", err)
	}
	var adminCount int64
	DB.Model(&User{}).Where("role = ?", "admin").Count(&adminCount)
	if adminCount == 0 {
		hashedPwd, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		defaultAdmin := User{
			ID:       "admin",
			Nickname: "Overall admin",
			Password: string(hashedPwd),
			Role:     "admin",
		}
		DB.Create(&defaultAdmin)
	}
}

func setupRouter() *gin.Engine {
	r := gin.Default()

	// Static web interface
	r.StaticFile("/", "./web/index.html")
	r.Static("/web", "./web")

	public := r.Group("/api")
	{
		public.POST("/register", RegisterHandler)
		public.POST("/login", LoginHandler)
		public.GET("/pool/info", GetPoolInfoHandler)
		public.GET("/notifications", SSEHandler)

		public.POST("/auth/email/send-code", SendEmailCodeHandler)
		public.POST("/auth/email/login", EmailLoginHandler)
		public.GET("/auth/github/login", GithubLoginHandler)
		public.GET("/auth/github/callback", GithubCallbackHandler)
		public.POST("/auth/github/callback", GithubCallbackHandler)
	}
	protected := r.Group("/api")
	protected.Use(AuthMiddleware())
	{
		protected.PUT("/user/profile", UpdateProfileHandler)
		protected.POST("/user/logout", LogoutHandler)
		protected.GET("/user/me", func(c *gin.Context) {
			userID, _ := c.Get("userID")
			role, _ := c.Get("role")
			var user User
			if err := DB.Where("id = ?", userID).First(&user).Error; err == nil {
				c.JSON(http.StatusOK, gin.H{
					"user_id":      user.ID,
					"nickname":     user.Nickname,
					"role":         user.Role,
					"bio":          user.Bio,
					"pity_s_count": user.PitySCount,
					"pity_a_count": user.PityACount,
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{"user_id": userID, "role": role})
		})

		protected.POST("/gacha/draw", DrawHandler)
		protected.GET("/gacha/inventory", GetUserInventoryHandler)
		protected.GET("/gacha/history", GetGachaHistoryHandler)
		protected.GET("/gacha/stats", GetGachaStatsHandler)
		protected.DELETE("/gacha/history", ClearHistoryHandler)

		adminOnly := protected.Group("/admin")
		adminOnly.Use(AdminRequired())
		{
			adminOnly.GET("/characters", GetAdminCharactersHandler)
			adminOnly.POST("/character", CreateCharacterHandler)
			adminOnly.POST("/pool/push", PushCharacterToPoolHandler)
			adminOnly.POST("/pool/load-presets", LoadPresetsHandler)
			adminOnly.PUT("/pool/config", UpdatePoolConfigHandler)
		}
	}
	return r
}

func main() {
	initDB()
	go StartGRPCServer(":50051")
	go StartGRPCConfigClient("localhost:50051")
	r := setupRouter()
	r.Run(":8080")
}
