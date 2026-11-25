package ui

import (
	"testing"
	"time"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/testutil"
)

// 历史里的提交像 VS Code 那样：点标题展开、再点收起，展开别的不收这个；回车也是切；双击只切一次；
// 方向键只移选中，不展开。"所有仓库"视图里一样，几个仓库的提交可以同时开着。
func TestHistToggle(t *testing.T) {
	alpha, _, _, ws := testutil.ThreeRepos(t)
	base := time.Now().Add(-48 * time.Hour)
	for i, s := range []string{"first", "second", "third"} {
		alpha.Write(s+".txt", "x\n")
		alpha.CommitAt("feat: "+s, base.Add(time.Duration(i)*time.Hour))
	}
	a, tt, wait := newUI(t, ws, "")
	tt.SetSize(1100, 700)
	r := a.Repos[0]
	for _, x := range a.Repos {
		a.RefreshRepo(x, app.WhyManual)
	}
	wait("git status", loaded(a))
	a.UseRepo(r)
	if !a.St.RightBar {
		a.Exec("view.right")
	}
	wait("历史", func() bool { return r.Hist.Loaded && !r.Hist.Loading && len(r.Hist.Commits) == 3+3 })
	tt.Frame()
	h := &r.Hist
	hash := func(subject string) string {
		for _, c := range h.Commits {
			if c.Subject == subject {
				return c.Hash
			}
		}
		t.Fatalf("没有提交 %q", subject)
		return ""
	}
	read := func() { wait("读文件", func() bool { return len(h.Open) == 0 || !h.FilesLoading }) }
	third, second := hash("feat: third"), hash("feat: second")

	mustClickTitle(t, tt, "feat: third")
	read()
	if !h.Open[third] || h.Expanded != third {
		t.Fatalf("点标题没展开：%v", h.Open)
	}
	mustClickTitle(t, tt, "feat: second")
	read()
	if !h.Open[third] || !h.Open[second] {
		t.Fatalf("展开 second 把 third 收起了：%v", h.Open)
	}
	mustClickTitle(t, tt, "feat: third")
	if h.Open[third] || !h.Open[second] || h.Expanded == third {
		t.Fatalf("再点 third 的标题该只收起它：%v expanded=%.7s", h.Open, h.Expanded)
	}

	// 方向键只移选中：走过 first 不展开
	before := len(h.Open)
	for range 6 {
		tt.Key(0, wui.KeyDown)
		tt.Frame()
	}
	if len(h.Open) != before {
		t.Fatalf("方向键展开了提交：%v", h.Open)
	}
	// 回车切：选着 third 时开、再回车收
	for i, row := range app.HistRows(r) {
		if row.CI == 0 && row.File < 0 {
			h.Sel = i
		}
	}
	tt.Frame()
	tt.Key(0, wui.KeyEnter)
	tt.Frame()
	read()
	if !h.Open[third] {
		t.Fatalf("回车没展开选中的提交：%v", h.Open)
	}
	tt.Key(0, wui.KeyEnter)
	tt.Frame()
	if h.Open[third] {
		t.Fatalf("再回车没收起：%v", h.Open)
	}

	// 双击只切一次：第一下开，第二下（双击）和它带来的 Submitted 不再收回去
	tt.Advance(time.Second)
	x, y := titleAt(t, tt, "feat: third")
	tt.ClickAt(x, y)
	tt.Frame()
	tt.ClickAt(x, y)
	tt.Frame()
	read()
	if !h.Open[third] {
		t.Fatalf("双击该开着（只切一次）：%v", h.Open)
	}

	// 所有仓库：两个仓库的提交可以同时开着，再点收起
	beta := a.Repos[1]
	a.SetHistAll(true)
	wait("所有仓库的历史", func() bool { return !a.AllHistLoading() && a.AllHistDone() })
	tt.Frame()
	betaTop := beta.Hist.Commits[0]
	mustClickTitle(t, tt, betaTop.Subject)
	wait("beta 的文件", func() bool { return beta.Hist.Open[betaTop.Hash] && !beta.Hist.FilesLoading })
	if !h.Open[third] || !h.Open[second] {
		t.Errorf("展开 beta 的提交把 alpha 的收起了：%v", h.Open)
	}
	mustClickTitle(t, tt, betaTop.Subject)
	if beta.Hist.Open[betaTop.Hash] {
		t.Errorf("所有仓库视图里再点标题没收起")
	}
}

// titleAt 是历史里提交 s 那一格第一行（标题）的中间。
func titleAt(t *testing.T, tt *wui.Tester, s string) (float32, float32) {
	t.Helper()
	r, ok := tt.Find(s)
	if !ok {
		t.Fatalf("历史里没有提交 %q", s)
	}
	return r.X + r.W/2, r.Y + min(r.H, histRowH())/2
}
