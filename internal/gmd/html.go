package gmd

import (
	"regexp"
	"strconv"
	"strings"

	ui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/lemoeade/grove/internal/css"
)

// HTML 块的排版：x/net/html 解析成树，internal/css 按全文的 <style>、style= 和默认样式层叠出每个元素的样式，
// 再照样式摆成 mygo 的元素——块是 Column（上下外边距合并），一段行内内容是一个 RichText（链接是里面的 wui.Link，
// 有底色、圆角的行内元素是带底色的字，空的 inline-block 是字里的一块），flex 是 Row / Column，grid 是 Grid，
// position: absolute 是 Absolute。画不出来的（浮动、transform、动画、伪类状态）照静态的、不浮动的样子摆。

// htmlState 是一个 HTML 块解析、排好样式的树：源码和样式表没变就一直用它。
type htmlState struct {
	src   string
	sheet *css.Sheet
	roots []*hnode
	root  *css.Style // 根（块外面的正文）的样式
	runs  map[*hnode]*irun
	seen  bool
}

// sheetCache 是全文的样式表：Options.CSS 加上文档里 HTML 块的 <style>。src 是收的时候的源码。
type sheetCache struct {
	src, css string
	key      string
	sheet    *css.Sheet
}

// hnode 是树上的一个节点：元素（tag 不空）或文字。
type hnode struct {
	n      *html.Node
	tag    string // 小写；文字是 ""，伪元素是 "::before" / "::after"
	text   string
	parent *hnode
	prev   *hnode // 前后的兄弟元素（不算文字）
	next   *hnode
	kids   []*hnode
	st     *css.Style
}

func (h *hnode) Tag() string { return h.tag }

func (h *hnode) Attr(name string) (string, bool) {
	if h.n == nil {
		return "", false
	}
	for _, a := range h.n.Attr {
		if a.Namespace == "" && a.Key == name {
			return a.Val, true
		}
	}
	return "", false
}

func (h *hnode) Parent() css.Element {
	if h.parent == nil {
		return nil
	}
	return h.parent
}

func (h *hnode) Prev() css.Element {
	if h.prev == nil {
		return nil
	}
	return h.prev
}

func (h *hnode) Next() css.Element {
	if h.next == nil {
		return nil
	}
	return h.next
}

func (h *hnode) attr(name string) string { v, _ := h.Attr(name); return v }

func (h *hnode) get(p string) string { return h.st.Get(p) }

// skipTags 是不显示的元素（<style> 收进样式表）。
var skipTags = map[string]bool{"style": true, "script": true, "head": true, "meta": true, "link": true, "title": true,
	"template": true, "noscript": true, "source": true, "track": true, "param": true, "base": true}

// parseHTML 把一个 HTML 块解析成节点。
func parseHTML(src string) []*hnode {
	ctx := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(src), ctx)
	if err != nil {
		return nil
	}
	return addNodes(nodes, nil)
}

func addNodes(nodes []*html.Node, parent *hnode) []*hnode {
	var out []*hnode
	var prevEl *hnode
	for _, n := range nodes {
		switch n.Type {
		case html.TextNode:
			out = append(out, &hnode{n: n, text: n.Data, parent: parent})
		case html.ElementNode:
			tag := strings.ToLower(n.Data)
			if skipTags[tag] {
				continue
			}
			h := &hnode{n: n, tag: tag, parent: parent, prev: prevEl}
			if prevEl != nil {
				prevEl.next = h
			}
			prevEl = h
			var kids []*html.Node
			for k := n.FirstChild; k != nil; k = k.NextSibling {
				kids = append(kids, k)
			}
			h.kids = addNodes(kids, h)
			out = append(out, h)
		}
	}
	return out
}

var styleRe = regexp.MustCompile(`(?is)<style\b[^>]*>(.*?)</style\s*>`)

// styles 是 HTML 里所有 <style> 的内容。
func styles(src string) []string {
	var out []string
	for _, m := range styleRe.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	return out
}

var invisibleRe = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style\s*>|<script\b[^>]*>.*?</script\s*>|<!--.*?-->|<link\b[^>]*>|<meta\b[^>]*>`)

// invisible 是这个 HTML 块什么都不显示（只有 <style>、注释……）：不占地方，也不隔开上下。
func invisible(src string) bool {
	return strings.TrimSpace(invisibleRe.ReplaceAllString(src, "")) == ""
}

// cssSheet 是全文的样式表：源码没变就用上次的；收出来的 CSS 和上次一样也还是上次那份（HTML 块的样式不用重排）。
func (st *viewState) cssSheet() *css.Sheet {
	src := st.doc.Source()
	if st.sheet != nil && st.sheet.src == src && st.sheet.css == st.o.CSS {
		return st.sheet.sheet
	}
	var b strings.Builder
	b.WriteString(st.o.CSS)
	d := st.doc
	for i := 1; i < d.Len(); i++ {
		if d.Block(i).Kind != parser.HTMLBlock {
			continue
		}
		for _, s := range styles(d.Lines(i)) {
			if st.o.CSS != "" && strings.Contains(st.o.CSS, s) {
				continue
			}
			b.WriteString("\n")
			b.WriteString(s)
		}
	}
	key := b.String()
	if st.sheet != nil && st.sheet.key == key {
		st.sheet.src, st.sheet.css = src, st.o.CSS
		return st.sheet.sheet
	}
	st.sheet = &sheetCache{src: src, css: st.o.CSS, key: key, sheet: css.ParseSheet(key)}
	return st.sheet.sheet
}

// DocCSS 是一篇 Markdown 里 HTML 块的所有 <style>（代码块里的不算），给 Options.CSS 用：分段画的大文件、
// 嵌在 <details> 里的段落都按全文的样式表排。
func DocCSS(src string) string {
	if !strings.Contains(src, "<style") && !strings.Contains(src, "<STYLE") {
		return ""
	}
	d := parser.New(parser.Default)
	d.Set(src)
	var b strings.Builder
	for i := 1; i < d.Len(); i++ {
		if d.Block(i).Kind != parser.HTMLBlock {
			continue
		}
		for _, s := range styles(d.Lines(i)) {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(s)
		}
	}
	return b.String()
}

// WantsCSS 说这段 HTML 要不要按 CSS 排（gmd 的 HTMLView）：有 <style>、用了全文样式表里的 class，
// 或者 style= 里写了底色、边框、布局这些。README 那种只有对齐、图片、徽章的 HTML 不用。
func WantsCSS(src, docCSS string) bool {
	l := strings.ToLower(src)
	if strings.Contains(l, "<style") {
		return true
	}
	if strings.TrimSpace(docCSS) != "" && (strings.Contains(l, "class=") || strings.Contains(l, "id=")) {
		return true
	}
	for _, m := range styleAttrRe.FindAllStringSubmatch(l, -1) {
		for _, k := range []string{"background", "border", "display", "padding", "grid", "flex", "position", "box-shadow", "width", "margin"} {
			if strings.Contains(m[1], k) {
				return true
			}
		}
	}
	return false
}

var styleAttrRe = regexp.MustCompile(`style\s*=\s*(?:"([^"]*)"|'([^']*)')`)

// htmlBlock 画第 i 块（HTML）：没有 Options.HTML 时到这里。
func (st *viewState) htmlBlock(c *ui.Context, i int, src string) ui.Element {
	if st.htmls == nil {
		st.htmls = map[int]*htmlState{}
	}
	start := st.doc.Block(i).Start
	sheet := st.cssSheet()
	hs := st.htmls[start]
	if hs == nil || hs.src != src || hs.sheet != sheet {
		hs = st.layoutHTML(src, sheet)
		st.htmls[start] = hs
	}
	hs.seen = true
	return st.renderHTML(c, hs, i)
}

// HTMLView 画一段 HTML（按 Options.CSS 和它自己的 <style>）：Grove 的 HTML 钩子把要按 CSS 排的块交回来。
func (o *Options) HTMLView(c *ui.Context, src string) ui.Element {
	box := ui.Column(c).AlignSelf(ui.Stretch)
	st := *ui.Local(box, htmlKey{}, newState)
	st.o = o
	if st.doc == nil {
		st.doc = parser.New(parser.Default)
	}
	st.theme(c)
	key := o.CSS
	if own := styles(src); len(own) > 0 {
		key += "\n" + strings.Join(own, "\n")
	}
	if st.sheet == nil || st.sheet.key != key {
		st.sheet = &sheetCache{key: key, sheet: css.ParseSheet(key)}
	}
	if st.htmls == nil {
		st.htmls = map[int]*htmlState{}
	}
	hs := st.htmls[0]
	if hs == nil || hs.src != src || hs.sheet != st.sheet.sheet {
		hs = st.layoutHTML(src, st.sheet.sheet)
		st.htmls[0] = hs
	}
	return box.Children(func() { st.renderHTML(c, hs, -1) })
}

type htmlKey struct{}

// layoutHTML 解析 src、按 sheet 算每个元素的样式。
func (st *viewState) layoutHTML(src string, sheet *css.Sheet) *htmlState {
	th := &st.th
	hs := &htmlState{src: src, sheet: sheet, roots: parseHTML(src)}
	rootPx := th.FontSize
	decls := []css.Decl{
		{Prop: "color", Value: hexColor(th.Text)},
		{Prop: "font-size", Value: ftoa(rootPx) + "px"},
		{Prop: "line-height", Value: ftoa(th.LineHeight)},
	}
	hs.root = css.Compute(nil, &hnode{tag: "html"}, "", decls, nil, rootPx)
	var walk func(hs []*hnode, parent *css.Style)
	walk = func(list []*hnode, parent *css.Style) {
		for _, h := range list {
			if h.tag == "" {
				h.st = parent
				continue
			}
			h.st = css.Compute(sheet, h, "", css.ParseDecls(h.attr("style")), parent, rootPx)
			walk(h.kids, h.st)
			// ::before / ::after：content 不是 none 才有，当成第一个、最后一个孩子
			for _, ps := range [...]string{"before", "after"} {
				s := css.Compute(sheet, h, ps, nil, h.st, rootPx)
				text, ok := pseudoContent(s.Get("content"), h)
				if !ok {
					continue
				}
				p := &hnode{tag: "::" + ps, parent: h, st: s}
				if text != "" {
					p.kids = []*hnode{{text: text, parent: p, st: s}}
				}
				if ps == "before" {
					h.kids = append([]*hnode{p}, h.kids...)
				} else {
					h.kids = append(h.kids, p)
				}
			}
		}
	}
	walk(hs.roots, hs.root)
	return hs
}

// pseudoContent 读 content：字符串（"\2192" 这种转义也认）、attr(x)；none、normal 和没写是没有。
func pseudoContent(v string, h *hnode) (string, bool) {
	v = strings.TrimSpace(v)
	switch strings.ToLower(v) {
	case "", "none", "normal":
		return "", false
	}
	var b strings.Builder
	for _, part := range css.Fields(v) {
		switch {
		case len(part) >= 2 && (part[0] == '"' || part[0] == '\''):
			b.WriteString(unescapeCSS(part[1 : len(part)-1]))
		case strings.HasPrefix(strings.ToLower(part), "attr(") && strings.HasSuffix(part, ")"):
			b.WriteString(h.attr(strings.ToLower(strings.TrimSpace(part[5 : len(part)-1]))))
		}
	}
	return b.String(), true
}

// unescapeCSS 解开 CSS 字符串里的 \XXXX 和 \x。
func unescapeCSS(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		j := i + 1
		for j < len(s) && j-i <= 6 && isHexByte(s[j]) {
			j++
		}
		if j == i+1 {
			b.WriteByte(s[j])
			i = j
			continue
		}
		n, _ := strconv.ParseUint(s[i+1:j], 16, 32)
		b.WriteRune(rune(n))
		if j < len(s) && s[j] == ' ' {
			j++
		}
		i = j - 1
	}
	return b.String()
}

func isHexByte(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func hexColor(c ui.Color) string {
	const digits = "0123456789abcdef"
	b := []byte{'#', 0, 0, 0, 0, 0, 0, 0, 0}
	for k, v := range [4]uint8{c.R, c.G, c.B, c.A} {
		b[1+2*k], b[2+2*k] = digits[v>>4], digits[v&15]
	}
	return string(b)
}

func ftoa(f float32) string { return strconv.FormatFloat(float64(f), 'f', -1, 32) }
