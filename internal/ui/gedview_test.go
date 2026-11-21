package ui

import (
	"math"
	"os"
	"slices"
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

// ged 的 Diff 上就地编辑（临时仓库）：点新的一边放光标、打字、连着打的一步撤销、⌫、回车照抄缩进、←、
// J 打进去而不是跳改动、输入法的组字先画着不进缓冲区、⇧End 选了 ⌘X 剪掉（剪贴板换成桩）、⌘V 粘贴，⌘S 存回磁盘。
func TestGedEdit(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	var copied string
	keepCopy, keepRead := app.CopyText, app.ReadClipboard
	app.CopyText = func(s string) { copied = s }
	app.ReadClipboard = func() string { return "yy" }
	t.Cleanup(func() { app.CopyText, app.ReadClipboard = keepCopy, keepRead })
	repo := testutil.NewRepo(t, "gededit")
	repo.Write("demo.go", "package demo\n\nfunc f() int {\n\treturn\n}\n")
	repo.Commit("初始提交")
	repo.Write("demo.go", "package demo\n\nfunc f() int {\n\treturn 1\n}\n")
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), t.TempDir())
	a.St.FullDiff, a.St.Unified = true, false
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "demo.go"}, true)
	wait("demo.go 的 ged diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && d.Ged != nil })
	d := diffDoc(a, "")
	a.ToggleDiffEdit(d)
	wait("文件标签读完、Diff 能编辑", func() bool { tt.Frame(); d := diffDoc(a, ""); return d != nil && d.DiffSt != nil && d.DiffSt.Editable })
	d = diffDoc(a, "")
	g := d.Ged
	if g == nil {
		t.Fatal("就地编辑时不是 ged 画的")
	}
	buf := d.DiffSt.New
	line := func(i int) string { return strings.Split(buf.String(), "\n")[i] }
	box, lh := g.Bounds(), g.RowHeight()
	tt.ClickAt(g.LineX(1)+1, box.Y+0.5*lh-g.ScrollY)
	tt.Frame()
	if l, o, ok := g.Caret(); !ok || l != 0 || o != 0 {
		t.Fatalf("点了新的一边第 1 行行首，光标在 %d:%d（%v）", l, o, ok)
	}
	if _, ok := tt.TextCaret(); !ok {
		t.Error("有光标时输入法没接上（TextCaret）")
	}

	g.PlaceCaret(3, len("\treturn 1"))
	tt.Frame()
	tt.Type("0")
	tt.Type("0")
	tt.Frame()
	if line(3) != "\treturn 100" {
		t.Fatalf("打了两个 0：%q", line(3))
	}
	tt.Command("undo")
	tt.Frame()
	if line(3) != "\treturn 1" {
		t.Fatalf("连着打的没有一步撤销：%q", line(3))
	}
	tt.Command("redo")
	tt.Frame()
	if line(3) != "\treturn 100" {
		t.Fatalf("重做：%q", line(3))
	}
	tt.Key(0, wui.KeyBackspace)
	tt.Frame()
	if line(3) != "\treturn 10" {
		t.Fatalf("⌫：%q", line(3))
	}
	tt.Key(0, wui.KeyEnter)
	tt.Type("x")
	tt.Frame()
	if line(4) != "\tx" || line(5) != "}" {
		t.Fatalf("回车没照抄缩进：%q %q", line(4), line(5))
	}
	if l, o, _ := g.Caret(); l != 4 || o != 2 {
		t.Fatalf("打了 x 后光标在 %d:%d", l, o)
	}
	tt.Key(0, wui.KeyLeft)
	tt.TypeKey(0, wui.KeyJ, "j")
	tt.Frame()
	if line(4) != "\tjx" {
		t.Fatalf("← 再打 j：%q", line(4))
	}
	tt.Compose("ni", 2)
	tt.Frame()
	if strings.Contains(buf.String(), "ni") {
		t.Fatal("组字没确定就进了缓冲区")
	}
	tt.Type("你")
	tt.Frame()
	if line(4) != "\tj你x" {
		t.Fatalf("输入法确定后：%q", line(4))
	}
	tt.Key(wui.Shift, wui.KeyEnd)
	tt.Command("cut")
	tt.Frame()
	if copied != "x" || line(4) != "\tj你" {
		t.Fatalf("⇧End 选了剪掉：剪贴板 %q，行 %q", copied, line(4))
	}
	tt.Command("paste")
	tt.Frame()
	if line(4) != "\tj你yy" {
		t.Fatalf("粘贴：%q", line(4))
	}
	if rows := g.Rows(); len(rows) != 7 || rows[4] != (ged.Row{Kind: ged.Added, Old: -1, New: 4}) {
		t.Errorf("ged 没跟上缓冲区：%v", rows)
	}
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		tt.SetScale(2)
		tt.SetTitleBar(shotBar)
		a.St.BottomBar = false
		tt.Frame()
		writeShot(t, tt, dir, "ged-edit.png")
		tt.SetScale(1)
		tt.Frame()
	}
	tt.Key(wui.Cmd, wui.KeyS) // 焦点在 ged 上：⌘S 落到窗口快捷键，存的是文件标签
	want := "package demo\n\nfunc f() int {\n\treturn 10\n\tj你yy\n}\n"
	wait("存回磁盘", func() bool {
		b, err := os.ReadFile(repo.Root + "/demo.go")
		return err == nil && string(b) == want
	})
}

// ged 的折叠条和 mygo 那个一样，在“N 行未修改”后面写下面那处改动所在的函数（scopeAt 查旧的一边的大纲）。
func TestGedFoldScope(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	var o strings.Builder
	o.WriteString("package demo\n\nfunc first() int {\n")
	for i := range 20 {
		o.WriteString("\tn := " + strconv.Itoa(i) + "\n")
	}
	o.WriteString("\treturn 0\n}\n\nfunc second() int {\n")
	for i := range 20 {
		o.WriteString("\tm := " + strconv.Itoa(i) + "\n")
	}
	o.WriteString("\treturn 1\n}\n")
	old := o.String()
	new := strings.Replace(old, "\tm := 12\n", "\tm := 12 * 2\n", 1)
	repo := testutil.NewRepo(t, "gedscope")
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
	rows := d.Ged.Rows()
	if rows[0].Kind != ged.Fold {
		t.Fatalf("开头没折：%v", rows[0])
	}
	line := -1
	for _, row := range rows {
		if row.Kind == ged.Changed {
			line = row.Old
			break
		}
	}
	if text, _ := scopeAt(d, line); text != "func second() int" {
		t.Errorf("改动所在的函数：%q", text)
	}
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		tt.SetScale(2)
		tt.SetTitleBar(shotBar)
		a.St.BottomBar = false
		tt.Frame()
		writeShot(t, tt, dir, "ged-scope.png")
	}
}

// ⌥Z 软换行：长行按那边的宽折成几行，那一行跟着变高、下面的行往下挪；点在折出来的第二行上光标落在那一段，
// ↑ ↓ 先在折出来的几行里走。再按 ⌥Z 回到一行一条。
func TestGedWrap(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	long := "\t// " + strings.Repeat("wrap these words onto the next row ", 12)
	repo := testutil.NewRepo(t, "gedwrap")
	repo.Write("demo.go", "package demo\n\nfunc f() int {\n\t// short\n\treturn 1\n}\n")
	repo.Commit("初始提交")
	repo.Write("demo.go", "package demo\n\nfunc f() int {\n"+long+"\n\treturn 1\n}\n")
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), t.TempDir())
	a.St.FullDiff, a.St.Unified = true, false
	a.St.Settings.Wrap = "off"
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "demo.go"}, true)
	wait("demo.go 的 ged diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && d.Ged != nil })
	d := diffDoc(a, "")
	a.ToggleDiffEdit(d)
	wait("Diff 能编辑", func() bool { tt.Frame(); d := diffDoc(a, ""); return d != nil && d.DiffSt != nil && d.DiffSt.Editable })
	d = diffDoc(a, "")
	g := d.Ged
	tt.Frame()
	ci := -1
	for i, row := range g.Rows() {
		if row.Kind == ged.Changed {
			ci = i
		}
	}
	lh := g.RowHeight()
	if ci < 0 || g.RowTop(ci+1)-g.RowTop(ci) != lh {
		t.Fatalf("没换行时改了的那行（第 %d 行）高 %v，行高 %v", ci, g.RowTop(ci+1)-g.RowTop(ci), lh)
	}
	a.Exec("view.softWrap")
	tt.Frame()
	tt.Frame()
	h := g.RowTop(ci+1) - g.RowTop(ci)
	if h < 3*lh {
		t.Fatalf("换行后那行只有 %v 高（行高 %v）", h, lh)
	}
	if g.RowTop(ci+2)-g.RowTop(ci+1) != lh {
		t.Errorf("短的那行也变高了：%v", g.RowTop(ci+2)-g.RowTop(ci+1))
	}
	box := g.Bounds()
	tt.ClickAt(g.LineX(1)+1, box.Y+g.RowTop(ci)+1.5*lh-g.ScrollY) // 折出来的第二行行首
	tt.Frame()
	l, o, ok := g.Caret()
	if !ok || l != 3 || o <= 4 || o >= len(long)-4 {
		t.Fatalf("点在折出来的第二行，光标在 %d:%d（%v）", l, o, ok)
	}
	tt.Key(0, wui.KeyDown)
	tt.Frame()
	if l2, o2, _ := g.Caret(); l2 != 3 || o2 <= o {
		t.Errorf("↓ 应在同一行里往下一段：%d:%d", l2, o2)
	}
	tt.Key(0, wui.KeyUp)
	tt.Key(0, wui.KeyUp)
	tt.Frame()
	if l2, o2, _ := g.Caret(); l2 != 3 || o2 >= o {
		t.Errorf("↑ 两下应在这一行折出来的第一行：%d:%d", l2, o2)
	}
	tt.Key(0, wui.KeyUp)
	tt.Frame()
	if l2, _, _ := g.Caret(); l2 != 2 {
		t.Errorf("再 ↑ 应到上一行：第 %d 行", l2)
	}
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		tt.SetScale(2)
		tt.SetTitleBar(shotBar)
		a.St.BottomBar = false
		tt.Frame()
		writeShot(t, tt, dir, "ged-wrap.png")
	}
	a.Exec("view.softWrap")
	tt.Frame()
	if g.RowTop(ci+1)-g.RowTop(ci) != lh {
		t.Errorf("关了换行那行还是 %v 高", g.RowTop(ci+1)-g.RowTop(ci))
	}
}

// ⌘F（edit.find）在 ged 的 Diff 上开它自己的查找条：敲字就找（不分大小写），两边都找、没改的行只算一遍；
// ↩ 跳下一处，跳到折起来的段里时展开那段；Esc 关掉。
func TestGedFind(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	old, new := gedFiles()
	repo := testutil.NewRepo(t, "gedfind")
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
	if g.Rows()[0].Kind != ged.Fold {
		t.Fatalf("开头没折：%v", g.Rows()[0])
	}
	a.Exec("edit.find")
	tt.Frame()
	if !g.Find.Open {
		t.Fatal("⌘F 没开 ged 的查找条")
	}
	tt.Type("FMT")
	tt.Frame()
	tt.Frame()
	// import "fmt"（没改，只算新的一边）、fmt.Errorf（加的）、fmt.Println（改的，旧的一边那行没有）
	if _, total, _ := g.FindStatus(); total != 3 {
		t.Fatalf("找 FMT 有 %d 处，应为 3", total)
	}
	for range 3 { // 绕一圈：总会跳到开头折起来的 import 那行
		tt.Key(0, wui.KeyEnter)
		tt.Frame()
	}
	if g.Rows()[0].Kind == ged.Fold && g.Rows()[0].N > 2 {
		t.Errorf("跳到折起来的 import 那行没展开：%v", g.Rows()[0])
	}
	cur, _, _ := g.FindStatus()
	if cur < 1 {
		t.Errorf("↩ 后没有当前那处：%d", cur)
	}
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		tt.SetScale(2)
		tt.SetTitleBar(shotBar)
		a.St.BottomBar = false
		tt.Frame()
		writeShot(t, tt, dir, "ged-find.png")
	}
	tt.Key(0, wui.KeyEscape)
	tt.Frame()
	if g.Find.Open {
		t.Error("Esc 没关查找条")
	}
}

// ged 的 Diff 右键开 Grove 的菜单（照 VS Code 的 Diff 编辑器）：对着按下的那一行、那处改动，复制这一行、暂存这一块，
// 旧的一边是“复制旧版本的这一行”；只看时没有剪切、粘贴；⇧F10 用键盘开同一份。
func TestGedMenu(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	var copied string
	keep := app.CopyText
	app.CopyText = func(s string) { copied = s }
	t.Cleanup(func() { app.CopyText = keep })
	old, new := gedFiles()
	repo := testutil.NewRepo(t, "gedmenu")
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
	h := g.Hunks()[0] // 第 8 步：n += 8 → n *= 8，翻倍
	row := -1
	for i, rw := range g.Rows() {
		if rw.New == h.NewStart {
			row = i
			break
		}
	}
	if row < 0 {
		t.Fatalf("找不到第一处改动的行：%+v", h)
	}
	g.ScrollY = 0
	tt.Frame()
	box, lh := g.Bounds(), g.RowHeight()
	y := box.Y + g.RowTop(row) + lh/2 - g.ScrollY
	newLine := strings.Split(new, "\n")[h.NewStart]
	oldLine := strings.Split(old, "\n")[h.OldStart]

	tt.RightClickAt(g.LineX(1)+30, y)
	tt.Frame()
	if o, ok := a.Menu.Owner.(menuOwner); !a.Menu.IsOpen() || !ok || o.Kind != "ged" {
		t.Fatalf("右键 ged 的 Diff 该开它的菜单：open=%v owner=%+v", a.Menu.IsOpen(), a.Menu.Owner)
	}
	if sp := g.MenuSpot(); sp.Side != 1 || sp.Line != h.NewStart || sp.Hunk != 0 || sp.New != h.NewStart {
		t.Fatalf("右键记下的位置 %+v，该是新的一边第 %d 行、第 0 处", sp, h.NewStart)
	}
	items := shownMenu(t, tt)
	for _, want := range []string{"复制", "暂存这一块", "丢弃这一块…", "复制这一行", "查找…", "下一处改动", "在编辑器中打开这一行", "统一 diff", "软换行", "全选"} {
		if !slices.Contains(items, want) {
			t.Errorf("菜单里没有 %q：%q", want, items)
		}
	}
	for _, no := range []string{"剪切", "粘贴", "取消暂存这一块"} {
		if slices.Contains(items, no) {
			t.Errorf("只看的未暂存 Diff 不该有 %q", no)
		}
	}
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		tt.SetScale(2)
		tt.SetTitleBar(shotBar)
		a.St.BottomBar = false
		tt.Frame()
		writeShot(t, tt, dir, "ged-menu.png")
		tt.SetScale(1)
		tt.Frame()
	}
	if err := chooseMenu(t, tt, "复制这一行"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if copied != newLine {
		t.Fatalf("复制这一行复制的是 %q，要 %q", copied, newLine)
	}

	// 旧的一边：复制旧版本的这一行
	box = g.Bounds()
	y = box.Y + g.RowTop(row) + lh/2 - g.ScrollY
	tt.RightClickAt(g.LineX(0)+30, y)
	tt.Frame()
	if err := chooseMenu(t, tt, "复制旧版本的这一行"); err != nil {
		t.Fatalf("%v：%q", err, shownMenu(t, tt))
	}
	tt.Frame()
	if copied != oldLine {
		t.Fatalf("复制旧版本的这一行复制的是 %q，要 %q", copied, oldLine)
	}

	// 菜单里暂存这一块：只暂存右键的那一块
	tt.RightClickAt(g.LineX(1)+30, y)
	tt.Frame()
	if err := chooseMenu(t, tt, "暂存这一块"); err != nil {
		t.Fatal(err)
	}
	wait("暂存第 1 处", func() bool { return idle(r)() && r.Status.Staged == 1 })
	cached := repo.Git("diff", "--cached", "--", "demo.go")
	if !strings.Contains(cached, "翻倍") || strings.Contains(cached, "Println") || strings.Contains(cached, "太大") {
		t.Fatalf("暂存区该只有翻倍那一块：\n%s", cached)
	}

	// ⇧F10：键盘开同一份菜单
	tt.Frame()
	tt.ClickAt(g.LineX(1)+30, box.Y+lh/2)
	tt.Frame()
	tt.Key(wui.Shift, wui.KeyF10)
	tt.Frame()
	if o, ok := a.Menu.Owner.(menuOwner); !a.Menu.IsOpen() || !ok || o.Kind != "ged" {
		t.Fatalf("⇧F10 该开 ged 的菜单：open=%v owner=%+v", a.Menu.IsOpen(), a.Menu.Owner)
	}
	if err := chooseMenu(t, tt, "查找…"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !g.Find.Open {
		t.Error("菜单里的查找…没开查找条")
	}
}

// 折叠条：行号那格里的箭头点一下往上 / 往下再露 20 行（开头那段只有往上的），点条上别处整段展开；右键折叠条的菜单也能露。
func TestGedFoldExpand(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	var o strings.Builder
	for i := range 130 {
		o.WriteString("line " + strconv.Itoa(i) + "\n")
	}
	old := o.String()
	new := strings.Replace(strings.Replace(old, "line 50\n", "line 50 改\n", 1), "line 100\n", "line 100 改\n", 1)
	repo := testutil.NewRepo(t, "gedfold")
	repo.Write("a.txt", old)
	repo.Commit("初始提交")
	repo.Write("a.txt", new)
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), t.TempDir())
	a.St.FullDiff, a.St.Unified = false, false
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "a.txt"}, true)
	wait("a.txt 的 ged diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && d.Ged != nil })
	tt.Frame()
	g := diffDoc(a, "").Ged
	rowY := func(i int) float32 { return g.Bounds().Y + g.RowTop(i) + g.RowHeight()/2 - g.ScrollY }
	foldAfter := func(old int) int { // 第一条从 old 往后的折叠
		for i, rw := range g.Rows() {
			if rw.Kind == ged.Fold && rw.Old >= old {
				return i
			}
		}
		return -1
	}
	// 开头那段 0..46：只有往上露的箭头
	if rw := g.Rows()[0]; rw.Kind != ged.Fold || rw.N != 47 {
		t.Fatalf("开头：%+v", rw)
	}
	if _, ok := g.FoldArrowX(0, -1); ok {
		t.Error("开头那段不该有往下露的箭头")
	}
	x, ok := g.FoldArrowX(0, 1)
	if !ok {
		t.Fatal("开头那段没有往上露的箭头")
	}
	tt.ClickAt(x, rowY(0))
	tt.Frame()
	if rw := g.Rows()[0]; rw.Kind != ged.Fold || rw.N != 27 || g.Rows()[1].Old != 27 {
		t.Fatalf("点往上露后：%+v %+v", rw, g.Rows()[1])
	}
	// 中间那段 54..96：两个箭头；悬停在往下露的上面截个图
	mid := foldAfter(54)
	if rw := g.Rows()[mid]; rw.Old != 54 || rw.N != 43 {
		t.Fatalf("中间那段：%+v", rw)
	}
	xd, ok1 := g.FoldArrowX(mid, -1)
	_, ok2 := g.FoldArrowX(mid, 1)
	if !ok1 || !ok2 {
		t.Fatalf("中间那段该有两个箭头：%v %v", ok1, ok2)
	}
	g.ScrollY = max(0, g.RowTop(mid)-4*g.RowHeight())
	tt.Frame()
	tt.Move(xd, rowY(mid))
	tt.Frame()
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		tt.SetScale(2)
		tt.SetTitleBar(shotBar)
		a.St.BottomBar = false
		tt.Frame()
		writeShot(t, tt, dir, "ged-fold.png")
		tt.SetScale(1)
		tt.Frame()
	}
	tt.ClickAt(xd, rowY(mid))
	tt.Frame()
	if rw := g.Rows()[foldAfter(54)]; rw.Old != 74 || rw.N != 23 {
		t.Fatalf("点往下露后：%+v", rw)
	}
	// 右键折叠条：往上展开 20 行，剩 3 行 → 再点条中间整段展开
	mid = foldAfter(54)
	g.ScrollY = max(0, g.RowTop(mid)-4*g.RowHeight())
	tt.Frame()
	tt.RightClickAt(g.LineX(1)+40, rowY(mid))
	tt.Frame()
	items := shownMenu(t, tt)
	for _, want := range []string{"展开这 23 行", "往下展开 20 行", "往上展开 20 行"} {
		if !slices.Contains(items, want) {
			t.Errorf("折叠条的菜单里没有 %q：%q", want, items)
		}
	}
	if err := chooseMenu(t, tt, "往上展开 20 行"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	mid = foldAfter(54)
	if rw := g.Rows()[mid]; rw.Kind != ged.Fold || rw.Old != 74 || rw.N != 3 {
		t.Fatalf("菜单往上露后：%+v", rw)
	}
	if _, ok := g.FoldArrowX(mid, 1); ok {
		t.Error("剩 3 行时不该画箭头")
	}
	tt.ClickAt(g.LineX(1)+40, rowY(mid))
	tt.Frame()
	if i := foldAfter(54); i >= 0 && g.Rows()[i].Old < 100 {
		t.Fatalf("点条中间该整段展开：%+v", g.Rows()[i])
	}
}

// 忽略行首行尾空白：菜单里勾上后只改了缩进的行不算改动，处数跟着变；暂存这一块只暂存看得见的那处。
func TestGedTrim(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	old := "package demo\n\nfunc f(x bool) int {\nif x {\nreturn 1\n}\nreturn 0\n}\n"
	new := "package demo\n\nfunc f(x bool) int {\n\tif x {\n\t\treturn 2\n\t}\n\treturn 0\n}\n"
	repo := testutil.NewRepo(t, "gedtrim")
	repo.Write("demo.go", old)
	repo.Commit("初始提交")
	repo.Write("demo.go", new)
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), t.TempDir())
	a.St.FullDiff, a.St.Unified, a.St.DiffTrim = true, false, false
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "demo.go"}, true)
	wait("demo.go 的 ged diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && d.Ged != nil })
	tt.Frame()
	g := diffDoc(a, "").Ged
	if h := g.Hunks(); len(h) != 1 || h[0].NewEnd-h[0].NewStart != 4 {
		t.Fatalf("不忽略空白时一处改 4 行：%+v", h)
	}
	y := g.Bounds().Y + g.RowHeight()/2
	tt.RightClickAt(g.LineX(1)+30, y)
	tt.Frame()
	if err := chooseMenu(t, tt, "忽略行首行尾空白"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.St.DiffTrim || !g.IgnoreTrim() {
		t.Fatal("菜单没打开忽略空白")
	}
	h := g.Hunks()
	if len(h) != 1 || h[0].NewStart != 4 || h[0].NewEnd != 5 {
		t.Fatalf("忽略空白后只剩 return 那一行：%+v", h)
	}
	g.GoToHunk(0)
	tt.Frame()
	tt.RightClickAt(g.LineX(1)+30, g.Bounds().Y+g.RowTop(4)+g.RowHeight()/2-g.ScrollY)
	tt.Frame()
	if err := chooseMenu(t, tt, "暂存这一块"); err != nil {
		t.Fatalf("%v：%q", err, shownMenu(t, tt))
	}
	wait("暂存", func() bool { return idle(r)() && r.Status.Staged == 1 })
	cached := repo.Git("diff", "--cached", "--", "demo.go")
	if !strings.Contains(cached, "+\t\treturn 2") || strings.Contains(cached, "+\tif x {") {
		t.Fatalf("暂存区该只有 return 那一行：\n%s", cached)
	}
	a.ToggleDiffTrim()
	tt.Frame()
	if g.IgnoreTrim() {
		t.Fatal("关掉忽略空白后 ged 还忽略着")
	}
}

// ⌃G 在 ged 的 Diff 上开命令面板的 ":"，回车跳到新的一边那一行：在折起来的段里时展开、滚进视图。
func TestGedGoToLine(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	var o strings.Builder
	for i := range 200 {
		o.WriteString("line " + strconv.Itoa(i) + "\n")
	}
	old := o.String()
	new := strings.Replace(old, "line 10\n", "line 10 改\n", 1)
	repo := testutil.NewRepo(t, "gedgoto")
	repo.Write("a.txt", old)
	repo.Commit("初始提交")
	repo.Write("a.txt", new)
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), t.TempDir())
	a.St.FullDiff, a.St.Unified = false, false
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "a.txt"}, true)
	wait("a.txt 的 ged diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && d.Ged != nil })
	tt.Frame()
	g := diffDoc(a, "").Ged
	a.Exec("edit.goToLine")
	if !a.Palette.Open || a.Palette.Text != ":" {
		t.Fatalf("⌃G 该开 \":\" 面板：open=%v text=%q", a.Palette.Open, a.Palette.Text)
	}
	a.Palette.Text = ":150"
	a.UpdatePalette()
	if len(a.Palette.Items) != 1 || !strings.Contains(a.Palette.Items[0].Detail, "共 201 行") {
		t.Fatalf("跳行的那一项：%+v %q", a.Palette.Items, a.Palette.Note)
	}
	a.AcceptPalette(0)
	tt.Frame()
	tt.Frame()
	row := -1
	for i, rw := range g.Rows() {
		if rw.Kind != ged.Fold && rw.New == 149 {
			row = i
		}
	}
	if row < 0 {
		t.Fatal("第 150 行还折着")
	}
	top := g.RowTop(row) - g.ScrollY
	if top < 0 || top > g.Bounds().H-g.RowHeight() {
		t.Fatalf("第 150 行不在视图里：top=%v h=%v", top, g.Bounds().H)
	}
	if t2 := g.Selected(); t2 != "" {
		t.Errorf("跳行后不该有选中的字：%q", t2)
	}
}

// TestGedOccurrences：选中一个词，两边别处一样的词都涂一层（VS Code 的 selection highlight）。
func TestGedOccurrences(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	repo := testutil.NewRepo(t, "gedoccur")
	repo.Write("a.go", "package a\n\nfunc total(xs []int) int {\n\tsum := 0\n\tfor _, x := range xs {\n\t\tsum += x\n\t}\n\treturn sum\n}\n")
	repo.Commit("初始提交")
	repo.Write("a.go", "package a\n\nfunc total(xs []int) int {\n\tsum := 0\n\tfor _, x := range xs {\n\t\tsum += x * 2\n\t}\n\treturn sum\n}\n")
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), t.TempDir())
	a.St.FullDiff, a.St.Unified = true, false
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "a.go"}, true)
	wait("a.go 的 ged diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && d.Ged != nil })
	tt.Frame()
	g := diffDoc(a, "").Ged
	g.Select(1, 3, 1, 3, 4) // 新的一边第 4 行的 sum
	tt.Frame()
	if got := g.Occurrences(1, 5); len(got) != 1 {
		t.Errorf("第 6 行的 sum 该涂：%v", got)
	}
	if got := g.Occurrences(0, 7); len(got) != 1 {
		t.Errorf("旧的一边第 8 行的 sum 该涂：%v", got)
	}
	if got := g.Occurrences(1, 3); len(got) != 0 {
		t.Errorf("选中的那处自己不算：%v", got)
	}
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		writeShot(t, tt, dir, "ged-occur.png")
	}
}

// TestGedSplit：拖并排中间那道线改两边的宽，双击还原；就地编辑时光标那一行垫一层（截图看）。
func TestGedSplit(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	repo := testutil.NewRepo(t, "gedsplit")
	repo.Write("demo.go", "package demo\n\nfunc f() int {\n\tx := 1\n\treturn\n}\n")
	repo.Commit("初始提交")
	repo.Write("demo.go", "package demo\n\nfunc f() int {\n\tx := 1\n\treturn x\n}\n")
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), t.TempDir())
	a.St.FullDiff, a.St.Unified = true, false
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "demo.go"}, true)
	wait("demo.go 的 ged diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && d.Ged != nil })
	tt.Frame()
	g := diffDoc(a, "").Ged
	box := g.Bounds()
	mid := box.X + float32(math.Floor(float64((box.W-1)/2))) + 0.5
	y := box.Y + 8*g.RowHeight()
	left0 := g.LineX(1)
	tt.Move(mid, y)
	tt.Frame()
	tt.Press(mid, y)
	tt.Move(box.X+box.W*0.3, y)
	tt.Release(box.X+box.W*0.3, y)
	tt.Frame()
	if f := g.Split(); f < 0.28 || f > 0.32 {
		t.Fatalf("拖到 30%% 处，左边占 %v", f)
	}
	if g.LineX(1) >= left0 {
		t.Errorf("右边的正文该往左挪：%v → %v", left0, g.LineX(1))
	}
	if g.Selected() != "" {
		t.Errorf("拖线不该选字：%q", g.Selected())
	}
	at := box.X + float32(math.Floor(float64((box.W-1)*g.Split()))) + 0.5
	tt.ClickAt(at, y)
	tt.ClickAt(at, y)
	tt.Frame()
	if g.Split() != 0 {
		t.Errorf("双击那道线该还原成一半：%v", g.Split())
	}

	d := diffDoc(a, "")
	a.ToggleDiffEdit(d)
	wait("Diff 能编辑", func() bool { tt.Frame(); d := diffDoc(a, ""); return d != nil && d.DiffSt != nil && d.DiffSt.Editable })
	g = diffDoc(a, "").Ged
	tt.ClickAt(g.LineX(1)+1, g.Bounds().Y+3.5*g.RowHeight()-g.ScrollY)
	tt.Frame()
	if l, _, ok := g.Caret(); !ok || l != 3 {
		t.Fatalf("点第 4 行放光标：%d %v", l, ok)
	}
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		writeShot(t, tt, dir, "ged-caretline.png")
	}
}

// 就地编辑时编辑器那套键在 ged 里也管用（和文件标签共用键位表）：⌥↓ 挪行、⌘/ 注释、⇧⌘K 删行、⌘] 缩进、
// 打 ( 补上 )；光标挨着括号时两个括号画框，空白按设置画。
func TestGedEditKeys(t *testing.T) {
	saved := gedOn
	gedOn = true
	t.Cleanup(func() { gedOn = saved })
	repo := testutil.NewRepo(t, "gedkeys")
	repo.Write("demo.go", "package demo\n\nfunc f() int {\n\ta := 1\n\tb := 2\n\treturn a\n}\n")
	repo.Commit("初始提交")
	repo.Write("demo.go", "package demo\n\nfunc f() int {\n\ta := 1\n\tb := 2\n\treturn a + b\n}\n")
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), t.TempDir())
	a.St.FullDiff, a.St.Unified = true, false
	a.St.Settings.Whitespace = "all"
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	a.OpenDiff(r, git.Change{Kind: git.Unstaged, Code: 'M', Path: "demo.go"}, true)
	wait("demo.go 的 ged diff", func() bool { d := diffDoc(a, ""); return d != nil && !d.Loading && d.Ged != nil })
	d := diffDoc(a, "")
	a.ToggleDiffEdit(d)
	wait("Diff 能编辑", func() bool { tt.Frame(); d := diffDoc(a, ""); return d != nil && d.DiffSt != nil && d.DiffSt.Editable })
	d = diffDoc(a, "")
	g, buf := d.Ged, d.DiffSt.New
	lines := func() []string { return strings.Split(buf.String(), "\n") }
	tt.ClickAt(g.LineX(1)+1, g.Bounds().Y+3.5*g.RowHeight()-g.ScrollY) // 第 4 行（a := 1）
	tt.Frame()
	if l, _, ok := g.Caret(); !ok || l != 3 {
		t.Fatalf("点第 4 行：%d %v", l, ok)
	}
	tt.Key(wui.Alt, wui.KeyDown)
	tt.Frame()
	if l := lines(); l[3] != "\tb := 2" || l[4] != "\ta := 1" {
		t.Fatalf("⌥↓ 挪行：%q", l[3:5])
	}
	tt.Key(wui.Cmd, wui.KeySlash)
	tt.Frame()
	if l := lines(); l[4] != "\t// a := 1" {
		t.Fatalf("⌘/ 注释：%q", l[4])
	}
	tt.Key(wui.Cmd, wui.KeySlash)
	tt.Key(wui.Cmd|wui.Shift, wui.KeyK)
	tt.Frame()
	if l := lines(); l[4] != "\treturn a + b" {
		t.Fatalf("⌘/ 再 ⇧⌘K：%q", l[3:6])
	}
	tt.Key(wui.Cmd, wui.KeyBracketRight)
	tt.Frame()
	if l := lines(); l[4] != "\t\treturn a + b" {
		t.Fatalf("⌘] 缩进：%q", l[4])
	}
	tt.Key(wui.Cmd, wui.KeyBracketLeft)
	tt.Key(wui.Cmd, wui.KeyRight)
	tt.TypeKey(wui.Shift, wui.Key9, "(")
	tt.Frame()
	if l := lines(); l[4] != "\treturn a + b()" {
		t.Fatalf("打 ( 补 )：%q", l[4])
	}
	if l, o, _ := g.Caret(); l != 4 || o != len("\treturn a + b(") {
		t.Fatalf("光标该在括号中间：%d:%d", l, o)
	}
	tt.Command("undo")
	tt.Frame()
	if l := lines(); l[4] != "\treturn a + b" {
		t.Fatalf("撤销补的括号：%q", l[4])
	}
	g.PlaceCaret(2, len("func f() int {"))
	tt.Frame()
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		tt.SetScale(2)
		tt.Frame()
		writeShot(t, tt, dir, "ged-keys.png")
		tt.SetScale(1)
		tt.Frame()
	}

	// ⌥⌘F：查找条下面开替换；↩ 换一处，“全部替换”换剩下的
	if c := app.Find("edit.replace"); !c.Enabled(a) {
		t.Fatal("就地编辑的 ged 上 ⌥⌘F 该能按")
	}
	a.Exec("edit.replace")
	tt.Frame()
	if !g.Find.Open || !g.Find.Replace {
		t.Fatal("⌥⌘F 没开替换")
	}
	tt.Type("b")
	tt.Frame()
	tt.Frame()
	g.Find.FocusWith = true
	tt.Frame()
	tt.Type("z")
	tt.Key(0, wui.KeyEnter)
	tt.Frame()
	if n := strings.Count(buf.String(), "z"); n != 1 {
		t.Fatalf("↩ 该换一处：%q", buf.String())
	}
	mustClick(t, tt, "全部替换（⌥⌘↩）")
	tt.Frame()
	if l := lines(); l[3] != "\tz := 2" || l[4] != "\treturn a + z" {
		t.Fatalf("全部替换：%q", l[3:5])
	}
	if dir := os.Getenv("WORKBENCH_SHOT"); dir != "" {
		tt.SetScale(2)
		tt.Frame()
		writeShot(t, tt, dir, "ged-replace.png")
		tt.SetScale(1)
		tt.Frame()
	}
}
