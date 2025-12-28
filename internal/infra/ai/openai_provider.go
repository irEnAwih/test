package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"go-pr-review/internal/domain"
	"strings"
)

type OpenAIProvider struct {
	client *openai.Client
}

func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api-inference.modelscope.cn/v1"
	return &OpenAIProvider{client: openai.NewClientWithConfig(config)}
}

func (o *OpenAIProvider) ReviewFile(ctx context.Context, diff *domain.FileDiff) ([]*domain.ReviewComment, error) {
	// 1. 构建 System Prompt：强制 JSON 格式
	systemPrompt := `You are a senior Golang code reviewer.
					Your task is to review the code diff provided.
					Output MUST be a raw JSON array of objects. Do not use Markdown code blocks.
					Format:
					[
					  {"file": "filename.go", "line": <line_number_in_new_file>, "content": "<comment>", "type": "ISSUE"}
					]
					If the code is good, return an empty array [].
					Focus on: Bugs, Security, and Performance. Ignore minor style issues.`

	// 2.构建 User Prompt
	userPrompt := fmt.Sprintf("File: %s\nDiff Content:\n%s", diff.FilePath, diff.Content)

	// 3.调用 AI
	resp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "deepseek-ai/DeepSeek-V3.2",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0.1,
	})

	if err != nil {
		return nil, err
	}

	content := resp.Choices[0].Message.Content

	// 4.清洗数据（防止AI有时候还是会加```json ... ```）
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	// 5.解析JSON
	var comments []*domain.ReviewComment
	if err := json.Unmarshal([]byte(content), &comments); err != nil {
		// 如果解析失败，可能是 AI 没发现问题返回了非 JSON 文本，或者是格式错误
		// 生产环境通常会重试，这里简化处理返回空
		fmt.Printf("⚠️ Failed to parse JSON for %s: %v. Response was: %s\n", diff.FilePath, err, content)
		return nil, nil // 忽略错误，避免中断流程
	}

	// 修正文件名 (防止 AI 幻觉搞错文件名)
	for _, c := range comments {
		c.FilePath = diff.FilePath
	}

	return comments, nil
}
