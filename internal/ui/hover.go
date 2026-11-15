package ui

import (
	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/i18n"
)

// hoverSetup 给文件标签装上悬停提示（app/hover.go）：指针停在名字上、跑 editor.showHover 时给声明、注释和位置；
// 设置关了、不是代码文件的不装（检查报的问题照样由编辑器自己出）。
func hoverSetup(a *app.App, d *docs.Doc) {
	d.St.OnHover = nil
	if a.St.Settings.NoHoverDef || !app.CanGoToDefinition(d) {
		return
	}
	d.St.OnHover = func(req editor.HoverRequest) {
		h := a.HoverAt(d, req.Offset)
		if h == nil {
			return
		}
		d.St.ShowHover(h.Start, h.End, h.Code, func(c *wui.Context) { hoverBody(a, c, d, h) })
	}
}

// hoverBody 是悬停提示里声明下面的：注释，再一行位置（点了打开）和"还有几处"（点了列出来挑）。
func hoverBody(a *app.App, c *wui.Context, d *docs.Doc, h *app.Hover) {
	t := c.Theme()
	if h.Doc != "" {
		wui.Text(c, h.Doc).FontSize(t.FontSize - 1).TextColor(t.TextMuted).Shrink(1)
	}
	wui.Row(c).Gap(t.Space(1)).AlignItems(wui.Center).Children(func() {
		if linkButton(c, h.Loc, i18n.T("打开定义")).Clicked() {
			a.OpenHoverSite(h)
		}
		if h.More > 0 && linkButton(c, i18n.Tn("还有 %d 处", h.More), i18n.T("列出同名的定义")).Clicked() {
			a.GoToDefinition(d, h.Start)
		}
	})
}
