package main

import (
	"github.com/gin-gonic/gin"
	"github.com/linweiyuan/go-chatgpt-api/api"
	"github.com/linweiyuan/go-chatgpt-api/api/chatgpt"
	"github.com/linweiyuan/go-chatgpt-api/api/platform"
	_ "github.com/linweiyuan/go-chatgpt-api/env"
	"github.com/linweiyuan/go-chatgpt-api/middleware"
	"log"
	"os"
	"strings"
)


func init() {
	gin.ForceConsoleColor()
	gin.SetMode(gin.ReleaseMode)
}

//goland:noinspection SpellCheckingInspection
func main() {
	router := gin.Default()

	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.CheckHeaderMiddleware())

	setupChatGPTAPIs(router)
	setupPlatformAPIs(router)
	setupPandoraAPIs(router)
	router.NoRoute(api.Proxy)

	router.GET("/healthCheck", api.HealthCheck)

	port := os.Getenv("GO_CHATGPT_API_PORT")
	if port == "" {
		port = "4141"
	}
	err := router.Run(":" + port)
	if err != nil {
		log.Fatal("Failed to start server: " + err.Error())
	}
}

//goland:noinspection SpellCheckingInspection
func setupChatGPTAPIs(router *gin.Engine) {
	chatgptGroup := router.Group("/chatgpt")
	{
		chatgptGroup.POST("/login", chatgpt.Login)

		conversationsGroup := chatgptGroup.Group("/conversations")
		{
			conversationsGroup.GET("", chatgpt.GetConversations)

			// PATCH is official method, POST is added for Java support
			conversationsGroup.PATCH("", chatgpt.ClearConversations)
			conversationsGroup.POST("", chatgpt.ClearConversations)
		}

		conversationGroup := chatgptGroup.Group("/conversation")
		{
			conversationGroup.POST("", chatgpt.CreateConversation)
			conversationGroup.POST("/gen_title/:id", chatgpt.GenerateTitle)
			conversationGroup.GET("/:id", chatgpt.GetConversation)

			// rename or delete conversation use a same API with different parameters
			conversationGroup.PATCH("/:id", chatgpt.UpdateConversation)
			conversationGroup.POST("/:id", chatgpt.UpdateConversation)

			conversationGroup.POST("/message_feedback", chatgpt.FeedbackMessage)
		}

		// misc
		chatgptGroup.GET("/models", chatgpt.GetModels)
		chatgptGroup.GET("/accounts/check", chatgpt.GetAccountCheck)
	}
}

func setupPlatformAPIs(router *gin.Engine) {
	platformGroup := router.Group("/platform")
	{
		platformGroup.POST("/login", platform.Login)

		apiGroup := platformGroup.Group("/v1")
		{
			apiGroup.POST("/chat/completions", platform.CreateChatCompletions)
			apiGroup.POST("/completions", platform.CreateCompletions)
			apiGroup.POST("/embeddings", platform.CreateEmbeddings)
			apiGroup.GET("/files", platform.ListFiles)
			apiGroup.POST("/moderations", platform.CreateModeration)
			apiGroup.GET("/dashboard/billing/credit_grants", platform.GetCreditGrants)
			apiGroup.GET("/dashboard/billing/subscription", platform.GetSubscription)
			apiGroup.GET("/dashboard/billing/usage", platform.GetGetUsage)
			apiGroup.GET("/dashboard/user/api_keys", platform.GetApiKeys)
		}

		//dashboardGroup := platformGroup.Group("/dashboard")
		//{
		//	billingGroup := dashboardGroup.Group("/billing")
		//	{
		//		billingGroup.GET("/credit_grants", platform.GetCreditGrants)
		//		billingGroup.GET("/subscription", platform.GetSubscription)
		//	}
		//
		//	userGroup := dashboardGroup.Group("/user")
		//	{
		//		userGroup.GET("/api_keys", platform.GetApiKeys)
		//	}
		//}

	}
}

//goland:noinspection SpellCheckingInspection
func setupPandoraAPIs(router *gin.Engine) {
	pandoraEnabled := os.Getenv("GO_CHATGPT_API_PANDORA") != ""
	if pandoraEnabled {
		router.GET("/api/*path", func(c *gin.Context) {
			c.Request.URL.Path = strings.ReplaceAll(c.Request.URL.Path, "/api", "/chatgpt/backend-api")
			router.HandleContext(c)
		})
		router.POST("/api/*path", func(c *gin.Context) {
			c.Request.URL.Path = strings.ReplaceAll(c.Request.URL.Path, "/api", "/chatgpt/backend-api")
			router.HandleContext(c)
		})
	}
}
