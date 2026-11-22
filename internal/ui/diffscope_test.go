package ui

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/git"
	"github.com/lemoeade/grove/internal/testutil"
)

// scopeFile 是两个长函数的 Go 文件（每个 40 行）；改了的是 First 和 Second 中间各一行，和一行超过 200 列的字符串。
func scopeFile(changed bool) string {
	var b strings.Builder
	b.WriteString("package demo\n\nimport \"fmt\"\n\n")
	for _, fn := range []string{"First", "Second"} {
		fmt.Fprintf(&b, "// %s 演示折叠条上的所在函数。\nfunc %s(n int) (int, error) {\n", fn, fn)
		for i := 1; i <= 40; i++ {
			line := fmt.Sprintf("\tn += %d // 第 %d 步", i, i)
			if changed && i == 20 {
				line = fmt.Sprintf("\tn *= %d // 第 %d 步，翻倍", i, i) // 有删有加：两边都有改了的词
			}
			b.WriteString(line + "\n")
		}
		if fn == "Second" {
			msg := strings.Repeat("一行很长的说明，", 12)
			if changed {
				msg = strings.Repeat("一行很长的说明（改过），", 12)
			}
			fmt.Fprintf(&b, "\tfmt.Println(%q)\n", msg)
		}
		b.WriteString("\treturn n, nil\n}\n\n")
	}
	return b.String()
}

func scopeRepo(t *testing.T) (*app.App, func(string, func() bool), *app.Repo) {
	t.Helper()
	repo := testutil.NewRepo(t, "scope")
	repo.Write("demo.go", scopeFile(false))
	repo.Commit("初始提交")
	repo.Write("demo.go", scopeFile(true))
	a, _, wait := newUI(t, testutil.Workspace(t, repo.Root), "")
	a.St.FullDiff, a.St.Unified = false, false
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	return a, wait, r
}

// 折叠条左半截写藏着的行之后那段所在的函数（像 GitHub 块头 @@ 后面的那段），函数头那行去掉缩进和 {；文件结尾那条不写。
func TestDiffSepScope(t *testing.T) {
	mygoDiff(t)
	repo := testutil.NewRepo(t, "scope")
	repo.Write("demo.go", scopeFile(false))
	repo.Commit("初始提交")
	repo.Write("demo.go", scopeFile(true))
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), "")
	a.St.FullDiff, a.St.Unified = false, false
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "demo.go"}, true)
	wait("demo.go 的 diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && len(folded(tt)) > 0 })
	tt.Frame()
	for _, want := range []string{"func First(n int) (int, error)", "func Second(n int) (int, error)"} {
		if !shows(tt, want) {
			t.Errorf("折叠条上没有 %q：%q", want, tt.Texts())
		}
	}
	if slices.ContainsFunc(tt.Texts(), func(s string) bool { return strings.HasSuffix(s, "{") && strings.HasPrefix(s, "func ") }) {
		t.Errorf("函数头应去掉行尾的 {：%q", tt.Texts())
	}
	// 结尾那条后面没有改动，不写所在函数
	n := 0
	for _, s := range tt.Texts() {
		if strings.HasPrefix(s, "func ") && strings.HasSuffix(s, "(n int) (int, error)") {
			n++
		}
	}
	if n == 0 || n > 4 { // Texts 里一条可能出现不止一次
		t.Errorf("带所在函数的折叠条数不对：%d", n)
	}

	if diffDoc(a, "").DiffOutline() == nil {
		t.Fatal("Go 文件的 Diff 应有旧的一边的大纲")
	}
}

// mygo 没导出折叠条从哪一行开始（DiffSeparator.a），sepStart 用 reflect 读；升级 mygo 改了字段这里先报。
func TestSepStart(t *testing.T) {
	f, ok := reflect.TypeOf(editor.DiffSeparator{}).FieldByName("a")
	if !ok || f.Type.Kind() != reflect.Int {
		t.Fatal("mygo 的 DiffSeparator 没有 int 字段 a 了：sepScope 拿不到折叠条的位置，折叠条上不再写所在函数")
	}
}

// ⌥Z 在 Diff 标签里也管用：只切这一个标签的软换行，DiffState.Wrap 跟着。
func TestDiffSoftWrap(t *testing.T) {
	a, wait, r := scopeRepo(t)
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "demo.go"}, true)
	wait("demo.go 的 diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && d.DiffSt != nil })
	d := diffDoc(a, "")
	if a.WrapOf(d) {
		t.Fatal("Go 文件的 Diff 默认不该软换行")
	}
	if c := app.Find("view.softWrap"); c.Enabled == nil || !c.Enabled(a) {
		t.Fatal("Diff 标签里 ⌥Z 应该能用")
	}
	a.Exec("view.softWrap")
	wait("软换行", func() bool { return d.DiffSt.Wrap })
	a.Exec("view.softWrap")
	wait("关掉软换行", func() bool { return !d.DiffSt.Wrap })
}

// TestScreenshotDiffLook 是 Diff 的截图（WORKBENCH_SHOT=目录 时才跑）：
//
//	diff-scope.png  并排、只看改动附近：折叠条左边写所在函数
//	diff-wrap.png   统一、⌥Z 软换行：长的那行折成几行
//	diff-dark.png   深色、并排：红绿和折叠条的淡底
func TestScreenshotDiffLook(t *testing.T) {
	dir := os.Getenv("WORKBENCH_SHOT")
	if dir == "" {
		t.Skip("设置 WORKBENCH_SHOT=目录 才渲染截图")
	}
	repo := testutil.NewRepo(t, "scope")
	repo.Write("demo.go", scopeFile(false))
	repo.Commit("初始提交")
	repo.Write("demo.go", scopeFile(true))
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), t.TempDir())
	tt.SetScale(2)
	tt.SetTitleBar(shotBar)
	a.St.BottomBar = false
	a.St.FullDiff, a.St.Unified = false, false
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.Exec("view.git")
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "demo.go"}, true)
	wait("demo.go 的 diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && len(folded(tt)) > 0 })
	writeShot(t, tt, dir, "diff-scope.png")

	a.Exec("diff.unified")
	a.Exec("view.softWrap")
	tt.Frame()
	writeShot(t, tt, dir, "diff-wrap.png")

	a.Exec("view.softWrap")
	a.Exec("diff.unified")
	tt.SetDark(true)
	tt.Frame()
	writeShot(t, tt, dir, "diff-dark.png")
}

// 按改动那行认所在符号，不按第一行看得见的：改动在两个函数之间（上下文里是上一个函数的 }）时不写；
// Markdown 写最里层的标题（## 那节），不是包着全文的 #。
func TestDiffSepScopeChangedLine(t *testing.T) {
	mygoDiff(t)
	body := func(changed bool) (goSrc, md string) {
		var g, m strings.Builder
		g.WriteString("package demo\n\n")
		m.WriteString("# 全文\n\n")
		for _, fn := range []string{"Alpha", "Beta"} {
			if fn == "Beta" {
				note := "// Beta 在后面。"
				if changed {
					note = "// Beta 在后面，改过的说明。"
				}
				g.WriteString(note + "\n")
			}
			fmt.Fprintf(&g, "func %s() {\n", fn)
			fmt.Fprintf(&m, "## %s 一节\n\n", fn)
			for i := range 30 {
				fmt.Fprintf(&g, "\t_ = %d\n", i)
				line := fmt.Sprintf("第 %d 行。", i)
				if changed && fn == "Beta" && i == 15 {
					line = "改过的第 15 行。"
				}
				m.WriteString(line + "\n\n")
			}
			g.WriteString("}\n\n")
		}
		return g.String(), m.String()
	}
	repo := testutil.NewRepo(t, "scope2")
	g0, m0 := body(false)
	repo.Write("demo.go", g0)
	repo.Write("notes.md", m0)
	repo.Commit("初始提交")
	g1, m1 := body(true)
	repo.Write("demo.go", g1)
	repo.Write("notes.md", m1)
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), "")
	a.St.FullDiff, a.St.Unified = false, false
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))

	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "demo.go"}, true)
	wait("demo.go 的 diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && len(folded(tt)) > 0 })
	tt.Frame()
	if slices.ContainsFunc(tt.Texts(), func(s string) bool { return strings.HasPrefix(s, "func ") }) {
		t.Errorf("改动在两个函数之间，折叠条不该写函数：%q", tt.Texts())
	}

	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "notes.md"}, true)
	wait("notes.md 的 diff", func() bool {
		d := diffDoc(a, "")
		return d != nil && !d.Loading && strings.HasSuffix(d.Title, "notes.md") && len(folded(tt)) > 0
	})
	tt.Frame()
	if !shows(tt, "## Beta 一节") {
		t.Errorf("Markdown 应写改动所在的那节：%q", tt.Texts())
	}
	if shows(tt, "# 全文") {
		t.Errorf("不该写包着全文的标题：%q", tt.Texts())
	}
}

// Diff 改了的行的底色是 Grove 画在底下的（difftint.go）：mygo 不涂底，红绿和 mygo 排的行对齐，没改的行、并排时对面留的空不涂。
func TestDiffTints(t *testing.T) {
	mygoDiff(t)
	repo := testutil.NewRepo(t, "tint")
	repo.Write("demo.go", scopeFile(false))
	repo.Commit("初始提交")
	repo.Write("demo.go", scopeFile(true))
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), "")
	a.St.FullDiff, a.St.Unified = false, false
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "demo.go"}, true)
	wait("demo.go 的 diff", func() bool { d := diffDoc(a, ""); return d != nil && len(d.DiffSt.Hunks()) > 0 })
	tt.Frame()
	d := diffDoc(a, "").DiffSt
	if bg := d.Theme.Background; !d.NoBackground || bg.A != 0 || bg.R+bg.G+bg.B == 0 {
		t.Fatalf("mygo 应该不涂 Diff 的底：NoBackground=%v Background=%+v", d.NoBackground, bg)
	}
	hunks := d.Hunks()
	if len(hunks) != 3 {
		t.Fatalf("改了 3 处，Hunks = %+v", hunks)
	}
	view := wui.Rect{X: 10, Y: 20, W: 801, H: 1e5} // 足够高，三段都看得见
	sel := wui.RGBA(0, 0, 255, 0.5)
	lh := float32(d.LineTop(editor.DiffNew, hunks[0].NewStart+1) - d.LineTop(editor.DiffNew, hunks[0].NewStart))
	at := func(s editor.DiffSide, line int) float32 { return view.Y + float32(d.LineTop(s, line)-d.ScrollY()) }
	split := func() []diffTintRect {
		var out []diffTintRect
		for _, x := range diffTints(d, view, 2, sel) {
			if x.C != sel {
				out = append(out, x)
			}
		}
		return out
	}
	got := split()
	if len(got) != 2*len(hunks) {
		t.Fatalf("并排：每处两边各一块，得到 %+v", got)
	}
	for i, hk := range hunks {
		old, nw := got[2*i], got[2*i+1]
		want := [2]diffTintRect{
			{wui.Rect{X: 10, Y: at(editor.DiffOld, hk.OldStart), W: 400, H: lh}, diffDeletedLine},
			{wui.Rect{X: 411, Y: at(editor.DiffNew, hk.NewStart), W: 400, H: lh}, diffAddedLine},
		}
		if old != want[0] || nw != want[1] {
			t.Errorf("第 %d 处：%+v %+v，应为 %+v", i, old, nw, want)
		}
	}
	// 框外的不涂：框只到第一处的中间
	short := view
	short.H = at(editor.DiffNew, hunks[0].NewStart) + lh/2 - view.Y
	for _, x := range diffTints(d, short, 2, sel) {
		if x.R.Y+x.R.H > short.Y+short.H+0.01 {
			t.Errorf("涂到框外了：%+v（框 %+v）", x, short)
		}
	}

	// 统一视图：删掉的那行在加的那行上面，整宽
	a.St.Unified = true
	tt.Frame()
	if d.Layout != editor.DiffUnified {
		t.Fatal("没换成统一视图")
	}
	got = split()
	if len(got) != 2*len(hunks) {
		t.Fatalf("统一：每处删一块、加一块，得到 %+v", got)
	}
	for i, hk := range hunks {
		del, add := got[2*i], got[2*i+1]
		if del.C != diffDeletedLine || del.R.X != 10 || del.R.W != 801 || del.R.H != lh || del.R.Y+del.R.H != add.R.Y {
			t.Errorf("第 %d 处删掉的行：%+v（加的 %+v）", i, del, add)
		}
		if add.C != diffAddedLine || add.R.Y != at(editor.DiffNew, hk.NewStart) || add.R.H != lh || add.R.W != 801 {
			t.Errorf("第 %d 处加的行：%+v", i, add)
		}
	}
}
