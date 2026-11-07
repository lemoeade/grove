package ui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"
	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/testutil"
)

// 预览标签被顶掉再点回来，接着看原来的地方；搜索结果那样指明了行的照跳。
func TestPreviewResumesView(t *testing.T) {
	r := testutil.NewRepo(t, "demo")
	r.Write("long.txt", strings.Repeat("一行\n", 2000))
	r.Write("b.txt", "b\n")
	r.Commit("init")
	a, tt, wait := newUI(t, testutil.Workspace(t, r.Root), t.TempDir())
	repo := a.Repos[0]
	a.RefreshRepo(repo, app.WhyManual)
	wait("读状态", func() bool { return repo.Loaded && !repo.Loading })
	open := func(rel string) *docs.Doc {
		t.Helper()
		d := a.OpenFile(repo, rel, false)
		wait("读好 "+rel, func() bool { return !d.Loading })
		return d
	}
	away := func() {
		t.Helper()
		open("b.txt")
		tt.Frame()
		if len(a.Docs) != 1 {
			t.Fatalf("预览没有顶掉上一个：%d 个标签", len(a.Docs))
		}
	}

	// 只滚过、光标还在开头：第一帧就在原来的地方，不露出文件开头
	d := open("long.txt")
	tt.Frame()
	d.St.SetScrollY(9000)
	tt.Frame()
	top, _ := d.St.VisibleLines()
	if top == 0 {
		t.Fatal("没滚动")
	}
	away()
	d = open("long.txt")
	if got, _ := d.St.VisibleLines(); got != top || d.St.CursorPos() != (editor.Pos{}) {
		t.Fatalf("第一帧：最上面第 %d 行（原来 %d），光标 %v", got, top, d.St.CursorPos())
	}

	// 光标也回去
	d.St.GoTo(editor.Pos{Line: 700, Col: 1})
	tt.Frame()
	top, _ = d.St.VisibleLines()
	away()
	d = open("long.txt")
	tt.Frame()
	tt.Frame()
	if got, _ := d.St.VisibleLines(); got != top || d.St.CursorPos() != (editor.Pos{Line: 700, Col: 1}) {
		t.Fatalf("最上面第 %d 行（原来 %d），光标 %v", got, top, d.St.CursorPos())
	}

	// 指明了行的不回原来的地方
	away()
	d = a.OpenAt(repo, "long.txt", docs.Jump{Line: 1500}, false)
	wait("读好", func() bool { return !d.Loading })
	tt.Frame()
	tt.Frame()
	if first, last := d.St.VisibleLines(); d.St.CursorPos().Line != 1500 || first > 1500 || last < 1500 {
		t.Fatalf("光标 %v，看到第 %d–%d 行", d.St.CursorPos(), first, last)
	}
}
