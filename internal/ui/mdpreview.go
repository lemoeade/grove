package ui

import (
	"slices"
	"sort"
	"strings"
	"time"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/mdprep"
)

// 这个文件是 Markdown 标签的预览（⇧⌘V）和旁边的预览标签（docs.MDPreview）：
//   - 源码先过 mdprep（front matter、跨块的 <details> / <div align> 换成带编号的围栏），围栏在 CodeBlock 钩子里画成卡片、折叠块；
//   - Block 钩子接手只有图片（和包着图片的链接）的段落、带图片的表格、放跳转记号；HTML 块见 mdhtml.go；
//   - 打字时源码停下来 mdSettle 才重排（预览一直开在旁边时，每敲一个字整篇重排太费）；
//   - 大文件（mdChunkMin 以上、没有脚注）按顶层块分段放进虚拟列表，只排看得见的几段；
//   - 状态放在 d.MDView：切回源码再切回来不重新整理，列表的滚动位置也在。

const (
	mdSettle    = 150 * time.Millisecond // 源码停下来多久才重排
	mdIdle      = 500 * time.Millisecond // 这么久没画过（刚切过来）：不等，马上排
	mdChunkMin  = 32 << 10               // 整理后的源码超过这么多字节才分段
	mdChunkHead = 512                    // 一段到这么多字节后遇到标题就另起一段
	mdChunkMax  = 4 << 10                // 一段到这么多字节后遇到哪个块都另起一段
	mdMaxDepth  = 6                      // <details> 套 <details>…最多套几层（再深的照源码显示）
)

// mdView 是一个预览的状态（Doc.MDView）。
type mdView struct {
	raw       string    // 正在显示的源码
	pending   string    // 打字时新来的源码：停下来 mdSettle 才换上
	pendingAt time.Time //
	drawn     time.Time // 上次画的时候
	prep      *mdprep.Doc
	lines     []int     // prep.Src 每行开头的偏移
	chunks    []mdChunk // 分段；nil 是不分段，一个 View 画完
	list      wui.ListState
	remoteOff bool // 选项建的时候设置里关没关网上的图片
	seek      int  // 跳转（#标题、行）滚到了第几段还没放上记号的帧数
	env       mdEnv
	srcTop    int  // 并排预览：上一帧源文件编辑器最上面那一行；变了预览跟过去
	srcSeen   bool // srcTop 记过了
}

// mdChunk 是大文件的一段：几个相连的顶层块。
type mdChunk struct {
	start, end int    // 在 prep.Src 里的偏移
	src        string // 这段的源码，后面接上全文的链接引用定义
	line       int    // 第一行（从 0 起）
	head       int    // 第一块是标题时它的级别（定上边距）
	slugs      []string
}

// mdEnv 是画一个 View 时钩子要用的：顶层的（v 不为空）还管跳转记号；<details> 里面、HTML 里的段落是嵌套的。
type mdEnv struct {
	a     *app.App
	d     *docs.Doc // 预览所在的标签：链接、图片相对它的目录，跳转记号看它的 Anchor / AnchorLine
	prep  *mdprep.Doc
	th    *markdown.Theme
	v     *mdView // 顶层的才有
	base  int     // 正在画的那段在 prep.Src 里的偏移
	cur   string  // 正在画的那段的源码（认出顶层文档：表格单元格里的小 View 不放行号记号）
	end   int     // 正在画的那段在 prep.Src 里到哪为止；-1 是一直到文末
	depth int
	align wui.Align // 外面的 <div align> / <p align>：图片一行靠哪边
	links map[string]string
	// anchor、line 是这一帧开始时的 d.Anchor / d.AnchorLine：只认帧开始时就有的跳转。这一帧里点链接时设的，
	// 留到下一帧再放记号（点击在这一帧排版后才处理，这一帧放的记号滚不过去）
	anchor string
	line   int
	o      *markdown.Options // setup 挂上钩子的那份
}

// mdViewOf 是 d 的预览状态，没有就建一个。
func mdViewOf(d *docs.Doc) *mdView {
	if v, ok := d.MDView.(*mdView); ok {
		return v
	}
	v := &mdView{}
	d.MDView = v
	return v
}

// mdHidden 记下 d 这一帧画的是源码不是预览：切回预览时直接换上新源码，不等打字停下（docContent 里调）。
func mdHidden(d *docs.Doc) {
	if v, ok := d.MDView.(*mdView); ok {
		v.drawn = time.Time{}
	}
}

// mdPreviewSource 是预览要显示的源码：旁边的预览标签显示源标签的（源标签关了为空、ok 为 false）。
func mdPreviewSource(a *app.App, d *docs.Doc) (string, bool) {
	if d.Kind != docs.MDPreview {
		return d.MarkdownSource(), true
	}
	if s := d.MDSource; s != nil && slices.Contains(a.AllDocs(), s) {
		return s.MarkdownSource(), true
	}
	return "", false
}

// markdownPreview 是 Markdown 的预览：居中的一栏（大文件是虚拟列表），点链接见 App.MarkdownLink。
// 点了 #标题 的链接（d.Anchor）、大纲里点了一项或旁边的编辑器滚了（d.AnchorLine）后，在那一块上放一个看不见、
// 比窗口还高的记号，让它出现在视图里——视图只滚到刚好露出它，记号比视图高，那一块就到了最上面；没有这个标题就提示。
func markdownPreview(a *app.App, c *wui.Context, d *docs.Doc) {
	src, ok := mdPreviewSource(a, d)
	if !ok {
		message(c, iconEye, i18n.T("源文件的标签关了"), app.KeyHint("view.markdownSidePreview", "在源文件上按 %s 再开旁边的预览", "在源文件上再开旁边的预览"), c.Theme().TextMuted)
		return
	}
	v := mdViewOf(d)
	if d.Kind == docs.MDPreview { // 跟着源文件的编辑器滚：最上面那一行变了，预览把那一行所在的块滚到最上面
		if first, last := d.MDSource.St.VisibleLines(); last > 0 {
			if v.srcSeen && first != v.srcTop {
				d.AnchorLine = first + 1
			}
			v.srcTop, v.srcSeen = first, true
		}
	}
	label := i18n.Tf("预览 %s", d.Title)
	if d.Kind == docs.MDPreview {
		label = d.Title // 标题已经是“预览 guide.md”
	}
	now := c.Now()
	switch {
	case v.prep == nil || now.Sub(v.drawn) > mdIdle:
		v.adopt(src)
	case src == v.raw:
		v.pending = ""
	default:
		if src != v.pending || v.pendingAt.IsZero() {
			v.pending, v.pendingAt = src, now
		}
		if wait := v.pendingAt.Add(mdSettle).Sub(now); wait > 0 {
			c.After(wait)
		} else {
			v.adopt(src)
		}
	}
	v.drawn = now

	off := a.St.Settings.NoRemoteImages
	if d.MD != nil && v.remoteOff != off {
		d.MD = nil // 换了图片的设置：重建选项（里面有 mygo 的图片缓存）
	}
	v.remoteOff = off
	o := a.MarkdownOptions(d)
	e := &v.env
	*e = mdEnv{a: a, d: d, prep: v.prep, th: mdTheme(a, c), v: v, links: e.links}
	if e.links == nil || len(e.links) > 512 {
		e.links = map[string]string{}
	}
	name, line := d.Anchor, d.AnchorLine
	e.anchor, e.line = name, line
	gen := e.setup(c, o)
	o.ShareSelection = v.chunks != nil

	if v.chunks != nil {
		v.seekChunk(c)
		th := e.th
		list := wui.List(c, &v.list, len(v.chunks), func(i int) {
			ch := &v.chunks[i]
			top, bottom := float32(0), float32(0)
			switch {
			case i == 0:
				top = 20
			case ch.head > 0:
				top = max(mdEm(th, th.BlockSpacing), mdEm(th, th.Headings[ch.head-1].Top))
			default:
				top = mdEm(th, th.BlockSpacing)
			}
			if i == len(v.chunks)-1 {
				bottom = 48
			}
			wui.Column(c.Key(previewGen{d, gen})).FillWidth().MaxWidth(860).AlignSelf(wui.Center).Padding(top, 32, bottom).Children(func() {
				e.base, e.cur, e.end = ch.start, ch.src, ch.end
				if i == len(v.chunks)-1 {
					e.end = -1
				}
				markdown.View(c, ch.src, o)
			})
		})
		list.Grow(1).MinHeight(0).Selectable().Label(label)
	} else {
		wui.Scroll(c.Key(previewKey{d})).Grow(1).MinHeight(0).Label(label).Children(func() {
			wui.Column(c.Key(previewGen{d, gen})).FillWidth().MaxWidth(860).AlignSelf(wui.Center).Padding(20, 32, 48).Children(func() {
				e.base, e.cur, e.end = 0, v.prep.Src, -1
				markdown.View(c, v.prep.Src, o)
			})
		})
	}
	switch {
	case v.chunks != nil && (d.Anchor != "" || d.AnchorLine > 0) && v.seek > 0 && v.seek < 4:
		c.Invalidate() // 滚到了那一段，等它排出来放记号
	case d.Anchor != "" && d.Anchor == name:
		a.Toast = i18n.Tf("没有叫 #%s 的标题", d.Anchor)
		d.Anchor, v.seek = "", 0
	case d.Anchor != "":
		c.Invalidate() // 这一帧画链接时点的，下一帧再找
	}
	if d.AnchorLine > 0 && d.AnchorLine == line && (v.chunks == nil || v.seek >= 4) {
		d.AnchorLine, v.seek = 0, 0 // 那一行没有块（脚注、文末空行）：不找了
	}
}

type previewKey struct{ d *docs.Doc }

// previewGen 是预览里那一栏的 key：渲染器有了新的行内公式（gen 变了）就换一个，段落重排（markdown 的段落缓存只认块变没变）。
type previewGen struct {
	d   *docs.Doc
	gen uint64
}

// adopt 换上新的源码：整理、记下行首，大的分段。
func (v *mdView) adopt(src string) {
	v.raw, v.pending, v.pendingAt = src, "", time.Time{}
	if v.prep != nil && v.prep.Src == src && len(v.prep.Regions) == 0 && v.prep.Defs == "" {
		return
	}
	v.prep = mdprep.Prepare(src)
	v.lines = v.lines[:0]
	v.lines = append(v.lines, 0)
	s := v.prep.Src
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			v.lines = append(v.lines, i+1)
		}
	}
	v.chunks = nil
	if len(s) > mdChunkMin && !v.prep.Footnotes {
		v.chunks = mdSplit(s, v.prep.Defs, v.lineOf)
	}
}

// lineOf 是 prep.Src 里偏移 off 在第几行（从 0 起）。
func (v *mdView) lineOf(off int) int {
	return sort.SearchInts(v.lines, off+1) - 1
}

// mdSplit 把 src 按顶层块分段：一段到 mdChunkHead 后遇到标题、到 mdChunkMax 后遇到哪块都另起；段从块所在行的行首起。
func mdSplit(src, defs string, lineOf func(int) int) []mdChunk {
	doc := parser.New(parser.Default | parser.ExtDollarMath)
	doc.Set(src)
	var chunks []mdChunk
	cur := mdChunk{head: -1}
	for j := range doc.Children(0) {
		b := doc.Block(j)
		ls := strings.LastIndexByte(src[:b.Start], '\n') + 1
		if ls > cur.start && (b.Kind == parser.Heading && ls-cur.start >= mdChunkHead || ls-cur.start >= mdChunkMax) {
			cur.end = ls
			chunks = append(chunks, cur)
			cur = mdChunk{start: ls, head: -1}
		}
		if cur.head < 0 {
			cur.head = 0
			if b.Kind == parser.Heading {
				cur.head = b.Level
			}
		}
		if b.Kind == parser.Heading {
			cur.slugs = append(cur.slugs, markdown.Slug(parser.PlainText(doc.Inlines(j))))
		}
	}
	cur.end = len(src)
	chunks = append(chunks, cur)
	for i := range chunks {
		ch := &chunks[i]
		ch.head = max(ch.head, 0)
		ch.src = src[ch.start:ch.end]
		if defs != "" {
			ch.src += "\n\n" + defs
		}
		ch.line = lineOf(ch.start)
	}
	return chunks
}

// seekChunk 在分段的预览里有跳转时把那一段滚到最上面（下一帧那段排出来，钩子再在块上放记号）。
func (v *mdView) seekChunk(c *wui.Context) {
	d := v.env.d
	if d.Anchor == "" && d.AnchorLine <= 0 {
		v.seek = 0
		return
	}
	k := -1
	if d.AnchorLine > 0 {
		L := d.AnchorLine - 1
		k = sort.Search(len(v.chunks), func(i int) bool { return v.chunks[i].line > L }) - 1
		k = max(k, 0)
	} else {
	find:
		for i, ch := range v.chunks {
			for _, s := range ch.slugs {
				if s == d.Anchor {
					k = i
					break find
				}
			}
		}
	}
	if k < 0 {
		v.seek = 4 // 没有这个标题：markdownPreview 提示
		return
	}
	if v.seek == 0 {
		v.list.ScrollTo(k, wui.Start)
	}
	v.seek++
}

// setup 给 o 挂上这个预览的钩子，返回行内公式的代数（见 mdRender）。
func (e *mdEnv) setup(c *wui.Context, o *markdown.Options) uint64 {
	gen := mdRender(e.a, c, o)
	e.o = o
	o.Theme = e.th
	o.HTML = func(c *wui.Context, html string) { mdHTML(c, e, html) }
	o.CodeBlock = e.codeBlock
	o.Block = e.block
	a, d := e.a, e.d
	o.LinkMenu = func(m *wui.Menu, u string) { mdLinkMenu(a, d, m, u) }
	o.Preview = func(c *wui.Context, dest string) func(wui.Element) { return mdLinkPreview(a, d, c, dest) }
	o.CodeLink = e.codeLink
	return gen
}

// block 是 Block 钩子：先放跳转记号，再接手只有图片的段落、带图片的表格。
func (e *mdEnv) block(c *wui.Context, doc *parser.Document, i int) bool {
	b := doc.Block(i)
	d := e.d
	// 分段的预览：seekChunk 这一帧才 ScrollTo 的话等下一帧（列表还没滚过去，这一帧放的记号白放）
	ready := e.v == nil || e.v.chunks == nil || e.v.seek >= 2
	if ready && e.anchor != "" && d.Anchor == e.anchor && b.Kind == parser.Heading && markdown.Slug(parser.PlainText(doc.Inlines(i))) == e.anchor {
		mdMarker(c)
		d.Anchor = ""
		if e.v != nil {
			e.v.seek = 0
		}
	}
	if ready && e.line > 0 && d.AnchorLine == e.line && e.v != nil && b.Parent == 0 && doc.Source() == e.cur {
		L := e.line - 1
		start := e.v.lineOf(e.base + b.Start)
		next := int(^uint(0) >> 1)
		switch {
		case b.Next < doc.Len():
			next = e.v.lineOf(e.base + doc.Block(b.Next).Start)
		case e.end >= 0: // 这一段的最后一块：到下一段的第一行为止，不是文末
			next = e.v.lineOf(e.end)
		}
		if L < next && (start <= L || i == 1) {
			mdMarker(c)
			d.AnchorLine, e.v.seek = 0, 0
		}
	}
	switch b.Kind {
	case parser.Paragraph:
		if items := mdImageItems(doc, i); items != nil {
			mdImages(c, e, items)
			return true
		}
		if mdHTMLImages(doc.Inlines(i)) {
			mdHTML(c, e, strings.TrimRight(doc.Lines(i), "\n"))
			return true
		}
	case parser.Table:
		if tableHasImage(doc, i) {
			return mdImageTable(c, e.options(), doc, i)
		}
	}
	return false
}

// options 是给表格单元格里的小 View 用的选项模板（mdImageTable 拷字段）：图片、链接、钩子照这个预览的。
func (e *mdEnv) options() *markdown.Options {
	o := &markdown.Options{}
	ne := &mdEnv{a: e.a, d: e.d, prep: &mdprep.Doc{}, th: e.th, depth: e.depth + 1, links: e.links, anchor: e.anchor}
	o.Theme, o.LoadImage = e.th, e.a.ImageLoader(e.d)
	if e.o != nil {
		o.Extensions, o.InlineMath, o.LinkMenu, o.Preview = e.o.Extensions, e.o.InlineMath, e.o.LinkMenu, e.o.Preview
	}
	o.OnLink = func(u string) { e.a.MarkdownLink(e.d, u) }
	o.HTML = func(c *wui.Context, html string) { mdHTML(c, ne, html) }
	o.Block = ne.block
	o.CodeLink = e.codeLink
	return o
}

// mdMarker 是跳转记号：看不见、比窗口高，让它所在的块滚到视图最上面。
func mdMarker(c *wui.Context) {
	wui.Box(c).Absolute().Top(0).Left(0).Width(1).Height(1e4).ScrollIntoView()
}

// nested 在 el 里画一段嵌套的 Markdown（<details> 的正文、HTML 里的段落）：选项、整理结果留在 el 上，和外面共用选区。
func (e *mdEnv) nested(c *wui.Context, el wui.Element, src string, align wui.Align) {
	st := *wui.Local(el, mdNestKey{}, func() *mdNest { return &mdNest{o: &markdown.Options{ShareSelection: true}} })
	if st.prep == nil || st.src != src {
		st.src, st.prep = src, mdprep.Prepare(src)
	}
	if st.remoteOff != e.a.St.Settings.NoRemoteImages {
		st.remoteOff = !st.remoteOff
		st.o = &markdown.Options{ShareSelection: true}
	}
	o := st.o
	o.LoadImage = e.a.ImageLoader(e.d)
	a, d := e.a, e.d
	o.OnLink = func(u string) { a.MarkdownLink(d, u) }
	ne := &mdEnv{a: a, d: d, prep: st.prep, th: e.th, depth: e.depth + 1, align: align, links: e.links, anchor: e.anchor}
	ne.setup(c, o)
	el.Children(func() { markdown.View(c, st.prep.Src, o) })
}

type mdNestKey struct{}

type mdNest struct {
	src       string
	prep      *mdprep.Doc
	o         *markdown.Options
	remoteOff bool
}
