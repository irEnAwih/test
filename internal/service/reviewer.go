package service

import (
	"context"
	"fmt"
	"go-pr-review/internal/domain"
	"log"
)

type ReviewService struct {
	git domain.GitProvider
	ai  domain.AIProvider
}

func NewReviewService(git domain.GitProvider, ai domain.AIProvider) *ReviewService {
	return &ReviewService{git: git, ai: ai}
}

func (s *ReviewService) Run(ctx context.Context, cfg *domain.Config) {
	log.Printf("🚀 Starting review for %s/%s PR #%d", cfg.RepoOwner, cfg.RepoName, cfg.PRNumber)

	// 1. 获取结构化 Diff
	diffs, err := s.git.GetPRDiff(ctx, cfg.RepoOwner, cfg.RepoName, cfg.PRNumber)
	if err != nil {
		log.Fatalf("❌ Error fetching diff: %v", err)
	}

	log.Printf("📄 Found %d modified files (excluding ignores)", len(diffs))
	for _, f := range diffs {
		log.Printf("   - %s", f.FilePath)
	}

	// 2. 发送给 AI
	log.Println("🤖 Sending to AI for analysis...")
	review, err := s.ai.ReviewDiff(ctx, diffs)
	if err != nil {
		log.Fatalf("❌ Error from AI: %v", err)
	}

	// 3. 处理结果 (目前先打印)
	fmt.Println("\n================ REVIEW RESULT ================")
	fmt.Println(review)
	fmt.Println("===============================================")
}
