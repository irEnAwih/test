package git

import (
	"context"
	"fmt"
	"github.com/google/go-github/v60/github"
	"github.com/waigani/diffparser"
	"go-pr-review/internal/domain"
)

type GitHubProvider struct {
	client *github.Client
}

func NewGitHubProvider(token string) *GitHubProvider {
	client := github.NewClient(nil).WithAuthToken(token)
	return &GitHubProvider{client: client}
}

// GetPRDiff 获取 PR 的 diff并解析为结构化数据
func (g *GitHubProvider) GetPRDiff(ctx context.Context, owner, repo string, prNumber int) ([]*domain.FileDiff, error) {
	// 1.获取Raw String Diff
	opts := github.RawOptions{Type: github.Diff}
	rawDiff, _, err := g.client.PullRequests.GetRaw(ctx, owner, repo, prNumber, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pull request diff: %w", err)
	}

	// 2.解析Diff(Core Logic)
	// waigani/diffparser 接收 []byte 并返回解析后的结构
	parsedDiff, err := diffparser.Parse(rawDiff)
	if err != nil {
		return nil, fmt.Errorf("failed to parse diff: %w", err)
	}

	// 3.转换为领域模型（Domain Model)
	var domainDiffs []*domain.FileDiff
	for _, file := range parsedDiff.Files {
		// 可以在这里过滤文件，例如忽略 go.sum, .lock 等
		if file.NewName == "go.sum" || file.NewName == "go.mod" {
			continue
		}

		// 重组 Patch 内容（如果不重组，AI 只能看到分开的 Hunk，可能丢失上下文）
		// 这里的逻辑是将该文件的所有变更块拼成一个字符串供 AI 阅读
		// 实际生产中可能需要更复杂的 Hunk 处理

		var fullPath string
		for _, hunk := range file.Hunks {
			fullPath += fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", hunk.OrigRange.Start, hunk.OrigRange.Length, hunk.NewRange.Start, hunk.NewRange.Length)
			for _, line := range hunk.WholeRange.Lines {
				fullPath += line.Content + "\n"
			}
		}

		domainDiffs = append(domainDiffs, &domain.FileDiff{
			FilePath: file.NewName,
			OldPath:  file.OrigName,
			Content:  fullPath,
			// diffparser 目前计算 Additions/Deletions 比较简略，这里仅作演示
			Additions: 0,
			Deletions: 0,
		})
	}
	return domainDiffs, nil
}
