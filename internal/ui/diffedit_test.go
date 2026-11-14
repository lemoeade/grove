package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/testutil"
)

// Diff 里就地编辑：未暂存的 Diff 标签条上有铅笔开关，点了新的一边可以直接打字（J / K 是字，不再跳改动），
// 标签条上出保存、状态栏写没保存，⌘S 写回文件；再点铅笔退回只看。已暂存的 Diff 上没有这个开关。
func TestDiffEditUI(t *testing.T) {
	m := testutil.NewRepo(t, "proj")
	m.Write("a.txt", "一\n二\n三\n")
	m.Write("b.txt", "甲\n")
	m.Commit("初始")
	m.Write("a.txt", "一\n二改\n三\n")
	m.Write("b.txt", "甲\n乙\n")
	m.Git("add", "b.txt")
	ws := filepath.Join(t.TempDir(), "d.code-workspace")
	os.WriteFile(ws, []byte(`{"folders":[{"path":"`+m.Root+`"}]}`), 0o644)
	a, tt, wait := newUI(t, ws, t.TempDir())
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyOpen)
	wait("读状态", idle(r))
	const on, off = "就地编辑：直接改工作区那一边，保存写回文件", "退出编辑，只看"

	st, _ := r.ChangeOf("b.txt")
	ds := a.OpenDiff(r, st, true)
	wait("读 b.txt 的 diff", func() bool { return !ds.Loading && ds.DiffSt != nil })
	tt.Frame()
	if found(tt, on) {
		t.Fatal("已暂存的 Diff 上不该有就地编辑")
	}

	ch, _ := r.ChangeOf("a.txt")
	d := a.OpenDiff(r, ch, true)
	wait("读 a.txt 的 diff", func() bool { return !d.Loading && d.DiffSt != nil })
	tt.Frame()
	if !tt.HasText("只读 · ") {
		t.Errorf("没编辑时状态栏该写只读：%q", tt.Texts())
	}
	if err := tt.Click(on); err != nil {
		t.Fatalf("Diff 标签条上没有就地编辑：%v", err)
	}
	tt.Frame()
	wait("文件标签读完", func() bool { return d.EditDoc != nil && d.EditDoc.Buf != nil && !d.EditDoc.Loading })
	tt.Frame()
	if a.CurrentDoc() != d || !d.DiffSt.Editable || d.DiffSt.New != d.EditDoc.Buf {
		t.Fatalf("就地编辑：cur=%v editable=%v", a.CurrentDoc() == d, d.DiffSt.Editable)
	}
	if !found(tt, off) || found(tt, on) {
		t.Error("按钮该换成退出编辑")
	}
	if !tt.HasText("就地编辑 · ") {
		t.Errorf("状态栏该写就地编辑：%q", tt.Texts())
	}
	if found(tt, "暂存这一块") {
		t.Error("就地编辑时不该有块级的暂存")
	}
	shot := diffEditShot(t, tt)
	shot("diff-edit")

	// 点进新的一边打字：j、k 是字
	focusNewSide(t, tt)
	tt.Type("jk")
	tt.Frame()
	f := d.EditDoc
	if got := f.Buf.String(); !strings.Contains(got, "jk") || !f.Modified() {
		t.Fatalf("j、k 该打进文件：%q modified=%v", got, f.Modified())
	}
	tt.Frame()
	if !found(tt, "保存（写回磁盘）") && !tt.HasText("就地编辑 · 没保存") {
		t.Errorf("改了以后标签条该有保存、状态栏写没保存：%q", tt.Texts())
	}
	if !tt.HasText("就地编辑 · 没保存") {
		t.Errorf("状态栏该写没保存：%q", tt.Texts())
	}
	shot("diff-edit-dirty")
	tt.Key(wui.Cmd, wui.KeyS)
	wait("保存", func() bool { return !f.Modified() })
	b, _ := os.ReadFile(filepath.Join(m.Root, "a.txt"))
	if !strings.Contains(string(b), "jk") {
		t.Fatalf("⌘S 该写回文件：%q", b)
	}

	// 再点铅笔：退回只看，文件标签还在
	tt.Frame()
	if err := tt.Click(off); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if d.EditDoc != nil || d.DiffSt.Editable || !found(tt, on) {
		t.Fatalf("退出编辑：edit=%v editable=%v", d.EditDoc != nil, d.DiffSt.Editable)
	}
	if a.CurrentDoc() != d {
		t.Error("退出编辑后还该在 Diff 上")
	}
}

// diffEditShot 在 WORKBENCH_SHOT=目录 时存浅色、深色各一张。
func diffEditShot(t *testing.T, tt *wui.Tester) func(name string) {
	return func(name string) {
		dir := os.Getenv("WORKBENCH_SHOT")
		if dir == "" {
			return
		}
		for _, dark := range []bool{false, true} {
			tt.SetDark(dark)
			tt.Frame()
			tt.Frame()
			suffix := "-light.png"
			if dark {
				suffix = "-dark.png"
			}
			savePNG(t, filepath.Join(dir, name+suffix), tt.Image())
		}
		tt.SetDark(false)
		tt.Frame()
	}
}

// found 是这一帧有元素显示或标着 s。
func found(tt *wui.Tester, s string) bool { _, ok := tt.Find(s); return ok }
