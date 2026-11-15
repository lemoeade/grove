package ui

import (
	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/lemoeade/grove/internal/app"
)

// 这个文件是 Markdown 预览的排版：比界面的字大一号、行距宽一些，一级、二级标题下面一条线（GitHub 的样子），
// 代码用编辑器的字体，表格隔行一点底色。按窗口主题和编辑器字体算好留着（主题的值每帧比一次）。

type mdThemeKey struct {
	t    wui.Theme
	font string
}

var mdThemes = map[mdThemeKey]*markdown.Theme{}

// mdTheme 是预览的排版主题；同一个主题、字体给同一个指针（mygo 按值比，指针稳定省得每帧分配）。
func mdTheme(a *app.App, c *wui.Context) *markdown.Theme {
	t := c.Theme()
	k := mdThemeKey{*t, a.EditorFont().Family}
	if th, ok := mdThemes[k]; ok {
		return th
	}
	if len(mdThemes) > 8 { // 换来换去的主题、字体：留几个就够
		clear(mdThemes)
	}
	th := markdown.DefaultTheme(t)
	base := t.FontSize
	if base == 0 {
		base = 13
	}
	th.FontSize = base * 15 / 13
	th.LineHeight = 1.65
	th.CodeLineHeight = 1.45
	th.BlockSpacing = 1
	th.ItemSpacing = 0.3
	th.CodeFont = k.font
	th.CodeSize = 0.86
	th.CodeBlockPadding = 0.9
	th.QuoteBarWidth = 3.5
	th.TableStripe = t.Text.Alpha(0.025)
	th.TableCellPadding = [2]float32{0.75, 0.45}
	sizes := [6]float32{2, 1.5, 1.25, 1.05, 0.95, 0.88}
	tops := [6]float32{1.6, 1.5, 1.35, 1.2, 1.2, 1.2}
	for i := range th.Headings {
		h := &th.Headings[i]
		h.Size, h.Top, h.Bottom = sizes[i], tops[i], 0.6
		h.Rule = i < 2
	}
	th.Headings[0].Weight = 700
	mdThemes[k] = th
	return th
}

// mdEm 是 th 里 v 个 em 的 DIP。
func mdEm(th *markdown.Theme, v float32) float32 { return v * th.FontSize }
