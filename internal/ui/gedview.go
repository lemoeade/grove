package ui

import (
	"os"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/ged"
	"github.com/lemoeade/grove/internal/git"
	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/outline"
)

// gedOn：设了 WORKBENCH_GED 时，Diff（并排、统一、就地编辑）用 Grove 自己画的（internal/ged），
// 不用 mygo 的 editor.DiffView。试做阶段，默认不开。
var gedOn = os.Getenv("WORKBENCH_GED") != ""

// useGed 是这个 Diff 这一帧用不用 ged：开了、是文字的 Diff（和 docs.Doc.UsingGed 一样，只是那个还要建过 Ged）。
func useGed(d *docs.Doc) bool {
	return gedOn && d.DiffSt != nil
}

type gedKey struct{ d *docs.Doc }

// gedText 是 ged.Diff.Set 的 key：两边的缓冲区和版本，一样就不重新对齐。
type gedText struct {
	old, new       *editor.Buffer
	oldVer, newVer uint64
}

// gedDiff 建 ged 的并排 Diff：内容取 DiffSt 的两个缓冲区，颜色和 mygo 那个 Diff 一样（diffTint、diffAddedLine……）。
func gedDiff(a *app.App, c *wui.Context, d *docs.Doc) wui.Element {
	st := d.DiffSt
	if d.Ged == nil {
		d.Ged = ged.NewDiff()
	}
	lang := st.New.Language()
	if lang == nil {
		lang = st.Old.Language()
	}
	g := d.Ged
	g.Set(gedText{st.Old, st.New, st.Old.Version(), st.New.Version()}, func() (o, n []string) {
		return bufLines(st.Old), bufLines(st.New)
	}, lang)
	g.SetFull(a.St.FullDiff)
	g.SetIgnoreTrim(a.St.DiffTrim)
	g.SetUnified(st.Layout == editor.DiffUnified) // diffLook 定的：设置里的统一，或者一边是空的（新文件、删掉的文件）
	switch f := d.GedFocus; {
	case f > 0:
		g.GoToHunk(g.HunkAtLine(1, f-1))
	case f < 0 && g.Refocus():
	case f < 0:
		g.GoToHunk(0)
	}
	d.GedFocus = 0
	s := gedStyle(a, c.Theme())
	s.Label = d.Where()
	if st.HunkActions { // diffContent 定的：未暂存 / 已暂存的改了的文件，不在就地编辑
		stage, unstage, discard := app.HunkOps(d)
		s.HunkBar = func(c *wui.Context, bar wui.Element, _ int, h ged.Hunk) {
			hunkActions(a, c, bar, d, editor.DiffHunk{OldStart: h.OldStart, OldEnd: h.OldEnd, NewStart: h.NewStart, NewEnd: h.NewEnd},
				stage, unstage, discard)
		}
	}
	if st.LineSelection { // 点行号选行（行级暂存）：和 mygo 的 Diff 用同一份选中的行，底下那栏、暂存选中的行照旧
		s.Lines = func() (side, start, end int, ok bool) {
			r, ok := st.SelectedLines()
			return int(r.Side), r.Start, r.End, ok
		}
		s.SetLines = func(side, start, end int, ok bool) {
			st.SetSelectedLines(editor.DiffLineRange{Side: editor.DiffSide(side), Start: start, End: end}, ok)
		}
		s.LineSel, s.LineMark = c.Theme().Accent.Alpha(0.2), c.Theme().Accent
	}
	s.FoldScope = func(old int) (mark, text string) { // 折叠条上写下面那处改动所在的函数，和 mygo 那个 Diff 的折叠条一样
		text, kind := scopeAt(d, old)
		if text != "" && kind != outline.Heading { // 标题那行自己带着 #
			mark = symbolGlyph(kind)
		}
		return mark, text
	}
	s.Copy = func(text string) { app.CopyText(text) }
	if st.Editable { // 就地编辑：新的一边就是文件标签的缓冲区（BindDiffEdit 接的），打字、撤销都在它上面
		s.Buf = st.New
		s.Paste = func() string { return app.ReadClipboard() }
		s.Bindings = app.DiffEditBindings() // 和文件标签的编辑器一张键位表（改过的键一起变）
		s.Indent = a.DocIndent(d)
	}
	s.Whitespace = a.St.Settings.Whitespace
	s.Wrap = a.WrapOf(d) // ⌥Z：diffLook 给 mygo 那个 Diff 的也是它
	s.FindBar = func(c *wui.Context, bar wui.Element) { gedFindBar(c, bar, g) }
	s.Keys = func(ev wui.InputEvent) bool { return gedKeys(a, d, ev) }
	owner := menuOwner{"ged", d.Where()}
	s.Menu = menuPointer(a, c, g.Bounds(), owner, func(m menu) { gedMenu(a, m, d) })
	el := ged.View(c, gedKey{d}, g, s)
	if el.FocusWithin() { // ⇧F10 / 菜单键：在光标（或当前那处改动）那一行下面开
		sp, x, y := g.KeySpot()
		b := g.Bounds()
		a.MenuKeyAt = &app.MenuKeyTarget{Owner: owner, X: b.X + x, Y: b.Y + y,
			Build: func(m app.MenuSink) { g.SetSpot(sp); gedMenu(a, m, d) }}
	}
	return el
}

// gedMenu 是 ged 的 Diff 的右键菜单，照 VS Code 的 Diff 编辑器：剪切 / 复制 / 粘贴；指针下那处改动的暂存 / 取消暂存 / 丢弃，
// 选中的行的；复制这一行；查找、上 / 下一处；在编辑器中打开这一行；统一、全文、软换行、就地编辑；全选；路径。
// 对着的是右键按下的那一处（ged.Diff.MenuSpot），不一定是当前标签，所以动作都按 d 做。
func gedMenu(a *app.App, m menu, d *docs.Doc) {
	g, st := d.Ged, d.DiffSt
	if g == nil || st == nil {
		return
	}
	sp := g.MenuSpot()
	cur := a.CurrentDoc() == d
	key := func(it menuItem, id string) menuItem {
		if cur {
			shortcut(it, id)
		}
		return it
	}
	_, _, caret := g.Caret()
	if g.CanEdit() { // 就地编辑：撤销、重做走编辑菜单那条路（ged 按连着打的一组撤）
		if m.Item(i18n.T("撤销")).Shortcut(wui.Super, wui.KeyZ).Chosen() {
			g.Run("undo")
		}
		if m.Item(i18n.T("重做")).Shortcut(wui.Super|wui.Shift, wui.KeyZ).Chosen() {
			g.Run("redo")
		}
		m.Separator()
	}
	if g.CanEdit() && m.Item(i18n.T("剪切")).Shortcut(wui.Super, wui.KeyX).Disabled(!g.HasSelection() || sp.Side != 1).Chosen() {
		g.Run("cut")
	}
	if m.Item(i18n.T("复制")).Shortcut(wui.Super, wui.KeyC).Disabled(!g.HasSelection()).Chosen() {
		g.Run("copy")
	}
	if g.CanEdit() && m.Item(i18n.T("粘贴")).Shortcut(wui.Super, wui.KeyV).Disabled(!caret && !(g.HasSelection() && sp.Side == 1)).Chosen() {
		g.Run("paste")
	}
	if n := g.FoldSize(sp.Fold); n > 0 { // 按在折叠条上：VS Code 那样整段展开，或往下 / 往上再露 20 行
		up, down := g.FoldDirs(sp.Fold)
		m.Separator()
		if m.Item(i18n.Tn("展开这 %d 行", n)).Chosen() {
			g.ExpandMore(sp.Fold, 0)
		}
		if n > ged.FoldStep+1 {
			if up && m.Item(i18n.Tf("往下展开 %d 行", ged.FoldStep)).Chosen() {
				g.ExpandMore(sp.Fold, -1)
			}
			if down && m.Item(i18n.Tf("往上展开 %d 行", ged.FoldStep)).Chosen() {
				g.ExpandMore(sp.Fold, 1)
			}
		}
	}
	if hs := g.Hunks(); st.HunkActions && sp.Hunk >= 0 && sp.Hunk < len(hs) {
		h := hs[sp.Hunk]
		eh := editor.DiffHunk{OldStart: h.OldStart, OldEnd: h.OldEnd, NewStart: h.NewStart, NewEnd: h.NewEnd}
		stage, unstage, discard := app.HunkOps(d)
		m.Separator()
		if stage && m.Item(i18n.T("暂存这一块")).Chosen() {
			a.StageHunk(d, eh)
		}
		if unstage && m.Item(i18n.T("取消暂存这一块")).Chosen() {
			a.UnstageHunk(d, eh)
		}
		if discard && m.Item(i18n.T("丢弃这一块…")).Danger().Chosen() {
			a.DiscardHunk(d, eh)
		}
	}
	if stage, unstage, discard := app.LinesOps(d); stage || unstage || discard {
		m.Separator()
		if stage && key(m.Item(i18n.T("暂存选中的行")), "diff.stageLines").Chosen() {
			a.StageLines(d)
		}
		if unstage && key(m.Item(i18n.T("取消暂存选中的行")), "diff.unstageLines").Chosen() {
			a.UnstageLines(d)
		}
		if discard && key(m.Item(i18n.T("丢弃选中的行…")), "diff.discardLines").Danger().Chosen() {
			a.DiscardLines(d)
		}
	}
	m.Separator()
	if t, ok := g.SpotLine(); ok {
		label := i18n.T("复制这一行")
		if sp.Side == 0 {
			label = i18n.T("复制旧版本的这一行")
		}
		if m.Item(label).Chosen() {
			app.CopyText(t)
		}
	}
	if key(m.Item(i18n.T("查找…")), "edit.find").Chosen() {
		g.OpenFind()
	}
	if key(m.Item(i18n.T("下一处改动")), "diff.nextHunk").Disabled(!app.CanGoToHunk(d, 1)).Chosen() {
		a.GoToHunk(d, 1)
	}
	if key(m.Item(i18n.T("上一处改动")), "diff.prevHunk").Disabled(!app.CanGoToHunk(d, -1)).Chosen() {
		a.GoToHunk(d, -1)
	}
	if r := a.RepoOf(d); r != nil && (d.Kind == docs.Diff && d.Change.Code != 'D' && d.Change.Kind != git.InCommit || d.Kind == docs.ReplacePreview) {
		if m.Item(i18n.T("在编辑器中打开这一行")).Chosen() {
			a.OpenAt(r, d.Rel, docs.Jump{Line: sp.New}, true)
		}
	}
	m.Separator()
	if key(m.Item(i18n.T("统一 diff")), "diff.unified").Checked(a.St.Unified).Disabled(oneSided(st)).Chosen() {
		a.ToggleLayout()
	}
	if key(m.Item(i18n.T("全文 diff")), "diff.full").Checked(a.St.FullDiff).Chosen() {
		a.ToggleScope()
	}
	if key(m.Item(i18n.T("忽略行首行尾空白")), "diff.ignoreTrim").Checked(a.St.DiffTrim).Chosen() {
		a.ToggleDiffTrim()
	}
	if key(m.Item(i18n.T("软换行")), "view.softWrap").Checked(a.WrapOf(d)).Chosen() {
		a.ToggleWrapOf(d)
	}
	if editing := app.DiffEditing(d); editing || a.CanEditDiff(d) {
		if key(m.Item(i18n.T("在 Diff 里就地编辑")), "diff.editInPlace").Checked(editing).Chosen() {
			a.ToggleDiffEdit(d)
		}
	}
	m.Separator()
	if m.Item(i18n.T("全选")).Shortcut(wui.Super, wui.KeyA).Chosen() {
		g.Run("selectAll")
	}
	pathItems(a, m, d)
}

// gedKeys 是 ged 的 Diff 有焦点时界面这边接的键：下一处 / 上一处改动（默认 J / K，按命令注册表里现在的键）；
// 查找条开着时 Esc 关掉它。⌘F / ⌘G 视图不接，走窗口上登记的 edit.find / edit.findNext。
func gedKeys(a *app.App, d *docs.Doc, ev wui.InputEvent) bool {
	mods := ev.Mods & (wui.Shift | wui.Ctrl | wui.Alt | wui.Super)
	if ev.Key == wui.KeyEscape && mods == 0 && d.Ged.Find.Open {
		d.Ged.CloseFind()
		return true
	}
	for _, k := range []struct {
		id  string
		dir int
	}{{"diff.nextHunk", 1}, {"diff.prevHunk", -1}} {
		if m, key, ok := app.KeyOf(k.id); ok && key == ev.Key && m == mods {
			a.GoToHunk(d, k.dir)
			return true
		}
	}
	return false
}

func bufLines(b *editor.Buffer) []string {
	out := make([]string, b.LineCount())
	for i := range out {
		out[i] = b.Line(i)
	}
	return out
}

// gedStyle 是 ged 的 Diff 按 Grove 的主题的样子。
func gedStyle(a *app.App, t *wui.Theme) ged.Style {
	ct := fullCodeTheme(t)
	ct.Background = diffBase(t)
	diffTint(ct, t)
	font := a.EditorFont()
	font.Size -= 0.5
	return ged.Style{
		Code:        ct,
		Font:        wui.Font{Family: font.Family, Size: font.Size},
		LineHeight:  font.LineHeight,
		AddedLine:   diffAddedLine,
		DeletedLine: diffDeletedLine,
		AddedWord:   wui.Hex("#9ccc2c").Alpha(0.3),
		DeletedWord: wui.Hex("#ff0000").Alpha(0.25),
		AddedMark:   ct.Added,
		DeletedMark: ct.Deleted,
		Filler:      t.TextMuted.Alpha(0.16),
		FoldBack:    ct.Text.Alpha(0.035),
		FoldHover:   toolFill(t, 0.08),
		FoldText:    t.TextMuted,
		ScopeText:   ct.Text.Alpha(0.72),
		Divider:     t.Border,
		Ruler:       ct.Text.Alpha(0.02),
		UIFont:      wui.Font{Size: 11},
		FoldLabel:   func(n int) string { return i18n.Tn("%d 行未修改", n) },
	}
}

// gedFindBar 是 ged 的 Diff 右上角的查找条（样子同终端的查找条）：敲字就找（不分大小写），↩ / ⌘G 下一处，
// ⇧↩ 上一处，Esc 关掉、键盘回到 Diff。
func gedFindBar(c *wui.Context, bar wui.Element, g *ged.Diff) {
	t := c.Theme()
	f := &g.Find
	edit := g.CanEdit()
	if !edit {
		f.Replace = false
	}
	bar.Padding(2, 4, 2, 4).Gap(2).AlignItems(wui.Start).Radius(8).
		Background(t.Background).Border(1, t.Border).Shadow(shadowFloat()).Children(func() {
		if edit { // 就地编辑：左边的箭头展开 / 收起替换（VS Code 的查找条那样）
			ic, tip := iconChevronRight, i18n.T("展开替换")
			if f.Replace {
				ic, tip = iconChevron, i18n.T("收起替换")
			}
			iconButton(c, ic, app.Tip(tip, "edit.replace"), func() { f.Replace = !f.Replace; f.FocusWith = f.Replace })
		}
		wui.Column(c).Gap(2).Children(func() {
			wui.Row(c).Height(28).Gap(4).AlignItems(wui.Center).Padding(0, 0, 0, 6).Children(func() {
				wui.Icon(c, iconSearch).FontSize(iconRow).TextColor(t.TextMuted).Shrink(0)
				in := wui.TextInputBase(c.Key("q"), &f.Text).Label(i18n.T("在 Diff 里查找")).Placeholder(i18n.T("查找")).FontSize(bodyFont).Width(170).
					OnTextKey(func(k wui.TextKey) bool {
						switch {
						case k.Key == wui.KeyEscape && k.Mods == 0:
							g.CloseFind()
						case k.Key == wui.KeyEnter && k.Mods == wui.Shift, k.Key == wui.KeyG && k.Mods == wui.Shift|wui.Super:
							g.FindStep(-1)
						case k.Key == wui.KeyG && k.Mods == wui.Super:
							g.FindStep(1)
						default:
							return false
						}
						return true
					})
				if f.Focus {
					f.Focus = false
					in.Focus().SetTextSelection(0, len([]rune(f.Text)))
				}
				g.SetFind(f.Text)
				if in.Submitted() {
					g.FindStep(1)
				}
				cur, total, more := g.FindStatus()
				count, col := "", t.TextMuted
				switch {
				case f.Text == "":
				case total == 0:
					count, col = i18n.T("没有结果"), t.Danger
				case more:
					count = i18n.Tf("%d / %d+", cur, total)
				default:
					count = i18n.Tf("%d / %d", cur, total)
				}
				wui.Text(c, count).FontSize(11).TextColor(col).SingleLine().MinWidth(52).TextAlign(wui.End).Margin(0, 4, 0, 0)
				iconButton(c, iconChevronUp, i18n.T("查找上一个（⇧↩）"), func() { g.FindStep(-1) })
				iconButton(c, iconChevron, app.Tip(i18n.T("查找下一个"), "edit.findNext"), func() { g.FindStep(1) })
				iconButton(c, iconClose, i18n.T("关闭查找"), g.CloseFind)
			})
			if !f.Replace {
				return
			}
			wui.Row(c).Height(28).Gap(4).AlignItems(wui.Center).Padding(0, 0, 0, 6).Children(func() {
				wui.Icon(c, iconReplace).FontSize(iconRow).TextColor(t.TextMuted).Shrink(0)
				in := wui.TextInputBase(c.Key("with"), &f.With).Label(i18n.T("替换为")).Placeholder(i18n.T("替换")).FontSize(bodyFont).Width(170).
					OnTextKey(func(k wui.TextKey) bool {
						switch {
						case k.Key == wui.KeyEscape && k.Mods == 0:
							g.CloseFind()
						case k.Key == wui.KeyEnter && k.Mods == wui.Super|wui.Alt:
							g.ReplaceAll()
						default:
							return false
						}
						return true
					})
				if f.FocusWith {
					f.FocusWith = false
					in.Focus().SetTextSelection(0, len([]rune(f.With)))
				}
				if in.Submitted() { // ↩ 替换当前那处、跳到下一处
					g.ReplaceOne()
				}
				iconButton(c, iconReplace, i18n.T("替换（↩）"), func() { g.ReplaceOne() })
				iconButton(c, iconCheck, i18n.T("全部替换（⌥⌘↩）"), func() { g.ReplaceAll() })
			})
		})
	})
}
