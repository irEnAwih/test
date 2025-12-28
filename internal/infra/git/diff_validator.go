package git

import "github.com/waigani/diffparser"

// ValidLineMap 用于快速查找某一行是否存在Diff中有效

type ValidLineMap map[int]bool

// BuildValidLineMap 根据 diffparser 的Hunks 构建有效行号集合
func BuildValidLineMap(hunks []*diffparser.DiffHunk) ValidLineMap {
	validLines := make(ValidLineMap)
	for _, hunk := range hunks {
		// Hunk 包含 OrigRange (旧文件) 和 NewRange (新文件)
		// 我们评论是针对“新文件” (Right Side) 的，所以只关心 NewRange

		// GitHub API 允许在 Hunk 的范围内发表评论（包括上下文行）
		startLine := hunk.NewRange.Start
		length := hunk.NewRange.Length

		for i := 0; i < length; i++ {
			// 将范围内的每一行都标记为有效
			validLines[startLine+1] = true
		}
	}
	return validLines
}

// IsLineValid 检查行号是否存在
func (m ValidLineMap) IsLineValid(line int) bool {
	return m[line]
}
