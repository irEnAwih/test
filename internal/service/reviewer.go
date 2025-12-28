package service

import (
	"context"
	"fmt"
	"go-pr-review/internal/config"
	"go-pr-review/internal/domain"
	"log"
	"path/filepath"
)

type ReviewService struct {
	git domain.GitProvider
	ai  domain.AIProvider
}

func NewReviewService(git domain.GitProvider, ai domain.AIProvider) *ReviewService {
	return &ReviewService{git: git, ai: ai}
}

func (s *ReviewService) Run(ctx context.Context, cfg *domain.Config) {
	log.Printf("🚀 Starting V0.6 Review & Describe...")

	repoConfig := config.LoadRepoConfig(ctx, s.git, cfg.RepoOwner, cfg.RepoName)
	cfg.RepoConfig = repoConfig // 保存起来

	// 1. 获取 Diff
	diffs, err := s.git.GetPRDiff(ctx, cfg.RepoOwner, cfg.RepoName, cfg.PRNumber)
	if err != nil {
		log.Fatalf("❌ Diff Error: %v", err)
	}

	filteredDiffs := filterDiffs(diffs, repoConfig.IgnorePatterns)
	// ===========================
	// Feature 1: AI Code Review
	// ===========================
	// ... (原有的并发 Review 逻辑保持不变) ...
	// wg.Wait()
	// postReview(...)

	// ===========================
	// Feature 2: AI Describe PR
	// ===========================
	log.Println("📝 Generating PR Description...")

	description, err := s.ai.DescribePR(ctx, filteredDiffs, cfg.RepoConfig)
	if err != nil {
		log.Printf("⚠️ Failed to generate description: %v", err)
	} else {
		// 组装最终的 Markdown Body
		// 我们通常保留用户原有的描述吗？pr-agent 的做法是覆盖，或者追加。
		// 这里我们演示：[AI Generated] 部分 + 详细变更

		fullBody := fmt.Sprintf(`
			## 🤖 AI Generated Summary
			%s
			
			### 🔍 Key Changes
			%s
			
			---
			*Powered by Go-PR-Review*
			`, description.Summary, description.Changes)

		log.Printf("✨ Updating PR Title to: %s", description.Title)

		err := s.git.UpdatePRInfo(ctx, cfg.RepoOwner, cfg.RepoName, cfg.PRNumber, description.Title, fullBody)
		if err != nil {
			log.Printf("❌ Failed to update PR info: %v", err)
		} else {
			log.Println("✅ PR Description updated successfully!")
		}
	}
}

// 简单的 glob 匹配辅助函数
func filterDiffs(diffs []*domain.FileDiff, patterns []string) []*domain.FileDiff {
	var keep []*domain.FileDiff
	for _, d := range diffs {
		ignored := false
		for _, p := range patterns {
			// 使用 path/filepath.Match
			if matched, _ := filepath.Match(p, d.FilePath); matched {
				ignored = true
				break
			}
		}
		if !ignored {
			keep = append(keep, d)
		}
	}
	return keep
}
