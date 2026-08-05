package route

import (
	"github.com/cashvio/cashvio-be/internal/config"
	"github.com/cashvio/cashvio-be/internal/handler"
	"github.com/cashvio/cashvio-be/internal/middleware"
	"github.com/gin-gonic/gin"
)

func Setup(cfg *config.Config, userHandler *handler.UserHandler, cardHandler *handler.CardHandler, walletHandler *handler.WalletHandler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(middleware.RecoveryMiddleware())
	r.Use(middleware.CORSMiddleware())

	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/register", userHandler.Register)
		auth.POST("/login", userHandler.Login)
	}

	api := r.Group("/api/v1")
	api.Use(middleware.JWTAuth(cfg.JWT.Secret))
	{
		users := api.Group("/users")
		{
			users.GET("", userHandler.GetUsers)
			users.GET("/:id", userHandler.GetUser)
			users.PUT("/:id", userHandler.UpdateUser)
			users.DELETE("/:id", userHandler.DeleteUser)
		}

		cards := api.Group("/cards")
		{
			cards.POST("", cardHandler.CreateCard)
			cards.GET("", cardHandler.GetCards)
			cards.GET("/:id", cardHandler.GetCard)
			cards.PUT("/:id", cardHandler.UpdateCard)
			cards.DELETE("/:id", cardHandler.DeleteCard)
		}

		wallets := api.Group("/wallets")
		{
			wallets.POST("", walletHandler.CreateWallet)
			wallets.GET("", walletHandler.GetWallets)
			wallets.GET("/:id", walletHandler.GetWallet)
			wallets.PUT("/:id", walletHandler.UpdateWallet)
			wallets.DELETE("/:id", walletHandler.DeleteWallet)
		}
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	r.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{"error": "not found"})
	})

	return r
}
