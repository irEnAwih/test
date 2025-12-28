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

//go:embed describe.tmpl
var describePromptStr string

type OpenAIProvider struct {
	client       *openai.Client
	reviewTmpl   *template.Template
	describeTmpl *template.Template
}

func (o *OpenAIProvider) DescribePR(ctx context.Context, diffs []*domain.FileDiff, config domain.RepoConfig) (*domain.PRDescription, error) {
	// 1. 拼接所有 Diff (注意 Token 限制)
	// Describe 需要看全局，所以我们把所有文件的 diff 拼起来，但要控制总长度
	// 策略：每个文件取前 1000 字符，或者总共取前 20000 字符

	var sb strings.Builder
	totallen := 0
	const MaxTotalChars = 20_000
	for _, diff := range diffs {
		if totallen > MaxTotalChars {
			sb.WriteString("\n...(remaining files truncated))")
			break
		}
		// 简单格式：File:xxx \n Content
		fragment := fmt.Sprintf("File: %s\n%s\n\n", diff.FilePath, diff.Content)

		// 如果单个文件太大，也进行一次截断
		if len(fragment) > 4_000 {
			fragment = fragment[:4_000] + "\n...(truncated file)\n"
		}
		sb.WriteString(fragment)
		totallen += len(fragment)

	}
	// 2。渲染模板
	var promptBuf bytes.Buffer
	data := struct {
		DiffContent string
	}{DiffContent: sb.String()}

	if err := o.describeTmpl.Execute(&promptBuf, data); err != nil {
		return nil, err
	}

	// 3.调用AI
	resp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "deepseek-ai/DeepSeek-V3.2",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: "You are a code reviewer. Output JSON only."},
			{Role: openai.ChatMessageRoleUser, Content: promptBuf.String()},
		},
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("create completion error: %w", err)
	}

	// 4.解析JSON
	raw := cleanJSON(resp.Choices[0].Message.Content)
	var result domain.PRDescription
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func NewOpenAIProvider(apiKey string) *OpenAIProvider {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api-inference.modelscope.cn/v1"
	client := openai.NewClientWithConfig(config)

	// 解析 Review模板
	rTmpl, err := template.New("review").Parse(promptTemplateStr)
	if err != nil {
		panic(err)
	}

	// 解析 Describe模板
	dTmpl, err := template.New("describe").Parse(describePromptStr)
	if err != nil {
		panic(err)
	}
	return &OpenAIProvider{
		client:       client,
		reviewTmpl:   rTmpl,
		describeTmpl: dTmpl,
	}
}

// PromptData 用于渲染模板
type PromptData struct {
	Language          string // 代码语言 (go, python)
	OutputLanguage    string // [New] 输出语言 (zh-CN, en-US)
	ExtraInstructions string // [New] 用户自定义指令
	FileName          string
	DiffContent       string
}

func (o *OpenAIProvider) ReviewFile(ctx context.Context, diff *domain.FileDiff, config domain.RepoConfig) ([]*domain.ReviewComment, error) {
	const MaxDiffChar = 15000
	content := diff.Content
	if len(content) > MaxDiffChar {
		// 截断，进阶策略是按Hunk分割多次请求
		content = content[:MaxDiffChar] + "\n\n... (Truncated due to length limit)"
	}

	// 模板渲染
	var promptBuf bytes.Buffer
	data := PromptData{
		Language:          detectLanguage(diff.FilePath),
		OutputLanguage:    config.Language,          // 注入
		ExtraInstructions: config.ExtraInstructions, // 注入
		FileName:          diff.FilePath,
		DiffContent:       content,
	}
	if err := o.reviewTmpl.Execute(&promptBuf, data); err != nil {
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
