package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// 0. 加载 .env 文件 (方便本地开发)
	_ = godotenv.Load()

	// 获取 API Key
	githubToken := os.Getenv("GITHUB_TOKEN")
	openaiKey := os.Getenv("OPENAI_API_KEY")

	if githubToken == "" || openaiKey == "" {
		log.Fatal("Error: GITHUB_TOKEN and OPENAI_API_KEY environment variables are required")
	}

	// 1. 解析命令行参数
	// 示例用法: ./go-pr-review -owner="qodo-ai" -repo="pr-agent" -pr=1
	owner := flag.String("owner", "", "GitHub Repository Owner")
	repo := flag.String("repo", "", "GitHub Repository Name")
	prNum := flag.Int("pr", 0, "Pull Request Number")
	flag.Parse()

	if *owner == "" || *repo == "" || *prNum == 0 {
		log.Fatal("Usage: go run . -owner=OWNER -repo=REPO -pr=NUMBER")
	}

	ctx := context.Background()

	// 2. 初始化 GitHub 客户端并获取 Diff
	fmt.Printf("🔍 Fetching Diff for %s/%s PR #%d...\n", *owner, *repo, *prNum)
	ghProvider := NewGithubProvider(githubToken)
	diff, err := ghProvider.GetPRDiff(ctx, *owner, *repo, *prNum)
	if err != nil {
		log.Fatalf("Failed to fetch diff: %v", err)
	}

	fmt.Printf("✅ Diff fetched! Size: %d chars.\n", len(diff))
	// 简单保护：如果 Diff 太长，先截断，防止爆 Token (生产环境需要更复杂的 Chunk 逻辑)
	if len(diff) > 10000 {
		fmt.Println("⚠️ Diff is too large, truncating to first 10000 chars for demo...")
		diff = diff[:10000]
	}

	// 3. 初始化 AI 客户端并进行审查
	fmt.Println("🤖 Asking AI to review...")
	aiReviewer := NewAIReviewer(openaiKey)
	review, err := aiReviewer.ReviewCode(ctx, diff)
	if err != nil {
		log.Fatalf("Failed to review code: %v", err)
	}

	// 4. 输出结果
	fmt.Println("\n================ AI CODE REVIEW RESULT ================\n")
	fmt.Println(review)
	fmt.Println("\n=======================================================")
}
