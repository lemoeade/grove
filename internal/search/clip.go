package search

// 这个文件是"按字边界截一段文字"：搜索结果的预览（preview）、搜索编辑器里太长的行（app 的 clipLine）、
// 符号索引的说明都用它，免得各写一遍往回退到 UTF-8 字边界的循环。

import "unicode/utf8"

// RuneFloor 把字节下标 i 往前挪到字边界（某个 UTF-8 字的第一个字节）；i 已经在边界上（含 0 和 len(s)）就不动。
func RuneFloor[T ~string | ~[]byte](s T, i int) int {
	for i > 0 && i < len(s) && !utf8.RuneStart(s[i]) {
		i--
	}
	return i
}

// RuneCeil 把字节下标 i 往后挪到字边界，最多挪到 len(s)。
func RuneCeil[T ~string | ~[]byte](s T, i int) int {
	for i < len(s) && !utf8.RuneStart(s[i]) {
		i++
	}
	return i
}

// Clip 把 s 截到最多 n 字节（往回退到字边界），截了就在后面加"…"；不超过 n 字节原样返回。
func Clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:RuneFloor(s, n)] + "…"
}
