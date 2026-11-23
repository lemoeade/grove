package gmd

import (
	"slices"
	"strings"
	"sync"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// View 画一篇 Markdown：一栏块，段落是带样式的字（RichText），链接是里面的 wui.Link。状态（解析结果、段落的字、
// 代码块的高亮、表格的列宽、HTML 块排好的样式）留在返回的元素上，src 没变的块下一帧不重算。
func View(c *wui.Context, src string, o *Options) wui.Element {
	if o == nil {
		o = &defaultOptions
	}
	root := wui.Column(c)
	st := *wui.Local(root, stateKey{}, newState)
	st.o = o
	ext := o.Extensions
	if ext == 0 {
		ext = parser.Default
	}
	if st.doc == nil || st.ext != ext {
		st.doc, st.ext = parser.New(ext), ext
		st.forget()
	}
	st.theme(c)
	th := &st.th
	if src != st.doc.Source() {
		if !strings.HasPrefix(src, st.doc.Source()) {
			st.forget()
		}
		st.doc.Set(src)
	}
	root.FontSize(th.FontSize).LineHeight(th.LineHeight).TextColor(th.Text)
	if th.Font != "" {
		root.Font(th.Font)
	}
	if o.ShareSelection {
		if outer := outerView(c); outer != nil {
			outer.nested = append(outer.nested, nestedView{st, outer.cur})
		}
	} else {
		root.Selectable()
		root.CopyData(st.copyData)
		root.ContextMenu(func(m *wui.Menu) {
			m.EditItems()
			if m.Item(o.Strings.or(o.Strings.CopyAsMarkdown, "Copy as Markdown")).Chosen() {
				c.Services().WriteClipboardData(markdownData(st.copyMarkdown(root.SelectedTexts())))
			}
		})
		if root.FocusWithin() && root.Shortcut(wui.Cmd|wui.Shift, wui.KeyC) {
			c.Services().WriteClipboardData(markdownData(st.copyMarkdown(root.SelectedTexts())))
		}
	}
	st.elems = st.elems[:0]
	clear(st.nested)
	st.nested, st.cur = st.nested[:0], 0
	st.numberNotes()
	root.Children(func() {
		prev := outerView(c)
		setBuilding(c, st)
		defer setBuilding(c, prev)
		st.blocks(c, 0, false, spanStyle{})
		st.footnotes(c)
	})
	st.sweep()
	return root
}

var defaultOptions Options

type stateKey struct{}

// viewState 是一个视图一帧一帧留着的。
type viewState struct {
	o     *Options
	doc   *parser.Document
	ext   parser.Extensions
	uiTh  wui.Theme
	def   *markdown.Theme
	th    markdown.Theme
	paras []paraCache
	elems []elemInfo // 复制用：每个字的元素是哪一块（代码块还有第几行起）
	codes map[int]*codeState
	// tables 按表格在源码里的偏移留列宽；htmls 按 HTML 块的偏移留解析、排好样式的树
	tables   map[int]*tableState
	htmls    map[int]*htmlState
	sheet    *sheetCache // 全文样式表 + 文档里所有 <style>：源码变了重收
	scrollTo int         // 要滚进视图的块，-1 是没有
	notes    map[string]int
	sb       spanBuilder
	// nested 是建在这个视图的块里、和它共用选区的视图（复制成 Markdown 时按块排进来）；cur 是正在建的块
	nested []nestedView
	cur    int
}

type nestedView struct {
	st    *viewState
	block int
}

// building 记着每个窗口的 Context 上正在建块的视图：里面共用选区的视图找到它、把自己记进去。
var building struct {
	sync.Mutex
	m map[*wui.Context]*viewState
}

func outerView(c *wui.Context) *viewState {
	building.Lock()
	defer building.Unlock()
	return building.m[c]
}

func setBuilding(c *wui.Context, st *viewState) {
	building.Lock()
	defer building.Unlock()
	if st == nil {
		delete(building.m, c)
		return
	}
	if building.m == nil {
		building.m = map[*wui.Context]*viewState{}
	}
	building.m[c] = st
}

type paraCache struct {
	start, end int
	kind       parser.Kind
	rev        uint32
	base       spanStyle
	ok         bool
	p          para
}

type elemInfo struct {
	id    uint64
	block int
	line  int // 代码块的字：这一段从第几行起；-1 是段落的字；-2 是 HTML 块里的字
}

func newState() *viewState { return &viewState{scrollTo: -1} }

// theme 算这一帧的样子；变了的话段落的字、代码的高亮都作废。
func (st *viewState) theme(c *wui.Context) {
	t := c.Theme()
	if st.def == nil || st.uiTh != *t {
		st.uiTh = *t
		st.def = markdown.DefaultTheme(t)
	}
	th := resolve(st.o.Theme, st.def)
	if th != st.th {
		st.th = th
		clear(st.paras)
		for _, cs := range st.codes {
			cs.chunks = cs.chunks[:0]
		}
		clear(st.htmls)
	}
	st.sb.d, st.sb.th = st.doc, &st.th
}

// forget 丢掉按块留着的东西（换了一篇）。
func (st *viewState) forget() {
	st.paras = st.paras[:0]
	clear(st.codes)
	clear(st.tables)
	clear(st.htmls)
	st.sheet = nil
}

// sweep 丢掉这一帧没画到的代码块、表格、HTML 块的状态（源码里没了）。
func (st *viewState) sweep() {
	for k, cs := range st.codes {
		if !cs.seen {
			delete(st.codes, k)
		}
		cs.seen = false
	}
	for k, ts := range st.tables {
		if !ts.seen {
			delete(st.tables, k)
		}
		ts.seen = false
	}
	for k, hs := range st.htmls {
		if !hs.seen {
			delete(st.htmls, k)
		}
		hs.seen = false
	}
}

func (st *viewState) note(id uint64, block, line int) {
	st.elems = append(st.elems, elemInfo{id, block, line})
}

// em 是 v 个 em 的 DIP。
func em(th *markdown.Theme, v float32) float32 { return v * th.FontSize }

// blocks 画 parent 的子块，上下隔开；tight 是紧凑列表的项。
func (st *viewState) blocks(c *wui.Context, parent int, tight bool, base spanStyle) {
	d := st.doc
	th := &st.th
	first := true
	prevHeading := 0
	for j := parent + 1; j < d.Block(parent).Next; j = d.Block(j).Next {
		b := d.Block(j)
		if b.Kind == parser.FootnoteDefinition || b.Kind == parser.Paragraph && d.LineCount(j) == 0 {
			continue // 脚注在文末；提示块（> [!NOTE]）那一行记号不显示
		}
		if b.Kind == parser.HTMLBlock && st.o.HTML == nil && invisible(d.Lines(j)) {
			continue // 只有 <style>、注释的 HTML 块：不占地方
		}
		top := float32(0)
		if !first {
			top = em(th, th.BlockSpacing)
			if tight {
				top = em(th, th.ItemSpacing)
			}
			if prevHeading > 0 {
				top = em(th, th.Headings[prevHeading-1].Bottom)
			}
			if b.Kind == parser.Heading {
				top = max(top, em(th, th.Headings[b.Level-1].Top))
			}
		}
		el := st.block(c, j, tight, base)
		if top > 0 {
			el.Margin(top, 0, 0, 0)
		}
		if st.scrollTo == j {
			el.ScrollIntoView()
			st.scrollTo = -1
		}
		prevHeading = 0
		if b.Kind == parser.Heading {
			prevHeading = b.Level
		}
		first = false
	}
}

// block 画第 i 块：先问 Options.Block。
func (st *viewState) block(c *wui.Context, i int, tight bool, base spanStyle) wui.Element {
	defer func(prev int) { st.cur = prev }(st.cur)
	st.cur = i
	if f := st.o.Block; f != nil {
		box := wui.Column(c)
		done := false
		box.Children(func() { done = f(c, st.doc, i) })
		if done {
			return box
		}
		return box.Children(func() { st.defaultBlock(c, i, tight, base) })
	}
	return st.defaultBlock(c, i, tight, base)
}

func (st *viewState) defaultBlock(c *wui.Context, i int, tight bool, base spanStyle) wui.Element {
	d := st.doc
	th := &st.th
	b := d.Block(i)
	switch b.Kind {
	case parser.Paragraph:
		if imagesOnly(d.Inlines(i)) {
			return st.images(c, i)
		}
		return st.paragraph(c, i, base)
	case parser.Heading:
		h := th.Headings[b.Level-1]
		s := base
		s.weight, s.size, s.color = h.Weight, em(th, h.Size), h.Color
		el := st.paragraph(c, i, s).Role(wui.RoleHeading).Level(b.Level)
		if h.Rule {
			el.BorderWidth(0, 0, 1, 0).BorderColor(th.Rule).Padding(0, 0, em(th, 0.3), 0)
		}
		return el
	case parser.ThematicBreak:
		return wui.Box(c).Height(1).Background(th.Rule).AlignSelf(wui.Stretch).Margin(em(th, 0.25), 0)
	case parser.CodeBlock:
		return st.codeBlock(c, i)
	case parser.HTMLBlock:
		html := strings.TrimRight(d.Lines(i), "\n")
		if f := st.o.HTML; f != nil {
			box := wui.Column(c)
			return box.Children(func() { f(c, html) })
		}
		return st.htmlBlock(c, i, html)
	case parser.MathBlock:
		tex := strings.TrimRight(d.Lines(i), "\n")
		if f := st.o.Math; f != nil {
			box := wui.Column(c)
			return box.Children(func() { f(c, tex) })
		}
		box := wui.ScrollHorizontal(c).AlignSelf(wui.Stretch)
		return box.Children(func() {
			el := wui.Text(c, tex).Font(th.CodeFont).FontSize(em(th, th.CodeSize)).TextColor(th.Muted).NoWrap().Margin(0, wui.Auto)
			st.note(el.ID(), i, 0)
		})
	case parser.Quote:
		if b.Alert != parser.NoAlert {
			return st.alert(c, i, b.Alert, base)
		}
		s := base
		s.color = th.QuoteText
		q := wui.Column(c).BorderStartWidth(th.QuoteBarWidth).BorderColor(th.QuoteBar).PaddingStart(em(th, 0.85)).TextColor(th.QuoteText)
		return q.Children(func() { st.blocks(c, i, false, s) })
	case parser.List:
		return st.list(c, i, base)
	case parser.Table:
		return st.table(c, i, base)
	}
	return wui.Box(c)
}

// alert 是 GitHub 的提示块：竖条、图标和标题用它那一种的颜色，字照正文的颜色。
func (st *viewState) alert(c *wui.Context, i int, a parser.Alert, base spanStyle) wui.Element {
	th := &st.th
	color := th.Alerts[a-1]
	q := wui.Column(c).BorderStartWidth(th.QuoteBarWidth).BorderColor(color).PaddingStart(em(th, 0.85))
	return q.Children(func() {
		title := wui.Row(c).AlignItems(wui.Center).Gap(em(th, 0.5)).TextColor(color).Margin(0, 0, em(th, th.ItemSpacing*2), 0)
		title.Children(func() {
			if icon := alertIcon(a); icon != nil {
				wui.Icon(c, icon)
			}
			wui.Text(c, a.String()).FontWeight(th.StrongWeight)
		})
		st.blocks(c, i, false, base)
	})
}

// para 是第 i 块（段落、标题、单元格）在 base 样式下的字，块没变就用上次的。
func (st *viewState) para(i int, base spanStyle) *para {
	for len(st.paras) < st.doc.Len() {
		st.paras = append(st.paras, paraCache{})
	}
	b := st.doc.Block(i)
	st.doc.Inlines(i) // 先解析：Revision 才是新的
	pc := &st.paras[i]
	if pc.ok && pc.start == b.Start && pc.end == b.End && pc.kind == b.Kind && pc.rev == b.Revision() && pc.base == base && st.sameLinks(i, &pc.p) {
		return &pc.p
	}
	pc.start, pc.end, pc.kind, pc.rev, pc.base, pc.ok = b.Start, b.End, b.Kind, b.Revision(), base, true
	sb := &st.sb
	sb.block = i
	sb.notes = st.noteNumber
	sb.math = st.o.InlineMath
	sb.codeLink = st.o.CodeLink
	sb.build(&pc.p, base)
	return &pc.p
}

// sameLinks 是第 i 块的行内代码现在成不成链接、链到哪，和建 p 的时候一样（CodeLink 的答案会变：文件新建了）。
func (st *viewState) sameLinks(i int, p *para) bool {
	if (st.o.CodeLink != nil) != p.codeLinks {
		return false
	}
	if len(p.refs) == 0 {
		return true
	}
	in := st.doc.Inlines(i)
	for k, x := range p.refs {
		dest, ok := st.o.CodeLink(in[x].Text)
		if !ok {
			dest = ""
		}
		if dest != p.resolved[k] {
			return false
		}
	}
	return true
}

// paragraph 画第 i 块的字：段落、标题或单元格。
func (st *viewState) paragraph(c *wui.Context, i int, base spanStyle) wui.Element {
	p := st.para(i, base)
	ps := p.pieces
	var el wui.Element
	switch {
	case len(ps) == 0:
		el = wui.Text(c, "")
	case ps[0].link == -1:
		el = wui.RichText(c, ps[0].spans...)
		ps = ps[1:]
	default:
		el = wui.RichText(c)
	}
	if len(ps) > 0 {
		el.Children(func() {
			for k := range ps {
				st.piece(c, i, &ps[k])
			}
		})
	}
	if objects := p.objects; len(objects) > 0 {
		el.DrawObjects(func(pa *wui.Painter, at int, r wui.Rect) {
			if k, ok := slices.BinarySearchFunc(objects, at, func(o mathObject, at int) int { return o.rune - at }); ok && objects[k].draw != nil {
				objects[k].draw(pa, r)
			}
		})
	}
	st.note(el.ID(), i, -1)
	return el
}

// piece 画段落里的一个链接，或链接后面的字。
func (st *viewState) piece(c *wui.Context, block int, pc *piece) {
	switch {
	case pc.link == -1:
		wui.RichText(c, pc.spans...)
	case pc.link < -1: // 脚注的引用：点了滚到脚注
		in := st.doc.Inlines(block)[-2-pc.link]
		l := wui.Link(c, "", "")
		l.Children(func() { wui.RichText(c, pc.spans...) })
		if l.Clicked() {
			if i, ok := st.doc.Footnote(in.Text); ok {
				st.scrollTo = i
			}
		}
	default:
		in := st.doc.Inlines(block)[pc.link]
		if pc.dest != "" { // CodeLink 让行内代码成了链接
			in.Dest, in.Auto = pc.dest, true
		}
		st.link(c, in.Dest, in.Title, in.Auto, func() { wui.RichText(c, pc.spans...) })
	}
}

// link 画一个到 dest 的链接，字由 children 画。
func (st *viewState) link(c *wui.Context, dest, title string, auto bool, children func()) wui.Element {
	o := st.o
	var l wui.Element
	switch {
	case o.OnLink != nil:
		l = wui.Link(c, "", "")
		if l.Clicked() {
			o.OnLink(dest)
		}
	case strings.HasPrefix(dest, "#"):
		l = wui.Link(c, "", "")
		if l.Clicked() {
			st.scrollTo = st.headingNamed(dest[1:])
		}
	case linkAllowed(dest):
		l = wui.Link(c, "", dest)
	default:
		l = wui.Link(c, "", "")
	}
	if o.LinkMenu != nil && dest != "" && !strings.HasPrefix(dest, "#") {
		l.ContextMenu(func(m *wui.Menu) { o.LinkMenu(m, dest) })
	}
	l.TextColor(st.th.Link)
	l.Children(children)
	tip := title
	if tip == "" && !auto && dest != "" && !strings.HasPrefix(dest, "#") {
		tip = dest
	}
	if o.Preview != nil && dest != "" {
		if fill := o.Preview(c, dest); fill != nil {
			if tip != "" {
				l.Description(tip)
			}
			wui.TooltipBase(c, l, fill)
			return l
		}
	}
	if tip != "" {
		l.Tooltip(tip)
	}
	return l
}

// linkAllowed：没有 OnLink 时哪些链接直接开（wui.SafeToOpen 认的地址）。
func linkAllowed(dest string) bool {
	if strings.ContainsAny(dest, "\\\x00") || strings.HasPrefix(dest, "//") {
		return false
	}
	if i := strings.IndexAny(dest, ":/?#"); i < 0 || dest[i] != ':' {
		return false
	}
	return wui.SafeToOpen(dest)
}

// headingNamed 是 slug 为 name 的标题（GitHub 的算法），没有是 -1。
func (st *viewState) headingNamed(name string) int {
	d := st.doc
	name = strings.ToLower(name)
	for i := 1; i < d.Len(); i++ {
		if d.Block(i).Kind == parser.Heading && markdown.Slug(parser.PlainText(d.Inlines(i))) == name {
			return i
		}
	}
	return -1
}

// list 画列表。
func (st *viewState) list(c *wui.Context, i int, base spanStyle) wui.Element {
	d := st.doc
	th := &st.th
	b := d.Block(i)
	depth := 0
	for p := b.Parent; p > 0; p = d.Block(p).Parent {
		if d.Block(p).Kind == parser.List {
			depth++
		}
	}
	width := em(th, th.ListIndent)
	if b.Ordered {
		n := 0
		for j := i + 1; j < b.Next; j = d.Block(j).Next {
			n++
		}
		last := b.Number + max(0, n-1)
		w, _ := c.MeasureText(0, wui.Span{Text: itoa(last) + string(b.Marker), Font: th.Font, Size: th.FontSize})
		width = max(width, w+em(th, 0.5))
	}
	col := wui.Column(c).Role(wui.RoleList)
	return col.Children(func() {
		n := b.Number
		first := true
		for j := i + 1; j < d.Block(i).Next; j = d.Block(j).Next {
			row := wui.Row(c).AlignItems(wui.Start).Role(wui.RoleListItem)
			if !first {
				gap := th.BlockSpacing
				if b.Tight {
					gap = th.ItemSpacing
				}
				row.Margin(em(th, gap), 0, 0, 0)
			}
			if st.scrollTo == j {
				row.ScrollIntoView()
				st.scrollTo = -1
			}
			num := n
			row.Children(func() {
				st.marker(c, b, j, num, depth, width)
				wui.Column(c).Grow(1).Shrink(1).MinWidth(0).Children(func() { st.blocks(c, j, b.Tight, base) })
			})
			n++
			first = false
		}
	})
}

// marker 画列表项前面的圆点、数字或勾选框（只读：Grove 的预览不改源码）。
func (st *viewState) marker(c *wui.Context, list *parser.Block, j, n, depth int, width float32) {
	th := &st.th
	item := st.doc.Block(j)
	box := wui.Row(c).Width(width).Shrink(0).Unselectable()
	box.Children(func() {
		switch {
		case item.Task != parser.NoTask:
			size := em(th, 0.95)
			lineH := th.FontSize * th.LineHeight * 1.2
			checked := item.Task == parser.Checked
			cb := wui.Box(c).Size(size, size).Radius(max(3, size/5)).Margin(max(0, (lineH-size)/2-1), 0, 0, 0).Role(wui.RoleCheckBox).Checked(checked).ReadOnly(true)
			if checked {
				cb.Background(th.Checkbox).Draw(func(p *wui.Painter, r wui.Rect) {
					path := &wui.Path{}
					path.MoveTo(r.X+r.W*0.25, r.Y+r.H*0.52)
					path.LineTo(r.X+r.W*0.43, r.Y+r.H*0.7)
					path.LineTo(r.X+r.W*0.76, r.Y+r.H*0.32)
					p.StrokePath(path, max(1.5, r.W/9), wui.RGB(255, 255, 255))
				})
			} else {
				cb.Border(1.5, th.Muted)
			}
		case list.Ordered:
			wui.Text(c, itoa(n)+string(list.Marker)).TextColor(th.Muted).Width(width - em(th, 0.4)).TextAlign(wui.End).FontFeatures("tnum")
		default:
			bullets := [...]string{"•", "◦", "▪"}
			wui.Text(c, bullets[min(depth, 2)]).TextColor(th.Muted).Width(width - em(th, 0.5)).TextAlign(wui.Center)
		}
	})
}

// numberNotes 按脚注定义的先后编号。
func (st *viewState) numberNotes() {
	clear(st.notes)
	n := 0
	for i := 1; i < st.doc.Len(); i++ {
		if b := st.doc.Block(i); b.Kind == parser.FootnoteDefinition {
			if st.notes == nil {
				st.notes = map[string]int{}
			}
			key := parser.NormalizeLabel(b.Label())
			if _, ok := st.notes[key]; !ok {
				n++
				st.notes[key] = n
			}
		}
	}
}

func (st *viewState) noteNumber(label string) int { return st.notes[parser.NormalizeLabel(label)] }

// footnotes 在文末一条线下面按顺序画脚注。
func (st *viewState) footnotes(c *wui.Context) {
	if len(st.notes) == 0 {
		return
	}
	d := st.doc
	th := &st.th
	wui.Box(c).Height(1).Background(th.Rule).AlignSelf(wui.Stretch).Margin(em(th, 1.2), 0, em(th, 0.6), 0)
	base := spanStyle{color: th.Muted, size: em(th, 0.875)}
	for i := 1; i < d.Len(); i++ {
		b := d.Block(i)
		if b.Kind != parser.FootnoteDefinition {
			continue
		}
		num := st.notes[parser.NormalizeLabel(b.Label())]
		row := wui.Row(c).AlignItems(wui.Start).Margin(em(th, 0.25), 0, 0, 0)
		if st.scrollTo == i {
			row.ScrollIntoView()
			st.scrollTo = -1
		}
		row.Children(func() {
			wui.Text(c, itoa(num)+".").TextColor(th.Muted).FontSize(em(th, 0.875)).Width(em(th, 1.6)).Shrink(0).Unselectable()
			wui.Column(c).Grow(1).Shrink(1).MinWidth(0).TextColor(th.Muted).Children(func() { st.blocks(c, i, false, base) })
		})
	}
}

func itoa(n int) string {
	if n >= 0 && n < 10 {
		return string(rune('0' + n))
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
