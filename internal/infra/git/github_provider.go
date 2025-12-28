package git

import (
	"context"
	"fmt"
	"github.com/google/go-github/v60/github"
	"github.com/waigani/diffparser"
	"go-pr-review/internal/domain"
	"log"
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

// ReplyToComment commentID 是用户那条评论的 ID
func (g *GitHubProvider) ReplyToComment(ctx context.Context, owner, repo string, prNumber int, commentID int64, body string) error {
	// 不同的事件类型，回复方式略有不同。
	// 对于 Pull Request Review Comment (Inline)，使用 CreateComment
	// 对于 Issue Comment (General)，使用 CreateComment
	// GitHub API 中，PR 也是 Issue。

	comment := &github.IssueComment{
		Body: github.String(body),
	}
	// 注意：CreateComment 是创建新评论，不是回复。
	// 如果是 Issue Comment，直接创建新的 Issue Comment 即可。
	// 如果要构建 "Thread" (盖楼)，对于 Issue Comment 没法显式指定 Parent，只能引用。
	// 对于 Review Comment，可以指定 InReplyTo。

	// 这里为了简化，我们统一作为 Issue Comment 回复，并在内容里 @用户
	_, _, err := g.client.Issues.CreateComment(ctx, owner, repo, prNumber, comment)
	return err
}

// GetCommitDiff 获取 Commit 的 Diff
func (g *GitHubProvider) GetCommitDiff(ctx context.Context, owner, repo, sha string) ([]*domain.FileDiff, error) {
	// Github API GetCommit 也会返回Files列表 和Patch
	commit, _, err := g.client.Repositories.GetCommit(ctx, owner, repo, sha, nil)
	if err != nil {
		return nil, err
	}

	var diffs []*domain.FileDiff
	for _, file := range commit.Files {
		if file.Patch == nil {
			continue
		}
		// 构建 FileDiff
		diffs = append(diffs, &domain.FileDiff{
			FilePath: *file.Filename,
			Content:  *file.Patch,
			// 注意：这里没有 diffparser 的 Hunks，如果复用 Line 校验逻辑可能需要适配
			// 对于 Commit Comment，GitHub API 不需要复杂的行号校验，通常是对 Commit 整体或特定位置评论
		})
	}

	return diffs, nil
}

// PostCommitComment 对Commit发表评论
func (g *GitHubProvider) PostCommitComment(ctx context.Context, owner, repo, sha string, comments []*domain.ReviewComment) error {
	for _, c := range comments {
		// Github CreateCommitComment API
		comment := &github.RepositoryComment{
			Path:     github.String(c.FilePath),
			Position: github.Int(c.LineNumber), // 注意：Commit Comment 的 Position 计算方式极其复杂
			// 简单起见，V0.9 我们只发 General Comment 到 Commit 下面，不发 Inline。
			// 因为 Inline 需要精准的 position_in_diff
			Body: github.String(fmt.Sprintf("[%s:%d] %s", c.FilePath, c.LineNumber, c.Content)),
		}
		_, _, err := g.client.Repositories.CreateComment(ctx, owner, repo, sha, comment)
		if err != nil {
			log.Printf("Failed to post commit comment: %v", err)
		}
	}
	return nil
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
