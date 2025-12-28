package config

import (
	"bytes"
	"context"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"go-pr-review/internal/domain"
	"log"
	"os"
)

// DefaultRepoConfig 提供兜底默认值
var DefaultRepoConfig = domain.RepoConfig{
	Language:          "en-US",
	ExtraInstructions: "",
	IgnorePatterns:    []string{"go.sum", "go.mod", "*.lock"},
}

// LoadRepoConfig 尝试从远程仓库加载配置，如果失败则返回默认值
// 注意：这需要依赖 GitProvider，所以不能在 main 的一开始就调用，而是在 Service 初始化后调用
func Load() *domain.Config {
	_ = godotenv.Load()
	cfg := &domain.Config{
		GithubToken:   os.Getenv("GITHUB_TOKEN"),
		OpenAIKey:     os.Getenv("OPENAI_API_KEY"),
		WebHookSecret: os.Getenv("WEBHOOK_SECRET"),
		Port:          os.Getenv("PORT"),
	}

	if cfg.GithubToken == "" || cfg.OpenAIKey == "" {
		log.Fatal("Missing env vars: GITHUB_TOKEN or OPENAI_API_KEY")
	}

	return cfg
}

// LoadRepoConfig 尝试从远程仓库加载配置，如果失败则返回默认值
// 注意：这需要依赖 GitProvider，所以不能在 main 的一开始就调用，而是在 Service 初始化后调用
func LoadRepoConfig(ctx context.Context, git domain.GitProvider, owner, repo string) domain.RepoConfig {
	config := DefaultRepoConfig

	// 尝试读取 .ai-review.yaml
	log.Println("🔍 Checking for .ai-review.yaml in repository...")
	content, err := git.GetFileContent(ctx, owner, repo, ".ai-review.yaml", "")
	if err != nil {
		log.Println("⚠️ Config file not found or unreadable, using defaults.")
		return config
	}

	// 使用Viper解析
	v := viper.New()
	v.SetConfigType("yaml")
	if err = v.ReadConfig(bytes.NewBufferString(content)); err != nil {
		log.Printf("⚠️ Failed to parse config file: %v", err)
		return config
	}
	if err := v.Unmarshal(&config); err != nil {
		log.Printf("⚠️ Failed to unmarshal config: %v", err)
		return config
	}
	log.Printf("✅ Loaded custom config: Language=%s, Ignores=%v", config.Language, config.IgnorePatterns)
	return config
}
