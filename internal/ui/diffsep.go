package ui

import (
	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/lemoeade/grove/internal/i18n"
)

// diffSep 画 Diff 里折起来的那段未修改的行：一根细线，中间压一颗小胶囊（“N 行未修改”），没有底色；
// 悬停时胶囊加深。点胶囊展开，见 expandSep。
func diffSep(c *wui.Context, sep wui.Element, s *editor.DiffSeparator) {
	t := c.Theme()
	ct := s.Theme()
	line := ct.Text.Alpha(0.09)
	text := i18n.Tn("%d 行未修改", s.Lines)
	sep.Children(func() {
		wui.Row(c).Grow(1).AlignSelf(wui.Stretch).AlignItems(wui.Center).Gap(10).Padding(0, 14).Children(func() {
			wui.Box(c).Height(1).Grow(1).Background(line)
			b := wui.ButtonBase(c.Key("sep")).Padding(3, 10).Radius(11).Gap(6).AlignItems(wui.Center).Cursor(wui.CursorPointer).
				Background(ct.Text.Alpha(0.04)).Label(i18n.Tn("展开 %d 行未修改", s.Lines)).
				Tooltip(i18n.T("点一下往改动那边展开；按住 ⇧ 一次展开全部"))
			if b.Hovered() {
				b.Background(ct.Text.Alpha(0.08))
			}
			b.Children(func() {
				wui.Icon(c, iconFold).FontSize(iconInline).TextColor(t.TextMuted)
				wui.Text(c, text).FontSize(11).TextColor(t.TextMuted).SingleLine()
			})
			if b.Clicked() {
				expandSep(c, s)
			}
			wui.Box(c).Height(1).Grow(1).Background(line)
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
