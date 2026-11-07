package ui

import (
	"path"
	"strconv"
	"strings"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/workspace"
)

// startPage 是没打开文件时编辑区的起始页。顶上工作区名和所有仓库的总况；下面三块：有改动的文件（按仓库，每个仓库最多 5 个，
// 点了开 Diff）、最近打开的文件（点了打开）、别的工作树（算过状态的写改动数、领先几个；点了开那个工作树）。哪块没东西就不出，
// 三块都没有时写一句怎么打开文件。最底下一行快速打开 / 打开文件。
//
// 行是按钮：Tab 进来，↑↓ 在行间走（a.StartFocus，FocusBind），⏎ / 空格打开。
func startPage(a *app.App, c *wui.Context) {
	t := c.Theme()
	startRows = 0
	if bareStartPage {
		wui.Box(c).Grow(1)
		return
	}
	page := wui.Scroll(c).Grow(1).MinHeight(0)
	page.Children(func() {
		wui.Row(c).Justify(wui.Center).Padding(36, 32, 28).Children(func() {
			wui.Column(c).Gap(22).MaxWidth(620).Grow(1).MinWidth(0).Children(func() {
				wui.Column(c).Gap(4).Children(func() {
					wui.Text(c, a.WsName()).FontSize(20).FontWeight(650).TextColor(t.Text).SingleLine()
					text, bad, _ := reposSummary(a.Repos)
					wui.Text(c, joinNonEmpty(" · ", text, bad)).FontSize(12).TextColor(t.TextMuted).SingleLine()
				})
				startChanges(a, c)
				startRecent(a, c)
				startWorktrees(a, c)
				if startRows == 0 {
					wui.Text(c, app.KeyHint("file.quickOpen", "在侧栏里点一个文件或一项变更，或 %s 按名字找", "在侧栏里点一个文件或一项变更，或用快速打开按名字找")).
						FontSize(12).TextColor(t.TextMuted)
				}
				wui.Row(c).Gap(14).AlignItems(wui.Center).Children(func() {
					startLink(c, iconSearch, app.KeyHint("file.quickOpen", "快速打开 %s", "快速打开"), func() { a.Exec("file.quickOpen") })
					startLink(c, iconFilePen, i18n.T("打开文件…"), a.ChooseFiles)
				})
			})
		})
	})
	n := startRows
	step := func(d int) func() {
		return func() {
			cur := wui.FocusedValue(c, &a.StartFocus)
			a.StartFocus = min(max(cur+d, 1), n)
		}
	}
	page.OnShortcut(0, wui.KeyDown, step(1)).OnShortcut(0, wui.KeyUp, step(-1))
}

// bareStartPage 只在测试里为 true（newUI 置上）：起始页画成空的，上面的文件名、"工作树"这些字不和侧栏、右栏里的撞
// （Tester 按字找元素）。测起始页的测试自己关掉它。
var bareStartPage bool

// startRows 是这一帧起始页上已经画了几行（startRow 数，FocusBind 的值）。
var startRows int

// startSection 是起始页的一块：灰的小标题，下面一块圆角底，里面是 body 画的行。
func startSection(c *wui.Context, title string, body func()) {
	t := c.Theme()
	wui.Column(c.Key(title)).Gap(6).Children(func() {
		wui.Text(c, title).FontSize(11.5).FontWeight(600).TextColor(t.TextMuted).Padding(0, 4)
		wui.Column(c).Radius(10).Padding(4).Background(toolFill(t, 0.035)).BorderWidth(1).BorderColor(t.Border.Alpha(0.55)).Children(body)
	})
}

// startRow 是起始页一块里的一行（高 28，悬停淡底、有键盘时深一点的底，点了 / ⏎ fn）。键由调用的一方给（c.Key）：
// 同一个文件可以既有暂存又有未暂存的改动，名字会重。
func startRow(a *app.App, c *wui.Context, label string, fn func(), body func()) {
	t := c.Theme()
	startRows++
	b := wui.ButtonBase(c).Height(28).Padding(0, 8).Gap(8).Radius(6).AlignItems(wui.Center).Cursor(wui.CursorPointer).Label(label).
		FocusBind(&a.StartFocus, startRows)
	switch {
	case b.Focused():
		b.Background(toolFill(t, 0.1))
	case b.Hovered():
		b.Background(toolFill(t, 0.07))
	}
	b.Children(body)
	if b.Clicked() {
		fn()
	}
}

func startChanges(a *app.App, c *wui.Context) {
	t := c.Theme()
	var dirty []*app.Repo
	for _, r := range a.Repos {
		if r.Loaded && r.Err == nil && r.Status.Count > 0 {
			dirty = append(dirty, r)
		}
	}
	if len(dirty) == 0 {
		return
	}
	startSection(c, i18n.T("有改动的文件"), func() {
		for _, r := range dirty {
			wui.Row(c.Key(r.Root)).Height(26).Padding(0, 8).Gap(6).AlignItems(wui.Center).Children(func() {
				repoDot(a, c, r, false)
				wui.Text(c, r.Name).FontSize(12.5).FontWeight(600).TextColor(t.Text).SingleLine()
				wui.Icon(c, iconBranch).FontSize(iconInline).TextColor(t.TextMuted)
				wui.Text(c, r.Status.Branch).FontSize(11.5).TextColor(t.TextMuted).SingleLine().Ellipsis("…").MinWidth(0)
				wui.Spacer(c).Grow(1)
				wui.Text(c, changeSummary(r.Status)).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
			})
			chs := r.Status.Changes
			for i, ch := range chs[:min(5, len(chs))] {
				startRow(a, c.Key(r.Root+strconv.Itoa(i)), r.Name+"/"+ch.Path, func() { a.OpenDiff(r, ch, true) }, func() {
					wui.Box(c).Width(14).Shrink(0)
					startFile(a, c, ch.Path)
					codeLetter(c, t, ch.Code, false)
				})
			}
			if n := len(chs) - 5; n > 0 {
				startRow(a, c.Key(r.Root+"more"), i18n.Tn("还有 %d 个…", n), func() { a.UseRepo(r); a.ShowSidebar(workspace.ModeGit) }, func() {
					wui.Box(c).Width(14).Shrink(0)
					wui.Text(c, i18n.Tn("还有 %d 个…", n)).FontSize(12).TextColor(t.TextMuted)
				})
			}
		}
	})
}

// startFile 是一行里的文件：图标、文件名，后面灰的目录。
func startFile(a *app.App, c *wui.Context, rel string) {
	t := c.Theme()
	fileGlyph(a, c, rel, false).Shrink(0)
	clean := strings.TrimSuffix(rel, "/") // 未跟踪的目录是 "store/"：名字写 "store/"，目录是它上面那层
	name := path.Base(clean)
	if clean != rel {
		name += "/"
	}
	wui.Text(c, name).FontSize(13).TextColor(t.Text).SingleLine().Shrink(0)
	if dir := path.Dir(clean); dir != "." {
		wui.Text(c, dir).FontSize(11.5).TextColor(t.TextMuted).SingleLine().Ellipsis("…").MinWidth(0)
	}
	wui.Spacer(c).Grow(1)
}

func startRecent(a *app.App, c *wui.Context) {
	t := c.Theme()
	type item struct {
		r   *app.Repo
		rel string
	}
	var items []item
	for _, abs := range a.St.Recent {
		if r, rel := a.RepoOfPath(abs); r != nil {
			items = append(items, item{r, rel})
			if len(items) == 6 {
				break
			}
		}
	}
	if len(items) == 0 {
		return
	}
	startSection(c, i18n.T("最近打开"), func() {
		for _, it := range items {
			startRow(a, c.Key(it.r.Root+"/"+it.rel), it.r.Name+"/"+it.rel, func() { a.OpenFile(it.r, it.rel, true) }, func() {
				startFile(a, c, it.rel)
				wui.Text(c, it.r.Name).FontSize(11.5).TextColor(t.TextMuted).SingleLine().Shrink(0)
			})
		}
	})
}

func startWorktrees(a *app.App, c *wui.Context) {
	t := c.Theme()
	type item struct {
		r *app.Repo
		w app.Worktree
	}
	var items []item
	for _, r := range a.Repos {
		for _, w := range r.Worktrees {
			if !w.Main && !w.Bare && !w.Prunable {
				items = append(items, item{r, w})
			}
		}
	}
	if len(items) == 0 {
		return
	}
	startSection(c, i18n.T("工作树"), func() {
		for _, it := range items {
			name := it.w.Branch
			if name == "" {
				name = path.Base(it.w.Path)
			}
			startRow(a, c.Key(it.w.Path), it.r.Name+" "+name, func() { a.OpenWorktree(it.w) }, func() {
				wui.Icon(c, iconBranch).FontSize(iconRow).TextColor(t.TextMuted).Shrink(0)
				wui.Text(c, name).FontSize(13).TextColor(t.Text).SingleLine().Shrink(0)
				wui.Text(c, it.r.Name).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
				wui.Spacer(c).Grow(1)
				var bits []string
				if x := it.r.WtInfo[it.w.Path]; x != nil && x.Err == nil {
					if x.Dirty > 0 {
						bits = append(bits, i18n.Tn("%d 个改动", x.Dirty))
					}
					if x.Ahead > 0 {
						bits = append(bits, i18n.Tn("领先 %d", x.Ahead))
					}
					if len(bits) == 0 {
						bits = append(bits, i18n.T("干净"))
					}
				}
				if len(bits) > 0 {
					wui.Text(c, strings.Join(bits, " · ")).FontSize(11.5).TextColor(t.TextMuted).SingleLine()
				}
			})
		}
	})
}

// startLink 是起始页底下的一个文字链接（图标 + 字，悬停加深）。
func startLink(c *wui.Context, ic *wui.SVG, label string, fn func()) {
	t := c.Theme()
	b := wui.ButtonBase(c.Key(label)).Gap(5).Padding(3, 6).Radius(6).AlignItems(wui.Center).Cursor(wui.CursorPointer).Label(label)
	col := t.TextMuted
	if b.Hovered() {
		col = t.Text
		b.Background(toolFill(t, 0.06))
	}
	b.Children(func() {
		wui.Icon(c, ic).FontSize(iconRow).TextColor(col)
		wui.Text(c, label).FontSize(12).TextColor(col)
	})
	if b.Clicked() {
		fn()
	}
}
