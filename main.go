package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)
func initDB(){
	var err error
	DB,err = gorm.Open(sqlite.Open("gacha.db"), &gorm.Config{})
	if err != nil{
		log.Fatalf("Connection to database error:%v",err)
	}
	err = DB.AutoMigrate(
		&User{},
		&Character{},
		&UserCharacter{},
		&GachaRecord{},
	)
	if err != nil{
		log.Fatalf("migration failed", err)
	}
	var adminCount int64
	DB.Model(&User{}).Where("role = ?", "admin").Count(&adminCount)
	if adminCount == 0 {
		hashedPwd,_:=bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		defaultAdmin := User{
			ID: "admin",
			Nickname:"Overall admin",
			Password: string(hashedPwd),
			Role: "admin",
		}
		DB.Create(&defaultAdmin)
	}
}
func main(){
	initDB()
	r:=gin.Default()
	public := r.Group("/api")
	{
		public.POST("/register", RegisterHandler)
		public.POST("/login", LoginHandler)
		public.GET("/pool/info", GetPoolInfoHandler)
	}
	protected:=r.Group("/api")
	protected.Use(AuthMiddleware())
	{
		protected.PUT("/user/profile",UpdateProfileHandler)
		protected.POST("/user/logout",LogoutHandler)
		protected.GET("/user/me", func(c *gin.Context) {
			userID, _ := c.Get("userID")
			role, _ := c.Get("role")
			c.JSON(http.StatusOK, gin.H{"user_id": userID, "role": role})
		})
		adminOnly := protected.Group("/admin")
		adminOnly.Use(AdminRequired())
		{
			adminOnly.POST("/character", CreateCharacterHandler)
			adminOnly.POST("/pool/push", PushCharacterToPoolHandler)
		}
	}
	r.Run(":8080")
}
