package ui

import (
	"os"
	"strconv"
	"strings"
	"testing"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/ged"
	"github.com/lemoeade/grove/internal/git"
	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/testutil"
)

// gedFiles 是 ged 并排 Diff 的测试文件：一行改了几个字、删了两行、加了三行（对面留空画斜线）、一行改成几乎全不一样，
// 中间隔着长段没改的（折起来）。
func gedFiles() (old, new string) {
	var o, n strings.Builder
	o.WriteString("package demo\n\nimport \"fmt\"\n\n")
	n.WriteString("package demo\n\nimport \"fmt\"\n\n")
	for i := 1; i <= 120; i++ {
		line := "\tn += " + strconv.Itoa(i) + " // 第 " + strconv.Itoa(i) + " 步\n"
		switch i {
		case 8:
			o.WriteString(line)
			n.WriteString("\tn *= " + strconv.Itoa(i) + " // 第 " + strconv.Itoa(i) + " 步，翻倍\n")
		case 15:
			o.WriteString("\t/* 旧的说明\n\t   跨两行 */\n")
			o.WriteString(line)
			n.WriteString(line)
			n.WriteString("\tif n > 100 {\n\t\treturn n, fmt.Errorf(\"太大\")\n\t}\n")
		case 24:
			o.WriteString("\tlog(n)\n")
			n.WriteString("\tfmt.Println(\"n 现在是\", n, strings.Repeat(\"=\", 3))\n")
		default:
			o.WriteString(line)
			n.WriteString(line)
		}
	}
	return o.String(), n.String()
}

// 开了 WORKBENCH_GED 时 Diff 标签的并排视图是 ged 的：只看改动附近时长段没改的折起来、点折叠条展开、滚轮滚得动；
// 切成全文就不折。
func TestGedDiff(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	old, new := gedFiles()
	repo := testutil.NewRepo(t, "ged")
	repo.Write("demo.go", old)
	repo.Commit("初始提交")
	repo.Write("demo.go", new)
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
	wait("demo.go 的 ged diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && d.Ged != nil })
	d := diffDoc(a, "")
	g := d.Ged
	kinds := map[ged.RowKind]int{}
	for _, row := range g.Rows() {
		kinds[row.Kind]++
	}
	if kinds[ged.Fold] == 0 || kinds[ged.Changed] < 2 || kinds[ged.Added] < 2 {
		t.Fatalf("行的种类不对：%v", kinds)
	}
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		writeShot(t, tt, dir, "ged-diff.png")
		tt.SetDark(true)
		tt.Frame()
		writeShot(t, tt, dir, "ged-diff-dark.png")
		tt.SetDark(false)
		tt.Frame()
	}

	box := g.Bounds()
	if box.W <= 0 {
		t.Fatal("ged 的视图没画出来")
	}
	// 点第一个折叠条：展开后没有那一段了
	n := len(g.Rows())
	fold := -1
	for i, row := range g.Rows() {
		if row.Kind == ged.Fold {
			fold = i
			break
		}
	}
	y := box.Y + (float32(fold)+0.5)*g.RowHeight() - g.ScrollY
	tt.ClickAt(box.X+box.W/4, y)
	tt.Frame()
	if len(g.Rows()) <= n {
		t.Fatalf("点了折叠条没展开：%d 行 → %d 行", n, len(g.Rows()))
	}
	// 展开最后那段（很长）再用滚轮往下滚
	g.Expand(len(g.Rows()) - 1)
	tt.Frame()
	tt.Scroll(box.X+box.W/2, box.Y+box.H/2, 0, 120)
	tt.Frame()
	if g.ScrollY <= 0 {
		t.Errorf("滚轮没滚动：ScrollY = %v", g.ScrollY)
	}
	a.ToggleScope()
	tt.Frame()
	for _, row := range g.Rows() {
		if row.Kind == ged.Fold {
			t.Fatal("全文还折着")
		}
	}
}

// ged 的 Diff 上：标签条的上 / 下一处、底栏的“第 i / n 处改动”按 ged 的改动走，跳到的那处右上角有“暂存这一块”，
// 点了只暂存那一块（临时仓库）。
func TestGedHunks(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	old, new := gedFiles()
	repo := testutil.NewRepo(t, "gedhunks")
	repo.Write("demo.go", old)
	repo.Commit("初始提交")
	repo.Write("demo.go", new)
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), t.TempDir())
	a.St.FullDiff, a.St.Unified = false, false
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.Exec("view.git")
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "demo.go"}, true)
	wait("demo.go 的 ged diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && d.Ged != nil })
	tt.Frame()
	d := diffDoc(a, "")
	g := d.Ged
	n := len(g.Hunks())
	if cur, total, _ := app.HunkPos(d); total != n || n < 3 || cur != -1 {
		t.Fatalf("HunkPos = %d / %d，ged 有 %d 处", cur, total, n)
	}
	if app.CanGoToHunk(d, -1) || !app.CanGoToHunk(d, 1) {
		t.Error("在第一处之前：上一处该灰、下一处能点")
	}
	if shows(tt, "暂存这一块") {
		t.Error("还没跳到哪处、指针也不在改动上，不该有按钮")
	}
	a.GoToHunk(d, 1)
	tt.Frame()
	a.GoToHunk(d, 1)
	tt.Frame()
	if cur, _, inside := app.HunkPos(d); cur != 1 || !inside {
		t.Fatalf("跳两次下一处后该在第 2 处：%d %v", cur, inside)
	}
	if !tt.HasText(i18n.Tf("第 %d / %d 处改动", 2, n)) {
		t.Errorf("底栏没写第 2 / %d 处", n)
	}
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		tt.SetScale(2)
		tt.SetTitleBar(shotBar)
		a.St.BottomBar = false
		tt.Frame()
		writeShot(t, tt, dir, "ged-hunk.png")
	}
	h := g.Hunks()[1]
	mustText(t, tt, "丢弃这一块")
	mustClick(t, tt, "暂存这一块")
	wait("暂存第 2 处", func() bool { return idle(r)() && r.Status.Staged == 1 })
	cached := repo.Git("diff", "--cached", "--", "demo.go")
	if strings.Contains(cached, "翻倍") || strings.Contains(cached, "Println") {
		t.Fatalf("暂存区里有别的块：\n%s", cached)
	}
	want := strings.Split(new, "\n")[h.NewStart:h.NewEnd]
	if h.NewEnd > h.NewStart && !strings.Contains(cached, "+"+want[0]) {
		t.Fatalf("暂存区里没有第 2 处（新的第 %d 行起）：\n%s", h.NewStart+1, cached)
	}
	wait("diff 重读", func() bool { d := diffDoc(a, ""); return d != nil && d.Ged != nil && len(d.Ged.Hunks()) == n-1 })
}

// ged 的 Diff 上拖着选字、⌘C 复制（剪贴板换成桩）、双击选词、Esc 取消；点过以后键盘归它：J 跳下一处，↓ / 翻页滚动。
func TestGedSelect(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	var copied string
	keep := app.CopyText
	app.CopyText = func(s string) { copied = s }
	t.Cleanup(func() { app.CopyText = keep })
	old, new := gedFiles()
	repo := testutil.NewRepo(t, "gedsel")
	repo.Write("demo.go", old)
	repo.Commit("初始提交")
	repo.Write("demo.go", new)
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), t.TempDir())
	a.St.FullDiff, a.St.Unified = true, false
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "demo.go"}, true)
	wait("demo.go 的 ged diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && d.Ged != nil })
	tt.Frame()
	d := diffDoc(a, "")
	g := d.Ged
	g.ScrollY = 0
	tt.Frame()
	box, lh := g.Bounds(), g.RowHeight()
	rowY := func(i int) float32 { return box.Y + (float32(i)+0.5)*lh - g.ScrollY }
	// 新的一边，从第 1 行行首（正文开头；行号上按下是选行）拖到第 3 行（“import "fmt"”）的最后
	x0 := g.LineX(1) + 1
	tt.Press(x0, rowY(0))
	tt.Move(box.X+box.W-30, rowY(2))
	tt.Release(box.X+box.W-30, rowY(2))
	tt.Frame()
	if got := g.Selected(); got != "package demo\n\nimport \"fmt\"" {
		t.Fatalf("拖选的是 %q", got)
	}
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		tt.SetScale(2)
		tt.SetTitleBar(shotBar)
		a.St.BottomBar = false
		tt.Frame()
		writeShot(t, tt, dir, "ged-select.png")
		tt.SetScale(1)
		tt.Frame()
	}
	tt.Command("copy")
	tt.Frame()
	if copied != "package demo\n\nimport \"fmt\"" {
		t.Fatalf("⌘C 复制的是 %q", copied)
	}
	// 双击选一个词：第 1 行的 package
	ex := g.LineX(1) + 20 // 正文开头往右一点（package 里面）
	tt.ClickAt(ex, rowY(0))
	tt.ClickAt(ex, rowY(0)) // 紧接着再点一下是双击（Tester 按它自己的钟数）
	tt.Frame()
	if got := g.Selected(); got != "package" {
		t.Errorf("双击选的是 %q", got)
	}
	tt.Key(0, wui.KeyEscape)
	tt.Frame()
	if g.Selected() != "" {
		t.Error("Esc 没取消选择")
	}
	// 键盘：J 跳下一处改动，↓ 滚一行
	cur, _, _ := app.HunkPos(d)
	tt.Key(0, wui.KeyJ)
	tt.Frame()
	if c, _, _ := app.HunkPos(d); c != cur+1 {
		t.Errorf("J 没跳到下一处：%d → %d", cur, c)
	}
	y := g.ScrollY
	tt.Key(0, wui.KeyPageDown)
	tt.Frame()
	if g.ScrollY <= y {
		t.Errorf("翻页没滚动：%v → %v", y, g.ScrollY)
	}
	if strings.Contains(d.DiffSt.New.String(), "j") {
		t.Error("J 打进 diff 里了")
	}
}

// ged 的 Diff 上点行号选行：拖着选几行、Esc 取消；点一行再 ⇧ 点一行选上中间的，底下那栏“暂存选中的行”只暂存这几行（临时仓库）。
func TestGedLines(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	old, new := gedFiles()
	repo := testutil.NewRepo(t, "gedlines")
	repo.Write("demo.go", old)
	repo.Commit("初始提交")
	repo.Write("demo.go", new)
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), t.TempDir())
	a.St.FullDiff, a.St.Unified = false, false
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "demo.go"}, true)
	wait("demo.go 的 ged diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && d.Ged != nil })
	tt.Frame()
	d := diffDoc(a, "")
	g := d.Ged
	// 加了三行的那处（if n > 100 { … }）
	hi := -1
	for i, h := range g.Hunks() {
		if h.OldStart == h.OldEnd && h.NewEnd-h.NewStart == 3 {
			hi = i
		}
	}
	if hi < 0 {
		t.Fatalf("没找到加了三行的那处：%v", g.Hunks())
	}
	h := g.Hunks()[hi]
	g.GoToHunk(hi)
	tt.Frame()
	tt.Frame()
	box, lh := g.Bounds(), g.RowHeight()
	rowOf := func(line int) float32 { // 新的第 line 行在视图里的 y（行中间）
		for i, row := range g.Rows() {
			if row.Kind != ged.Fold && row.New == line {
				return box.Y + (float32(i)+0.5)*lh - g.ScrollY
			}
		}
		t.Fatalf("新的第 %d 行不在视图的行里", line+1)
		return 0
	}
	num := g.LineX(1) - 24 // 新的一边的行号上
	tt.Press(num, rowOf(h.NewStart))
	tt.Move(num, rowOf(h.NewStart+2))
	tt.Release(num, rowOf(h.NewStart+2))
	tt.Frame()
	if sel, ok := d.DiffSt.SelectedLines(); !ok || sel.Side != 1 || sel.Start != h.NewStart || sel.End != h.NewStart+2 {
		t.Fatalf("拖着选行：%+v %v，应为新的 %d–%d", sel, ok, h.NewStart, h.NewStart+2)
	}
	if g.Selected() != "" {
		t.Errorf("选行时还选了字：%q", g.Selected())
	}
	if !tt.HasText(i18n.Tn("选中 %d 行", 3)) {
		t.Error("底下没有“选中 3 行”那栏")
	}
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		tt.SetScale(2)
		tt.SetTitleBar(shotBar)
		a.St.BottomBar = false
		tt.Frame()
		writeShot(t, tt, dir, "ged-lines.png")
		tt.SetScale(1)
		a.St.BottomBar = true
		tt.Frame()
		box = g.Bounds()
		num = g.LineX(1) - 24
	}
	tt.Key(0, wui.KeyEscape)
	tt.Frame()
	if _, ok := d.DiffSt.SelectedLines(); ok {
		t.Fatal("Esc 没取消选中的行")
	}
	// 点第一行、⇧ 点第二行：只选这两行，暂存
	tt.ClickAt(num, rowOf(h.NewStart))
	tt.Frame()
	tt.ClickAtWith(wui.Shift, num, rowOf(h.NewStart+1))
	tt.Frame()
	if sel, ok := d.DiffSt.SelectedLines(); !ok || sel.Start != h.NewStart || sel.End != h.NewStart+1 {
		t.Fatalf("点一行再 ⇧ 点：%+v %v", sel, ok)
	}
	mustClick(t, tt, "暂存选中的行")
	wait("暂存选中的两行", func() bool { return idle(r)() && r.Status.Staged == 1 })
	cached := repo.Git("diff", "--cached", "--", "demo.go")
	lines := strings.Split(new, "\n")
	for _, want := range lines[h.NewStart : h.NewStart+2] {
		if !strings.Contains(cached, "+"+want+"\n") {
			t.Fatalf("暂存区里没有 %q：\n%s", want, cached)
		}
	}
	if strings.Contains(cached, "+"+lines[h.NewStart+2]+"\n") || strings.Contains(cached, "翻倍") || strings.Count(cached, "\n+") != 3 /* 两行加上 +++ 那行 */ {
		t.Fatalf("暂存区里多了别的行：\n%s", cached)
	}
	if _, ok := d.DiffSt.SelectedLines(); ok {
		t.Error("暂存完还选着")
	}
}

// 设置里选了统一时 ged 画成一栏：每处改动先删后加、没有 Changed 行；上 / 下一处照走；点删掉的那行的行号选的是旧的一边，
// 暂存选中的行只暂存删掉那一行（临时仓库）；切回并排还是并排。
func TestGedUnified(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	old, new := gedFiles()
	repo := testutil.NewRepo(t, "gedunified")
	repo.Write("demo.go", old)
	repo.Commit("初始提交")
	repo.Write("demo.go", new)
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), t.TempDir())
	a.St.FullDiff, a.St.Unified = false, true
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.Exec("view.git")
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "demo.go"}, true)
	wait("demo.go 的 ged diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && d.Ged != nil })
	tt.Frame()
	d := diffDoc(a, "")
	g := d.Ged
	if !g.Unified() {
		t.Fatal("设置里是统一，ged 还是并排")
	}
	for _, row := range g.Rows() {
		if row.Kind == ged.Changed {
			t.Fatalf("统一视图里有配成对的行：%v", row)
		}
	}
	n := len(g.Hunks())
	if _, total, _ := app.HunkPos(d); total != n || n < 3 {
		t.Fatalf("HunkPos 共 %d 处，ged 有 %d 处", total, n)
	}
	a.GoToHunk(d, 1)
	tt.Frame()
	if cur, _, inside := app.HunkPos(d); cur != 0 || !inside {
		t.Fatalf("下一处后该在第 1 处：%d %v", cur, inside)
	}
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		tt.SetScale(2)
		tt.SetTitleBar(shotBar)
		a.St.BottomBar = false
		tt.Frame()
		writeShot(t, tt, dir, "ged-unified.png")
		tt.SetDark(true)
		tt.Frame()
		writeShot(t, tt, dir, "ged-unified-dark.png")
		tt.SetDark(false)
		tt.SetScale(1)
		a.St.BottomBar = true
		tt.Frame()
	}
	// 删掉两行注释的那处：点第一行注释的行号
	h := g.Hunks()[1]
	if h.NewStart != h.NewEnd || h.OldEnd-h.OldStart != 2 {
		t.Fatalf("第 2 处该是只删两行：%+v", h)
	}
	g.GoToHunk(1)
	tt.Frame()
	tt.Frame()
	box, lh := g.Bounds(), g.RowHeight()
	ri := -1
	for i, row := range g.Rows() {
		if row.Kind == ged.Deleted && row.Old == h.OldStart {
			ri = i
		}
	}
	if ri < 0 {
		t.Fatal("统一视图里没有删掉的那行")
	}
	tt.ClickAt(g.LineX(0)-24, box.Y+(float32(ri)+0.5)*lh-g.ScrollY)
	tt.Frame()
	if sel, ok := d.DiffSt.SelectedLines(); !ok || sel.Side != 0 || sel.Start != h.OldStart || sel.End != h.OldStart {
		t.Fatalf("点删掉那行的行号：%+v %v", sel, ok)
	}
	mustClick(t, tt, "暂存选中的行")
	wait("暂存删掉的那一行", func() bool { return idle(r)() && r.Status.Staged == 1 })
	cached := repo.Git("diff", "--cached", "--", "demo.go")
	if !strings.Contains(cached, "-\t/* 旧的说明\n") || strings.Contains(cached, "跨两行 */\n-") || strings.Count(cached, "\n-") != 2 /* 一行加上 --- 那行 */ {
		t.Fatalf("暂存区该只删第一行注释：\n%s", cached)
	}
	a.St.Unified = false
	tt.Frame()
	if g.Unified() {
		t.Error("切回并排还是一栏")
	}
}
