package app

import (
	"os"
	"path/filepath"
	"testing"

	ui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/git"
	"github.com/lemoeade/grove/internal/testutil"
)

// 比工作区那一边的 Diff 能就地编辑：改的是同一个文件标签的缓冲区，⌘S 在 Diff 里存那个文件；
// 文件标签关了退回只看；暂存了（不再是比工作区的）也退回只看；已暂存的 Diff 不能改。
func TestDiffEditInPlace(t *testing.T) {
	m := testutil.NewRepo(t, "proj")
	m.Write("a.txt", "一\n二\n三\n")
	m.Commit("初始")
	m.Write("a.txt", "一\n二改\n三\n")
	a, wait := newTestApp(t, wsFile(t, "diff", m.Root))
	r := a.Repos[0]
	a.RefreshRepo(r, WhyOpen)
	wait("读状态", idle(r))
	ch, ok := r.ChangeOf("a.txt")
	if !ok || ch.Kind != git.Unstaged {
		t.Fatalf("改动：%+v %v", ch, ok)
	}
	d := a.OpenDiff(r, ch, false)
	wait("读 diff", func() bool { return !d.Loading && d.DiffSt != nil })
	edit := Find("diff.editInPlace")
	if !a.CanEditDiff(d) || DiffEditing(d) || !a.CanRun(edit) || edit.Checked(a) {
		t.Fatalf("未暂存的 diff 应能就地编辑：can=%v editing=%v", a.CanEditDiff(d), DiffEditing(d))
	}
	a.Exec("diff.editInPlace")
	if a.CurrentDoc() != d || d.EditDoc == nil || d.EditDoc.Kind != docs.File || !d.Pinned || !edit.Checked(a) {
		t.Fatalf("编辑：焦点应留在 diff 上、diff 固定下来，后面开一个文件标签：cur=%v pinned=%v edit=%+v", a.CurrentDoc() == d, d.Pinned, d.EditDoc)
	}
	f := d.EditDoc
	wait("读文件", func() bool { return f.Buf != nil && !f.Loading })
	a.BindDiffEdit(d)
	if !d.DiffSt.Editable || d.DiffSt.New != f.Buf {
		t.Fatal("新的一边应是文件标签的缓冲区")
	}
	if DiffDirty(d) {
		t.Fatal("还没改，不该算有没保存的改动")
	}
	f.Buf.Edit(editor.Edit{Text: "零\n"})
	if !f.Modified() || !DiffDirty(d) || !a.CanRun(Find("file.save")) {
		t.Fatal("改了应能存")
	}
	a.Exec("file.save") // 当前标签是 diff：存的是文件标签
	wait("保存", func() bool { return !f.Modified() })
	b, _ := os.ReadFile(filepath.Join(m.Root, "a.txt"))
	if string(b) != "零\n一\n二改\n三\n" {
		t.Fatalf("写进去的：%q", b)
	}

	// 关掉编辑：文件标签留着，diff 退回只看
	a.Exec("diff.editInPlace")
	a.BindDiffEdit(d)
	if DiffEditing(d) || d.DiffSt.Editable {
		t.Fatal("退出编辑后应只看")
	}
	if d.DiffSt.New == f.Buf {
		t.Fatal("退出编辑后新的一边不该还是文件标签的缓冲区")
	}
	f.Buf.Edit(editor.Edit{Text: "没存的\n"}) // 文件标签里没存的字不该出现在只看的 diff 里
	wait("重读 diff", func() bool { return !d.Loading && d.DiffSt != nil })
	if got := d.DiffSt.New.String(); got != "零\n一\n二改\n三\n" {
		t.Fatalf("只看的新一边该是磁盘上的：%q", got)
	}

	// 再开，然后把文件标签关了：退回只看
	a.Exec("diff.editInPlace")
	a.BindDiffEdit(d)
	if !d.DiffSt.Editable {
		t.Fatal("再开应能编辑")
	}
	a.RemoveDoc(f)
	a.BindDiffEdit(d)
	if d.EditDoc != nil || d.DiffSt != nil && d.DiffSt.Editable {
		t.Fatal("文件标签关了应退回只看")
	}

	// 已暂存的不能改
	m.Git("add", "a.txt")
	a.RefreshRepo(r, WhyOpen)
	wait("读状态", idle(r))
	st, _ := r.ChangeOf("a.txt")
	ds := a.OpenDiff(r, st, true)
	wait("读 diff", func() bool { return !ds.Loading && ds.DiffSt != nil })
	if st.Kind != git.Staged || a.CanEditDiff(ds) || a.CanRun(edit) {
		t.Fatalf("已暂存的 diff 不该能改：%+v can=%v", st, a.CanEditDiff(ds))
	}
}

// 就地编辑着的 Diff 不拿不带修饰键的跳改动键（默认的 J / K）：那是要打的字。别的绑定原样留着。
func TestDiffEditBindings(t *testing.T) {
	plain := func(bs []editor.Binding) (n int) {
		for _, b := range bs {
			if (b.Command == editor.NextHunk || b.Command == editor.PreviousHunk) && b.Mods&(ui.Cmd|ui.Ctrl|ui.Alt) == 0 {
				n++
			}
		}
		return n
	}
	all, ed := DiffBindings(), DiffEditBindings()
	if plain(all) != 2 {
		t.Fatalf("Diff 平时该有 J / K 两条跳改动：%d", plain(all))
	}
	if plain(ed) != 0 || len(ed) != len(all)-2 {
		t.Fatalf("就地编辑时该去掉这两条、别的留着：%d 条里还有 %d 条，平时 %d 条", len(ed), plain(ed), len(all))
	}
}
