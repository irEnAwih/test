package ai

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"go-pr-review/internal/domain"
	"strings"
	"text/template"
)

//go:embed review.tmpl
var promptTemplateStr string

type OpenAIProvider struct {
	client *openai.Client
	tmpl   *template.Template
}

func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api-inference.modelscope.cn/v1"

	// 解析模板
	tmpl, err := template.New("review").Parse(promptTemplateStr)
	if err != nil {
		panic(fmt.Sprintf("failed to parse prompt template: %v", err))
	}
	client := openai.NewClientWithConfig(config)

	return &OpenAIProvider{
		client: client,
		tmpl:   tmpl,
	}
}

// PromptData 用于渲染模板
type PromptData struct {
	Language    string
	FileName    string
	DiffContent string
}

func (o *OpenAIProvider) ReviewFile(ctx context.Context, diff *domain.FileDiff) ([]*domain.ReviewComment, error) {
	const MaxDiffChar = 15000
	content := diff.Content
	if len(content) > MaxDiffChar {
		// 截断，进阶策略是按Hunk分割多次请求
		content = content[:MaxDiffChar] + "\n\n... (Truncated due to length limit)"
	}

	// 模板渲染
	var promptBuf bytes.Buffer
	data := PromptData{
		Language:    detectLanguage(diff.FilePath),
		FileName:    diff.FilePath,
		DiffContent: content,
	}
	if err := o.tmpl.Execute(&promptBuf, data); err != nil {
		return nil, fmt.Errorf("template execute error: %w", err)
	}

	// 3. 调用 AI (System Prompt 可以简化，因为 User Prompt 里已经包含了规则)
	// 但为了效果更好，我们保留一个简短的 System Prompt

	finalPrompt := promptBuf.String()
	if strings.TrimSpace(finalPrompt) == "" {
		return nil, fmt.Errorf("rendered prompt is empty, check your template string")
	}
	resp, err := o.client.CreateChatCompletion(ctx,
		openai.ChatCompletionRequest{
			Model: "deepseek-ai/DeepSeek-V3.2",
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: "You are a code reviewer. Output JSON only."},
				{Role: openai.ChatMessageRoleUser, Content: finalPrompt},
			},
			Temperature: 0.2,
		})

	if err != nil {
		return nil, fmt.Errorf("create completion error: %w", err)
	}

	// 解析json
	raw := resp.Choices[0].Message.Content
	raw = cleanJSON(raw)

	var comments []*domain.ReviewComment

	if err := json.Unmarshal([]byte(raw), &comments); err != nil {
		// 容错：如果解析失败，不仅返回空，还要打印日志方便调试
		fmt.Printf("⚠️ JSON Parse Error for %s. Raw: %s\n", diff.FilePath, raw)
		return nil, nil
	}

	for _, c := range comments {
		c.FilePath = diff.FilePath
	}
	return comments, nil
}

func detectLanguage(filename string) string {
	if strings.HasSuffix(filename, ".go") {
		return "Golang"
	}
	if strings.HasSuffix(filename, ".py") {
		return "Python"
	}
	if strings.HasSuffix(filename, ".js") || strings.HasSuffix(filename, ".ts") {
		return "TypeScript/JavaScript"
	}
	return "Code"
}

func cleanJSON(s string) string {
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
