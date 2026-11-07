package ui

import (
	"slices"
	"strconv"
	"strings"
	"time"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/git"
	"github.com/lemoeade/grove/internal/i18n"
)

// 右栏历史的几何，单位 DIP。
const (
	histHeadH = 46 // 头部一行
	histBtn   = 26 // 头部的图标按钮见方
	histRowH  = 30 // 一个提交一行
	histDayH  = 26 // 日期段头
	histFileH = 24 // 卡片里的一个文件
	histDotD  = 7  // "所有仓库"每行左边的仓库圆点

	histNarrow = 300 // 右栏比这窄：头部不写分支名，卡片的按钮只留"更多"
)

// rightBar 是右栏：提交历史。头部一行是仓库名和分支（"所有仓库"时是层叠图标和"全部仓库"），右边是图标开关：
// 搜索（点开在下面展开搜索框）、所有分支、只看 agent 写的、所有仓库（记进 state.json），竖线后是关闭；
// 文件历史、工作树、比较的胶囊在头部下面。"当前仓库"工作区有改动时第一行是"未提交的改动"，然后一个提交一行、
// 按日期分段，段头滚动时钉在顶上；"所有仓库"先是有改动的仓库各一行，再是各仓库的提交按时间合并、每行左边一个仓库圆点。
// 选中的提交展开成一张卡片：标题读全、作者 / agent / 时间、分支、正文、短哈希、几个按钮，和它改的文件（点文件在主区开 Diff 标签）。
func rightBar(a *app.App, c *wui.Context) {
	r := a.Current()
	card(c, wui.Column(c).Width(a.St.RightW).Shrink(0), func() {
		t := c.Theme()
		histHeader(a, c, r)
		if a.LocalHistPath == "" {
			histSearch(a, c)
			if r != nil && !a.St.HistAll {
				histPills(a, c, r)
			}
		}
		switch {
		case a.LocalHistPath != "": // 本地历史的路径模式盖过下面几条提示
			localPathView(a, c)
		case r == nil && a.WsPath == "":
			message(c, iconHistory, i18n.T("没有打开的工作区"), "", t.TextMuted)
		case r == nil:
			message(c, iconHistory, i18n.T("没有仓库"), "", t.TextMuted)
		case r.Plain && !a.St.HistAll:
			message(c, iconFolder, i18n.T("这个文件夹不是 Git 仓库"), "", t.TextMuted)
		case a.St.HistAll:
			allHistory(a, c.Key("all"))
		default:
			history(a, c.Key("one"), r)
		}
	})
}

// histHeader 是右栏的头部：左边是看的是什么，右边一排 26 见方的图标开关（开着是灰底）和关闭。
// 右键头部能开关提交图。
func histHeader(a *app.App, c *wui.Context, r *app.Repo) {
	t := c.Theme()
	line := cardLine(t)
	head := wui.Row(c).Height(histHeadH).Padding(0, 8, 0, 14).Gap(4).AlignItems(wui.Center).Shrink(0).Label(i18n.T("历史")).
		Draw(func(p *wui.Painter, rc wui.Rect) {
			p.Fill(wui.Rect{X: rc.X, Y: rc.Y + rc.H - 1, W: rc.W, H: 1}, line, 0)
		})
	ctxMenu(a, c, head, menuOwner{"histHeader", ""}, func(m menu) {
		if m.Item(i18n.T("提交图")).Checked(!a.St.HistNoGraph).Chosen() {
			a.ToggleHistGraph()
		}
	})
	head.Children(func() {
		wui.Row(c).Gap(6).AlignItems(wui.Center).Grow(1).MinWidth(0).Children(func() { histTitle(a, c, r) })
		if a.St.HistAll && a.AllHistLoading() || !a.St.HistAll && r != nil && (r.Hist.Loading || r.Hist.FilesLoading) {
			wui.Spinner(c)
		}
		find := a.HistFind || a.HistQuery != ""
		if histToggle(c, "find", iconSearch, i18n.T("搜索历史"), i18n.T("在历史里找提交信息、作者或哈希"), find) {
			if find {
				closeHistSearch(a)
			} else {
				a.HistFind, a.HistFocus = true, true
			}
		}
		if histToggle(c, "branches", iconBranch, i18n.T("所有分支"), i18n.T("所有分支：也列出其他本地分支、远程分支和标签上的提交（否则只当前分支）"), a.St.HistBranches) {
			a.ToggleHistBranches()
		}
		if histToggle(c, "agent", iconSparkle, i18n.T("只看 agent 写的"), i18n.T("只看 agent 写的：正文里有编码 agent（Claude、Codex、Copilot、Gemini……）的 Co-Authored-By 的提交"), a.HistAgent) {
			a.ToggleHistAgent()
		}
		if histToggle(c, "all", iconLayers, i18n.T("所有仓库"), i18n.T("所有仓库：各仓库的提交按时间合并（否则只看当前仓库）"), a.St.HistAll) {
			a.SetHistAll(!a.St.HistAll)
		}
		wui.Box(c).Size(1, 16).Margin(0, 4).Background(line).Shrink(0)
		closeTip := app.Tip(i18n.T("关闭右栏"), "view.right")
		squareButton(c, iconClose, closeTip, closeTip, histBtn, iconRow, false, func() { a.Exec("view.right") })
	})
}

// histTitle 是头部左边：仓库名（14.5 半粗）加灰色的分支图标和分支名；看工作树 / 标签时是它们；"所有仓库"是层叠图标和"全部仓库"。
func histTitle(a *app.App, c *wui.Context, r *app.Repo) {
	t := c.Theme()
	name := func(s string) {
		wui.Text(c, s).FontSize(14.5).FontWeight(600).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
	}
	sub := func(ic *wui.SVG, s, tip string) {
		if a.St.RightW < histNarrow { // 看工作树 / 标签时下面的胶囊里还写着
			return
		}
		wui.Icon(c, ic).FontSize(iconRow).TextColor(t.TextMuted).Shrink(0)
		wui.Text(c, s).FontSize(12).TextColor(t.TextMuted).SingleLine().Ellipsis("…").MinWidth(0).Shrink(2).Tooltip(tip)
	}
	switch {
	case a.St.HistAll && a.LocalHistPath == "":
		wui.Icon(c, iconLayers).FontSize(iconRow).TextColor(t.TextMuted).Shrink(0)
		name(i18n.T("全部仓库"))
	case r == nil || a.LocalHistPath != "":
		name(i18n.T("历史"))
	default:
		name(r.Name)
		switch h := &r.Hist; {
		case h.RefKind == app.RefWorktree:
			sub(iconWorktree, i18n.Tf("%s（工作树）", h.RefName), h.RefPath)
		case h.RefKind == app.RefTag:
			sub(iconTag, i18n.Tf("%s（标签）", h.RefName), "")
		case r.Loaded && r.Err == nil && !r.Plain:
			sub(iconBranch, r.Status.Branch, "")
		}
	}
}

// histToggle 是头部的一个图标开关（histBtn 见方，开着灰底、图标字色）；读屏念"开 / 关"。返回点了。
func histToggle(c *wui.Context, key string, ic *wui.SVG, label, tip string, on bool) bool {
	v := on
	b := wui.ToggleBase(c.Key(key), &v).Size(histBtn, histBtn).Label(label)
	tooltip(c, b, tip)
	fg := toolStyle(c, b, on, false)
	b.Children(func() { wui.Icon(c, ic).FontSize(iconRow).TextColor(fg) })
	return b.Changed()
}

// closeHistSearch 收起历史的搜索框；里面有字就清掉、按不筛重读。
func closeHistSearch(a *app.App) {
	a.HistFind = false
	if a.HistQuery != "" {
		a.HistQuery = ""
		a.ReloadHistories()
	}
}

// histSearch 是头部下面展开的搜索框（点放大镜才有，有字时一直在）：Esc 收起，右边的 ? 是查询语法。
func histSearch(a *app.App, c *wui.Context) {
	if !a.HistFind && a.HistQuery == "" {
		return
	}
	wui.Row(c).Padding(6, 8, 2, 8).Gap(6).AlignItems(wui.Center).Shrink(0).Children(func() {
		f := wui.SearchField(c.Key("histq"), &a.HistQuery).Label(i18n.T("搜索提交")).Placeholder(i18n.T("提交信息、作者或哈希")).
			Tooltip(i18n.T("在历史里找提交信息或作者里有这些字的提交，或哈希以它开头的提交")).FontSize(12).Grow(1).MinWidth(0).Bind(&a.HistField)
		if f.Changed() {
			a.HistQueryChanged()
		}
		if f.Submitted() {
			a.ReloadHistories()
		}
		wui.Icon(c, iconQuestion).FontSize(iconRow).TextColor(c.Theme().TextMuted).Shrink(0).Tooltip(histSyntaxTip())
	})
	a.HistField.OnShortcut(c, 0, wui.KeyEscape, func() { closeHistSearch(a) })
	if a.HistFocus {
		a.HistFocus = false
		a.HistField.Focus(c)
	}
	if msg := a.HistQueryErr(); msg != "" {
		wui.Text(c, msg).FontSize(11).TextColor(c.Theme().Danger).Padding(0, 10, 2, 10).Shrink(0).SingleLine().Ellipsis("…").Tooltip(msg)
	}
}

// histPills 是看工作树 / 标签 / 文件历史 / 比较时头部下面的胶囊（× 去掉）。
func histPills(a *app.App, c *wui.Context, r *app.Repo) {
	if r.Hist.Path == "" && r.Hist.RefKind == app.RefNone && r.Hist.Compare == nil {
		return
	}
	wui.Row(c).Padding(6, 8, 2, 8).Gap(6).Shrink(0).Children(func() {
		switch h := &r.Hist; h.RefKind {
		case app.RefWorktree:
			filterPill(c, iconWorktree, i18n.Tf("工作树：%s", h.RefName), h.RefPath, i18n.T("回到这个仓库自己的历史"), func() { a.ClearRefHistory(r) })
		case app.RefTag:
			filterPill(c, iconTag, i18n.Tf("标签：%s", h.RefName), h.RefName+"  "+git.Short(h.Ref), i18n.T("回到这个仓库自己的历史"), func() { a.ClearRefHistory(r) })
		}
		if p := r.Hist.Path; p != "" {
			filterPill(c, iconFile, i18n.Tf("文件历史：%s", p), p, i18n.T("不再只看这个文件"), func() { a.ClearFileHistory(r) })
		}
		if cp := r.Hist.Compare; cp != nil {
			text := i18n.Tf("比较：%s → %s", cp.From.Short, cp.To.Short)
			if !cp.Loading {
				text = i18n.Tn("比较：%s → %s · %d 个文件", cp.From.Short, cp.To.Short, len(cp.Files))
			}
			filterPill(c, iconDiff, text, compareTip(cp), i18n.T("不再比较这两个提交"), func() { a.ClearCompare(r) })
		}
	})
}

// histSyntaxTip 是搜索框右边 ? 的悬停提示：查询语法。
func histSyntaxTip() string {
	return strings.Join([]string{
		i18n.T("裸词：提交信息或作者里有，或哈希开头"),
		i18n.T("“带 空格 的词”：引号里算一个词"),
		i18n.T("/正则/：提交信息或作者匹配这个正则"),
		i18n.T("author:名字（作者:）：作者名或邮箱里有，可写几个"),
		i18n.T("after:2026-10-01（之后:）：这天起；也认 2026-10、yesterday、“2 weeks ago”"),
		i18n.T("before:2026-10（之前:）：这天（这个月）之前"),
		i18n.T("path:internal/app（路径:）：动过这个目录或文件，可写几个"),
	}, "\n")
}

// compareTip 是比较的胶囊和比较段标题的悬停提示：起点和终点的短哈希、标题。
func compareTip(cp *app.Compare) string {
	return cp.From.Short + "  " + cp.From.Subject + "\n" + cp.To.Short + "  " + cp.To.Subject
}

// compareTitle 是比较段的标题："比较 abc1234 → def5678（3 个文件，+12 −4）"。
func compareTitle(cp *app.Compare) string {
	from, to := cp.From.Short, cp.To.Short
	switch add, del := lineTotals(cp.Files); {
	case cp.Loading:
		return i18n.Tf("比较 %s → %s（读取中…）", from, to)
	case len(cp.Files) == 0:
		return i18n.Tf("比较 %s → %s（没有不同）", from, to)
	case add+del == 0:
		return i18n.Tn("比较 %s → %s（%d 个文件）", from, to, len(cp.Files))
	default:
		return i18n.Tn("比较 %s → %s（%d 个文件，+%d −%d）", from, to, len(cp.Files), add, del)
	}
}

// filterPill 是右栏历史筛选条下面的一个条件胶囊（文件历史、工作树）：图标、文字（悬停 tip），× 是 clear（读屏念 closeLabel）。
// 灰胶囊、正文色的字，× 次级色。
func filterPill(c *wui.Context, ic *wui.SVG, text, tip, closeLabel string, clear func()) {
	t := c.Theme()
	wui.Row(c).Gap(4).AlignItems(wui.Center).Padding(1, 2, 1, 8).Radius(999).Background(quietFill(t)).MinWidth(0).
		Shrink(1).Children(func() {
		wui.Icon(c, ic).FontSize(iconInline).TextColor(t.Text)
		wui.Text(c, text).FontSize(11).TextColor(t.Text).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1).Tooltip(tip)
		b := wui.Box(c).Padding(2).Radius(999).Cursor(wui.CursorPointer).Label(closeLabel).Tooltip(closeLabel).Shrink(0)
		if b.Hovered() {
			b.Background(toolFill(t, 0.1))
		}
		b.Children(func() { wui.Icon(c, iconClose).FontSize(iconInline).TextColor(t.TextMuted) })
		if b.Clicked() {
			clear()
		}
	})
}

// history 是右栏的"当前仓库"视图。
func history(a *app.App, c *wui.Context, r *app.Repo) {
	t := c.Theme()
	h := &r.Hist
	if r.Loaded {
		a.EnsureHistory(r)
	}
	switch {
	case h.Err != nil:
		message(c, iconAlert, i18n.T("读历史失败"), h.Err.Error(), t.Danger)
		return
	case !h.Loaded:
		wui.Spacer(c).Grow(1)
		return
	}
	rows, cp := app.HistRows(r), h.Compare // 列表在帧之间也可能调 Key：用这一帧的比较段，别读 h.Compare
	a.HistList.Key = func(i int) any {
		row := rows[i]
		switch {
		case row.CI == app.CompareRow && row.File < 0:
			return "compare"
		case row.CI == app.CompareRow:
			return "compare:" + cp.Files[row.File].Path
		case row.CI == app.LocalRow:
			return h.Local[row.File].T
		case row.CI == app.DayRow:
			return "day:" + h.Commits[row.File].Hash
		case row.CI < 0:
			return "uncommitted"
		case row.File < 0:
			return h.Commits[row.CI].Hash
		}
		return h.Commits[row.CI].Hash + ":" + h.Files[row.File].Path
	}
	a.HistList.Label = func(i int) string {
		row := rows[i]
		switch {
		case row.CI == app.CompareRow && row.File < 0:
			return compareTitle(cp)
		case row.CI == app.CompareRow:
			return cp.Files[row.File].Path
		case row.CI == app.LocalRow:
			return app.LocalTitle(h.Local[row.File]) + " · " + app.LocalMeta(h.Local[row.File])
		case row.CI == app.DayRow:
			return groupLabel(h.Commits[row.File].Date, time.Now())
		case row.CI < 0:
			return i18n.T("未提交的改动")
		case row.File < 0:
			return h.Commits[row.CI].Subject
		}
		return h.Files[row.File].Path
	}
	a.HistList.Header = func(i int) bool { return rows[i].CI == app.DayRow }
	a.HistList.Selected = &h.Sel
	titleHover := false // 鼠标在比较段的标题上：回车 / 双击带来的 Submitted 不再切（点它时已经切过了）
	l := newListLook(a, c, 0)
	var list wui.Element
	l.build(c, func() {
		list = wui.List(c, &a.HistList, len(rows)).Grow(1).MinHeight(0).PaddingX(6).PaddingY(2).
			Rows(func(row wui.ListRow) {
				switch hr := rows[row.Index]; {
				case hr.CI == app.CompareRow:
					l.row(row.Context, row.Selected(), row.ListFocused(), func() {
						titleHover = compareRow(a, row, r, cp, hr) || titleHover
					})
				case hr.CI == app.LocalRow:
					l.row(row.Context, row.Selected(), row.ListFocused(), func() { localHistRow(a, row, r, rows, row.Index) })
				case hr.CI == app.DayRow:
					histDayRow(a, l, row, r, hr.File)
				case hr.CI < 0:
					l.row(row.Context, row.Selected(), row.ListFocused(), func() { uncommittedRow(a, row, r) })
				case hr.File >= 0:
					last := row.Index+1 >= len(rows) || rows[row.Index+1].CI != hr.CI || rows[row.Index+1].File < 0
					histFileRow(a, l, row, r, h.Files[hr.File], last, 4, func() { histPass(a, row.Context, r, hr.CI) })
				default:
					histCommitRow(a, l, row, r, hr.CI)
				}
			})
	})
	if list.Changed() {
		a.OnHistSelected(r)
	}
	if list.Submitted() {
		// 回车固定选中文件的 Diff 标签（Changed 可能刚展开了提交，行要重算）；比较段的标题展开 / 收起
		if rows := app.HistRows(r); h.Sel >= 0 && h.Sel < len(rows) {
			switch hr := rows[h.Sel]; {
			case hr.CI == app.CompareRow && hr.File < 0:
				if !titleHover {
					a.ToggleCompare(r)
				}
			case hr.CI == app.CompareRow:
				a.OpenDiff(r, h.Compare.Files[hr.File], true)
			case hr.CI == app.LocalRow:
				a.OpenLocalCompare(a.LocalAbs(r), h.Local[hr.File], true)
			case hr.CI == app.DayRow:
			case hr.File >= 0:
				a.OpenDiff(r, h.Files[hr.File], true)
			}
		}
	}
	switch {
	case len(h.Commits) == 0 && a.HistFiltered(r):
		wui.Text(c, i18n.T("没有对得上的提交")).FontSize(11).TextColor(t.TextMuted).Padding(6, 14).Shrink(0)
	case len(h.Commits) == 0:
		wui.Text(c, i18n.T("还没有提交")).FontSize(11).TextColor(t.TextMuted).Padding(6, 14).Shrink(0)
	}
	if !h.Done {
		moreRow(c, len(h.Commits), h.Loading, "", func() { a.LoadHistory(r, false) })
	}
}

// moreRow 是列表底下的"已显示 N 条 · 再读 200 条"。
func moreRow(c *wui.Context, shown int, loading bool, tip string, more func()) {
	t := c.Theme()
	wui.Row(c).Padding(6, 10).Gap(8).AlignItems(wui.Center).Shrink(0).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		wui.Textf(c, i18n.T("已显示 %d 条"), shown).FontSize(11).TextColor(t.TextMuted).Grow(1).Tooltip(tip)
		if linkButton(c, i18n.Tf("再读 %s 条", strconv.Itoa(git.LogPage)), "").Clicked() && !loading {
			more()
		}
	})
}

// histParts 建历史列表的一行：body 在选中的底里面（listLook.row），tail（展开的提交的卡片）在下面、底的外面。
// 展开着的提交不画选中的底：卡片就是它选中的样子。实心的选中（设置里关了淡底）由列表铺满整行，分不开。
func histParts(l *listLook, row wui.ListRow, open bool, body, tail func()) {
	c := row.Context
	sel := row.Selected() && !open
	if tail == nil {
		l.row(c, sel, row.ListFocused(), body)
		return
	}
	wui.Column(c).Shrink(0).Children(func() {
		l.row(c, sel, row.ListFocused(), body)
		withTheme(c, l.real, tail)
	})
}

// histGraphW 是这个仓库的历史左边图格的宽：画提交图按最宽的一行，画圆点一条泳道，都不画是 0。
func histGraphW(a *app.App, r *app.Repo) float32 {
	switch {
	case a.HistGraph(r):
		return graphWidth(r.Hist.GraphW)
	case a.HistDots(r):
		return graphWidth(1)
	}
	return 0
}

// histPass 是提交 ci 下面（卡片、文件行）左边的图格：这个提交连到下一个的线竖着穿过去。
func histPass(a *app.App, c *wui.Context, r *app.Repo, ci int) {
	h := &r.Hist
	switch w := histGraphW(a, r); {
	case a.HistGraph(r):
		dirty := [2]int{}
		dirty[0], dirty[1] = dirtyLine(r)
		passGraph(c, h, ci, dirty, false, w)
	case a.HistDots(r):
		_, down := h.DotLinks(ci)
		dotPass(c, down, w, false)
	}
}

// dayLabel 是日期段头的字："今天""昨天""11 月 3 日"，不是今年的带年份。
func dayLabel(d, now time.Time) string {
	d, now = d.Local(), now.Local()
	switch {
	case app.SameDay(d, now):
		return i18n.T("今天")
	case app.SameDay(d, now.AddDate(0, 0, -1)):
		return i18n.T("昨天")
	case d.Year() == now.Year():
		return i18n.Tf("%d 月 %d 日", int(d.Month()), d.Day())
	}
	return i18n.Tf("%d 年 %d 月 %d 日", d.Year(), int(d.Month()), d.Day())
}

// dayHead 是日期段头（高 26，11.5 半粗灰字）：列表的 Header，滚动时钉在顶上，所以有底色；
// graph 画左边的图格（线穿过去），可以为 nil。
func dayHead(c *wui.Context, text string, left float32, graph func()) {
	t := c.Theme()
	wui.Row(c).Height(histDayH).AlignItems(wui.Stretch).Shrink(0).Background(t.Background).Children(func() {
		if graph != nil {
			graph()
		}
		wui.Row(c).Padding(4, 8, 0, left).AlignItems(wui.Center).Grow(1).MinWidth(0).Children(func() {
			wui.Text(c, text).FontSize(11.5).FontWeight(600).TextColor(t.TextMuted).SingleLine()
		})
	})
}

// histLeft 是"当前仓库"视图里标题离图格多远（没有图格时离行左边多远）。
func histLeft(w float32) float32 {
	if w == 0 {
		return 10
	}
	return 4
}

// histDayRow 是"当前仓库"视图的日期段头，下面第一个提交是 ci；左边的图格接着上一行的线。
func histDayRow(a *app.App, l *listLook, row wui.ListRow, r *app.Repo, ci int) {
	c := row.Context
	h := &r.Hist
	w := histGraphW(a, r)
	withTheme(c, l.real, func() {
		dayHead(c, groupLabel(h.Commits[ci].Date, time.Now()), histLeft(w), func() {
			switch {
			case a.HistGraph(r):
				var dirty [2]int
				dirty[0], dirty[1] = dirtyLine(r)
				headGraph(c, h, ci, dirty, w)
			case a.HistDots(r):
				up, _ := h.DotLinks(ci)
				dotPass(c, up, w, false)
			}
		})
	})
}

// histCommitRow 是"当前仓库"视图的一个提交：一行 30 高；展开着标题读全，是下面卡片的第一行。
func histCommitRow(a *app.App, l *listLook, row wui.ListRow, r *app.Repo, ci int) {
	c := row.Context
	h := &r.Hist
	cm := h.Commits[ci]
	g, dots := a.HistGraph(r), a.HistDots(r)
	w := histGraphW(a, r)
	var dirty [2]int
	lane := -1
	if g {
		dirty[0], dirty[1] = dirtyLine(r)
		lane = h.Graph[ci].Lane
	}
	head, _ := histHead(r)
	left := histLeft(w)
	open := cm.Hash == h.Expanded
	var tail func()
	if open {
		tail = func() {
			wui.Row(c).AlignItems(wui.Stretch).Shrink(0).Children(func() {
				histPass(a, c, r, ci)
				commitCard(a, c, r, cm, left, false, lane)
			})
		}
	}
	histParts(l, row, open, func() {
		sel := onAccent(a, row.Selected())
		el := commitRowBox(c, open)
		ctxMenuRow(a, c, row, el, menuOwner{"commit", r.Root + "\x00" + cm.Hash}, func(m menu) { commitMenu(a, m, r, cm) })
		hover := el.Hovered()
		el.Children(func() {
			switch {
			case g:
				commitGraph(c, h, ci, head, dirty, sel)
			case dots:
				up, down := h.DotLinks(ci)
				dotGraph(c, cm, head, up, down, w, sel)
			}
			if open {
				commitTitle(c, l, cm, left)
				return
			}
			commitLine(a, c, r, cm, commitLook{sel: sel, hover: hover, left: left, lane: lane})
		})
	}, tail)
}

// commitRowBox 是提交那一行的外框：展开着标题可能折行，高度跟着字。
func commitRowBox(c *wui.Context, open bool) wui.Element {
	el := wui.Row(c).AlignItems(wui.Stretch)
	if open {
		return el.MinHeight(histRowH)
	}
	return el.Height(histRowH)
}

// commitTitle 是展开的提交的标题：卡片的第一行，读全。在选中的行里建，字色要写明（实心的选中默认是强调色上的字）。
func commitTitle(c *wui.Context, l *listLook, cm git.Commit, left float32) {
	t := l.real
	wui.Column(c).Margin(0, 8, 0, left).Padding(7, 10, 5, 10).Grow(1).MinWidth(0).Background(histCardBg(t)).
		BorderColor(cardLine(t)).BorderWidth(1, 1, 0, 1).Radius(10, 10, 0, 0).Children(func() {
		wui.Text(c, cm.Subject).FontSize(13).FontWeight(600).TextColor(t.Text).MinWidth(0)
	})
}

// commitLook 是 commitLine 怎么画一行提交。
type commitLook struct {
	sel, hover bool
	all        bool    // 所有仓库：左边仓库圆点
	repoName   bool    // 所有仓库：右边写仓库名（和上一行不是一个仓库时）
	left       float32 // 标题离行左边 / 图格多远
	lane       int     // 提交图里的泳道，没有提交图是 -1
}

// commitLine 是收起的提交那一行：（所有仓库时仓库圆点）标题（合并提交次要色），右边依次是分支 / 标签、
// 没推送的箭头、（所有仓库时仓库名）、时间（悬停时换成短哈希）。agent 写在展开的卡片里。
func commitLine(a *app.App, c *wui.Context, r *app.Repo, cm git.Commit, k commitLook) {
	t := c.Theme()
	muted := t.TextMuted
	if k.sel {
		muted = t.AccentText.Alpha(0.75)
	}
	wui.Row(c).Padding(0, 8, 0, k.left).Gap(6).AlignItems(wui.Center).Grow(1).MinWidth(0).Children(func() {
		if k.all {
			repoDot(a, c, r, k.sel)
		}
		s := wui.Text(c, cm.Subject).FontSize(13).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
		if len(cm.Parents) > 1 {
			s.TextColor(muted)
		}
		refChips(c, r, cm, k.lane, k.sel, muted)
		if r.Extra.Unpushed[cm.Hash] {
			col := t.Warning
			if k.sel {
				col = muted
			}
			unpushedMark(c, col)
		}
		if k.repoName {
			repoChip(a, c, r, k.sel)
		}
		wui.Row(c).MinWidth(48).Justify(wui.End).Shrink(0).Children(func() {
			if k.hover {
				wui.Text(c, cm.Short).Font(mono()).FontSize(11).TextColor(muted).SingleLine()
				return
			}
			wui.Text(c, rowWhen(cm.Date)).FontSize(11.5).TextColor(muted).SingleLine().
				Tooltip(cm.Date.Local().Format("2006-01-02 15:04"))
		})
	})
}

// unpushedMark 是没推送的箭头。
func unpushedMark(c *wui.Context, col wui.Color) {
	wui.Icon(c, iconArrowUp).FontSize(iconInline).TextColor(col).Shrink(0).Label(i18n.T("未推送")).Tooltip(i18n.T("还没推送到任何远程分支"))
}

// remoteBranch 是远程分支 ref（origin/main 这种，按已有的远程认）的分支名；不是远程分支为 false。
func remoteBranch(r *app.Repo, ref string) (string, bool) {
	for _, x := range r.Extra.Remotes {
		if name, ok := strings.CutPrefix(ref, x.Name+"/"); ok {
			return name, true
		}
	}
	return "", false
}

// commitRef 是提交上的一个分支 / 标签标记；twin 是折进这个本地分支的同名远程分支（几个时一行一个）。
type commitRef struct{ name, twin string }

// tip 是这个标记的悬停提示。
func (x commitRef) tip() string {
	if x.twin == "" {
		return x.name
	}
	return x.name + "\n" + x.twin
}

// foldRefs 把和本地分支同名的远程分支折进本地分支：main 和 origin/main 在同一个提交上只占一个胶囊。
func foldRefs(r *app.Repo, refs []string) []commitRef {
	out := make([]commitRef, 0, len(refs))
	twins := map[string][]string{}
	for _, ref := range refs {
		if name, ok := remoteBranch(r, ref); ok && slices.Contains(refs, name) {
			twins[name] = append(twins[name], ref)
			continue
		}
		out = append(out, commitRef{name: ref})
	}
	for i := range out {
		out[i].twin = strings.Join(twins[out[i].name], "\n")
	}
	return out
}

// refChips 是提交的分支 / 标签标记：最多两个，多的写 +N、名字在悬停提示里。
func refChips(c *wui.Context, r *app.Repo, cm git.Commit, lane int, onAccent bool, muted wui.Color) {
	refs := foldRefs(r, cm.Refs)
	for i, ref := range refs {
		if i >= 2 {
			var rest []string
			for _, x := range refs[2:] {
				rest = append(rest, x.tip())
			}
			wui.Textf(c, "+%d", len(refs)-2).FontSize(10.5).TextColor(muted).Shrink(0).Tooltip(strings.Join(rest, "\n"))
			break
		}
		refChip(c, r, ref, lane, onAccent)
	}
}

// refChip 是一个分支 / 标签标记：本地分支是灰底胶囊（分支图标加名字），当前分支（HEAD）强调色字配强调色 12% 的底，
// 同名的远程分支在同一个提交上时名字后面一朵云；远程分支只一个云朵图标（名字在悬停提示里）；标签是标签图标加名字。
// lane > 0（提交图里不在主线上）时胶囊的底和图标用泳道的颜色，一眼对得上是哪条线。
func refChip(c *wui.Context, r *app.Repo, ref commitRef, lane int, onAccent bool) {
	t := c.Theme()
	if _, ok := remoteBranch(r, ref.name); ok {
		col := t.TextMuted
		if onAccent {
			col = t.AccentText
		}
		wui.Icon(c, iconCloud).FontSize(iconInline).TextColor(col).Shrink(0).Tooltip(ref.name)
		return
	}
	name := ref.name
	bg, fg, ic := quietFill(t), t.TextMuted, iconBranch
	switch {
	case strings.HasPrefix(name, "tag: "):
		name, ic = strings.TrimPrefix(name, "tag: "), iconTag
	case r.Hist.RefKind == app.RefNone && name == r.Status.Branch:
		bg, fg = t.Accent.Alpha(0.12), t.Accent
	}
	icCol := fg
	switch {
	case onAccent:
		bg, fg, icCol = t.AccentText.Alpha(0.22), t.AccentText, t.AccentText
	case lane > 0:
		icCol = laneColor(t, lane, false)
		bg = icCol.Alpha(0.16)
	}
	wui.Row(c).Gap(3).AlignItems(wui.Center).Padding(1, 6).Radius(999).Background(bg).Shrink(1).MinWidth(0).Tooltip(ref.tip()).Children(func() {
		wui.Icon(c, ic).FontSize(iconInline).TextColor(icCol)
		wui.Text(c, name).FontSize(10.5).FontWeight(500).TextColor(fg).SingleLine().Ellipsis("…").MinWidth(0).MaxWidth(110)
		if ref.twin != "" {
			wui.Icon(c, iconCloud).FontSize(iconInline).TextColor(fg).Shrink(0)
		}
	})
}

// histCardBg 是展开的提交那张卡片的底：浅色白，深色 #2C2C30。
func histCardBg(t *wui.Theme) wui.Color {
	if t.Dark {
		return wui.Hex("#2c2c30")
	}
	return wui.Hex("#ffffff")
}

// commitCard 是展开的提交的卡片（commitTitle 是它的第一行；left 同 commitTitle，lane 同 commitLook）：一行作者 / agent / 时间
// 和分支、正文、一行短哈希（点了复制完整哈希）/ 文件数 / 增删行和按钮，文件读到了就一条细线，
// 下面的文件行（histFileRow）接着画卡片的两边和底。all 是"所有仓库"视图：信息行开头写仓库名。
func commitCard(a *app.App, c *wui.Context, r *app.Repo, cm git.Commit, left float32, all bool, lane int) {
	t := c.Theme()
	h := &r.Hist
	muted := t.TextMuted
	files := h.FilesFor == cm.Hash && len(h.Files) > 0
	el := wui.Column(c).Grow(1).MinWidth(0).Gap(6).Background(histCardBg(t)).BorderColor(cardLine(t))
	if files {
		el.Margin(0, 8, 0, left).Padding(0, 10, 0, 10).BorderWidth(0, 1, 0, 1)
	} else {
		el.Margin(0, 8, 8, left).Padding(0, 10, 8, 10).BorderWidth(0, 1, 1, 1).Radius(0, 0, 10, 10)
	}
	el.Children(func() {
		commitMeta(c, r, cm, all, lane)
		commitBody(a, c, r, cm)
		wui.Row(c).Gap(6).AlignItems(wui.Center).MinWidth(0).Children(func() {
			hash := wui.Row(c).AlignItems(wui.Center).Padding(2, 0).Shrink(0).
				Cursor(wui.CursorPointer).Label(i18n.T("复制提交哈希")).Tooltip(i18n.T("复制提交哈希") + "\n" + cm.Hash)
			hash.Children(func() { wui.Text(c, cm.Short).Font(mono()).FontSize(11).TextColor(muted).SingleLine() })
			if hash.Clicked() {
				copyText(cm.Hash)
				a.Toast = i18n.T("已复制提交哈希")
			}
			if files {
				add, del := lineTotals(h.Files)
				wui.Text(c, i18n.Tn("%d 个文件", len(h.Files))).FontSize(11).TextColor(muted).Shrink(0)
				if add > 0 {
					wui.Textf(c, "+%d", add).FontSize(11).TextColor(t.Success).Shrink(0)
				}
				if del > 0 {
					wui.Textf(c, "−%d", del).FontSize(11).TextColor(t.Danger).Shrink(0)
				}
			}
			wui.Spacer(c).Grow(1)
			wui.Row(c).AlignItems(wui.Center).Shrink(0).Children(func() {
				if a.St.RightW >= histNarrow { // 窄了放不下，它们都在"更多"里
					headCm, ok := histHeadCommit(r)
					same := ok && headCm.Hash == cm.Hash
					squareButton(c, iconDiff, i18n.T("和 HEAD 比较"), i18n.T("和 HEAD 比较：在历史上面一段列出这个提交到 HEAD 改了的文件"),
						24, iconRow, !ok || same, func() { a.CompareCommits(r, cm, headCm) })
					w := a.WebOf(r)
					web := i18n.T("在网页打开")
					if w.OK() {
						web = i18n.Tf("在 %s 打开", w.Site())
					}
					squareButton(c, iconExternal, web, web, 24, iconRow, !w.OK() || !r.Extra.Pushed(cm.Hash), func() { a.OpenWeb(w.Commit(cm.Hash)) })
				}
				more := squareButton(c, iconMore, i18n.T("更多"), i18n.T("更多：完整哈希、邮箱、时间、父提交和提交的操作"), 24, iconRow, false, func() {})
				menuButton(a, c, more, menuOwner{"commitMore", r.Root + "\x00" + cm.Hash}, func(m menu) { commitMoreMenu(a, m, r, cm) })
			})
		})
		switch {
		case h.FilesLoading:
			wui.Text(c, i18n.T("读取文件…")).FontSize(11).TextColor(muted)
		case h.FilesFor == cm.Hash && len(h.Files) == 0:
			wui.Text(c, i18n.T("没有改动文件")).FontSize(11).TextColor(muted)
		case files:
			wui.Box(c).Height(1).Margin(0, 0, 2, 0).Background(cardLine(t))
		}
	})
}

// commitMeta 是卡片的信息行："作者 · agent · 今天 08:32"（悬停是邮箱、agent 和完整时间），右边是分支 / 标签和没推送的箭头。
func commitMeta(c *wui.Context, r *app.Repo, cm git.Commit, all bool, lane int) {
	t := c.Theme()
	muted := t.TextMuted
	var meta, tip []string
	if all {
		meta = append(meta, r.Name)
	}
	meta = append(meta, cm.Author)
	if cm.Email != "" {
		tip = append(tip, cm.Email)
	}
	if cm.Agent != "" {
		meta = append(meta, cm.Agent)
		tip = append(tip, cm.Agent+" · Co-Authored-By")
	}
	d := cm.Date.Local()
	meta = append(meta, dayLabel(cm.Date, time.Now())+" "+d.Format("15:04"))
	tip = append(tip, d.Format("2006-01-02 15:04"))
	wui.Row(c).Gap(6).AlignItems(wui.Center).MinWidth(0).Children(func() {
		wui.Text(c, strings.Join(meta, " · ")).FontSize(11.5).TextColor(muted).SingleLine().Ellipsis("…").MinWidth(0).Shrink(1).
			Tooltip(strings.Join(tip, "\n"))
		wui.Spacer(c).Grow(1)
		wui.Row(c).Gap(4).AlignItems(wui.Center).Shrink(0).MaxWidth(150).Children(func() {
			refChips(c, r, cm, lane, false, muted)
			if r.Extra.Unpushed[cm.Hash] {
				unpushedMark(c, t.Warning)
			}
		})
	})
}

// bodyItems 把提交正文拆成一条条：一个"- "/"* "开头的列表项或一段是一条，折行的接回去；末尾的
// Co-Authored-By、Signed-off-by 不要（信息行写了 agent）。
func bodyItems(body string) []string {
	ls := strings.Split(strings.TrimSpace(body), "\n")
	for len(ls) > 0 {
		l := strings.ToLower(strings.TrimSpace(ls[len(ls)-1]))
		if l != "" && !strings.HasPrefix(l, "co-authored-by:") && !strings.HasPrefix(l, "signed-off-by:") {
			break
		}
		ls = ls[:len(ls)-1]
	}
	var items []string
	gap := true
	for _, l := range ls {
		l = strings.TrimSpace(l)
		switch {
		case l == "":
			gap = true
			continue
		case gap, strings.HasPrefix(l, "- "), strings.HasPrefix(l, "* "):
			items = append(items, l)
		default:
			items[len(items)-1] += " " + l
		}
		gap = false
	}
	return items
}

// bodyFits 是 items 收起时放不放得下：最多三条，一条时三行，别的一条一行。列表里量不了字，按字宽估：
// 一个汉字算 1，ASCII 算 0.55，右栏宽减去图格和卡片的边是能放的字数。
func bodyFits(items []string, rightW float32) bool {
	if len(items) > 3 {
		return false
	}
	room := (rightW - 70) / 12.5
	if len(items) == 1 {
		room *= 3
	}
	for _, it := range items {
		var w float32
		for _, ch := range it {
			if ch < 0x80 {
				w += 0.55
			} else {
				w++
			}
		}
		if w > room {
			return false
		}
	}
	return true
}

// commitBody 是卡片里的正文：列表项是悬挂缩进的小圆点。收起时最多三条，放不下有"展开全文"。
func commitBody(a *app.App, c *wui.Context, r *app.Repo, cm git.Commit) {
	items := bodyItems(cm.Body)
	if len(items) == 0 {
		return
	}
	t := c.Theme()
	muted := t.TextMuted
	open := r.Hist.BodyOpen == cm.Hash
	fits := bodyFits(items, a.St.RightW)
	shown, lines := items, 1
	if !open {
		shown = items[:min(len(items), 3)]
		if len(items) == 1 {
			lines = 3
		}
	}
	wui.Column(c).Gap(3).MinWidth(0).Children(func() {
		for _, it := range shown {
			wui.Row(c).Gap(6).AlignItems(wui.Start).MinWidth(0).Children(func() {
				text := it
				if strings.HasPrefix(it, "- ") || strings.HasPrefix(it, "* ") {
					text = it[2:]
					wui.Box(c).Width(3).Height(3).Margin(7, 0, 0, 1).Radius(2).Background(muted.Alpha(0.7)).Shrink(0)
				}
				b := wui.Text(c, text).FontSize(12.5).TextColor(muted).Grow(1).MinWidth(0)
				if !open {
					b.MaxLines(lines).Ellipsis("…")
				}
			})
		}
	})
	if fits {
		return
	}
	s := i18n.T("展开全文")
	if open {
		s = i18n.T("收起")
	}
	b := wui.ButtonBase(c).Label(s).AlignSelf(wui.Start).Shrink(0)
	b.Children(func() { wui.Text(c, s).FontSize(11.5).FontWeight(500).TextColor(t.Accent).SingleLine() })
	if b.Clicked() {
		a.ToggleCommitBody(r, cm)
	}
}

// histHeadCommit 是读到的历史里 HEAD 那个提交（看工作树 / 标签时是它们的），没读到为 false。
func histHeadCommit(r *app.Repo) (git.Commit, bool) {
	head, _ := histHead(r)
	if i := r.Hist.HeadIndex(head); i >= 0 {
		return r.Hist.Commits[i], true
	}
	return git.Commit{}, false
}

// commitMoreMenu 是卡片上"更多"的菜单：完整哈希、作者和邮箱、精确时间、父提交（点了复制），然后是提交的右键菜单。
func commitMoreMenu(a *app.App, m menu, r *app.Repo, cm git.Commit) {
	copyItem := func(label, text string) {
		if m.Item(label).Chosen() {
			copyText(text)
			a.Toast = i18n.T("已复制")
		}
	}
	copyItem(cm.Hash, cm.Hash)
	who := cm.Author
	if cm.Email != "" {
		who += " <" + cm.Email + ">"
	}
	copyItem(who, who)
	date := cm.Date.Local().Format("2006-01-02 15:04:05 -0700")
	copyItem(date, date)
	if len(cm.Parents) > 0 {
		ps := make([]string, len(cm.Parents))
		for i, p := range cm.Parents {
			ps[i] = git.Short(p)
		}
		copyItem(i18n.Tf("父提交 %s", strings.Join(ps, " ")), strings.Join(cm.Parents, " "))
	}
	m.Separator()
	headCm, ok := histHeadCommit(r)
	if m.Item(i18n.T("和 HEAD 比较")).Disabled(!ok || headCm.Hash == cm.Hash).Chosen() {
		a.CompareCommits(r, cm, headCm)
	}
	commitMenu(a, m, r, cm)
}

// histFileLabel 是历史里的一个文件：commitFileLabel 的小一号，行数不加粗，让文件名先被看到。
func histFileLabel(a *app.App, c *wui.Context, ch git.Change, sel bool, muted wui.Color) {
	changeLabel(a, c, ch, sel, muted, 12.5)
	lineCounts(c, ch, sel, muted, 400)
	codeLetter(c, c.Theme(), ch.Code, sel)
}

// histFileRow 是展开的提交改的一个文件：接着画卡片的两边（last 时还有底和下面的圆角），里面一行高 24：
// 状态徽标、文件名、灰色的目录、+/− 计数。graph 画左边的图格；left 同 commitCard。
func histFileRow(a *app.App, l *listLook, row wui.ListRow, r *app.Repo, ch git.Change, last bool, left float32, graph func()) {
	c := row.Context
	withTheme(c, l.real, func() {
		t := c.Theme()
		chRow := wui.Row(c).AlignItems(wui.Stretch).Shrink(0)
		ctxMenuRow(a, c, row, chRow, menuOwner{"change", r.Root + "\x00" + ch.Path}, func(m menu) { changeMenu(a, m, r, ch) })
		chRow.Children(func() {
			graph()
			slice := wui.Column(c).Grow(1).MinWidth(0).Background(histCardBg(t)).BorderColor(cardLine(t))
			if last {
				slice.Margin(0, 8, 8, left).Padding(0, 4, 4, 4).BorderWidth(0, 1, 1, 1).Radius(0, 0, 10, 10)
			} else {
				slice.Margin(0, 8, 0, left).Padding(0, 4).BorderWidth(0, 1, 0, 1)
			}
			slice.Children(func() {
				l.row(c, row.Selected(), row.ListFocused(), func() {
					sel := onAccent(a, row.Selected())
					muted := c.Theme().TextMuted
					if sel {
						muted = c.Theme().AccentText.Alpha(0.75)
					}
					el := wui.Row(c).Height(histFileH).Padding(0, 6).Gap(8).AlignItems(wui.Center)
					if el.RowDoubleClicked() {
						a.OpenDiff(r, ch, true)
					}
					el.Children(func() { histFileLabel(a, c, ch, sel, muted) })
				})
			})
		})
	})
}

// uncommittedRow 是历史列表的第一行：工作区里还没提交的改动，字加改动数；有提交图时左边是虚线圈，连到 HEAD。
// 工作区干净时没有这一行，看工作树 / 标签时还在（圈是空的）。
func uncommittedRow(a *app.App, row wui.ListRow, r *app.Repo) {
	c := row.Context
	t := c.Theme()
	sel := onAccent(a, row.Selected())
	g := a.HistGraph(r)
	el := wui.Row(c).Height(histRowH).Tooltip(app.Tip(i18n.T("未提交的改动：在侧栏的 Git 栏里看"), "view.git"))
	body := func() {
		_, n := histHead(r)
		ref := r.Hist.RefKind != app.RefNone
		wui.Text(c, i18n.T("未提交的改动")).FontSize(13).SingleLine().Shrink(0)
		switch {
		case !r.Loaded || n < 0:
		case n > 0:
			pill(c, strconv.Itoa(n), t.Warning, sel)
		default:
			wui.Icon(c, iconCheck).FontSize(iconInline).TextColor(t.TextMuted).Tooltip(i18n.T("工作区干净"))
		}
		wui.Spacer(c).Grow(1)
		if s := app.UpDown(r.Status); s != "" && !ref {
			muted := t.TextMuted
			if sel {
				muted = t.AccentText.Alpha(0.75)
			}
			wui.Text(c, s).FontSize(11).TextColor(muted).SingleLine().Shrink(0).Tooltip(i18n.T("领先 / 落后上游的提交数"))
		}
	}
	if g {
		var dirty [2]int
		dirty[0], dirty[1] = dirtyLine(r)
		el.AlignItems(wui.Stretch).Children(func() {
			uncommittedGraph(c, r, dirty, sel)
			wui.Row(c).Padding(0, 8, 0, 4).Gap(6).AlignItems(wui.Center).Grow(1).MinWidth(0).Children(body)
		})
		return
	}
	el.Padding(0, 8, 0, 10).Gap(6).AlignItems(wui.Center).Children(body)
}

// repoColor 是仓库的颜色（文件树、Git 栏的仓库行、"所有仓库"的历史、终端标签、起始页上的圆点和胶囊）：按仓库在工作区里的顺序
// 在灰蓝、灰绿、灰沙、灰紫、灰玫、灰青六个之间轮换。
func repoColor(a *app.App, t *wui.Theme, r *app.Repo) wui.Color {
	light := [...]string{"#6f86a0", "#7fa393", "#b79a73", "#9286a8", "#a7808f", "#6f9aa0"}
	dark := [...]string{"#8fa4bc", "#86b09d", "#c9ab82", "#a79cc0", "#c59aab", "#86b3b8"}
	i := max(0, slices.Index(a.Repos, r)) % len(light)
	if t.Dark {
		return wui.Hex(dark[i])
	}
	return wui.Hex(light[i])
}

// repoDot 是"所有仓库"视图每行左边 7 px 的仓库圆点（悬停是仓库的路径）。
func repoDot(a *app.App, c *wui.Context, r *app.Repo, onAccent bool) {
	t := c.Theme()
	col := repoColor(a, t, r)
	if onAccent {
		col = t.AccentText
	}
	wui.Box(c).Size(histDotD, histDotD).Radius(histDotD / 2).Background(col).Shrink(0).Tooltip(r.Name + "\n" + r.Root)
}

// allTitleLeft 是"所有仓库"视图里标题离行左边多远（左 padding 8、圆点、间距 6）：卡片和它对齐。
const allTitleLeft = 8 + histDotD + 6

// allHistory 是右栏的"所有仓库"视图。没有提交图；每行左边一个仓库圆点。
func allHistory(a *app.App, c *wui.Context) {
	t := c.Theme()
	a.EnsureAllHistory()
	rows := a.AllHistRows()
	shown := app.AllHistShown(rows)
	if len(rows) == 0 {
		switch {
		case a.AllHistLoading() || a.AnyLoading():
			wui.Spacer(c).Grow(1)
		case strings.TrimSpace(a.HistQuery) != "" || a.HistAgent:
			message(c, iconHistory, i18n.T("没有对得上的提交"), "", t.TextMuted)
		default:
			message(c, iconHistory, i18n.T("还没有提交"), "", t.TextMuted)
		}
		return
	}
	a.AllHist.Key = func(i int) any { return rows[i].Key() }
	a.AllHist.Header = func(i int) bool { return rows[i].Kind == app.AllHeader || rows[i].Kind == app.AllDay }
	a.AllHist.Label = func(i int) string {
		row := rows[i]
		switch row.Kind {
		case app.AllDirty:
			return i18n.Tf("%s 未提交的改动", row.Repo.Name)
		case app.AllCommit:
			return row.Repo.Name + " " + row.Commit().Subject
		case app.AllFile:
			return row.Repo.Hist.Files[row.File].Path
		case app.AllDay:
			return groupLabel(row.Commit().Date, time.Now())
		}
		return row.Title
	}
	a.AllHist.Selected = &a.HistAllSel
	l := newListLook(a, c, 0)
	var list wui.Element
	l.build(c, func() {
		list = wui.List(c, &a.AllHist, len(rows)).Grow(1).MinHeight(0).PaddingX(6).PaddingY(2).
			Rows(func(row wui.ListRow) { allHistRow(a, l, row, rows) })
	})
	if list.Changed() {
		a.OnAllHistSelected()
	}
	if list.Submitted() {
		if rows := a.AllHistRows(); a.HistAllSel >= 0 && a.HistAllSel < len(rows) && rows[a.HistAllSel].Kind == app.AllFile {
			row := rows[a.HistAllSel]
			a.OpenDiff(row.Repo, row.Repo.Hist.Files[row.File], true)
		}
	}
	if !a.AllHistDone() {
		moreRow(c, shown, a.AllHistLoading(), i18n.T("每个仓库各读一页再按时间合并；有仓库那一页读完了，后面的等再读一页才排得准"), a.LoadMoreAllHistory)
	}
}

// allHistRow 画"所有仓库"视图的一行（rows[row.Index]）。
func allHistRow(a *app.App, l *listLook, row wui.ListRow, rows []app.AllHistRow) {
	hr := rows[row.Index]
	c := row.Context
	r := hr.Repo
	switch hr.Kind {
	case app.AllHeader:
		withTheme(c, l.real, func() {
			t := c.Theme()
			if hr.Title != i18n.T("未提交的改动") { // "提交"段头：一条细线和上面那段分开
				wui.Box(c).Height(9).Background(t.Background).Draw(func(p *wui.Painter, rc wui.Rect) {
					p.Fill(wui.Rect{X: rc.X + 8, Y: rc.Y + 4, W: rc.W - 16, H: 1}, cardLine(t), 0)
				})
				return
			}
			wui.Row(c).Height(histDayH).Padding(0, 8).AlignItems(wui.Center).Background(t.Background).Children(func() {
				wui.Text(c, hr.Title).FontSize(11.5).FontWeight(600).TextColor(t.TextMuted).SingleLine()
			})
		})
	case app.AllDay:
		withTheme(c, l.real, func() { dayHead(c, groupLabel(hr.Commit().Date, time.Now()), 10, nil) })
	case app.AllDirty:
		l.row(c, row.Selected(), row.ListFocused(), func() {
			t := c.Theme()
			sel := onAccent(a, row.Selected())
			repoRow := wui.Row(c).Height(histRowH).Padding(0, 8).Gap(6).AlignItems(wui.Center).Tooltip(i18n.T("在侧栏的 Git 栏里看这个仓库"))
			ctxMenuRow(a, c, row, repoRow, menuOwner{"histRepo", r.Root}, func(m menu) { repoMenu(a, m, r, false) })
			repoRow.Children(func() {
				repoDot(a, c, r, sel)
				wui.Text(c, r.Name).FontSize(13).SingleLine().Ellipsis("…").Grow(1).MinWidth(0).Tooltip(changeSummary(r.Status))
				pill(c, strconv.Itoa(r.Status.Count), t.Warning, sel)
			})
		})
	case app.AllFile:
		last := row.Index+1 >= len(rows) || rows[row.Index+1].Kind != app.AllFile
		histFileRow(a, l, row, r, r.Hist.Files[hr.File], last, allTitleLeft, func() {})
	default:
		cm := hr.Commit()
		prev := -1
		for j := row.Index - 1; j >= 0; j-- {
			if rows[j].Kind == app.AllCommit {
				prev = j
				break
			}
		}
		open := cm.Hash == r.Hist.Expanded
		var tail func()
		if open {
			tail = func() { commitCard(a, c, r, cm, allTitleLeft, true, -1) }
		}
		histParts(l, row, open, func() {
			sel := onAccent(a, row.Selected())
			el := commitRowBox(c, open)
			ctxMenuRow(a, c, row, el, menuOwner{"commit", r.Root + "\x00" + cm.Hash}, func(m menu) { commitMenu(a, m, r, cm) })
			hover := el.Hovered()
			el.Children(func() {
				if open { // 圆点还在左边，标题和下面的卡片对齐
					wui.Row(c).Padding(0, 0, 0, 8).Gap(6).AlignItems(wui.Start).Grow(1).MinWidth(0).Children(func() {
						wui.Box(c).Margin(11.5, 0, 0, 0).Shrink(0).Children(func() { repoDot(a, c, r, sel) })
						commitTitle(c, l, cm, 0)
					})
					return
				}
				commitLine(a, c, r, cm, commitLook{sel: sel, hover: hover, all: true, repoName: prev < 0 || rows[prev].Repo != r, left: 8, lane: -1})
			})
		}, tail)
	}
}

// compareRow 是比较段的一行：标题（箭头、点它展开 / 收起，右键能清掉）或两个提交之间改了的一个文件。
// 有提交图时左边留一格同宽的空白，只让"未提交的改动"连到 HEAD 的虚线穿过去。返回鼠标在标题上。
func compareRow(a *app.App, row wui.ListRow, r *app.Repo, cp *app.Compare, hr app.HistRow) bool {
	c := row.Context
	t := c.Theme()
	sel := onAccent(a, row.Selected())
	muted, mark := t.TextMuted, t.TextMuted // mark：短哈希、计数、diff 图标（不上强调色）
	if sel {
		muted, mark = t.AccentText.Alpha(0.75), t.AccentText
	}
	g := a.HistGraph(r)
	var dirty [2]int
	if g {
		dirty[0], dirty[1] = dirtyLine(r)
	}
	line := func(el wui.Element, pad [4]float32, body func()) {
		if !g {
			el.Padding(pad[0], pad[1], pad[2], pad[3]).Gap(8).AlignItems(wui.Center).Children(body)
			return
		}
		el.AlignItems(wui.Stretch).Children(func() {
			gapGraph(c, &r.Hist, dirty, sel)
			wui.Row(c).Padding(pad[0], pad[1], pad[2], pad[3]-4).Gap(8).AlignItems(wui.Center).Grow(1).MinWidth(0).Children(body)
		})
	}
	if hr.File >= 0 {
		ch := cp.Files[hr.File]
		el := wui.Row(c)
		ctxMenuRow(a, c, row, el, menuOwner{"change", r.Root + "\x00" + ch.Path}, func(m menu) { changeMenu(a, m, r, ch) })
		if el.RowDoubleClicked() {
			a.OpenDiff(r, ch, true)
		}
		line(el, [4]float32{3, 8, 3, 28}, func() { histFileLabel(a, c, ch, sel, muted) })
		return false
	}
	toggle := func() { a.ToggleCompare(r) }
	el := wui.Row(c).Tooltip(compareTip(cp))
	ctxMenuRow(a, c, row, el, menuOwner{"compare", r.Root}, func(m menu) {
		if r.Hist.Compare != cp { // 菜单开着时比较段没了 / 换了：什么也不加，菜单关掉
			return
		}
		look := i18n.T("收起")
		if !cp.Open {
			look = i18n.T("展开")
		}
		if m.Item(look).Chosen() {
			toggle()
		}
		if m.Item(i18n.T("不再比较")).Chosen() {
			a.ClearCompare(r)
		}
	})
	if el.Clicked() {
		toggle()
	}
	line(el, [4]float32{6, 8, 6, 8}, func() {
		chevron(c, iconRow, cp.Open, cp.From.Short+" → "+cp.To.Short, muted, toggle)
		wui.Icon(c, iconDiff).FontSize(iconRow).TextColor(mark)
		wui.Text(c, compareTitle(cp)).FontSize(12.5).FontWeight(500).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
	})
	return el.Hovered()
}

// groupLabel 是日期段头的字（app.Bucket 的段）："今天""昨天""本周""上周""本月"，再早的"10 月"，不是今年的"2025 年 10 月"。
func groupLabel(d, now time.Time) string {
	b, start := app.Bucket(d, now)
	switch b {
	case 0:
		return i18n.T("今天")
	case 1:
		return i18n.T("昨天")
	case 2:
		return i18n.T("本周")
	case 3:
		return i18n.T("上周")
	case 4:
		return i18n.T("本月")
	}
	switch {
	case i18n.Lang() == "en" && start.Year() == now.Year():
		return start.Format("January")
	case i18n.Lang() == "en":
		return start.Format("January 2006")
	case start.Year() == now.Year():
		return i18n.Tf("%d 月", int(start.Month()))
	}
	return i18n.Tf("%d 年 %d 月", start.Year(), int(start.Month()))
}

// weekdays 是"周日"…"周六"（time.Weekday 的顺序）。
var weekdays = [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

// rowWhen 是提交行行尾的时间，配着段头（groupLabel）看：今天、昨天"15:04"，本周、上周"周四 15:04"，再早的"10-25"
// （悬停有完整的日期时间）。
func rowWhen(d time.Time) string {
	d = d.Local()
	switch b, _ := app.Bucket(d, time.Now()); {
	case b <= 1:
		return d.Format("15:04")
	case b <= 3:
		return i18n.T(weekdays[d.Weekday()]) + " " + d.Format("15:04")
	}
	return d.Format("01-02")
}

// repoChip 是"所有仓库"历史行尾的仓库名：仓库颜色（repoColor，和左边的圆点、文件树里的一样）的淡底胶囊、深一点的字。
func repoChip(a *app.App, c *wui.Context, r *app.Repo, sel bool) {
	t := c.Theme()
	col := repoColor(a, t, r)
	fg, bg := col.MixOklab(t.Text, 0.35), col.Alpha(0.22)
	if sel {
		fg, bg = t.AccentText, t.AccentText.Alpha(0.18)
	}
	wui.Row(c).Padding(1, 6).Radius(999).Background(bg).Shrink(0).MaxWidth(96).Tooltip(r.Root).Children(func() {
		wui.Text(c, r.Name).FontSize(11).FontWeight(500).TextColor(fg).SingleLine().Ellipsis("…")
	})
}
