package docs

import "testing"

// 每种标签都归了类：要么只在内存里，要么关了能再开，要么是编辑器区的终端（记进会话、关了就没了）；
// 新加一种 Kind 时这里先报，提醒去 InMemory / Reopenable / Titled 里把它归好。
func TestKindClasses(t *testing.T) {
	for k := Kind(0); k < numKinds; k++ {
		n := 0
		for _, in := range []bool{k.InMemory(), k.Reopenable(), k == Term} {
			if in {
				n++
			}
		}
		if n != 1 {
			t.Errorf("Kind %d 归了 %d 类，该正好 1 类", k, n)
		}
	}
	if !Term.Titled() || File.Titled() || ReplacePreview.Titled() {
		t.Error("Titled 分错了")
	}
}
