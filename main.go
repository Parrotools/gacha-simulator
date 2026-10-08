package main

import (
	"flag"
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
		&DivinationRecord{},
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

func setupManagementRoutes(public *gin.RouterGroup, protected *gin.RouterGroup, adminOnly *gin.RouterGroup) {
	if public != nil {
		public.POST("/register", RegisterHandler)
		public.POST("/login", LoginHandler)
		public.GET("/notifications", SSEHandler)

		public.POST("/auth/email/send-code", SendEmailCodeHandler)
		public.POST("/auth/email/login", EmailLoginHandler)
		public.GET("/auth/github/login", GithubLoginHandler)
		public.GET("/auth/github/callback", GithubCallbackHandler)
		public.POST("/auth/github/callback", GithubCallbackHandler)
	}

	if protected != nil {
		protected.PUT("/user/profile", UpdateProfileHandler)
		protected.POST("/user/logout", LogoutHandler)
		protected.POST("/user/divination", DivinationHandler)
		protected.GET("/user/divination", GetDivinationHandler)
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
	}

	if adminOnly != nil {
		adminOnly.GET("/characters", GetAdminCharactersHandler)
		adminOnly.POST("/character", CreateCharacterHandler)
		adminOnly.POST("/pool/push", PushCharacterToPoolHandler)
		adminOnly.POST("/pool/load-presets", LoadPresetsHandler)
		adminOnly.PUT("/pool/config", UpdatePoolConfigHandler)
	}
}

func setupGameRoutes(public *gin.RouterGroup, protected *gin.RouterGroup) {
	if public != nil {
		public.GET("/pool/info", GetPoolInfoHandler)
	}
	if protected != nil {
		protected.POST("/gacha/draw", DrawHandler)
		protected.GET("/gacha/inventory", GetUserInventoryHandler)
		protected.GET("/gacha/history", GetGachaHistoryHandler)
		protected.GET("/gacha/stats", GetGachaStatsHandler)
		protected.DELETE("/gacha/history", ClearHistoryHandler)
		protected.POST("/gacha/simulate", SimulateGachaHandler)
	}
}

func setupManagementEngine() *gin.Engine {
	r := gin.Default()

	r.StaticFile("/", "./web/index.html")
	r.Static("/web", "./web")

	public := r.Group("/api")
	protected := r.Group("/api")
	protected.Use(AuthMiddleware())
	adminOnly := protected.Group("/admin")
	adminOnly.Use(AdminRequired())

	setupManagementRoutes(public, protected, adminOnly)
	return r
}

func setupGameEngine() *gin.Engine {
	r := gin.Default()

	public := r.Group("/api")
	protected := r.Group("/api")
	protected.Use(AuthMiddleware())

	setupGameRoutes(public, protected)
	return r
}

func setupRouter() *gin.Engine {
	r := gin.Default()

	// Static web interface
	r.StaticFile("/", "./web/index.html")
	r.Static("/web", "./web")

	public := r.Group("/api")
	protected := r.Group("/api")
	protected.Use(AuthMiddleware())
	adminOnly := protected.Group("/admin")
	adminOnly.Use(AdminRequired())

	setupManagementRoutes(public, protected, adminOnly)
	setupGameRoutes(public, protected)
	return r
}

func main() {
	mode := flag.String("mode", "all", "Server run mode: all, management, game")
	port := flag.String("port", "", "Server HTTP port (defaults: 8080 for all/management, 8081 for game)")
	flag.Parse()

	switch *mode {
	case "management":
		initDB()
		go StartGRPCServer(":50051")
		r := setupManagementEngine()
		p := ":8080"
		if *port != "" {
			p = ":" + *port
		}
		log.Printf("Starting Management Server on %s (gRPC on :50051)...", p)
		if err := r.Run(p); err != nil {
			log.Fatalf("Management Server failed: %v", err)
		}
	case "game":
		initDB()
		go StartGRPCConfigClient("localhost:50051")
		r := setupGameEngine()
		p := ":8081"
		if *port != "" {
			p = ":" + *port
		}
		log.Printf("Starting Game Server on %s (gRPC connecting to localhost:50051)...", p)
		if err := r.Run(p); err != nil {
			log.Fatalf("Game Server failed: %v", err)
		}
	default:
		initDB()
		go StartGRPCServer(":50051")
		go StartGRPCConfigClient("localhost:50051")
		r := setupRouter()
		p := ":8080"
		if *port != "" {
			p = ":" + *port
		}
		log.Printf("Starting Combined Server on %s (gRPC on :50051)...", p)
		if err := r.Run(p); err != nil {
			log.Fatalf("Combined Server failed: %v", err)
		}
	}
}
