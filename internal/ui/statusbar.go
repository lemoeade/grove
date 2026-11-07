package ui

import (
	"fmt"
	"strings"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/git"
	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/workspace"
)

// 状态栏两头和上面那一列的内容对齐（不跟着侧栏、右栏的显隐变）：
//   - 左边第一个字和侧栏头部的字（"仓库"、"大纲"、Git 栏的标题，Padding 左 14）一条竖线；
//   - 右边最后一个字和右栏卡片里分段控件、搜索框、开关的右缘一条竖线：卡片外的缝、1 px 描边、卡片里的 8。
const (
	statusPadL = 14
	statusPadR = cardGap + 1 + 8
)

// statusBar 是窗口最底下一行。左：所有仓库的总况（reposSummary，点了打开 Git 栏）、检查的问题、刷新状态；
// 右：当前标签的行列、语言、换行、大小和状态。高 statusHeight 加上卡片底下那条缝（center 底下不留 padding）：
// 卡片底边到窗口底边整条带子是同色的，字在这一整条里居中（只在下面 22 里居中的话，上面空 11、下面空 6.5，看着偏下）。
func statusBar(a *app.App, c *wui.Context) {
	t := c.Theme()
	muted := t.TextMuted
	wui.Row(c).Height(statusHeight+cardGap).Padding(0, statusPadR, 0, statusPadL).Gap(12).AlignItems(wui.Center).Shrink(0).Children(func() {
		r := a.Current()
		wui.Row(c).Gap(12).AlignItems(wui.Center).Grow(1).MinWidth(0).Children(func() {
			if b := a.Batch; b != nil {
				batchStatus(a, c, b)
			}
			if r == nil {
				wui.Text(c, i18n.T("没有仓库")).FontSize(10.5).TextColor(muted).SingleLine()
				return
			}
			// 所有仓库的总况（当前仓库和分支在标题栏、右栏头上已经有了）：点了打开 Git 栏
			if text, bad, tip := reposSummary(a.Repos); text != "" || bad != "" {
				x := wui.Row(c).Gap(6).AlignItems(wui.Center).MinWidth(0).Shrink(1).Cursor(wui.CursorPointer).
					Label(i18n.Tf("所有仓库：%s", joinNonEmpty(" · ", text, bad))).Tooltip(tip)
				fg := muted
				if x.Hovered() {
					fg = t.Text
				}
				x.Children(func() {
					if text != "" {
						wui.Text(c, text).FontSize(10.5).FontWeight(500).TextColor(fg).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
					}
					if bad != "" {
						wui.Text(c, bad).FontSize(10.5).FontWeight(500).TextColor(t.Danger).SingleLine().Shrink(0)
					}
				})
				if x.Clicked() {
					a.ShowSidebar(workspace.ModeGit)
				}
			}
			problemStatus(a, c) // 检查报的问题（所有仓库一起）
			state := ""
			switch {
			case r.Busy != "":
				state = i18n.Tf("正在%s…", r.Busy)
			case r.Loading:
				state = i18n.T("正在刷新…")
			case r.Loaded:
				state = i18n.Tf("%s刷新", git.Ago(r.LoadedAt))
			}
			if state != "" {
				wui.Text(c, state).FontSize(10.5).TextColor(muted.Alpha(0.8)).SingleLine().Shrink(0).Tooltip(app.Tip(i18n.T("刷新"), "git.refresh"))
			}
		})
		if d := a.CurrentDoc(); d != nil {
			docStatus(a, c, d)
		}
		if a.Zoomed() {
			zoomStatus(a, c)
		}
	})
}

// zoomKeys 是放大 / 缩小 / 实际大小现在的键各一句（“⌘= 放大”），没绑键的那个不列。
func zoomKeys() []string {
	var keys []string
	for _, z := range []struct{ id, what string }{{"view.zoomIn", "%s 放大"}, {"view.zoomOut", "%s 缩小"}, {"view.zoomReset", "%s 回到 100%%"}} {
		if k := app.KeyText(z.id); k != "" {
			keys = append(keys, i18n.Tf(z.what, k))
		}
	}
	return keys
}

// zoomTip 是界面缩放的工具提示。
func zoomTip() string {
	if keys := zoomKeys(); len(keys) > 0 {
		return i18n.Tf("界面缩放：点开换一档；%s", i18n.List(keys))
	}
	return i18n.T("界面缩放：点开换一档")
}

// zoomStatus 是状态栏最右边的界面缩放（不是 100% 时才有）：点开是缩放的几档和“实际大小”。
func zoomStatus(a *app.App, c *wui.Context) {
	t := c.Theme()
	z := a.St.Settings.ZoomPercent()
	x := wui.Text(c, fmt.Sprintf("%d%%", z)).FontSize(10.5).FontWeight(500).TextColor(t.TextMuted).SingleLine().Shrink(0).
		Label(i18n.Tf("界面缩放 %d%%", z)).Tooltip(zoomTip())
	if x.Hovered() {
		x.TextColor(t.Text)
	}
	menuButton(a, c, x, menuOwner{"zoom", ""}, func(m menu) {
		z := a.St.Settings.ZoomPercent() // 每帧重建：按现在的
		for _, s := range workspace.ZoomSteps {
			if m.Item(fmt.Sprintf("%d%%", s)).Checked(s == z).Chosen() {
				a.SetZoom(s)
			}
		}
		m.Separator()
		if shortcut(m.Item(i18n.T("实际大小")), "view.zoomReset").Chosen() {
			a.ZoomReset()
		}
	})
}

// reposSummary 是状态栏左边所有仓库的总况：text 如 "4 个仓库 · 2 个有改动 · 1 个要推送 · infra 合并中"，
// 只有一个仓库时是它的 changeSummary（再加上合并中之类）；bad 是读取失败的（红字，"1 个读取失败"），
// tip 是每个仓库一行的明细。不是 git 仓库的文件夹不算；一个都没有时三个都空。
func reposSummary(repos []*app.Repo) (text, bad, tip string) {
	var rs []*app.Repo
	for _, r := range repos {
		if !r.Plain {
			rs = append(rs, r)
		}
	}
	if len(rs) == 0 {
		return "", "", ""
	}
	var parts, ongoing, lines []string
	changed, ahead, failed, loaded := 0, 0, 0, 0
	for _, r := range rs {
		line := r.Name + "  "
		switch {
		case r.Err != nil:
			failed++
			line += i18n.T("git status 失败")
		case !r.Loaded:
			line += i18n.T("读取中…")
		default:
			loaded++
			if r.Status.Count > 0 {
				changed++
			}
			if r.Status.Ahead > 0 {
				ahead++
			}
			line += changeSummary(r.Status)
			if s := app.UpDown(r.Status); s != "" {
				line += " · " + s
			}
			if op := r.Extra.Ongoing; op != git.NoOp {
				ongoing = append(ongoing, i18n.Tf("%s %s中", r.Name, op.String()))
				line += " · " + i18n.Tf("%s中", op.String())
			}
		}
		lines = append(lines, line)
	}
	if len(rs) == 1 {
		r := rs[0]
		if r.Loaded && r.Err == nil {
			parts = append(parts, changeSummary(r.Status))
			if op := r.Extra.Ongoing; op != git.NoOp {
				parts = append(parts, i18n.Tf("%s中", op.String()))
			}
		}
	} else {
		parts = append(parts, i18n.Tn("%d 个仓库", len(rs)))
		switch {
		case changed > 0:
			parts = append(parts, i18n.Tn("%d 个有改动", changed))
		case loaded == len(rs):
			parts = append(parts, i18n.T("都干净"))
		}
		if ahead > 0 {
			parts = append(parts, i18n.Tn("%d 个要推送", ahead))
		}
		if len(ongoing) > 2 {
			ongoing = append(ongoing[:2], i18n.Tn("另 %d 个进行中", len(ongoing)-2))
		}
		parts = append(parts, ongoing...)
	}
	if failed > 0 {
		bad = i18n.Tn("%d 个读取失败", failed)
	}
	return strings.Join(parts, " · "), bad, strings.Join(lines, "\n") + "\n" + app.Tip(i18n.T("点了打开 Git 栏"), "view.git")
}

// joinNonEmpty 用 sep 接起 parts 里不空的几段。
func joinNonEmpty(sep string, parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// changeSummary 是 "冲突 1 · 已暂存 2 · 未暂存 4"（未暂存含未跟踪的，和 Git 栏的分组一样），没有改动时是 "工作区干净"。
func changeSummary(st git.Status) string {
	var parts []string
	for _, p := range []struct {
		name string
		n    int
	}{{i18n.T("冲突"), st.Conflicts}, {i18n.T("已暂存"), st.Staged}, {i18n.T("未暂存"), st.Unstaged + st.Untracked}} {
		if p.n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", p.name, p.n))
		}
	}
	if len(parts) == 0 {
		return i18n.T("工作区干净")
	}
	return strings.Join(parts, " · ")
}
