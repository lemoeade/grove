package ui

import (
	"strconv"
	"strings"

	wui "github.com/egoist/mygo/ui"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/lemoeade/grove/internal/i18n"
)

// 这个文件是预览里的 HTML 块（README 常用的那些）：解析成节点，块级的标签各自成块（<p align>、<div align>、<center> 对齐，
// <h1>…<h6>、<ul>、<ol>、<table>、<pre>、<blockquote>、<details>、<hr>），连着的行内内容转成 Markdown 交给嵌套的 View 排
// （<b> → **、<a> → [](…)、<img> → ![](…)、<code>、<br>…）；只有图片（<img>、<picture>、包着图片的 <a>）的一串画成一排图片，
// 按 width / height 的大小。<script>、<style>、注释不显示，不认识的标签只显示里面的字。

type mdHTMLKey struct{}

type mdHTMLState struct {
	src   string
	nodes []*html.Node
}

var mdBody = &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}

// mdHTML 画一个 HTML 块。解析结果留在外面那个 Column 上，源码不变不重新解析。
func mdHTML(c *wui.Context, e *mdEnv, src string) {
	box := wui.Column(c).AlignSelf(wui.Stretch).Gap(mdEm(e.th, 0.5))
	st := *wui.Local(box, mdHTMLKey{}, func() *mdHTMLState { return &mdHTMLState{} })
	if st.nodes == nil || st.src != src {
		nodes, err := html.ParseFragment(strings.NewReader(src), mdBody)
		if err != nil {
			nodes = []*html.Node{{Type: html.TextNode, Data: src}}
		}
		st.src, st.nodes = src, nodes
	}
	mdAlign(box, e.align)
	box.Children(func() { e.htmlFlow(c, st.nodes, e.align) })
}

// htmlFlow 画一串兄弟节点：块级的各自成块，连着的行内节点合成一段。
func (e *mdEnv) htmlFlow(c *wui.Context, nodes []*html.Node, align wui.Align) {
	var run []*html.Node
	flush := func() {
		if len(run) > 0 {
			e.htmlInline(c, run, align)
			run = nil
		}
	}
	for _, n := range nodes {
		switch {
		case n.Type == html.CommentNode || n.Type == html.DoctypeNode:
		case n.Type == html.ElementNode && mdSkip(n):
		case n.Type == html.ElementNode && mdBlockTag(n):
			flush()
			e.htmlBlock(c, n, align)
		default:
			run = append(run, n)
		}
	}
	flush()
}

func mdSkip(n *html.Node) bool {
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Template, atom.Head, atom.Title, atom.Meta, atom.Link, atom.Noscript, atom.Iframe, atom.Object, atom.Embed, atom.Video, atom.Audio:
		return true
	}
	return false
}

func mdBlockTag(n *html.Node) bool {
	switch n.DataAtom {
	case atom.P, atom.Div, atom.Center, atom.Section, atom.Article, atom.Header, atom.Footer, atom.Main, atom.Nav, atom.Aside,
		atom.Figure, atom.Figcaption, atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6, atom.Hr, atom.Pre, atom.Blockquote,
		atom.Ul, atom.Ol, atom.Dl, atom.Table, atom.Details, atom.Body, atom.Html:
		return true
	}
	return false
}

func mdChildren(n *html.Node) []*html.Node {
	var out []*html.Node
	for k := n.FirstChild; k != nil; k = k.NextSibling {
		out = append(out, k)
	}
	return out
}

func mdAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// htmlBlock 画一个块级标签。
func (e *mdEnv) htmlBlock(c *wui.Context, n *html.Node, align wui.Align) {
	th := e.th
	em := func(v float32) float32 { return mdEm(th, v) }
	if a := mdAlignOf(`align="` + mdAttr(n, "align") + `"`); a != 0 {
		align = a
	}
	switch n.DataAtom {
	case atom.Center:
		align = wui.Center
	case atom.Hr:
		wui.Box(c).Height(1).Background(th.Rule).AlignSelf(wui.Stretch).Margin(em(0.25), 0)
		return
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		level := int(n.Data[1] - '0')
		md := strings.Repeat("#", level) + " " + strings.ReplaceAll(mdInline(mdChildren(n)), "\n", " ")
		e.nestedHTML(c, md, align)
		return
	case atom.Pre:
		code, lang := mdText(n), ""
		if k := n.FirstChild; k != nil && k.DataAtom == atom.Code {
			for _, cls := range strings.Fields(mdAttr(k, "class")) {
				if l, ok := strings.CutPrefix(cls, "language-"); ok {
					lang = l
				}
			}
		}
		code = strings.TrimSuffix(code, "\n")
		fence := strings.Repeat("`", max(3, mdLongestRun(code, '`')+1))
		e.nestedHTML(c, fence+lang+"\n"+code+"\n"+fence, align)
		return
	case atom.Ul, atom.Ol:
		e.nestedHTML(c, mdList(n, 0), align)
		return
	case atom.Table:
		if md, ok := mdTable(n); ok {
			e.nestedHTML(c, md, align)
			return
		}
	case atom.Details:
		e.htmlDetails(c, n, align)
		return
	case atom.Blockquote:
		q := wui.Column(c).AlignSelf(wui.Stretch).BorderStartWidth(th.QuoteBarWidth).BorderColor(th.QuoteBar).
			PaddingStart(em(0.85)).TextColor(th.QuoteText).Gap(em(0.5))
		q.Children(func() { e.htmlFlow(c, mdChildren(n), align) })
		return
	}
	box := wui.Column(c).AlignSelf(wui.Stretch).Gap(em(0.5))
	mdAlign(box, align)
	box.Children(func() { e.htmlFlow(c, mdChildren(n), align) })
}

// htmlDetails 是一个块里就配平了的 <details>：和跨块的一样折叠，正文是里面的 HTML。
func (e *mdEnv) htmlDetails(c *wui.Context, n *html.Node, align wui.Align) {
	th := e.th
	em := func(v float32) float32 { return mdEm(th, v) }
	summary := i18n.T("详情")
	var body []*html.Node
	for _, k := range mdChildren(n) {
		if k.DataAtom == atom.Summary {
			if s := strings.Join(strings.Fields(mdText(k)), " "); s != "" {
				summary = s
			}
			continue
		}
		body = append(body, k)
	}
	box := wui.Column(c).AlignSelf(wui.Stretch).Border(1, th.CodeBlockBorder).Radius(th.CodeBlockRadius).Clip()
	box.Role(wui.RoleGroup).Label(summary)
	_, open0 := mdAttrOK(n, "open")
	isOpen := wui.Local(box, mdOpenKey{}, func() bool { return open0 })
	box.Children(func() {
		p := wui.CollapsibleBase(c, isOpen)
		p.Trigger.AlignSelf(wui.Stretch).AlignItems(wui.Center).Gap(em(0.4)).Padding(em(0.45), em(0.75)).
			Cursor(wui.CursorPointer).Label(summary)
		if *isOpen {
			p.Trigger.BorderWidth(0, 0, 1, 0).BorderColor(th.CodeBlockBorder)
		}
		p.Trigger.Children(func() {
			ic := iconChevronRight
			if *isOpen {
				ic = iconChevron
			}
			wui.Icon(c, ic).FontSize(iconInline).TextColor(th.Muted)
			wui.Text(c, summary).FontWeight(th.StrongWeight).Grow(1).MinWidth(0)
		})
		p.Panel(func() {
			b := wui.Column(c).AlignSelf(wui.Stretch).Padding(em(0.7), em(0.9)).Gap(em(0.5))
			mdAlign(b, align)
			b.Children(func() { e.htmlFlow(c, body, align) })
		})
	})
}

func mdAttrOK(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

// htmlInline 画连着的一串行内节点：只有图片的画成一排图片，别的转成 Markdown 排成一段。
func (e *mdEnv) htmlInline(c *wui.Context, run []*html.Node, align wui.Align) {
	if items, ok := mdHTMLImageItems(run, "", c.Theme().Dark); ok {
		if len(items) > 0 {
			ne := *e
			ne.align = align
			mdImages(c, &ne, items)
		}
		return
	}
	md := strings.TrimSpace(mdInline(run))
	if md == "" {
		return
	}
	e.nestedHTML(c, md, align)
}

// nestedHTML 把 HTML 转出来的 Markdown 排进一个嵌套的 View。
func (e *mdEnv) nestedHTML(c *wui.Context, md string, align wui.Align) {
	if e.depth >= mdMaxDepth {
		wui.Text(c, md).TextColor(e.th.Muted).Selectable()
		return
	}
	el := wui.Column(c).AlignSelf(wui.Stretch)
	mdAlign(el, align)
	e.nested(c, el, md, align)
}

// mdHTMLImageItems 是一串行内节点里的图片：只有 <img>、<picture>、包着它们的 <a>、<br> 和空白时 ok。
func mdHTMLImageItems(run []*html.Node, href string, dark bool) ([]mdImg, bool) {
	var items []mdImg
	for _, n := range run {
		switch {
		case n.Type == html.CommentNode:
		case n.Type == html.TextNode:
			if strings.TrimSpace(n.Data) != "" {
				return nil, false
			}
		case n.Type != html.ElementNode:
			return nil, false
		case n.DataAtom == atom.Br:
		case n.DataAtom == atom.Img:
			items = append(items, mdHTMLImg(n, href))
		case n.DataAtom == atom.Picture:
			var img *html.Node
			src := ""
			for k := n.FirstChild; k != nil; k = k.NextSibling {
				switch k.DataAtom {
				case atom.Source:
					if src == "" && mdMediaMatches(mdAttr(k, "media"), dark) {
						src = mdSrcset(mdAttr(k, "srcset"))
					}
				case atom.Img:
					img = k
				}
			}
			if img == nil {
				return nil, false
			}
			it := mdHTMLImg(img, href)
			if src != "" {
				it.src = src
			}
			items = append(items, it)
		case n.DataAtom == atom.A || n.DataAtom == atom.Span || n.DataAtom == atom.Sup || n.DataAtom == atom.Sub:
			h := href
			if n.DataAtom == atom.A {
				h = mdAttr(n, "href")
			}
			sub, ok := mdHTMLImageItems(mdChildren(n), h, dark)
			if !ok || len(sub) == 0 && n.DataAtom == atom.A {
				return nil, false
			}
			items = append(items, sub...)
		default:
			return nil, false
		}
	}
	return items, true
}

// mdMediaMatches 是 <source media> 在这个主题下用不用（<picture> 按 prefers-color-scheme 挑深色、浅色的图）。
func mdMediaMatches(media string, dark bool) bool {
	m := strings.ToLower(strings.ReplaceAll(media, " ", ""))
	switch {
	case m == "":
		return true
	case strings.Contains(m, "prefers-color-scheme:dark"):
		return dark
	case strings.Contains(m, "prefers-color-scheme:light"):
		return !dark
	}
	return false
}

// mdSrcset 取 srcset 的第一个地址（"a.png 1x, a@2x.png 2x" → a.png）。
func mdSrcset(s string) string {
	first, _, _ := strings.Cut(s, ",")
	f := strings.Fields(first)
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

func mdHTMLImg(n *html.Node, href string) mdImg {
	it := mdImg{src: mdAttr(n, "src"), alt: mdAttr(n, "alt"), title: mdAttr(n, "title"), href: href}
	if it.src == "" {
		it.src = mdSrcset(mdAttr(n, "srcset"))
	}
	it.w, it.pct = mdSize(mdAttr(n, "width"))
	it.h, _ = mdSize(mdAttr(n, "height"))
	return it
}

// mdText 是节点里的字（<br> 换行）。
func mdText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.TextNode:
			b.WriteString(n.Data)
		case n.Type == html.ElementNode && n.DataAtom == atom.Br:
			b.WriteByte('\n')
		case n.Type == html.ElementNode && mdSkip(n):
		default:
			for k := n.FirstChild; k != nil; k = k.NextSibling {
				walk(k)
			}
		}
	}
	walk(n)
	return b.String()
}

// mdInline 把行内节点转成 Markdown：字里的标记符转义，空白并成一个空格。
func mdInline(nodes []*html.Node) string {
	var b strings.Builder
	for _, n := range nodes {
		mdInlineNode(&b, n)
	}
	return b.String()
}

func mdInlineNode(b *strings.Builder, n *html.Node) {
	wrap := func(open, close string) {
		inner := strings.TrimSpace(mdInline(mdChildren(n)))
		if inner == "" {
			return
		}
		b.WriteString(open + inner + close)
	}
	switch n.Type {
	case html.TextNode:
		b.WriteString(mdEscape(strings.Join(strings.FieldsFunc(n.Data, mdSpace), " "), n.Data))
		return
	case html.ElementNode:
	default:
		return
	}
	switch n.DataAtom {
	case atom.Br:
		b.WriteString("<br>")
	case atom.B, atom.Strong:
		wrap("**", "**")
	case atom.I, atom.Em, atom.Cite, atom.Var, atom.Dfn:
		wrap("*", "*")
	case atom.S, atom.Del, atom.Strike:
		wrap("~~", "~~")
	case atom.Code, atom.Kbd, atom.Samp, atom.Tt:
		code := strings.Join(strings.FieldsFunc(mdText(n), mdSpace), " ")
		if code == "" {
			return
		}
		ticks := strings.Repeat("`", mdLongestRun(code, '`')+1)
		pad := ""
		if strings.HasPrefix(code, "`") || strings.HasSuffix(code, "`") {
			pad = " "
		}
		b.WriteString(ticks + pad + code + pad + ticks)
	case atom.Sub, atom.Sup, atom.Mark, atom.U, atom.Ins, atom.Small:
		b.WriteString("<" + n.Data + ">" + mdInline(mdChildren(n)) + "</" + n.Data + ">")
	case atom.A:
		inner := strings.TrimSpace(mdInline(mdChildren(n)))
		href := mdAttr(n, "href")
		switch {
		case href == "":
			b.WriteString(inner)
		case inner == "":
		default:
			b.WriteString("[" + inner + "](<" + strings.ReplaceAll(href, ">", "%3E") + ">)")
		}
	case atom.Img:
		it := mdHTMLImg(n, "")
		if it.src != "" {
			b.WriteString("![" + mdEscape(it.alt, it.alt) + "](<" + strings.ReplaceAll(it.src, ">", "%3E") + ">)")
		}
	case atom.Picture:
		if items, ok := mdHTMLImageItems([]*html.Node{n}, "", false); ok && len(items) == 1 {
			b.WriteString("![" + mdEscape(items[0].alt, items[0].alt) + "](<" + items[0].src + ">)")
		}
	default:
		if mdSkip(n) {
			return
		}
		b.WriteString(mdInline(mdChildren(n)))
	}
}

func mdSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' }

// mdEscape 转义 s 里会被当成 Markdown 的符号；原文 raw 首尾有空白时 s 首尾留一个空格（行内元素之间的空格）。
func mdEscape(s, raw string) string {
	var b strings.Builder
	if raw != "" && mdSpace(rune(raw[0])) {
		b.WriteByte(' ')
	}
	for _, r := range s {
		if strings.ContainsRune("\\`*_[]<>|#~$!", r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	if s != "" && raw != "" && mdSpace(rune(raw[len(raw)-1])) {
		b.WriteByte(' ')
	}
	return b.String()
}

func mdLongestRun(s string, ch byte) int {
	best, n := 0, 0
	for i := 0; i < len(s); i++ {
		if s[i] == ch {
			n++
			best = max(best, n)
		} else {
			n = 0
		}
	}
	return best
}

// mdList 把 <ul> / <ol> 转成 Markdown 列表（套着的缩进）。
func mdList(n *html.Node, depth int) string {
	var b strings.Builder
	k := 0
	for li := n.FirstChild; li != nil; li = li.NextSibling {
		if li.DataAtom != atom.Li {
			continue
		}
		k++
		marker := "- "
		if n.DataAtom == atom.Ol {
			marker = strconv.Itoa(k) + ". "
		}
		var inline []*html.Node
		var subs []string
		for ch := li.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.DataAtom == atom.Ul || ch.DataAtom == atom.Ol {
				subs = append(subs, mdList(ch, depth+1))
				continue
			}
			inline = append(inline, ch)
		}
		indent := strings.Repeat("   ", depth)
		b.WriteString(indent + marker + strings.ReplaceAll(strings.TrimSpace(mdInline(inline)), "\n", " ") + "\n")
		for _, s := range subs {
			b.WriteString(s)
		}
	}
	return b.String()
}

// mdTable 把 <table> 转成 Markdown 表格：第一行是表头（<thead> 里的，或第一行）；单元格里有块级的（列表、套表格）就不转。
func mdTable(n *html.Node) (string, bool) {
	var rows [][]string
	var aligns []string
	var walk func(*html.Node) bool
	walk = func(n *html.Node) bool {
		for k := n.FirstChild; k != nil; k = k.NextSibling {
			switch k.DataAtom {
			case atom.Thead, atom.Tbody, atom.Tfoot:
				if !walk(k) {
					return false
				}
			case atom.Tr:
				var row []string
				for cell := k.FirstChild; cell != nil; cell = cell.NextSibling {
					if cell.DataAtom != atom.Td && cell.DataAtom != atom.Th {
						continue
					}
					for d := cell.FirstChild; d != nil; d = d.NextSibling {
						if d.Type == html.ElementNode && mdBlockTag(d) && d.DataAtom != atom.P && d.DataAtom != atom.Div {
							return false
						}
					}
					if len(rows) == 0 {
						aligns = append(aligns, strings.ToLower(mdAttr(cell, "align")))
					}
					s := strings.TrimSpace(mdInline(mdChildren(cell)))
					s = strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "|", `\|`)
					row = append(row, s)
				}
				rows = append(rows, row)
			}
		}
		return true
	}
	if !walk(n) || len(rows) == 0 {
		return "", false
	}
	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	if cols == 0 {
		return "", false
	}
	var b strings.Builder
	line := func(r []string) {
		b.WriteString("|")
		for i := range cols {
			s := ""
			if i < len(r) {
				s = r[i]
			}
			b.WriteString(" " + s + " |")
		}
		b.WriteString("\n")
	}
	line(rows[0])
	b.WriteString("|")
	for i := range cols {
		a := ""
		if i < len(aligns) {
			a = aligns[i]
		}
		switch a {
		case "center":
			b.WriteString(" :---: |")
		case "right":
			b.WriteString(" ---: |")
		default:
			b.WriteString(" --- |")
		}
	}
	b.WriteString("\n")
	for _, r := range rows[1:] {
		line(r)
	}
	return b.String(), true
}
