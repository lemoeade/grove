package ui

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/git"
	"github.com/lemoeade/grove/internal/testutil"
)

// 正文拆成一条条：列表项、段落各一条，折行接回去，末尾的尾注不要。
func TestBodyItems(t *testing.T) {
	for _, c := range []struct {
		body string
		want []string
	}{
		{"", nil},
		{"Co-Authored-By: Claude <x@y>", nil},
		{"一段话\n接着这段\n\n第二段", []string{"一段话 接着这段", "第二段"}},
		{"- 一\n  接着一\n* 二\n\nCo-Authored-By: Claude <x@y>\nSigned-off-by: A <a@b>\n", []string{"- 一 接着一", "* 二"}},
		{"说明\n- 一\n- 二", []string{"说明", "- 一", "- 二"}},
		{"Co-Authored-By: 写在中间的不算\n\n结尾", []string{"Co-Authored-By: 写在中间的不算", "结尾"}},
	} {
		if got := bodyItems(c.body); !slices.Equal(got, c.want) {
			t.Errorf("bodyItems(%q) = %q，想要 %q", c.body, got, c.want)
		}
	}
}

// 收起放不放得下：多于三条放不下；一条有三行的地方；英文比汉字窄；右栏宽了放得多。
func TestBodyFits(t *testing.T) {
	han := func(n int) string { return strings.Repeat("字", n) }
	for _, c := range []struct {
		items []string
		w     float32
		want  bool
	}{
		{[]string{"一", "二", "三"}, 340, true},
		{[]string{"一", "二", "三", "四"}, 340, false},
		{[]string{han(30), "二"}, 340, false},
		{[]string{han(30)}, 340, true},
		{[]string{han(80)}, 340, false},
		{[]string{strings.Repeat("a", 30), "二"}, 340, true},
		{[]string{han(30), "二"}, 600, true},
	} {
		if got := bodyFits(c.items, c.w); got != c.want {
			t.Errorf("bodyFits(%q, %v) = %v", c.items, c.w, got)
		}
	}
}

// 同名的远程分支折进本地分支；没有本地分支的远程分支和标签照旧。
func TestFoldRefs(t *testing.T) {
	r := &app.Repo{}
	r.Extra.Remotes = []git.Remote{{Name: "origin"}, {Name: "up"}}
	got := foldRefs(r, []string{"main", "origin/main", "up/main", "tag: v1", "origin/dev", "dev2"})
	want := []commitRef{{"main", "origin/main\nup/main"}, {"tag: v1", ""}, {"origin/dev", ""}, {"dev2", ""}}
	if !slices.Equal(got, want) {
		t.Errorf("折叠后 %q，想要 %q", got, want)
	}
	if s := got[0].tip(); s != "main\norigin/main\nup/main" {
		t.Errorf("悬停 %q", s)
	}
}

// histUI 开一个带历史的界面，等历史读完。
func histUI(t *testing.T, repo *testutil.Repo, commits int) (*app.App, *wui.Tester, func(string, func() bool), *app.Repo) {
	t.Helper()
	a, tt, wait := newUI(t, testutil.Workspace(t, repo.Root), "")
	r := a.Repos[0]
	a.RefreshRepo(r, app.WhyManual)
	wait("git status", idle(r))
	if !a.St.RightBar {
		a.Exec("view.right")
	}
	wait("历史", func() bool { return r.Hist.Loaded && !r.Hist.Loading && len(r.Hist.Commits) == commits })
	tt.Frame()
	return a, tt, wait, r
}

// 日期段头滚动时钉在顶上，方向键跳过它，点它不选中。
func TestHistDayHeader(t *testing.T) {
	repo := testutil.NewRepo(t, "days")
	base := time.Date(2025, 3, 1, 9, 0, 0, 0, time.Local)
	for d := range 3 {
		for i := range 12 {
			msg := fmt.Sprintf("第 %d 天 %d", d, i)
			repo.Write("a.txt", msg+"\n")
			repo.CommitAt(msg, base.AddDate(0, 0, d).Add(time.Duration(i)*time.Minute))
		}
	}
	a, tt, wait, r := histUI(t, repo, 36)
	tt.SetSize(1100, 500)
	tt.Frame()
	h := &r.Hist
	rows := app.HistRows(r)
	var days int
	for _, row := range rows {
		if row.CI == app.DayRow {
			days++
		}
	}
	if days != 3 || rows[0] != (app.HistRow{CI: app.DayRow, File: 0}) {
		t.Fatalf("段头 %d 个，第一行 %+v", days, rows[0])
	}

	// 滚到第二天中间：它的段头钉在顶上，盖不住滚上来的那行
	ci := slices.IndexFunc(h.Commits, func(c git.Commit) bool { return c.Subject == "第 1 天 5" })
	i := slices.Index(rows, app.HistRow{CI: ci, File: -1})
	a.HistList.ScrollTo(i, wui.Start)
	tt.Frame()
	tt.Frame()
	head, ok := tt.Find(dayLabel(h.Commits[ci].Date, time.Now()))
	if !ok {
		t.Fatalf("段头没钉住；texts=%q", tt.Texts())
	}
	row, ok := tt.Find("第 1 天 5")
	if !ok {
		t.Fatalf("滚到的那行没显示；texts=%q", tt.Texts())
	}
	if row.Y < head.Y+head.H {
		t.Errorf("钉着的段头 %v 盖住了 %v", head, row)
	}

	// 点段头不选中
	sel := h.Sel
	tt.ClickAt(head.X+head.W/2, head.Y+head.H/2)
	tt.Frame()
	if rows := app.HistRows(r); h.Sel != sel {
		t.Errorf("点段头选中了 %d（%+v）", h.Sel, rows[h.Sel])
	}

	// 一天最后一个提交的文件行往下：跳过下一天的段头，落在提交上
	a.HistList.ScrollTo(0, wui.Start)
	tt.Frame()
	tt.Frame()
	mustClickRow(t, tt, "第 2 天 0")
	wait("展开", func() bool { return h.FilesFor != "" && !h.FilesLoading })
	tt.Frame()
	tt.Key(0, wui.KeyDown)
	tt.Frame()
	tt.Key(0, wui.KeyDown)
	tt.Frame()
	rows = app.HistRows(r)
	if h.Sel < 0 || h.Sel >= len(rows) || rows[h.Sel].CI < 0 || h.Commits[rows[h.Sel].CI].Subject != "第 1 天 11" {
		t.Errorf("方向键落在第 %d 行：%+v", h.Sel, rows)
	}
}

// 卡片：标题读全，正文去掉尾注、收起时最多三条，"展开全文"开合；窄右栏只留"更多"。
func TestHistCard(t *testing.T) {
	repo := testutil.NewRepo(t, "card")
	repo.Write("a.txt", "1\n")
	repo.Commit("初始提交")
	repo.Write("a.txt", "1\n2\n")
	repo.Commit("改了四件事\n\n- 第一件\n- 第二件\n- 第三件\n- 第四件\n\nCo-Authored-By: Claude <noreply@anthropic.com>")
	a, tt, wait, r := histUI(t, repo, 2)
	mustClickRow(t, tt, "改了四件事")
	wait("展开", func() bool { return r.Hist.FilesFor != "" && !r.Hist.FilesLoading })
	tt.Frame()
	mustText(t, tt, "第三件")
	if tt.HasText("第四件") || tt.HasText("Co-Authored-By: Claude <noreply@anthropic.com>") {
		t.Errorf("收起时不该有第四件和尾注：%q", tt.Texts())
	}
	if !strings.Contains(strings.Join(tt.Texts(), "\n"), "Claude · ") {
		t.Errorf("信息行没写 agent：%q", tt.Texts())
	}
	mustClickRow(t, tt, "展开全文")
	tt.Frame()
	mustText(t, tt, "第四件")
	mustClickRow(t, tt, "收起")
	tt.Frame()
	if tt.HasText("第四件") {
		t.Error("收起后还有第四件")
	}

	mustText(t, tt, "和 HEAD 比较")
	a.St.RightW = histNarrow - 1
	tt.Frame()
	tt.Frame()
	if tt.HasText("和 HEAD 比较") {
		t.Error("窄右栏不该有和 HEAD 比较的按钮")
	}
	mustText(t, tt, "更多")
}
