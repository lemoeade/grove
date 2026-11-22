package ged

import (
	"testing"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// 缩进参考线：每行画到第几格；空行接着上下的（块里的空行不断，块后面的不多画）；光标那一块的线。
func TestGuides(t *testing.T) {
	if indentCols("\t\tx", 4) != 8 || indentCols("  \tx", 4) != 4 || indentCols("   ", 4) != -1 || indentCols("", 4) != -1 {
		t.Fatal("indentCols 算错了")
	}
	d := NewDiff()
	lines := []string{
		"func f() {", // 0
		"    if x {", // 1
		"        a()",
		"",            // 3：块里的空行
		"        b()", // 4
		"    }",
		"",  // 6：两层之间
		"}", // 7
	}
	d.lines = [2][]string{lines, lines}
	d.guide = guides{on: true, unit: 4, tab: 4}
	want := []int{0, 4, 8, 8, 8, 4, 4, 0}
	for i, w := range want {
		if got := d.guideCols(1, i); got != w {
			t.Errorf("第 %d 行画到第 %d 格，应是 %d", i, got, w)
		}
	}
	// 就地编辑时光标在 a() 那一行：那一块是第 2–4 行、第 4 格那道线
	d.editable, d.focused = true, true
	d.sel = selection{side: 1, a: pos{2, 0}, b: pos{2, 0}}
	s := &Style{Guides: true, Indent: "4", Code: editor.DefaultTheme(wui.LightTheme())}
	d.prepGuides(s)
	if !d.guide.act || d.guide.actCol != 4 || d.guide.actLines != [2]int{2, 4} {
		t.Errorf("光标那一块：%+v", d.guide)
	}
	s.Guides = false
	if d.prepGuides(s); d.guide.on {
		t.Error("Guides 关着也画了")
	}
}
