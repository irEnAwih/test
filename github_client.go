package main

import (
	"context"
	"fmt"
	"github.com/google/go-github/v60/github"
)

type GithubProvider struct {
	client *github.Client
}

func NewGithubProvider(token string) *GithubProvider {
	client := github.NewClient(nil).WithAuthToken(token)
	return &GithubProvider{
		client: client,
	}
}

// GetPRDiff 获取指定 PR 的 diff原始内容
func (g *GithubProvider) GetPRDiff(ctx context.Context, owner, repo string, prNumber int) (string, error) {
	// 关键点：使用 GetRaw 方法，并指定类型为 Diff
	// 这会告诉 GitHub API 我们要的是 git diff 文本，而不是 JSON 元数据
	opts := github.RawOptions{Type: github.Diff}

	diffContent, _, err := g.client.PullRequests.GetRaw(ctx, owner, repo, prNumber, opts)
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %v", err)
	}

	return diffContent, nil
}
