package ui

import (
	"os"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/lemoeade/grove/internal/gmd"
	"github.com/lemoeade/grove/internal/i18n"
)

// gmdOn 是 Markdown 预览用 Grove 自己画的 internal/gmd（默认）；WORKBENCH_GMD=off 退回 mygo 的 markdown.View
// （同一份选项经 gmd.Options.Mygo 换过去，带样式的 HTML 块也回到 mdhtml.go 的画法）。
var gmdOn = os.Getenv("WORKBENCH_GMD") != "off"

// mdShow 画一段 Markdown：gmd 的或 mygo 的视图。
func mdShow(c *wui.Context, src string, o *gmd.Options) wui.Element {
	if gmdOn {
		return gmd.View(c, src, o)
	}
	return markdown.View(c, src, o.Mygo())
}

// mdStrings 是 gmd 视图里的几句文案。
func mdStrings() gmd.Strings {
	return gmd.Strings{
		Copy:           i18n.T("复制"),
		Copied:         i18n.T("已复制"),
		CopyCode:       i18n.T("复制代码"),
		CopyAsMarkdown: i18n.T("复制为 Markdown"),
		Image:          i18n.T("图片"),
		Code:           i18n.T("代码"),
	}
}

// styles 是整理后全文的 <style>（HTML 块按它排）：整理结果没换不重收。
func (v *mdView) styles() string {
	if v.cssPrep != v.prep {
		v.cssPrep, v.css = v.prep, ""
		if v.prep != nil {
			v.css = gmd.DocCSS(v.prep.Src)
		}
	}
	return v.css
}
