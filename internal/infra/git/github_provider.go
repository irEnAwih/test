package git

import (
	"context"
	"fmt"
	"github.com/google/go-github/v60/github"
	"github.com/waigani/diffparser"
	"go-pr-review/internal/domain"
	"strings"
)

type GitHubProvider struct {
	client *github.Client
}

func (g *GitHubProvider) UpdatePRInfo(ctx context.Context, owner, repo string, prNumber int, title string, body string) error {
	// 构建Update 请求
	req := &github.PullRequest{
		Title: github.String(title),
		Body:  github.String(body),
	}

	_, _, err := g.client.PullRequests.Edit(ctx, owner, repo, prNumber, req)
	if err != nil {
		return fmt.Errorf("failed to update pull request info: %w", err)
	}
	return nil
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
		if shouldIgnore(file.NewName) {
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

		// 构建有效行
		validMap := BuildValidLineMap(file.Hunks)

		domainDiffs = append(domainDiffs, &domain.FileDiff{
			FilePath:  file.NewName,
			Content:   fullPath,
			ValidLine: validMap,
		})
	}
	return domainDiffs, nil
}

// PostReview 提交评论到 GitHub
func (g *GitHubProvider) PostReview(ctx context.Context, owner, repo string, prNumber int, comments []*domain.ReviewComment) error {
	if len(comments) == 0 {
		return nil
	}

	// 1.获取PR的最新CommitSHA()
	pr, _, err := g.client.PullRequests.Get(ctx, owner, repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get pull request: %w", err)
	}
	commitID := pr.Head.GetSHA()

	// 2.转换 domain.ReceiveComment -> github.DraftReviewComment
	var ghComments []*github.DraftReviewComment

	// 简单去重
	seen := make(map[string]bool)
	for _, c := range comments {
		key := fmt.Sprintf("%s:%d:%s", c.FilePath, c.LineNumber, c.Content)
		if seen[key] {
			continue
		}
		seen[key] = true

		msg := fmt.Sprintf("🤖 **AI Review**: %s", c.Content)
		// GitHub API 要求行号 (Line)
		// 注意：在旧版 API 中需要计算 Position，但新版 API 支持直接传 Line，前提是能对应上
		// 这里简化处理，直接传 Line。如果报错，通常是因为该行在 Diff 中不存在（AI 幻觉）
		ghComments = append(ghComments, &github.DraftReviewComment{
			Path: &c.FilePath,
			Line: &c.LineNumber,
			Side: github.String("RIGHT"), // 评论在代码变更的右侧（新代码）
			Body: &msg,
		})
	}

	// 3.提交Review
	review := &github.PullRequestReviewRequest{
		CommitID: &commitID,
		Body:     github.String("🤖 AI Code Review Summary\n\nI have reviewed your code. See inline comments for details."),
		Event:    github.String("COMMENT"), // 或者 "REQUEST_CHANGES", "APPROVE",
		Comments: ghComments,
	}
	_, _, err = g.client.PullRequests.CreateReview(ctx, owner, repo, prNumber, review)
	if err != nil {
		return fmt.Errorf("failed to create review: %w", err)
	}

	return nil
}

// GetFileContent 获取仓库内指定文件的内容
func (g *GitHubProvider) GetFileContent(ctx context.Context, owner, repo, path string, ref string) (string, error) {
	// ref 可以是 commit sha 或者 branch name,传空字符串默认 default branch
	opts := &github.RepositoryContentGetOptions{Ref: ref}

	fileContent, _, _, err := g.client.Repositories.GetContents(ctx, owner, repo, path, opts)
	if err != nil {
		return "", fmt.Errorf("failed to get file content: %w", err)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return "", err
	}

	return content, err
}

func shouldIgnore(filename string) bool {
	ignores := []string{
		"go.sum", "go.mod", "yarn.lock", "package-lock.json",
		".pb.go", "_test.go", // 可选：忽略测试文件
		".png", ".jpg", ".svg",
	}
	for _, ignore := range ignores {
		if strings.HasSuffix(filename, ignore) {
			return true
		}
	}
	return false
}
