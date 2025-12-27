package domain

import "context"

// FileDiff 代表一个文件的变更详情
type FileDiff struct {
	FilePath  string `json:"filePath"`  // 文件路径
	OldPath   string `json:"oldPath"`   // 旧文件路径 (用于重命名检测)
	Content   string `json:"content"`   // 仅该文件的 Diff 片段 (Patch)
	Additions int    `json:"additions"` // 新增行数
	Deletions int    `json:"deletions"` // 删除行数
	// 后续 V0.3 会在这里增加 Hunks 结构，用于行级评论定位
}

// GitProvider 定义与代码托管平台交互的标准接口
type GitProvider interface {
	GetPRDiff(ctx context.Context, owner, repo string, prNumber int) ([]*FileDiff, error)
}

// AIProvider 定义与 LLM 交互的标准接口
type AIProvider interface {
	ReviewDiff(ctx context.Context, diffs []*FileDiff) (string, error)
}

// Config 聚合配置对象
type Config struct {
	GithubToken string
	OpenAIKey   string
	RepoOwner   string
	RepoName    string
	PRNumber    int
}
