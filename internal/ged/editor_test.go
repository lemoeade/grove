package ged

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
)

var goSrc = strings.Split(`package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Println(os.Args[1])
	}
	fmt.Println("hi") // {
}

def:
    a
    b

tail`, "\n")

func plainEd(t *testing.T, lines []string) *Diff {
	t.Helper()
	d := NewEditor()
	d.SetText(1, func() []string { return lines }, editor.LanguageFor("main.go"))
	return d
}

// 单栏：每行一个 Same，Old == New，没有改动。
func TestEditorRows(t *testing.T) {
	d := plainEd(t, goSrc)
	if len(d.Rows()) != len(goSrc) || len(d.Hunks()) != 0 {
		t.Fatalf("行 %d（应 %d）、改动 %d", len(d.Rows()), len(goSrc), len(d.Hunks()))
	}
	for i, r := range d.Rows() {
		if r.Kind != Same || r.Old != i || r.New != i {
			t.Fatalf("第 %d 行：%+v", i, r)
		}
	}
}

// 折到哪：行尾开括号折到闭括号前一行；按缩进的折到缩进深的最后一行；不能折的不折。
func TestEditorFoldEnd(t *testing.T) {
	d := plainEd(t, goSrc)
	for _, c := range []struct {
		line, end int
		ok        bool
	}{
		{2, 4, true},   // import ( → ) 前
		{7, 11, true},  // func main() { → } 前
		{8, 9, true},   // if … { 只有一行在里面
		{11, 0, false}, // 行尾注释里的 { 不算
		{14, 16, true}, // def: 按缩进
		{0, 0, false},
		{1, 0, false}, // 空行
	} {
		end, ok := d.foldEnd(c.line)
		if ok != c.ok || ok && end != c.end {
			t.Errorf("第 %d 行：折到 %d（%v），应为 %d（%v）", c.line, end, ok, c.end, c.ok)
		}
		if f := d.foldable(c.line); f != c.ok {
			t.Errorf("第 %d 行 foldable = %v", c.line, f)
		}
	}
}

// 折起、展开、光标挪出藏起来的行、上下挪跳过折着的行、改了上面的行折叠跟着挪。
func TestEditorFolding(t *testing.T) {
	d := plainEd(t, goSrc)
	d.PlaceCaret(9, 3)
	if !d.Fold(7) || !d.IsFolded(7) {
		t.Fatal("折不起 func main")
	}
	if len(d.Rows()) != len(goSrc)-4 {
		t.Fatalf("折了 4 行，还剩 %d 行", len(d.Rows()))
	}
	if d.sel.b != (pos{7, len(goSrc[7])}) {
		t.Errorf("光标该挪到折起来那行的行尾：%v", d.sel.b)
	}
	if got := d.stepLine(7, 1); got != 12 {
		t.Errorf("从折着的行往下一行该到 12（闭括号）：%d", got)
	}
	if got := d.stepLine(12, -1); got != 7 {
		t.Errorf("从 12 往上一行该到 7：%d", got)
	}
	if d.plainRow(9) != -1 || d.plainRow(12) != 8 {
		t.Errorf("plainRow：%d %d", d.plainRow(9), d.plainRow(12))
	}
	// 上面加两行：折叠跟着挪到 9
	more := append([]string{"// a", "// b"}, goSrc...)
	d.SetText(2, func() []string { return more }, d.lang)
	if !slices.Equal(d.Folded(), []int{9}) || d.folded[9] != 13 {
		t.Fatalf("加了两行后折叠：%v %v", d.Folded(), d.folded)
	}
	// 光标去藏着的行：展开
	if d.rowOfLine(1, 11) < 0 || d.IsFolded(9) {
		t.Errorf("挪到藏着的行该展开")
	}
	d.FoldAll() // 里面的 if 也折（展开外层时它还折着）
	if !slices.Equal(d.Folded(), []int{4, 9, 10, 16}) {
		t.Errorf("全部折起：%v", d.Folded())
	}
	if len(d.Rows()) != len(more)-2-4-2 { // import 里两行、func main 里四行、def 下两行
		t.Errorf("全部折起后 %d 行", len(d.Rows()))
	}
	d.foldLevel(2) // 第二层：只有 if
	if !slices.Equal(d.Folded(), []int{10}) {
		t.Errorf("折叠级别 2：%v", d.Folded())
	}
	d.UnfoldAll()
	d.PlaceCaret(10, 0)
	if !d.FoldRecursive() || !slices.Equal(d.Folded(), []int{10}) {
		t.Errorf("光标在 if 上递归折叠：%v", d.Folded())
	}
	d.PlaceCaret(12, 0)
	d.UnfoldAll()
	d.PlaceCaret(12, 0) // func main 里、if 外
	if !d.FoldRecursive() || !slices.Equal(d.Folded(), []int{9, 10}) {
		t.Errorf("递归折叠 func main：%v", d.Folded())
	}
	d.PlaceCaret(9, 0)
	if !d.UnfoldRecursive() || len(d.Folded()) != 0 {
		t.Errorf("递归展开：%v", d.Folded())
	}
	d.UnfoldAll()
	if len(d.Folded()) != 0 || len(d.Rows()) != len(more) {
		t.Errorf("全部展开：%v", d.Folded())
	}
	// 头一行变得不能折了：丢掉
	d.Fold(16)
	gone := slices.Clone(more)
	gone[16] = "x"
	d.SetText(3, func() []string { return gone }, d.lang)
	if d.IsFolded(16) {
		t.Errorf("不能折了的还折着")
	}
}
