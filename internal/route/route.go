package route

import (
	"github.com/cashvio/cashvio-be/internal/config"
	"github.com/cashvio/cashvio-be/internal/handler"
	"github.com/cashvio/cashvio-be/internal/middleware"
	"github.com/gin-gonic/gin"
)

func Setup(cfg *config.Config, userHandler *handler.UserHandler, cardHandler *handler.CardHandler, walletHandler *handler.WalletHandler, transactionHandler *handler.TransactionHandler, transferHandler *handler.TransferHandler, budgetOverviewHandler *handler.BudgetOverviewHandler, categoryBudgetHandler *handler.CategoryBudgetHandler, cashHandler *handler.CashHandler, dashboardHandler *handler.DashboardHandler) *gin.Engine {
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
			users.GET("/me", userHandler.Me)
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

		transactions := api.Group("/transactions")
		{
			transactions.POST("", transactionHandler.CreateTransaction)
			transactions.GET("", transactionHandler.GetTransactions)
			transactions.GET("/:id", transactionHandler.GetTransaction)
			transactions.PUT("/:id", transactionHandler.UpdateTransaction)
			transactions.DELETE("/:id", transactionHandler.DeleteTransaction)
		}

		transfers := api.Group("/transfers")
		{
			transfers.POST("", transferHandler.CreateTransfer)
			transfers.GET("", transferHandler.GetTransfers)
			transfers.GET("/:id", transferHandler.GetTransfer)
			transfers.DELETE("/:id", transferHandler.DeleteTransfer)
		}

		cash := api.Group("/cash")
		{
			cash.GET("", cashHandler.GetCash)
			cash.POST("/withdrawals", cashHandler.CreateWithdrawal)
			cash.GET("/withdrawals", cashHandler.GetWithdrawals)
			cash.GET("/withdrawals/:id", cashHandler.GetWithdrawal)
			cash.DELETE("/withdrawals/:id", cashHandler.DeleteWithdrawal)
		}

		budgets := api.Group("/budgets")
		{
			budgets.GET("/overview", budgetOverviewHandler.GetBudgetOverview)
		}

		dashboard := api.Group("/dashboard")
		{
			dashboard.GET("/overview", dashboardHandler.GetOverview)
		}

		categoryBudgets := api.Group("/category-budgets")
		{
			categoryBudgets.POST("", categoryBudgetHandler.CreateCategoryBudget)
			categoryBudgets.GET("", categoryBudgetHandler.GetCategoryBudgets)
			categoryBudgets.GET("/:id", categoryBudgetHandler.GetCategoryBudget)
			categoryBudgets.PUT("/:id", categoryBudgetHandler.UpdateCategoryBudget)
			categoryBudgets.DELETE("/:id", categoryBudgetHandler.DeleteCategoryBudget)
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
