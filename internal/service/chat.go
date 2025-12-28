package service

import (
	"context"
	"fmt"
	"go-pr-review/internal/domain"
	"log"
	"strings"
)

type ChatService struct {
	git domain.GitProvider
	ai  domain.AIProvider
}

func NewChatService(git domain.GitProvider, ai domain.AIProvider) *ChatService {
	return &ChatService{git: git, ai: ai}
}

func (s *ChatService) HandleComment(ctx context.Context, owner, repo string, prNumber int, commentBody string, sender string) {
	// 1.检查是否触发命令
	if !strings.HasPrefix(strings.TrimSpace(commentBody), "/ask") {
		return
	}

	question := strings.TrimPrefix(commentBody, "/ask")
	if len(strings.TrimSpace(question)) == 0 {
		return
	}

	log.Printf("💬 Processing /ask from %s on PR #%d", sender, prNumber)

	// 2. 获取上下文 (Diff)
	// 注意：这里简单地获取所有 Diff。对于大 PR 可能会超长。
	// 优化点：应该缓存 Diff 摘要，或者只获取相关文件的 Diff。
	diffs, err := s.git.GetPRDiff(ctx, owner, repo, prNumber)
	if err != nil {
		log.Printf("❌ Failed to get diff: %v", err)
		return
	}

	// 简单的Context组装
	var sb strings.Builder
	for _, d := range diffs {
		if len(sb.String()) > 10_000 {
			// 截断
			break
		}
		sb.WriteString(fmt.Sprintf("File: %s\n%s\n", d.FilePath, d.Content))
	}

	// 3. 提问AI
	answer, err := s.ai.AskQuestion(ctx, sb.String(), question)
	if err != nil {
		log.Printf("❌ AI Error: %v", err)
		answer = "Sorry, I encountered an error while processing your request."
	}

	// 4. 回复用户
	replyMsg := fmt.Sprintf("@%s %s", sender, answer)
	if err := s.git.ReplyToComment(ctx, owner, repo, prNumber, 0, replyMsg); err != nil {
		log.Printf("❌ Failed to reply: %v", err)
	}
}
