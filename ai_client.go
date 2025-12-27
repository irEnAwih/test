package main

import (
	"context"
	"fmt"
	"github.com/sashabaranov/go-openai"
)

type AIReviewer struct {
	client *openai.Client
}

func NewAIReviewer(apiKey string) *AIReviewer {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api.openai.com/v1"
	client := openai.NewClientWithConfig(config)
	return &AIReviewer{client: client}
}

// ReviewCode 将 Diff 发送给 AI 并获取建议
func (a *AIReviewer) ReviewCode(ctx context.Context, diff string) (string, error) {
	// 定义 System Prompt (你可以参考 pr-agent 的 prompt 进行优化)
	systemPrompt := `You are an expert Golang code reviewer. 
					Your task is to review the provided pull request diff.
					Focus on:
					1. Potential bugs and concurrency issues.
					2. Code style improvements (idiomatic Go).
					3. Security vulnerabilities.
					
					Output the review in a concise markdown format.`

	// 构建 user prompt
	// 注意：实际生产中需要检查 diff 长度，如果超过 Token 限制需要进行截断或分块处理
	userPrompt := fmt.Sprintf("Here is the PR diff:\n\n%s", diff)

	resp, err := a.client.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: "deepseek-ai/DeepSeek-V3.2",
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: userPrompt},
			},
		},
	)

	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
}
