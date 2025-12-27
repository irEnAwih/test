package ai

import (
	"context"
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

func (o *OpenAIProvider) ReviewDiff(ctx context.Context, diffs []*domain.FileDiff) (string, error) {
	//组装用户提示词
	var sb strings.Builder
	sb.WriteString("Please review the following code changes:\\n\\n")

	for _, file := range diffs {
		sb.WriteString(fmt.Sprintf("--- File: %s ---\n", file.FilePath))
		sb.WriteString(file.Content)
		sb.WriteString("\n\n")
	}

	// 简单的截断策略，防止 Demo崩溃
	userPrompt := sb.String()
	if len(userPrompt) > 20000 {
		userPrompt = userPrompt[:20000] + "\n... (truncated)"
	}

	systemPrompt := `You are a strict code reviewer. 
					Analyze the provided code patches. 
					Group your comments by file name. 
					If the code is good, say "LGTM".`
	resp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "deepseek-ai/DeepSeek-V3.2",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0.1,
	})

	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
}
