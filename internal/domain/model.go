package domain

import "context"

// FileDiff 代表一个文件的变更详情
type FileDiff struct {
	FilePath  string       `json:"filePath"`   // 文件路径
	Content   string       `json:"content"`    // 仅该文件的 Diff 片段 (Patch)
	ValidLine map[int]bool `json:"valid_line"` //  使用函数闭包或 Map 来验证行号，避免暴露 diffparser 的具体类型
}

// ReviewComment 代表一条AI 生成的评审意见
type ReviewComment struct {
	FilePath   string `json:"filePath"`   // 对应文件
	LineNumber int    `json:"lineNumber"` // 对应文件中的行号
	Content    string `json:"content"`    // 评审意见
	Type       string `json:"type"`       // 比如“ISSUE”，“SUGGESTION“
}

// PRDescription PR 描述结果
type PRDescription struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Changes string `json:"changes"`
}

// GitProvider 定义与代码托管平台交互的标准接口
type GitProvider interface {
	GetPRDiff(ctx context.Context, owner, repo string, prNumber int) ([]*FileDiff, error)
	PostReview(ctx context.Context, owner, repo string, prNumber int, comments []*ReviewComment) error
	UpdatePRInfo(ctx context.Context, owner, repo string, prNumber int, title string, body string) error
	GetFileContent(ctx context.Context, owner, repo, path string, ref string) (string, error)
	ReplyToComment(ctx context.Context, owner, repo string, prNumber int, commentID int64, body string) error
	GetCommitDiff(ctx context.Context, owner, repo, sha string) ([]*FileDiff, error)
	PostCommitComment(ctx context.Context, owner, repo, sha string, comments []*ReviewComment) error
}

// AIProvider 定义与 LLM 交互的标准接口
type AIProvider interface {
	ReviewFile(ctx context.Context, diff *FileDiff, config RepoConfig) ([]*ReviewComment, error)
	DescribePR(ctx context.Context, diffs []*FileDiff, config RepoConfig) (*PRDescription, error)
	AskQuestion(ctx context.Context, diffSummary string, question string) (string, error)
}

// RepoConfig 代表仓库级别的自定义配置 (.ai-review.yaml)
type RepoConfig struct {
	Language          string   `mapstructure:"language" json:"language"`                     // 输出语言: "zh-CN", "en-US"
	ExtraInstructions string   `mapstructure:"extra_instructions" json:"extra_instructions"` // 额外的 Prompt 指令
	IgnorePatterns    []string `mapstructure:"ignore_patterns" json:"ignore_patterns"`       // 忽略的文件 glob 模式
}

// Config 聚合配置对象
type Config struct {
	GithubToken   string
	OpenAIKey     string
	RepoOwner     string
	RepoName      string
	PRNumber      int
	RepoConfig    RepoConfig // 动态加载的仓库配置
	WebHookSecret string
	Port          string
}
