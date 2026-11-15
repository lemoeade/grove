package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/testutil"
)

// identAt：词中间、紧挨着词尾都算；`pkg.Name` 带限定词；数字、空白不算。
func TestIdentAt(t *testing.T) {
	b := editor.NewBuffer("x := util.Parse(42) + café_1\n")
	for _, c := range []struct {
		off        int
		word, qual string
	}{
		{0, "x", ""},
		{1, "x", ""}, // 词尾
		{12, "Parse", "util"},
		{5, "util", ""},
		{17, "", ""}, // 42
		{3, "", ""},  // ":="
		{24, "café_1", ""},
	} {
		w, _, q := identAt(b, c.off)
		if w != c.word || q != c.qual {
			t.Errorf("identAt(%d) = %q %q，应为 %q %q", c.off, w, q, c.word, c.qual)
		}
	}
}

// 跳到定义（三个仓库）：同文件的直接跳；`util.Parse` 按限定词挑 util 目录那个（别的仓库也有 Parse）；
// Shared 在 beta、gamma 都有、离得一样远，开面板只列两处同名的；本身就是定义、哪里都没有的提示一下；
// 索引建着时先等，建完接着跳；‹ 回到点的地方。
func TestGoToDefinition(t *testing.T) {
	alpha, beta, gamma, ws := testutil.ThreeRepos(t)
	alpha.Write("main.go", "package main\n\nfunc main() {\n\thelper()\n\tutil.Parse()\n\tShared()\n\tnowhere()\n}\n\nfunc helper() {}\n")
	alpha.Write("util/parse.go", "package util\n\nfunc Parse() {}\n")
	alpha.Commit("go")
	beta.Write("other/parse.go", "package other\n\nfunc Parse() {}\n")
	beta.Write("shared.go", "package beta\n\nfunc Shared() {}\n")
	beta.Commit("go")
	gamma.Write("shared.go", "package gamma\n\nfunc Shared() {}\n")
	gamma.Commit("go")
	a, wait := newTestApp(t, ws)
	t.Cleanup(a.Close)
	d := a.OpenFile(a.Repos[0], "main.go", true)
	wait("读好", func() bool { return d.Buf != nil && !d.Loading })
	if !CanGoToDefinition(d) {
		t.Fatal("Go 文件应能跳到定义")
	}
	at := func(line, col int) int {
		off := d.Buf.Offset(editor.Pos{Line: line, Col: col})
		d.St.Select(off, off)
		return off
	}
	back := func() {
		if a.OpenFile(a.Repos[0], "main.go", true) != d {
			t.Fatal("回 main.go 开了新标签")
		}
	}
	idle := func() {
		wait("索引", func() bool { return a.syms.files != nil && !a.defPending() && a.defWait == nil })
	}

	// 第一次：文件列表都还没读、索引还没建，等好了接着跳（helper 在这个文件里）
	a.GoToDefinition(d, at(3, 3))
	idle()
	if a.CurrentDoc() != d || d.St.CursorPos().Line != 9 {
		t.Fatalf("helper 应跳到第 10 行，在 %s:%d（%s）", a.CurrentDoc().Rel, d.St.CursorPos().Line, a.Toast)
	}
	a.Back()
	if d.St.CursorPos().Line != 3 {
		t.Errorf("‹ 应回到第 4 行，在 %d", d.St.CursorPos().Line)
	}

	// util.Parse：别的仓库也有 Parse，按限定词挑 alpha/util
	a.GoToDefinition(d, at(4, 8))
	idle()
	if c := a.CurrentDoc(); c.Root != alpha.Root || c.Rel != "util/parse.go" {
		t.Fatalf("util.Parse 跳到 %s/%s", c.Root, c.Rel)
	}
	wait("跳到行", func() bool { c := a.CurrentDoc(); return c.Buf != nil && c.St.CursorPos().Line == 2 })
	back()

	// Shared：beta、gamma 各一处，开面板列同名的两处
	a.GoToDefinition(d, at(5, 2))
	idle()
	if !a.Palette.Open || a.Palette.Text != "#Shared" {
		t.Fatalf("Shared 有两处应开面板：%v %q", a.Palette.Open, a.Palette.Text)
	}
	a.UpdatePalette()
	if got := symLocs(a.Palette.Items); !slices.Equal(got, []string{"Shared beta/shared.go:3", "Shared gamma/shared.go:3"}) {
		t.Fatalf("面板：%q（%s）", got, a.Palette.Note)
	}
	a.AcceptPalette(1)
	if c := a.CurrentDoc(); c.Root != gamma.Root || c.Rel != "shared.go" {
		t.Fatalf("选第二个打开了 %s/%s", c.Root, c.Rel)
	}
	back()
	// 改了输入就是普通的按名字找
	a.OpenPalette("#Shared")
	a.UpdatePalette()
	if a.Palette.defs != "" {
		t.Error("自己开的面板不该带着跳到定义的列表")
	}
	a.ClosePalette()

	// 本身就是定义、哪里都没有
	a.GoToDefinition(d, at(9, 7))
	if a.Toast != i18n.Tf("这里就是 %s 的定义", "helper") {
		t.Errorf("在定义上：%q", a.Toast)
	}
	a.GoToDefinition(d, at(6, 3))
	if a.Toast != i18n.Tf("没找到 %s 的定义", "nowhere") || a.CurrentDoc() != d {
		t.Errorf("没有定义：%q", a.Toast)
	}

	// 没保存的改动也算：在这个文件里新加的函数
	d.Buf.Edit(editor.Edit{Start: d.Buf.Len(), End: d.Buf.Len(), Text: "\nfunc nowhere() {}\n"})
	a.GoToDefinition(d, at(6, 3))
	if d.St.CursorPos().Line != 11 {
		t.Errorf("新加的 nowhere 应跳到第 12 行，在 %d（%s）", d.St.CursorPos().Line, a.Toast)
	}

	// 不是文件的标签没有跳到定义
	if CanGoToDefinition(&docs.Doc{Kind: docs.SearchDoc}) {
		t.Error("搜索结果不该能跳到定义")
	}
	if !strings.HasPrefix(Find("editor.goToDefinition").Hint, "F12") {
		t.Errorf("跳到定义的快捷键提示：%q", Find("editor.goToDefinition").Hint)
	}
}
