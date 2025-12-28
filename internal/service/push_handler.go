package service

import (
	"context"
	"go-pr-review/internal/domain"
	"log"
)

type PushService struct {
	git domain.GitProvider
	ai  domain.AIProvider
}

func NewPushService(git domain.GitProvider, ai domain.AIProvider) *PushService {
	return &PushService{git: git, ai: ai}
}

func (s *PushService) HandlePush(ctx context.Context, owner, repo, sha string) {
	log.Printf("🚀 Processing Push %s/%s Commit %s", owner, repo, sha)

	// 1.获取 Diff
	diffs, err := s.git.GetCommitDiff(ctx, owner, repo, sha)
	if err != nil {
		log.Printf("❌ Failed to get commit diff: %v", err)
		return
	}

	// 2. 复用 Review 逻辑
	// 这里我们可以复用 ReviewFile，但需要注意 Commit Review 不需要 PRConfig 里的 prNumber
	// 假设我们有一个默认 Config
	config := domain.RepoConfig{Language: "en-US"} // 简化，实际应 load
	for _, d := range diffs {
		comments, err := s.ai.ReviewFile(ctx, d, config)
		if err != nil {
			continue
		}

		// 3.提交评论
		if len(comments) > 0 {
			s.git.PostCommitComment(ctx, owner, repo, sha, comments)
		}
	}
}
