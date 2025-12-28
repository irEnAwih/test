package service

import (
	"context"
	"go-pr-review/internal/domain"
	"log"
	"sync"
)

type ReviewService struct {
	git domain.GitProvider
	ai  domain.AIProvider
}

func NewReviewService(git domain.GitProvider, ai domain.AIProvider) *ReviewService {
	return &ReviewService{git: git, ai: ai}
}

func (s *ReviewService) Run(ctx context.Context, cfg *domain.Config) {
	log.Printf("🚀 Starting V0.3 Concurrent Review for %s/%s #%d", cfg.RepoOwner, cfg.RepoName, cfg.PRNumber)

	// 1. 获取 Diff
	diffs, err := s.git.GetPRDiff(ctx, cfg.RepoOwner, cfg.RepoName, cfg.PRNumber)
	if err != nil {
		log.Fatalf("❌ Error fetching diff: %v", err)
	}

	// 2. 并发审查 (Concurrency)
	var allComments []*domain.ReviewComment
	var mu sync.Mutex // 保护 allComments
	var wg sync.WaitGroup

	// 限制并发数为 5，防止触发 OpenAI 429 Rate Limit
	semaphore := make(chan struct{}, 5)

	log.Printf("🔥 analyzing %d files concurrently...", len(diffs))

	for _, file := range diffs {
		wg.Add(1)
		go func(f *domain.FileDiff) {
			defer wg.Done()

			// 获取令牌
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// 调用 AI
			comments, err := s.ai.ReviewFile(ctx, f)
			if err != nil {
				log.Printf("⚠️ Error reviewing file %s: %v", f.FilePath, err)
				return
			}

			var validComments []*domain.ReviewComment
			if len(comments) > 0 {
				for _, c := range comments {
					// 校验行号
					if f.ValidLine[c.LineNumber] {
						validComments = append(validComments, c)
					} else {
						// 如果无效，打印日志，或者将其改为 "文件级评论" (行号设为0或特殊处理)
						log.Printf("⚠️ Skip invalid line comment on %s:%d (AI Hallucination)", c.FilePath, c.LineNumber)
						// 进阶做法：收集这些无效评论，最后作为 General Comment 发送
					}
				}
			}
			mu.Lock()
			allComments = append(allComments, validComments...)
			mu.Unlock()
		}(file)
	}

	wg.Wait()

	// 3. 提交回 GitHub
	if len(allComments) > 0 {
		log.Printf("📝 Posting %d comments to GitHub...", len(allComments))
		err := s.git.PostReview(ctx, cfg.RepoOwner, cfg.RepoName, cfg.PRNumber, allComments)
		if err != nil {
			log.Fatalf("❌ Failed to post review: %v", err)
		}
		log.Println("🎉 Review posted successfully!")
	} else {
		log.Println("🎉 Great job! No issues found by AI.")
	}
}
