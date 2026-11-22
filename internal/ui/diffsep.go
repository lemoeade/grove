package ui

import (
	"reflect"
	"strings"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/outline"
)

// diffSep 画 Diff 里折起来的那段未修改的行，照 VS Code 折起来的未修改区：整行一条淡底，左边“N 行未修改”（点了展开，
// 见 expandSep；悬停时加深），后面是下面那段改动所在的函数 / 类 / 标题那一行（sepScope；种类的小字和大纲一样，标题不加）。
func diffSep(c *wui.Context, sep wui.Element, s *editor.DiffSeparator, scope string, kind outline.Kind) {
	t := c.Theme()
	ct := s.Theme()
	text := i18n.Tn("%d 行未修改", s.Lines)
	sep.Background(ct.Text.Alpha(0.035)) // 整行一条淡底，照 VS Code 折起来的未修改区
	sep.Children(func() {
		wui.Row(c).Grow(1).AlignSelf(wui.Stretch).AlignItems(wui.Center).Gap(10).Padding(0, 14).Children(func() {
			b := wui.ButtonBase(c.Key("sep")).Padding(3, 10).Radius(11).Gap(6).AlignItems(wui.Center).Cursor(wui.CursorPointer).
				Label(i18n.Tn("展开 %d 行未修改", s.Lines)).Shrink(0).
				Tooltip(i18n.T("点一下往改动那边展开；按住 ⇧ 一次展开全部"))
			if b.Hovered() {
				b.Background(toolFill(t, 0.08))
			}
			b.Children(func() {
				wui.Icon(c, iconFold).FontSize(iconInline).TextColor(t.TextMuted)
				wui.Text(c, text).FontSize(11).TextColor(t.TextMuted).SingleLine()
			})
			if b.Clicked() {
				expandSep(c, s)
			}
			if scope != "" {
				f := s.Font()
				wui.Row(c).Gap(6).AlignItems(wui.Center).MinWidth(0).Shrink(1).Tooltip(i18n.Tf("下面的改动在：%s", scope)).Children(func() {
					if kind != outline.Heading { // 标题那行自己带着 #
						wui.Text(c, symbolGlyph(kind)).FontSize(sideSub).FontWeight(600).TextColor(symbolColor(t, kind, t.TextMuted)).Shrink(0)
					}
					wui.Text(c, scope).Font(f.Family).FontSize(f.Size - 0.5).TextColor(ct.Text.Alpha(0.72)).
						SingleLine().Ellipsis("…").MinWidth(0).Shrink(1)
				})
			}
		})
	})
}

// expandSep 展开一段折起来的行：在文件开头 / 结尾的往改动那边展开 Step 行（mygo 默认 100），在两处改动中间的
// 一次展开全部；按住 ⇧、或者本来就不到 Step 行的，一律全部展开。
func expandSep(c *wui.Context, s *editor.DiffSeparator) {
	switch {
	case c.Modifiers()&wui.Shift != 0 || s.Lines <= s.Step:
		s.ExpandAll()
	case s.First:
		s.ExpandUp()
	case s.Last:
		s.ExpandDown()
	default:
		s.ExpandAll()
	}
}

// sepScope 是折叠条 s 下面那段改动的第一行所在的函数 / 类 / 标题（d.DiffOutline 里包着它的最里层符号）那一行的原文，
// 去掉缩进和行尾的 {，和那个符号的种类；文件结尾那条（后面没有改动了）、没有大纲的语言、改动不在任何符号里的为空。
// 按改动那行认、不按第一行看得见的：上下文里可能是上一个函数的 }，改动在两个函数之间（注释、空行）时就不写。
//
// 改动的第一行是：间隔开头 + 上面留的上下文（文件开头那条是 0，中间的是 ContextLines，Grove 点中间那条一次全展开，
// 所以上面留的不会比这多）+ 藏着的行数 + 下面留的上下文。
func sepScope(d *docs.Doc, s *editor.DiffSeparator) (string, outline.Kind) {
	if s.Last || s.Lines == 0 || d.DiffSt == nil {
		return "", 0
	}
	start, ok := sepStart(s)
	if !ok {
		return "", 0
	}
	ctx := diffContext(d.DiffSt)
	changed := start + s.Lines + ctx
	if !s.First {
		changed += ctx
	}
	return scopeAt(d, changed)
}

// scopeAt 是 Diff 旧的一边第 line 行所在的函数 / 类 / 标题（d.DiffOutline 里包着它的最里层符号）那一行的原文，
// 去掉缩进和行尾的 {，和那个符号的种类；没有大纲、不在任何符号里的为空。mygo 的折叠条（sepScope）和 ged 的（FoldScope）共用。
func scopeAt(d *docs.Doc, line int) (string, outline.Kind) {
	tree := d.DiffOutline()
	if tree == nil {
		return "", 0
	}
	i := tree.Scope(line)
	if i < 0 {
		return "", 0
	}
	f := &tree.Flat[i]
	text := strings.TrimSpace(d.DiffSt.Old.Line(f.Line))
	return strings.TrimSpace(strings.TrimSuffix(text, "{")), f.Kind
}

// diffContext 是改动上下各留几行未修改的（mygo DiffState.context 的同一个规则：0 是 3，负数是 0）。
func diffContext(st *editor.DiffState) int {
	switch {
	case st.ContextLines < 0:
		return 0
	case st.ContextLines == 0:
		return 3
	}
	return st.ContextLines
}

// sepStart 是折叠条那段间隔从旧文本的第几行开始（mygo 没导出的 DiffSeparator.a）。mygo 只给了行数和在不在头尾，
// 认不出是哪一段；读不到（mygo 改了字段）就当没有，折叠条照旧只写行数。TestSepStart 盯着这个字段。
func sepStart(s *editor.DiffSeparator) (int, bool) {
	f := reflect.ValueOf(s).Elem().FieldByName("a")
	if !f.IsValid() || f.Kind() != reflect.Int {
		return 0, false
	}
	return int(f.Int()), true
}
