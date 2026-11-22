package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/testutil"
)

// gedEditor 让这个测试的文件标签用 ged 的单栏编辑器（测完还原）。
func gedEditor(t *testing.T, on bool) {
	t.Helper()
	keep := gedEditorOn
	gedEditorOn = on
	t.Cleanup(func() { gedEditorOn = keep })
}

const gedFileSrc = "package demo\n\nfunc foobar() int {\n\tif true {\n\t\treturn 1\n\t}\n\treturn 2\n}\n\nfunc g() {\n\tfoobar()\n}\n"

// openGedFile 在临时仓库里开 demo.go，等到 ged 的编辑器画好。
func openGedFile(t *testing.T) (*app.App, *wui.Tester, *docs.Doc, string, func(string, func() bool)) {
	t.Helper()
	gedEditor(t, true)
	keepCopy, keepRead := app.CopyText, app.ReadClipboard
	clip := ""
	app.CopyText = func(s string) { clip = s }
	app.ReadClipboard = func() string { return clip }
	t.Cleanup(func() { app.CopyText, app.ReadClipboard = keepCopy, keepRead })
	repo := testutil.NewRepo(t, "gedfile")
	repo.Write("demo.go", gedFileSrc)
	repo.Commit("初始提交")
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), t.TempDir())
	r := a.Repos[0]
	d := a.OpenFile(r, "demo.go", true)
	wait("ged 的编辑器", func() bool { tt.Frame(); return d.Buf != nil && !d.Loading && d.UsingEd() && d.Ed.Bounds().H > 0 })
	a.FocusDoc = true
	wait("键盘给编辑器", func() bool { tt.Frame(); return d.Ed.Focused() })
	return a, tt, d, filepath.Join(repo.Root, "demo.go"), wait
}

// 文件标签用 ged 画：点了放光标（St 跟着），打字进缓冲区，App 挪 St 的光标 ged 跟着，保存写盘，看得见的行、
// 查找条、折叠命令都走 ged。
func TestGedFileEdit(t *testing.T) {
	a, tt, d, path, _ := openGedFile(t)
	g := d.Ed
	line := func(i int) string { return strings.Split(d.Buf.String(), "\n")[i] }
	box, lh := g.Bounds(), g.RowHeight()
	tt.ClickAt(g.LineX(1)+1, box.Y+2.5*lh-g.ScrollY)
	tt.Frame()
	if l, o, ok := g.Caret(); !ok || l != 2 || o != 0 {
		t.Fatalf("点了第 3 行行首，ged 的光标在 %d:%d（%v）", l, o, ok)
	}
	if want := len("package demo\n\n"); d.St.Cursor() != want {
		t.Fatalf("St 的光标 %d，应跟到 %d", d.St.Cursor(), want)
	}
	tt.Type("x")
	tt.Frame()
	if line(2) != "xfunc foobar() int {" {
		t.Fatalf("打了 x：%q", line(2))
	}
	if want := len("package demo\n\nx"); d.St.Cursor() != want {
		t.Fatalf("打字后 St 的光标 %d，应是 %d", d.St.Cursor(), want)
	}

	d.St.GoTo(editor.Pos{Line: 6, Col: 1}) // App 挪光标（跳到符号、搜索结果那样）
	tt.Frame()
	if l, o, _ := g.Caret(); l != 6 || o != 1 {
		t.Fatalf("St.GoTo 后 ged 的光标在 %d:%d", l, o)
	}
	if first, last := d.VisibleLines(); last <= first {
		t.Errorf("看得见的行 %d–%d：该问 ged 的", first, last)
	}

	a.Save(d)
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "xfunc foobar") {
		t.Fatalf("保存后磁盘上是 %q", b)
	}

	a.Exec("edit.find")
	tt.Frame()
	if !g.Find.Open {
		t.Error("⌘F 没开 ged 的查找条")
	}
	tt.Key(0, wui.KeyEscape)
	tt.Frame()
	if g.Find.Open {
		t.Error("Esc 没关查找条")
	}

	if !d.Run(editor.FoldAll) {
		t.Fatal("全部折叠没做")
	}
	tt.Frame()
	if f := g.Folded(); len(f) == 0 {
		t.Error("全部折叠后没有折起来的行")
	}
	d.Run(editor.UnfoldAll)
	tt.Frame()
	if f := g.Folded(); len(f) != 0 {
		t.Errorf("全部展开后还折着 %v", f)
	}
}

// 多光标：St 有两个光标时打字两处都进，ged 把另一个光标当 Extra 画；Esc 只留一个；⌥点击加光标。
func TestGedFileMultiCursor(t *testing.T) {
	_, tt, d, _, _ := openGedFile(t)
	g := d.Ed
	line := func(i int) string { return strings.Split(d.Buf.String(), "\n")[i] }
	off := func(l, c int) int { return d.Buf.Offset(editor.Pos{Line: l, Col: c}) }
	d.St.SetSelections(editor.Selection{Anchor: off(4, 2), Head: off(4, 2)}, editor.Selection{Anchor: off(6, 1), Head: off(6, 1)})
	tt.Frame()
	tt.Type("z")
	tt.Frame()
	if line(4) != "\t\tzreturn 1" || line(6) != "\tzreturn 2" {
		t.Fatalf("两个光标打 z：%q / %q", line(4), line(6))
	}
	if l, o, _ := g.Caret(); l != 6 || o != 2 {
		t.Errorf("ged 画的主光标在 %d:%d，应是最新的那个 6:2", l, o)
	}
	tt.Key(0, wui.KeyBackspace)
	tt.Frame()
	if line(4) != "\t\treturn 1" || line(6) != "\treturn 2" {
		t.Fatalf("两个光标 ⌫：%q / %q", line(4), line(6))
	}
	tt.Key(0, wui.KeyEscape)
	tt.Frame()
	if n := len(d.St.Selections()); n != 1 {
		t.Fatalf("Esc 后还有 %d 个光标", n)
	}

	box, lh := g.Bounds(), g.RowHeight()
	tt.ClickAt(g.LineX(1)+1, box.Y+0.5*lh-g.ScrollY)
	tt.ClickAtWith(wui.Alt, g.LineX(1)+1, box.Y+1.5*lh-g.ScrollY)
	tt.Frame()
	if n := len(d.St.Selections()); n != 2 {
		t.Fatalf("⌥点击后 %d 个光标", n)
	}
	tt.Type("#")
	tt.Frame()
	if line(0) != "#package demo" || line(1) != "#" {
		t.Fatalf("点击 + ⌥点击两处打 #：%q / %q", line(0), line(1))
	}
}

// 补全：打标识符里的字时开列表（片段和文件里的词），接着打按打的字筛，回车接选中的那项；Esc 关列表。
func TestGedFileCompletion(t *testing.T) {
	_, tt, d, _, _ := openGedFile(t)
	line := func(i int) string { return strings.Split(d.Buf.String(), "\n")[i] }
	d.St.GoTo(editor.Pos{Line: 12, Col: 0}) // 末尾的空行
	tt.Frame()
	tt.Type("f")
	tt.Frame()
	tt.Type("o")
	tt.Frame()
	tt.Type("o") // 筛到只剩 foobar（片段 for 的前缀对不上 foo）
	tt.Frame()
	if d.EdCompl == nil {
		t.Fatal("打了 foo 没开补全列表")
	}
	if !tt.HasText("foobar") {
		t.Errorf("补全列表里没有 foobar：%q", tt.Texts())
	}
	tt.Key(0, wui.KeyEnter)
	tt.Frame()
	if line(12) != "foobar" {
		t.Fatalf("回车接补全：%q", line(12))
	}
	if d.EdCompl != nil {
		t.Error("接了以后列表还开着")
	}
	tt.Type(" ")
	tt.Type("f")
	tt.Frame()
	tt.Type("o")
	tt.Frame()
	if d.EdCompl == nil {
		t.Fatal("又打了 fo 没开列表")
	}
	tt.Key(0, wui.KeyEscape)
	tt.Frame()
	if d.EdCompl != nil {
		t.Error("Esc 没关补全列表")
	}
	if line(12) != "foobar fo" {
		t.Errorf("Esc 改了字：%q", line(12))
	}
}

// 关着开关、只读、blame 时还是 mygo 的编辑器。
func TestGedFileOff(t *testing.T) {
	gedEditor(t, false)
	repo := testutil.NewRepo(t, "gedoff")
	repo.Write("demo.go", gedFileSrc)
	repo.Commit("初始提交")
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), t.TempDir())
	d := a.OpenFile(a.Repos[0], "demo.go", true)
	wait("读好", func() bool { tt.Frame(); return d.Buf != nil && !d.Loading })
	tt.Frame()
	if d.UsingEd() || d.Ed != nil {
		t.Fatal("开关关着也用了 ged 的编辑器")
	}
}

// 悬停提示：编辑器的“显示悬停”命令（和指针停稳一样走 Style.Hover 的那条路）在名字上开声明的提示，Esc 关掉。
func TestGedFileHover(t *testing.T) {
	a, tt, d, _, wait := openGedFile(t)
	at := d.Buf.Offset(editor.Pos{Line: 10, Col: len("\tfoo")}) // g 里调 foobar 的那处
	wait("符号索引", func() bool { tt.Frame(); return a.HoverAt(d, at) != nil })
	d.St.Select(at, at)
	tt.Frame()
	d.Run(editor.ShowHover)
	tt.Frame()
	if d.EdTip == nil {
		t.Fatal("显示悬停没开提示")
	}
	if !strings.Contains(d.EdTip.Code, "foobar") {
		t.Errorf("提示的声明是 %q", d.EdTip.Code)
	}
	tt.Key(0, wui.KeyEscape)
	tt.Frame()
	if d.EdTip != nil {
		t.Error("Esc 没关提示")
	}
}

// 改动竖条和就地卡：改了的行在行号右边有竖条（Style.Changes 从 DocHunks 来），点那一行的行号开这一块的卡，再点关上；
// 诊断画波浪线，指针停在上面的提示里有消息。
func TestGedFileMarks(t *testing.T) {
	a, tt, d, _, wait := openGedFile(t)
	g := d.Ed
	d.St.GoTo(editor.Pos{Line: 4, Col: 2})
	tt.Frame()
	tt.Type("y")
	tt.Frame()
	wait("改动块", func() bool { tt.Frame(); return len(app.DocHunks(d)) > 0 })
	cs := edChanges(d)
	if len(cs) != 1 || cs[0].Start != 4 || cs[0].End != 5 || cs[0].Kind != 1 {
		t.Fatalf("改了第 5 行：%+v", cs)
	}
	box, lh := g.Bounds(), g.RowHeight()
	tt.ClickAt(box.X+6, box.Y+4.5*lh-g.ScrollY)
	tt.Frame()
	if a.Peek.Doc != d {
		t.Fatal("点改了的那行的行号没开卡")
	}
	if !tt.HasText(i18n.Tf("第 %d / %d 处改动", 1, 1)) {
		t.Errorf("卡没画出来：%q", tt.Texts())
	}
	tt.ClickAt(box.X+6, box.Y+4.5*lh-g.ScrollY)
	tt.Frame()
	if a.Peek.Doc != nil {
		t.Error("再点没关卡")
	}

	at := d.Buf.Offset(editor.Pos{Line: 6, Col: 1})
	d.St.SetDiagnostics(d.Buf.Version(), []editor.Diagnostic{{Start: at, End: at + len("return"), Severity: editor.SeverityWarning, Message: "别这样写", Source: "vet"}})
	tt.Frame()
	sq := edSquiggles(d, fullCodeTheme(lightTheme()))
	if len(sq) != 1 || sq[0].AL != 6 || sq[0].AO != 1 || sq[0].BO != 7 {
		t.Fatalf("波浪线：%+v", sq)
	}
	edHover(a, d, at+2)
	tt.Frame()
	if d.EdTip == nil || !tt.HasText("别这样写") {
		t.Fatalf("停在诊断上的提示：%v %q", d.EdTip, tt.Texts())
	}
}

// 行内 blame 开着时 ged 在光标那一行行尾画淡字（Style.Notes 从 Doc.BlameInlay 来）。和弦由 ged 自己按键位表查（ged 的 keys_test）；
// ⌘K 开头的那几个被菜单的“运行 git 命令…”先拿走，和 mygo 的编辑器一样。
func TestGedFileBlame(t *testing.T) {
	a, tt, d, _, wait := openGedFile(t)
	a.St.Settings.InlineBlame = true
	d.St.GoTo(editor.Pos{Line: 2, Col: 0})
	wait("行内 blame", func() bool { tt.Frame(); return d.BlameInlay.Text != "" })
	if p := d.Buf.Pos(d.BlameInlay.Offset); p.Line != 2 {
		t.Errorf("行内 blame 在第 %d 行", p.Line)
	}
	tt.Frame()
}
