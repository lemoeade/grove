package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/testutil"
)

// declText：参数折行的声明连着取、去掉末尾的 {、去掉共同缩进；上面紧挨着的行注释、块注释、跳过注解；
// 隔了空行的注释不算；Python 取声明后面的文档字符串。
func TestDeclText(t *testing.T) {
	for _, c := range []struct {
		name, src, ext string
		line           int
		code, doc      string
	}{
		{"行注释", "package x\n\n// Sum 加起来。\n// 第二行。\nfunc Sum(a int) int {\n", ".go", 4,
			"func Sum(a int) int", "Sum 加起来。\n第二行。"},
		{"参数折行", "\t// Join 拼。\n\tfunc Join(a string,\n\t\tb string) string {\n\t\treturn a\n", ".go", 1,
			"func Join(a string,\n\tb string) string", "Join 拼。"},
		{"块注释和注解", "/**\n * 这是文档。\n * 第二行\n */\n@Override\npublic void run() {\n", ".java", 5,
			"public void run()", "这是文档。\n第二行"},
		{"单行块注释", "/* 简短 */\nint f(void);\n", ".c", 1, "int f(void);", "简短"},
		{"一行写完的函数体", "func A() int { return 1 }\n", ".go", 0, "func A() int", ""},
		{"参数里的大括号", "func F(x struct{ a int }) {\n", ".go", 0, "func F(x struct{ a int })", ""},
		{"隔空行", "// 别的注释\n\nfunc g() {}\n", ".go", 2, "func g()", ""},
		{"Python 文档字符串", "def area(w,\n         h):\n    \"\"\"算面积。\n\n    w、h 是边长。\n    \"\"\"\n", ".py", 0,
			"def area(w,\n         h):", "算面积。\n\nw、h 是边长。"},
		{"Python 单行文档字符串", "# 不是这个\ndef f():\n    '''一行。'''\n", ".py", 1, "def f():", "一行。"},
		{"Python 没有文档字符串用注释", "# 上面的注释\ndef f():\n    pass\n", ".py", 1, "def f():", "上面的注释"},
		{"Rust 属性", "/// 文档\n#[inline]\npub fn f() {\n", ".rs", 2, "pub fn f()", "文档"},
	} {
		code, doc := declText(strings.Split(c.src, "\n"), c.line, c.ext)
		if code != c.code || doc != c.doc {
			t.Errorf("%s：\n%q %q\n应为\n%q %q", c.name, code, doc, c.code, c.doc)
		}
	}
}

// 悬停提示（三个仓库）：同文件的、别的仓库的都给声明、注释和位置；`util.Parse` 按限定词挑；
// 同名的有几处写"还有几处"；停在声明本身、找不到的不给；没存的改动也算；设置关了不给。
func TestHoverAt(t *testing.T) {
	alpha, beta, gamma, ws := testutil.ThreeRepos(t)
	alpha.Write("main.go", "package main\n\nfunc main() {\n\thelper()\n\tutil.Parse()\n\tCombine(1, \"x\")\n\tShared()\n\tnowhere()\n}\n\n"+
		"/* helper 帮忙。 */\nfunc helper() {}\n")
	alpha.Write("util/parse.go", "package util\n\n// Parse 解析 util 的。\nfunc Parse() {}\n")
	alpha.Commit("go")
	beta.Write("other/parse.go", "package other\n\n// Parse 是 other 的。\nfunc Parse() {}\n")
	beta.Write("combine.go", "package beta\n\n// Combine 把两样合起来。\n// 第二行。\nfunc Combine(a int,\n\tb string) error {\n\treturn nil\n}\n")
	beta.Write("shared.go", "package beta\n\nfunc Shared() {}\n")
	beta.Commit("go")
	gamma.Write("shared.go", "package gamma\n\nfunc Shared() {}\n")
	gamma.Commit("go")
	a, wait := newTestApp(t, ws)
	t.Cleanup(a.Close)
	d := a.OpenFile(a.Repos[0], "main.go", true)
	wait("读好", func() bool { return d.Buf != nil && !d.Loading })
	at := func(line, col int) int { return d.Buf.Offset(editor.Pos{Line: line, Col: col}) }

	// 第一次悬停时索引还没建：先不给，建好了再停就有
	var h *Hover
	wait("建索引", func() bool { h = a.HoverAt(d, at(5, 3)); return h != nil && !a.defPending() })

	if h.Code != "func Combine(a int,\n\tb string) error" || h.Doc != "Combine 把两样合起来。\n第二行。" ||
		h.Loc != "beta/combine.go:5" || h.More != 0 {
		t.Errorf("Combine：%+v", h)
	}
	if s, e := h.Start, h.End; d.Buf.Slice(s, e) != "Combine" {
		t.Errorf("范围：%q", d.Buf.Slice(s, e))
	}
	if h := a.HoverAt(d, at(3, 1)); h == nil || h.Code != "func helper()" || h.Doc != "helper 帮忙。" || h.Loc != "alpha/main.go:12" {
		t.Errorf("helper：%+v", h)
	}
	if h := a.HoverAt(d, at(4, 8)); h == nil || h.Loc != "alpha/util/parse.go:4" || h.Doc != "Parse 解析 util 的。" {
		t.Errorf("util.Parse：%+v", h)
	}
	if h := a.HoverAt(d, at(6, 2)); h == nil || h.More != 1 || !slices.Contains([]string{"beta/shared.go:3", "gamma/shared.go:3"}, h.Loc) {
		t.Errorf("Shared：%+v", h)
	}
	if h := a.HoverAt(d, at(11, 7)); h != nil {
		t.Errorf("在声明上不该给：%+v", h)
	}
	if h := a.HoverAt(d, at(7, 3)); h != nil {
		t.Errorf("找不到的不该给：%+v", h)
	}
	if h := a.HoverAt(d, at(2, 0)); h != nil {
		t.Errorf("关键字 func 不该给：%+v", h)
	}

	// 点位置打开那一处
	h = a.HoverAt(d, at(5, 3))
	a.OpenHoverSite(h)
	if c := a.CurrentDoc(); c.Root != beta.Root || c.Rel != "combine.go" {
		t.Fatalf("打开了 %s/%s", c.Root, c.Rel)
	}
	if a.OpenFile(a.Repos[0], "main.go", true) != d {
		t.Fatal("回 main.go 开了新标签")
	}

	// 没存的改动也算（开着的文件读缓冲区）
	d.Buf.Edit(editor.Edit{Start: d.Buf.Len(), End: d.Buf.Len(), Text: "\n// nowhere 新加的。\nfunc nowhere(x int) {\n}\n"})
	if h := a.HoverAt(d, at(7, 3)); h == nil || h.Code != "func nowhere(x int)" || h.Doc != "nowhere 新加的。" {
		t.Errorf("没存的 nowhere：%+v", h)
	}

	a.St.Settings.NoHoverDef = true
	if h := a.HoverAt(d, at(5, 3)); h != nil {
		t.Errorf("设置关了还给：%+v", h)
	}
}

// 查找引用（三个仓库）：整词、区分大小写、只搜同一种语言的文件，所有仓库的都在结果里；搜索页打开。
func TestFindReferences(t *testing.T) {
	alpha, beta, gamma, ws := testutil.ThreeRepos(t)
	alpha.Write("main.go", "package main\n\nfunc main() {\n\thelper()\n}\n\nfunc helper() {}\n")
	alpha.Commit("go")
	beta.Write("use.go", "package beta\n\nvar _ = helper\nvar _ = helperX\nvar _ = Helper\n")
	beta.Write("notes.md", "helper 在文档里\n")
	beta.Commit("go")
	gamma.Write("web/app.ts", "helper()\n")
	gamma.Write("web/lib.tsx", "export const helper = 1\n")
	gamma.Commit("ts")
	a, wait := newTestApp(t, ws)
	t.Cleanup(a.Close)
	d := a.OpenFile(a.Repos[0], "main.go", true)
	wait("读好", func() bool { return d.Buf != nil && !d.Loading })

	a.FindReferences(d, d.Buf.Offset(editor.Pos{Line: 3, Col: 2}))
	q := a.Search.Query
	if q.Text != "helper" || !q.WholeWord || !q.CaseSensitive || q.Include != "*.go" || a.St.SidebarMode != "search" {
		t.Fatalf("查询：%+v，侧栏 %q", q, a.St.SidebarMode)
	}
	wait("搜索", func() bool { return a.Search.Done && !a.Search.Running })
	var got []string
	for _, m := range a.Search.Res.Matches {
		got = append(got, m.Repo+"/"+m.Path)
	}
	if !slices.Equal(got, []string{"alpha/main.go", "alpha/main.go", "beta/use.go"}) {
		t.Errorf("结果：%q", got)
	}

	// TypeScript：js 那一家的扩展名都搜
	ts := a.OpenFile(a.RepoFor(gamma.Root), "web/app.ts", true)
	wait("读好 app.ts", func() bool { return ts.Buf != nil && !ts.Loading })
	a.Exec("editor.findReferences")
	if !strings.Contains(a.Search.Query.Include, "*.tsx") {
		t.Errorf("ts 的包含：%q", a.Search.Query.Include)
	}
	wait("搜索", func() bool { return a.Search.Done && !a.Search.Running })
	if n := len(a.Search.Res.Matches); n != 2 {
		t.Errorf("ts 应找到 2 处，%d", n)
	}

	// 不在名字上
	ts.St.Select(ts.Buf.Len(), ts.Buf.Len())
	a.FindReferences(ts, ts.Buf.Len())
	if a.Toast == "" {
		t.Error("不在名字上该提示")
	}
	if referenceGlobs("Makefile") != "" || referenceGlobs("a.rs") != "*.rs" {
		t.Errorf("包含：%q %q", referenceGlobs("Makefile"), referenceGlobs("a.rs"))
	}
}
