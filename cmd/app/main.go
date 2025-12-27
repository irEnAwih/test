package main

import (
	"context"
	"go-pr-review/internal/config"
	"go-pr-review/internal/infra/ai"
	"go-pr-review/internal/infra/git"
	"go-pr-review/internal/service"
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

	// 3. 运行
	reviewer.Run(context.Background(), cfg)
}
