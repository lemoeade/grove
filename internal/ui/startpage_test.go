package ui

import (
	"os"
	"path/filepath"
	"testing"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/testutil"
	"github.com/lemoeade/grove/internal/workspace"
)

// 起始页：编辑区什么都没开时出来，列着有改动的文件和最近打开的文件；↑↓ 在行间走（到头停住），⏎ 打开；
// 点一行有改动的文件开它的 Diff，起始页就让给它。
func TestStartPage(t *testing.T) {
	alpha := testutil.NewRepo(t, "alpha")
	alpha.Write("a.txt", "a\n")
	alpha.Write("b.txt", "b\n")
	alpha.Commit("初始提交")
	alpha.Write("a.txt", "a2\n")
	alpha.Write("new.txt", "n\n")
	a, tt, wait := newUI(t, testutil.Workspace(t, alpha.Root), "")
	a.St.BottomBar = false
	bareStartPage = false
	t.Cleanup(func() { bareStartPage = true })
	a.St.Recent = []string{filepath.Join(alpha.Root, "b.txt")}
	a.RefreshRepo(a.Repos[0], app.WhyManual)
	wait("仓库的状态", loaded(a))
	tt.Frame()
	for _, s := range []string{"有改动的文件", "最近打开", "打开文件…"} {
		if !tt.HasText(s) {
			t.Fatalf("起始页上没有 %q：%q", s, tt.Texts())
		}
	}
	chs := a.Repos[0].Status.Changes
	if len(chs) != 2 {
		t.Fatalf("改动 = %v，要两个", chs)
	}
	rows := []string{"alpha/" + chs[0].Path, "alpha/" + chs[1].Path, "alpha/b.txt"}

	a.StartFocus = 1
	tt.Frame()
	tt.Frame()
	focus := func(want int) {
		t.Helper()
		for i, r := range rows {
			if tt.Focused(r) != (i == want) {
				t.Fatalf("焦点应在第 %d 行 %q；%q 是 %v", want+1, rows[want], r, tt.Focused(r))
			}
		}
	}
	focus(0)
	for _, k := range []struct {
		key  wui.Key
		want int
	}{{wui.KeyDown, 1}, {wui.KeyDown, 2}, {wui.KeyDown, 2}, {wui.KeyUp, 1}, {wui.KeyDown, 2}} {
		tt.Key(0, k.key)
		tt.Frame()
		tt.Frame()
		focus(k.want)
	}
	tt.Key(0, wui.KeyEnter)
	tt.Frame()
	if d := a.CurrentDoc(); d == nil || d.Rel != "b.txt" {
		t.Fatalf("⏎ 应打开 b.txt：%+v", d)
	}
	if tt.HasText("最近打开") {
		t.Fatal("开了文件以后起始页应让给编辑器")
	}
}

// 点一行有改动的文件开它的 Diff。
func TestStartPageClickChange(t *testing.T) {
	alpha := testutil.NewRepo(t, "alpha")
	alpha.Write("a.txt", "a\n")
	alpha.Commit("初始提交")
	alpha.Write("a.txt", "a2\n")
	a, tt, wait := newUI(t, testutil.Workspace(t, alpha.Root), "")
	a.St.BottomBar = false
	bareStartPage = false
	t.Cleanup(func() { bareStartPage = true })
	a.RefreshRepo(a.Repos[0], app.WhyManual)
	wait("仓库的状态", loaded(a))
	tt.Frame()
	mustClick(t, tt, "alpha/a.txt")
	tt.Frame()
	if d := a.CurrentDoc(); d == nil || d.Rel != "a.txt" {
		t.Fatalf("点了应开 a.txt 的 Diff：%+v", d)
	}
	if tt.HasText("有改动的文件") {
		t.Fatal("开了 Diff 以后起始页应让给它")
	}
}

// TestScreenshotStartPage 是 start-page.png / start-page-dark.png（WORKBENCH_SHOT=目录 时才跑）：两个仓库都有改动，
// 最近打开过三个文件，api-server 有一个有改动的工作树；编辑区没开文件，画的是起始页。
func TestScreenshotStartPage(t *testing.T) {
	dir := os.Getenv("WORKBENCH_SHOT")
	if dir == "" {
		t.Skip("设置 WORKBENCH_SHOT=目录 才渲染截图")
	}
	api := testutil.NewRepo(t, "api-server")
	api.Write("go.mod", "module example.com/api\n\ngo 1.25\n")
	api.Write("main.go", "package main\n\nfunc main() {}\n")
	api.Write("handlers/user.go", "package handlers\n")
	api.Write("handlers/order.go", "package handlers\n")
	api.Commit("初始化服务骨架")
	wt := filepath.Join(filepath.Dir(api.Root), "api-server-search")
	api.Git("worktree", "add", "-q", "-b", "wt/search", wt)
	api.In(wt).Write("search/index.go", "package search\n")
	api.Write("handlers/order.go", "package handlers\n\n// ListOrders 列出订单。\n")
	api.Write("handlers/user.go", "package handlers\n\n// ListUsers 列出用户。\n")
	api.Write("store/cache.go", "package store\n")
	web := testutil.NewRepo(t, "web-client")
	web.Write("package.json", "{}\n")
	web.Write("src/App.tsx", "export {}\n")
	web.Commit("前端脚手架")
	web.Write("src/App.tsx", "export default function App() {}\n")

	a, tt, wait := newUI(t, testutil.Workspace(t, api.Root, web.Root), termState(t, false))
	bareStartPage = false
	t.Cleanup(func() { bareStartPage = true })
	showGroups(a)
	tt.SetScale(2)
	tt.SetTitleBar(shotBar)
	a.ShowSidebar(workspace.ModeGit)
	a.St.Recent = []string{filepath.Join(api.Root, "main.go"), filepath.Join(web.Root, "package.json"), filepath.Join(api.Root, "go.mod")}
	for _, r := range a.Repos {
		a.RefreshRepo(r, app.WhyManual)
	}
	wait("两个仓库的状态和工作树", func() bool { return loaded(a)() && len(a.Repos[0].Worktrees) >= 1 })
	a.ToggleWorktrees(a.Repos[0])
	wait("工作树的状态", func() bool { return a.Repos[0].WtInfo != nil && !a.Repos[0].WtLoading })
	writeShot(t, tt, dir, "start-page.png")
	tt.SetDark(true)
	writeShot(t, tt, dir, "start-page-dark.png")
}
