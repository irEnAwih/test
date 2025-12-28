package main

import (
	"github.com/gin-gonic/gin"
	"go-pr-review/internal/api"
	"go-pr-review/internal/config"
	"go-pr-review/internal/infra/ai"
	"go-pr-review/internal/infra/git"
	"go-pr-review/internal/service"
	"log"
)

func main() {
	// 1. 加载配置
	cfg := config.Load()

	// 2. 依赖注入 (Dependency Injection)
	// 初始化基础设施
	gitProvider := git.NewGitHubProvider(cfg.GithubToken)
	aiProvider := ai.NewOpenAIProvider(cfg.OpenAIKey)

	// 初始化业务服务
	reviewer := service.NewReviewService(gitProvider, aiProvider)

	r := gin.Default()

	// 出书画handler
	webhookHandler := api.NewWebhookHandler(reviewer, cfg.WebHookSecret)

	// 注册路由
	r.POST("/webhook", webhookHandler.Handler)
	r.GET("/health", func(c *gin.Context) { c.String(200, "OK") })

	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Server listening on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
