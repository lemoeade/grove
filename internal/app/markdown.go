package app

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"

	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/fonts"
	"github.com/lemoeade/grove/internal/i18n"
)

// 这个文件是 Markdown 预览（⇧⌘V）和编辑器的显示选项（软换行 ⌥Z、空白字符、缩进、字体），
// 显示选项存在 state.json 的设置里，所有标签共用。

// TogglePreview 在当前 Markdown / SVG 标签的源码和预览之间切换；切回源码时焦点给编辑器，SVG 预览时给画布（= - 0 缩放）。
func (a *App) TogglePreview() {
	d := a.CurrentDoc()
	if d == nil || !d.CanPreview() {
		return
	}
	d.Preview = !d.Preview
	d.Anchor = ""
	a.FocusDoc = !d.Preview || docs.IsSVG(d.Name())
}

// MarkdownOptions 是 d 的预览选项：图片从文件所在的目录读，链接交给 MarkdownLink。
func (a *App) MarkdownOptions(d *docs.Doc) *markdown.Options {
	if d.MD == nil {
		d.MD = &markdown.Options{
			LoadImage: markdown.FileImages(d.Dir()),
			OnLink:    func(u string) { a.MarkdownLink(d, u) },
		}
	}
	return d.MD
}

// MarkdownLink 处理预览里点的链接：#标题 滚到那个标题；http、https、mailto 用系统浏览器 / 邮件开；
// 其它的当文件路径（相对 d 所在的目录，可带 :行[:列]）在编辑器里打开，是 Markdown 就直接显示预览，
// 目录在文件树里显示。别的协议不开。
func (a *App) MarkdownLink(d *docs.Doc, u string) {
	switch scheme := linkScheme(u); {
	case strings.HasPrefix(u, "#"):
		d.Anchor = anchorName(u[1:])
	case scheme == "http" || scheme == "https" || scheme == "mailto":
		if !ui.SafeToOpen(u) {
			a.Toast = i18n.Tf("不打开这个链接：%s", u)
			return
		}
		if err := a.OpenURL(u); err != nil {
			a.Fail(i18n.T("打开链接失败"), err)
		}
	case scheme != "" && scheme != "file":
		a.Toast = i18n.Tf("不打开 %s: 链接", scheme)
	default:
		p, frag, _ := strings.Cut(u, "#")
		p, _, _ = strings.Cut(p, "?")
		if s, err := url.PathUnescape(p); err == nil {
			p = s
		}
		if p == "" {
			return
		}
		if !a.OpenLink(d.Dir(), p) {
			a.Toast = i18n.Tf("找不到 %s", p)
			return
		}
		if nd := a.CurrentDoc(); nd != nil && nd != d && nd.Kind != docs.Diff && docs.IsMarkdown(nd.Name()) {
			nd.Preview, nd.Anchor = true, anchorName(frag)
			a.FocusDoc = false
		}
	}
}

// linkScheme 是链接的协议（小写），没有就是 ""。foo.go:12 这种带行号的路径不算协议。
func linkScheme(u string) string {
	i := strings.IndexAny(u, ":/?#")
	if i < 2 || u[i] != ':' {
		return ""
	}
	for j, c := range u[:i] {
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || j > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-')
		if !ok {
			return ""
		}
	}
	if r := u[i+1:]; r != "" && r[0] >= '0' && r[0] <= '9' {
		return ""
	}
	return strings.ToLower(u[:i])
}

// anchorName 是 #后面那段解码、转小写后的样子，和 markdown.Slug 比。
func anchorName(s string) string {
	if v, err := url.PathUnescape(s); err == nil {
		s = v
	}
	return strings.ToLower(s)
}

// ToggleSoftWrap 开关当前标签的软换行（⌥Z）：只这一个标签，设置里的默认不动；切回和设置一样就又跟着设置。
func (a *App) ToggleSoftWrap() {
	d := a.CurrentDoc()
	if !canWrap(d) {
		return
	}
	on := !a.WrapOf(d)
	switch {
	case on == a.wrapDefault(d):
		d.Wrap = 0
	case on:
		d.Wrap = 1
	default:
		d.Wrap = -1
	}
}

// WrapOf 是 d 软换不换行：⌥Z 给它定过的按定的，不然按设置（Settings.Wrap）。
func (a *App) WrapOf(d *docs.Doc) bool {
	if d.Wrap != 0 {
		return d.Wrap > 0
	}
	return a.wrapDefault(d)
}

// wrapDefault 是设置里给 d 的软换行：默认只给 Markdown 和纯文本（Doc.Prose），"all" 都换，"off" 都不换。
func (a *App) wrapDefault(d *docs.Doc) bool {
	switch a.St.Settings.Wrap {
	case "all":
		return true
	case "off":
		return false
	}
	return d.Prose()
}

// canWrap 是 ⌥Z 对 d 有没有用：有编辑器的标签（十六进制的不折行）。
func canWrap(d *docs.Doc) bool { return d != nil && d.Buf != nil && !d.Hex }

// ToggleStickyScroll 开关粘性滚动（view.stickyScroll；设置里存的是反面 noStickyScroll）。
func (a *App) ToggleStickyScroll() {
	a.St.Settings.NoStickyScroll = !a.St.Settings.NoStickyScroll
	a.SaveState()
}

// ToggleWhitespace 在"只在选中的文字里"和"全部"之间切换空白字符的显示。
func (a *App) ToggleWhitespace() {
	if a.St.Settings.Whitespace == "all" {
		a.St.Settings.Whitespace = ""
	} else {
		a.St.Settings.Whitespace = "all"
	}
	a.SaveState()
}

// EditorFont 是设置里的编辑器字体。
func (a *App) EditorFont() ui.Font {
	s := &a.St.Settings
	f := ui.Font{Family: s.EditorFont, Size: s.EditorSize}
	if f.Family == "" {
		f.Family = fonts.Mono()
	}
	if f.Size == 0 {
		f.Size = 12.5
	}
	return f
}

// EditorOptions 把设置里的字体、软换行（WrapOf）、空白字符和缩进（状态栏里给这个文件选过的优先）套到 d 的编辑器上；
// 界面每帧画编辑器前调。
func (a *App) EditorOptions(d *docs.Doc) {
	s := &a.St.Settings
	st, indent := &d.St, d.Indent
	st.Font = a.EditorFont()
	st.Bindings = EditorBindings() // 改过键的编辑器命令（keybind.go）
	st.SoftWrap = a.WrapOf(d)
	switch s.Whitespace {
	case "all":
		st.Whitespace = editor.WhitespaceAll
	case "trailing":
		st.Whitespace = editor.WhitespaceTrailing
	case "none":
		st.Whitespace = editor.WhitespaceNone
	default:
		st.Whitespace = editor.WhitespaceSelection
	}
	if indent == "" {
		indent = s.Indent
	}
	switch indent {
	case "", docs.IndentAuto:
		st.TabSize, st.Indent = 0, editor.IndentAuto
	case "tab":
		st.TabSize, st.Indent = 0, editor.IndentTabs
	default:
		st.TabSize, _ = strconv.Atoi(indent)
		st.Indent = editor.IndentSpaces
	}
}

// IndentLabel 是状态栏里 d 的缩进："空格 2"、"Tab 4"。
func (a *App) IndentLabel(d *docs.Doc) string {
	size, tabs := a.IndentOf(d)
	if tabs {
		return fmt.Sprintf("Tab %d", size)
	}
	return i18n.Tf("空格 %d", size)
}

// IndentOf 是 d 现在的缩进宽度、是不是用 Tab：这个文件在状态栏里选的，没选跟设置；设置是"按语言"时用语言的习惯，
// 不认识的语言是空格 4（和 mygo 的一样）。
func (a *App) IndentOf(d *docs.Doc) (size int, tabs bool) {
	size = 4
	if d.Buf != nil {
		if l := d.Buf.Language(); l != nil {
			if l.TabSize > 0 {
				size = l.TabSize
			}
			tabs = l.HardTabs
		}
	}
	switch s := a.DocIndent(d); s {
	case "":
	case "tab":
		tabs = true
	default:
		size, _ = strconv.Atoi(s)
		tabs = false
	}
	return size, tabs
}
