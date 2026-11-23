// Package gmd 是 Grove 自己画的 Markdown 视图（预览标签、旁边的预览、<details> 里嵌套的那几段）：
// 解析照旧用 mygo 的 ui/markdown/parser，排版和画只用 mygo 公开的界面接口（Column / Row / Grid、RichText、Painter）；
// HTML 块有自己的 HTML + CSS 排版（html.go：<style>、class、style=、grid、flex、边框、底色），像 VS Code 的预览那样画出来。
// 不认 Grove 的主题和 internal/ui：样子由 markdown.Theme 给，文案由 Options.Strings 给。
//
// 段落、列表、表格、代码块的画法改自 mygo 的 ui/markdown（MIT，见 resources/licenses/mygo.txt），去掉了流式显示（Reveal）、
// 提及 / 引用（Resolve）和链接图标这些 Grove 不用的。
package gmd

import (
	"sync"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// InlineObject 是段落里由调用方画的一块（行内公式）：和 mygo 的同一个类型，钩子两边共用。
type InlineObject = markdown.InlineObject

// Options 是 View 的选项。字段和 mygo 的 markdown.Options 一一对应（同一份钩子两边都能用，见 Mygo），
// 只是代码块的钩子拿 gmd 自己的 CodeBlockView；另外多了 CSS、Image 和 Strings。
type Options struct {
	// Theme 是样子：nil 时用窗口主题的 markdown.DefaultTheme，没写的（零值）也从它取。
	Theme *markdown.Theme
	// Extensions 是 CommonMark 之外认的语法：0 是 parser.Default。
	Extensions parser.Extensions

	// LinkMenu 给链接的右键菜单加项（拷贝路径、在访达里显示）；#标题 和脚注的链接不用它。
	LinkMenu func(m *wui.Menu, url string)
	// OnLink 接点链接；nil 时 http / https / mailto 照 wui.Link 开，#标题 滚过去，别的不管。
	OnLink func(url string)
	// CodeLink 让行内代码成链接（`src/app.go` 这种找得到的路径）：给出地址和 true。
	CodeLink func(code string) (dest string, ok bool)
	// Preview 给链接一个悬停的预览卡：返回往卡里画东西的函数，nil 是没有（照常显示地址）。
	Preview func(c *wui.Context, dest string) func(tip wui.Element)
	// LoadImage 读图片（在后台 goroutine 里调）；Image 给了就先用 Image。
	LoadImage func(url string) (wui.ImageSource, error)
	// Image 取图片（在主线程上调，读好了 ok 为 true，读不出来 err 不为空；还在读返回 false，读好了调用方让界面重画）。
	// 给了它 LoadImage 就不用了：调用方自己管缓存。
	Image func(url string) (src wui.ImageSource, err error, ok bool)

	// CodeBlock 接手代码块的画法，Code 按语言接手（"mermaid"）；DefaultCodeBlock 是不接手时的样子。
	CodeBlock func(c *wui.Context, b *CodeBlockView)
	Code      map[string]func(c *wui.Context, b *CodeBlockView)
	// WrapCode 让代码块里的长行折行（默认横着滚）。
	WrapCode bool
	// Math 画 $$…$$ 的块，InlineMath 把 $…$ 变成段落里的一块（没好返回 false，照 TeX 显示）。
	Math       func(c *wui.Context, tex string)
	InlineMath func(tex string, size float32) (InlineObject, bool)
	// HTML 接手 HTML 块；nil 时用 gmd 自己的 HTML + CSS 排版（HTMLView）。
	HTML func(c *wui.Context, html string)
	// Block 先看每一块，接手了返回 true（只有图片的段落、带记号的块……）。
	Block func(c *wui.Context, d *parser.Document, i int) bool

	// ShareSelection：这个视图嵌在另一个 Selectable 的容器里（<details> 的正文、大文件的一段），和外面共用选区，
	// 自己不再 Selectable、不接复制。
	ShareSelection bool

	// CSS 是全文的样式表（DocCSS 收的 HTML 块里所有的 <style>）：HTML 块按它、再加上块里自己的 <style> 排。
	CSS string

	// Strings 是视图里的几句文案（Grove 给译好的），空的用英文。
	Strings Strings

	images sync.Map // url → *imageEntry（LoadImage 的缓存）
	mygo   *markdown.Options
	codes  map[string]bool // mygo.Code 是按哪几种语言建的
}

// Strings 是视图里写的字。
type Strings struct {
	Copy, Copied, CopyCode, CopyAsMarkdown, Image, Code string
}

func (s *Strings) or(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

// Mygo 是同一份选项换成 mygo 的 markdown.Options（WORKBENCH_GMD=off 时 Grove 用 mygo 的视图）：每次调都把字段拷过去，
// 代码块的钩子包一层（mygo 的 CodeBlockView 换成 gmd 的，DefaultCodeBlock 回到 mygo 的那个）。返回的指针一直是同一个，
// mygo 的图片缓存在它里面。
func (o *Options) Mygo() *markdown.Options {
	if o.mygo == nil {
		o.mygo = &markdown.Options{}
	}
	m := o.mygo
	m.Theme, m.Extensions = o.Theme, o.Extensions
	m.LinkMenu, m.OnLink, m.CodeLink, m.Preview, m.LoadImage = o.LinkMenu, o.OnLink, o.CodeLink, o.Preview, o.LoadImage
	m.WrapCode, m.Math, m.InlineMath, m.HTML, m.Block, m.ShareSelection = o.WrapCode, o.Math, o.InlineMath, o.HTML, o.Block, o.ShareSelection
	m.CodeBlock = nil
	if o.CodeBlock != nil {
		m.CodeBlock = func(c *wui.Context, b *markdown.CodeBlockView) {
			if f := o.CodeBlock; f != nil {
				f(c, foreign(b))
			} else {
				markdown.DefaultCodeBlock(c, b)
			}
		}
	}
	same := len(o.codes) == len(o.Code)
	for k := range o.Code {
		same = same && o.codes[k]
	}
	if !same {
		o.codes = map[string]bool{}
		m.Code = nil
		for lang := range o.Code {
			o.codes[lang] = true
			if m.Code == nil {
				m.Code = map[string]func(*wui.Context, *markdown.CodeBlockView){}
			}
			m.Code[lang] = func(c *wui.Context, b *markdown.CodeBlockView) {
				if f := o.Code[lang]; f != nil {
					f(c, foreign(b))
				} else {
					markdown.DefaultCodeBlock(c, b)
				}
			}
		}
	}
	return m
}

// foreign 把 mygo 的代码块包成 gmd 的（Mygo 用）。
func foreign(b *markdown.CodeBlockView) *CodeBlockView {
	return &CodeBlockView{Language: b.Language, Info: b.Info, Open: b.Open, Index: b.Index, code: b.Code,
		def: func(c *wui.Context) wui.Element { return markdown.DefaultCodeBlock(c, b) }}
}
