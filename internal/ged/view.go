package ged

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// Style 是 View 的样子，由界面那边按主题给（ged 不认 Grove 的主题）。
type Style struct {
	Code       *editor.Theme // 代码颜色、行号色、底色
	Font       wui.Font      // 等宽字体（Family、Size）
	LineHeight float32       // 行高是字号的几倍，0 为 1.5

	AddedLine, DeletedLine wui.Color // 改了的整行的底
	AddedWord, DeletedWord wui.Color // 行里改了的字的底，叠在整行的底上
	AddedMark, DeletedMark wui.Color // 改了的行的行号和 + −
	Filler                 wui.Color // 对面多出的行在这边留的空：斜线
	FoldBack, FoldText     wui.Color // 折起来的一段
	FoldHover              wui.Color
	Divider                wui.Color // 两边中间的线
	Ruler                  wui.Color // 右边概览条的底

	UIFont    wui.Font           // 折叠条上的字
	FoldLabel func(n int) string // 折叠条上写什么（“12 行未修改”）
	Label     string             // 无障碍名
	Tab       int                // 制表符几格，0 为 4

	// HunkBar 往指针下（或刚跳到的）那处改动右上角的那排按钮里放东西，nil 不放；i 是第几处改动。
	// 按钮的无障碍名、悬停提示由界面那边给。
	HunkBar func(c *wui.Context, bar wui.Element, i int, h Hunk)
	// Copy 接 ⌘C（编辑菜单的“复制”）复制选中的字；nil 不接。
	Copy func(text string)
	// Keys 在视图有键盘焦点时先拿到按下的键（界面那边接 J / K 跳改动这类），接了返回 true；没接的视图自己接
	// （↑ ↓ 翻页 Home End 滚动、Esc 取消选择）。
	Keys func(ev wui.InputEvent) bool
	// Lines / SetLines 是点行号选的行（行级暂存用）：哪边（0 旧、1 新）、第 start 到第 end 行（两头都算，从 0 起）。
	// 选中的行存在调用方那里（和 mygo 的 Diff 共用一份）。都给了才能选；没给时点行号和点正文一样是选字。
	Lines    func() (side, start, end int, ok bool)
	SetLines func(side, start, end int, ok bool)
	LineSel  wui.Color // 选中的行的底
	LineMark wui.Color // 选中的行左边的竖条
}

// Context 是改动上下各留几行不折。
const Context = 3

// Diff 是一个 Diff 视图的状态，跨帧留着（调用方按标签存）：两边的行、对齐的结果、折叠、滚动、排好的字。
type Diff struct {
	key     any
	lines   [2][]string
	lang    *editor.Language
	aligned []Row
	rows    []Row
	open    map[int]bool
	full    bool // 不折
	unified bool // 统一的一栏（不是并排）
	hunks   []Hunk
	pairs   [2]map[int]int    // 配成对（Changed）的行：旧行号 → 新行号、新行号 → 旧行号（统一视图里找行里改了的字）
	hunkRow []int             // 每处改动的第一行在 rows 里是第几行
	hunkEnd []int             // 每处改动最后一行的下一行在 rows 里是第几行
	rowHunk []int             // rows 的每一行是第几处改动的，-1 不是
	cur     int               // 上次跳到、点过的那处改动，-1 没有
	want    int               // 下一帧要滚到的那行（rows 的下标），-1 没有
	words   map[int][2][]Span // Changed 行按旧行号
	states  [2][]uint64       // 每行开头的分词状态（块注释、多行字符串跨行）
	toks    []editor.Token
	layouts map[lineKey]*lineLayout
	font    wui.Font // layouts 是按这个字体、这套颜色排的
	theme   editor.Theme

	ScrollY float32
	scrollX [2]float32
	widest  [2]float32
	w, h    float32  // 上一帧的大小（本地坐标），输入按它换算
	box     wui.Rect // 上一帧画在哪（画的坐标）
	lh      float32
	drag    bool
	dragOff float32
	hover   int // 指针在第几行（rows 的下标），-1 不在
	onBar   bool
	barBox  wui.Rect // 改动上那排按钮在哪（本地坐标；大小是上一帧量的）：按在上面的归按钮

	sel       selection
	selecting bool // 正按着拖选
	side      int  // 上次按在哪边（⌘A 全选那边，← → 横着滚那边）
	focus     bool // 按下了：下一帧把键盘焦点给视图

	picking  bool // 正按着在行号上拖着选行
	anchor   int  // 选行的起点（⇧ 点、拖动从这算），-1 没有
	gutter   bool // 指针在行号上（能选行时）
	lsel     lineSel
	textLeft [2]float32 // 两边正文从哪开始（画的坐标，上一帧的），给测试用
}

// pos 是一边里的一个位置：第 line 行（从 0 起）第 off 个字节前。
type pos struct{ line, off int }

func (p pos) less(q pos) bool { return p.line < q.line || p.line == q.line && p.off < q.off }

// lineSel 是这一帧选中的行（Style.Lines 给的）。
type lineSel struct {
	side, a, b int
	ok         bool
}

func (l lineSel) has(side, line int) bool {
	return l.ok && l.side == side && line >= l.a && line <= l.b
}

// selection 是一边（side）里选中的字，从 a 到 b（拖的起点和现在的点，不一定 a 在前）。按原文的行算：
// 拖过折叠条时折起来的行也算在里面，展开、折起不影响它。
type selection struct {
	side int
	a, b pos
}

func (s selection) span() (pos, pos) {
	if s.b.less(s.a) {
		return s.b, s.a
	}
	return s.a, s.b
}

func (s selection) empty() bool { return s.a == s.b }

type lineKey struct{ side, line int }

type lineLayout struct {
	l   *wui.TextLayout
	u16 []int // 原文第几个字节在排出来的字里是第几个 UTF-16 单元（制表符展开过）
	w   float32
}

// NewDiff 建一个空的 Diff。
func NewDiff() *Diff {
	return &Diff{open: map[int]bool{}, hover: -1, cur: -1, want: -1, side: 1, anchor: -1}
}

// Set 换上两边的内容：key 和上次一样就什么都不做（调用方拿两边的版本号当 key），不一样才调 load 重新对齐。
// 返回重新对齐了没有。滚动位置留着，折叠的展开状态清掉。
func (d *Diff) Set(key any, load func() (old, new []string), lang *editor.Language) bool {
	if d.aligned != nil && d.key == key && d.lang == lang {
		return false
	}
	d.key, d.lang = key, lang
	old, new := load()
	d.lines = [2][]string{old, new}
	d.aligned = Align(old, new)
	d.open = map[int]bool{}
	d.words = map[int][2][]Span{}
	d.states = [2][]uint64{}
	d.layouts = nil
	d.widest = [2]float32{}
	d.hunks, d.cur = Hunks(d.aligned), -1
	d.pairs = [2]map[int]int{{}, {}}
	for _, r := range d.aligned {
		if r.Kind == Changed {
			d.pairs[0][r.Old], d.pairs[1][r.New] = r.New, r.Old
		}
	}
	d.sel, d.selecting = selection{}, false
	d.refold()
	return true
}

// SetFull 换成显示全文（on）或只看改动附近（没改的长段折起来）。
func (d *Diff) SetFull(on bool) {
	if d.full != on {
		d.full = on
		d.refold()
	}
}

// SetUnified 换成统一的一栏（on：每处改动先列删的、再列加的，左边两列行号）或并排。滚动位置留着，选中的字、行照旧。
func (d *Diff) SetUnified(on bool) {
	if d.unified != on {
		d.unified = on
		d.refold()
	}
}

// Unified 是现在是不是统一的一栏。
func (d *Diff) Unified() bool { return d.unified }

// refold 按 full、open、unified 重排显示的行，跟着重算行和改动的对应。
func (d *Diff) refold() {
	if d.full {
		d.rows = d.aligned
	} else {
		d.rows = Folded(d.aligned, Context, d.open)
	}
	if d.unified {
		d.rows = Unified(d.rows)
	}
	// 改了的行折不起来，按改动的顺序挨着：第 k 个改了的行属于哪处改动按个数数（并排时一处占 Row1 - Row0 行，
	// 统一时占两边的行数之和）
	d.rowHunk = make([]int, len(d.rows))
	d.hunkRow = make([]int, len(d.hunks))
	d.hunkEnd = make([]int, len(d.hunks))
	h, left := -1, 0
	for i, r := range d.rows {
		d.rowHunk[i] = -1
		if r.Kind == Same || r.Kind == Fold {
			continue
		}
		for left == 0 && h+1 < len(d.hunks) {
			h++
			hk := d.hunks[h]
			left = hk.Row1 - hk.Row0
			if d.unified {
				left = hk.OldEnd - hk.OldStart + hk.NewEnd - hk.NewStart
			}
			d.hunkRow[h], d.hunkEnd[h] = i, i+left
		}
		d.rowHunk[i] = h
		left--
	}
}

// rowSide 是统一视图里 row 的字是哪边的：删掉的行是旧的，别的（没改的、加的、折叠）按新的。
func (d *Diff) rowSide(r Row) int {
	if r.Kind == Deleted {
		return 0
	}
	return 1
}

// Rows 是现在显示的行（折过的）。
func (d *Diff) Rows() []Row { return d.rows }

// Expand 展开 rows[i] 那段折叠；不是折叠时返回 false。
func (d *Diff) Expand(i int) bool {
	if i < 0 || i >= len(d.rows) || d.rows[i].Kind != Fold {
		return false
	}
	d.open[d.rows[i].Old] = true
	d.refold()
	return true
}

// Selected 是选中的字（没选是 ""）。行尾的 \r 不算。
func (d *Diff) Selected() string {
	if d.sel.empty() {
		return ""
	}
	a, b := d.sel.span()
	lines := d.lines[d.sel.side]
	var out strings.Builder
	for l := a.line; l <= b.line && l < len(lines); l++ {
		t := strings.TrimSuffix(lines[l], "\r")
		s, e := 0, len(t)
		if l == a.line {
			s = min(a.off, e)
		}
		if l == b.line {
			e = min(b.off, e)
		}
		out.WriteString(t[s:max(s, e)])
		if l < b.line {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// Select 选中 side 那边从 (l0, o0) 到 (l1, o1) 的字（行号从 0 起，o 是字节）。
func (d *Diff) Select(side, l0, o0, l1, o1 int) {
	d.sel = selection{side: side, a: pos{l0, o0}, b: pos{l1, o1}}
}

// SelectAll 选中 side 那边的全部。
func (d *Diff) SelectAll(side int) {
	lines := d.lines[side]
	if len(lines) == 0 {
		d.sel = selection{}
		return
	}
	last := len(lines) - 1
	d.Select(side, 0, 0, last, len(strings.TrimSuffix(lines[last], "\r")))
}

// Hunks 是一处处改动。
func (d *Diff) Hunks() []Hunk { return d.hunks }

// GoToHunk 把第 i 处改动滚到离顶上三行的地方，记成当前的那处（Current）。
func (d *Diff) GoToHunk(i int) {
	if i >= 0 && i < len(d.hunks) {
		d.cur = i
		d.ScrollToRow(d.hunkRow[i])
	}
}

// Refocus 把当前的那处改动滚回视图（折起、展开之后）；还没有当前的那处时返回 false。
func (d *Diff) Refocus() bool {
	if d.cur < 0 {
		return false
	}
	d.GoToHunk(d.cur)
	return true
}

// HunkAtLine 是 side（0 旧、1 新）那边第 line 行所在的、或者它后面的第一处改动；都在它前面时是最后一处，没有改动是 -1。
func (d *Diff) HunkAtLine(side, line int) int {
	for i, h := range d.hunks {
		s, e := h.OldStart, h.OldEnd
		if side == 1 {
			s, e = h.NewStart, h.NewEnd
		}
		if e > line || s >= line {
			return i
		}
	}
	return len(d.hunks) - 1
}

// Current 是标签条的上 / 下一处、底栏的“第 i / n 处改动”按的那处：刚跳到、点过的那处还在视图里就是它（inside）；
// 不然按视口，顶上往下三行以前开始的最后一处（inside 是它还看得见），一处都没有是 -1。
func (d *Diff) Current() (cur int, inside bool) {
	if len(d.hunks) == 0 {
		return -1, false
	}
	if d.lh <= 0 || d.h <= 0 {
		return d.cur, d.cur >= 0
	}
	top := int(d.ScrollY / d.lh)
	bottom := int((d.ScrollY + d.h) / d.lh)
	seen := func(i int) bool { return d.hunkEnd[i] > top && d.hunkRow[i] < bottom }
	if d.cur >= 0 && d.cur < len(d.hunks) && seen(d.cur) {
		return d.cur, true
	}
	cur = -1
	for i, r := range d.hunkRow {
		if r > top+Context {
			break
		}
		cur = i
	}
	return cur, cur >= 0 && seen(cur)
}

// Bounds 是上一帧视图画在哪（窗口里的坐标，测试按它点）。
func (d *Diff) Bounds() wui.Rect { return d.box }

// RowHeight 是上一帧的行高（还没画过是 0）。
func (d *Diff) RowHeight() float32 { return d.lh }

// ScrollToRow 把 rows[i] 滚到离顶上三行的地方（下一帧画的时候生效：那时才知道行高）。
func (d *Diff) ScrollToRow(i int) { d.want = i }

func (d *Diff) wordsOf(r Row) [2][]Span {
	if w, ok := d.words[r.Old]; ok {
		return w
	}
	o, n := Words(d.lines[0][r.Old], d.lines[1][r.New])
	w := [2][]Span{o, n}
	d.words[r.Old] = w
	return w
}

func (d *Diff) stateAt(side, line int) uint64 {
	if d.lang == nil || d.lang.Tokenizer == nil {
		return 0
	}
	st := d.states[side]
	if len(st) == 0 {
		st = append(st, 0)
	}
	for len(st) <= line {
		i := len(st) - 1
		var next uint64
		d.toks, next = d.lang.Tokenizer.Tokenize(clean(d.lines[side][i]), st[i], d.toks[:0])
		st = append(st, next)
	}
	d.states[side] = st
	return st[line]
}

// lineMax 是一行最多排多少字节：再长的（压缩过的 JS）后面看不到也没关系，排它太慢。
const lineMax = 4000

func clean(s string) string {
	s = strings.TrimSuffix(s, "\r")
	if len(s) > lineMax {
		e := lineMax
		for e > 0 && !utf8.RuneStart(s[e]) {
			e--
		}
		s = s[:e]
	}
	return s
}

// layout 排一行字：语法高亮的颜色、制表符展开成空格。排好的留着，留太多了整个清掉重排。
func (d *Diff) layout(side, line int, s *Style) *lineLayout {
	if d.layouts == nil || len(d.layouts) > 4000 {
		d.layouts = map[lineKey]*lineLayout{}
	}
	k := lineKey{side, line}
	if l := d.layouts[k]; l != nil {
		return l
	}
	src := clean(d.lines[side][line])
	ct := s.Code
	type seg struct {
		end  int
		kind editor.TokenKind
		tok  bool
	}
	var segs []seg
	if d.lang != nil && d.lang.Tokenizer != nil {
		var toks []editor.Token
		toks, _ = d.lang.Tokenizer.Tokenize(src, d.stateAt(side, line), d.toks[:0])
		d.toks = toks
		at := 0
		for _, t := range toks {
			if t.End <= at || t.End > len(src) {
				continue
			}
			segs = append(segs, seg{t.End, t.Kind, true})
			at = t.End
		}
	}
	if len(segs) == 0 || segs[len(segs)-1].end < len(src) {
		segs = append(segs, seg{end: len(src)})
	}
	tab := s.Tab
	if tab <= 0 {
		tab = 4
	}
	u16 := make([]int, len(src)+1)
	var spans []wui.Span
	col, units, at := 0, 0, 0
	var b strings.Builder
	for _, sg := range segs {
		b.Reset()
		for i := at; i < sg.end; {
			r, w := utf8.DecodeRuneInString(src[i:])
			for j := range w {
				u16[i+j] = units
			}
			if r == '\t' {
				n := tab - col%tab
				b.WriteString(strings.Repeat(" ", n))
				col, units = col+n, units+n
			} else {
				b.WriteRune(r)
				col++
				units += utf16Len(r)
			}
			i += w
		}
		sp := wui.Span{Text: b.String(), Font: s.Font.Family, Size: s.Font.Size, Color: ct.Text}
		if sg.tok {
			ts := ct.Tokens[sg.kind]
			sp.Color, sp.Italic = ct.TokenColor(sg.kind), ts.Italic
			if ts.Bold {
				sp.Weight = 600
			}
		}
		spans = append(spans, sp)
		at = sg.end
	}
	u16[len(src)] = units
	f := s.Font
	f.LineHeight = s.lineHeight()
	tl := wui.ShapeRichText(spans, f, 0)
	w, _ := tl.Size()
	l := &lineLayout{l: tl, u16: u16, w: w}
	d.layouts[k] = l
	d.widest[side] = max(d.widest[side], w)
	return l
}

func utf16Len(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

func (s *Style) lineHeight() float32 {
	if s.LineHeight > 0 {
		return s.LineHeight
	}
	return 1.5
}

// geom 是这一帧的版面（本地坐标，左上角是 0, 0）。
type geom struct {
	w, h    float32
	lh      float32
	paneX   [2]float32
	paneW   [2]float32
	numR    float32 // 行号右边对齐到格子里的哪（统一视图：旧的行号）
	numR2   float32 // 统一视图：新的行号右边对齐到哪（并排时和 numR 一样）
	markX   float32 // + − 的位置
	textX   float32 // 正文从格子里的哪开始
	content float32 // 全部行多高
	ruler   wui.Rect
	asc     float32
	desc    float32
	cw      float32 // 一个数字多宽
}

const rulerW = 14

func (d *Diff) geom(s *Style, w, h, scale float32) geom {
	size := s.Font.Size
	if size <= 0 {
		size = 13
	}
	scale = max(scale, 1)
	g := geom{w: w, h: h}
	g.lh = float32(math.Round(float64(size*s.lineHeight()*scale))) / scale
	digits := len(strconv.Itoa(max(len(d.lines[0]), len(d.lines[1]), 1)))
	cw := charWidth(s.Font)
	g.cw = cw
	g.numR = 10 + float32(digits)*cw
	g.numR2 = g.numR
	if d.unified { // 两列行号：旧的、新的
		g.numR2 = g.numR + 8 + float32(digits)*cw
	}
	g.markX = g.numR2 + 8
	g.textX = g.markX + cw + 8
	if d.unified { // 一栏：两边的“格子”是同一个
		g.paneW = [2]float32{w, w}
	} else {
		g.paneW[0] = float32(math.Floor(float64((w - 1) / 2)))
		g.paneX[1] = g.paneW[0] + 1
		g.paneW[1] = w - g.paneX[1]
	}
	g.content = float32(len(d.rows)) * g.lh
	g.ruler = wui.Rect{X: w - rulerW, Y: 0, W: rulerW, H: h}
	m := s.Font.Metrics()
	g.asc, g.desc = m.Ascent, m.Descent
	return g
}

var widthCache sync.Map // wui.Font → 一个数字多宽

func charWidth(f wui.Font) float32 {
	if w, ok := widthCache.Load(f); ok {
		return w.(float32)
	}
	w := float32(7.8)
	if gs := wui.Shape("0", f); len(gs) > 0 {
		w = gs[0].Advance
	}
	widthCache.Store(f, w)
	return w
}

func (d *Diff) clamp(g geom) {
	d.ScrollY = min(d.ScrollY, max(0, g.content-g.h+g.lh*3))
	d.ScrollY = max(d.ScrollY, 0)
	if d.unified { // 一栏横着一起滚：两边的数一样（按上次动的那边），按宽的那边算能滚多远
		room := g.paneW[1] - g.textX - rulerW
		x := max(0, min(d.scrollX[d.side], max(d.widest[0], d.widest[1])-room+g.lh))
		d.scrollX = [2]float32{x, x}
		return
	}
	for side := range 2 {
		room := g.paneW[side] - g.textX - rulerW
		d.scrollX[side] = max(0, min(d.scrollX[side], d.widest[side]-room+g.lh))
	}
}

func (d *Diff) bar(g geom) wui.Scrollbar {
	return wui.Scrollbar{Box: g.ruler, View: g.h, Content: g.content + g.lh*3, Offset: d.ScrollY,
		Style: wui.ScrollbarStyle{TrackWidth: rulerW, ThumbWidth: rulerW, Square: true, Flush: true}}
}

// View 建 Diff 的视图（并排，或 SetUnified 后统一的一栏）：自己管滚动（一个 ScrollY，两边一起走；并排时横着两边各滚各的），
// 只排、只画看得见的行。
// 点折叠条展开那一段。
func View(c *wui.Context, key any, d *Diff, s Style) wui.Element {
	if d.hover >= len(d.rows) {
		d.hover = -1
	}
	if d.cur >= len(d.hunks) {
		d.cur = -1
	}
	if d.w > 0 { // 跳到哪处在建视图时就滚过去：改动上那排按钮按滚好的位置摆（还没画过的等画的时候）
		g := d.geomNow(&s)
		d.applyWant(g)
		d.clamp(g)
	}
	el := wui.Box(c.Key(key)).Grow(1).MinHeight(0).MinWidth(0).Clip().Background(s.Code.Background).Label(s.Label).
		Focusable().FocusRing(false)
	if d.focus {
		d.focus = false
		el.Focus(wui.FocusOptions{PreventScroll: true})
	}
	if !el.Hovered() && !d.drag { // 指针走了（没有“离开”事件，问 Hovered）
		d.hover, d.onBar = -1, false
	}
	switch {
	case d.hover >= 0 && d.rows[d.hover].Kind == Fold && !d.onBar:
		el.Cursor(wui.CursorPointer)
	case d.gutter && !d.selecting || d.picking:
		el.Cursor(wui.CursorDefault)
	case d.hover >= 0 && !d.onBar || d.selecting:
		el.Cursor(wui.CursorText)
	}
	el.Draw(func(p *wui.Painter, r wui.Rect) {
		g := d.geom(&s, r.W, r.H, p.Scale())
		d.w, d.h, d.lh, d.box = r.W, r.H, g.lh, r
		d.textLeft = [2]float32{r.X + g.paneX[0] + g.textX, r.X + g.paneX[1] + g.textX}
		d.applyWant(g)
		d.clamp(g)
		d.paint(p, r, g, &s)
	})
	el.HandleInput(func(ev wui.InputEvent) bool { return d.input(ev, &s) })
	size := d.barBox
	d.barBox = wui.Rect{}
	if i := d.barHunk(); i >= 0 && s.HunkBar != nil {
		el.Children(func() {
			top := d.hunkTop(i) + 2
			bar := wui.Row(c).Key("hunk bar").Absolute().Right(rulerW + 6).Top(top).
				Cursor(wui.CursorDefault).KeepFocus()
			s.HunkBar(c, bar, i, d.hunks[i])
			// Bounds 是上一帧的（跳了一处改动后位置就不对了），只拿它的大小，位置按这一帧摆的算；还没量过先当 80 × 一行
			if b := bar.Bounds(); b.W > 0 {
				size = b
			} else if size.W == 0 {
				size = wui.Rect{W: 80, H: d.lh}
			}
			d.barBox = wui.Rect{X: d.w - rulerW - 6 - size.W, Y: top, W: size.W, H: size.H}
		})
	}
	return el
}

func (d *Diff) applyWant(g geom) {
	if d.want >= 0 && g.lh > 0 {
		d.ScrollY, d.want = max(0, float32(d.want-Context)*g.lh), -1
	}
}

// barHunk 是哪处改动上放那排按钮：指针下的那处；指针不在改动上时是刚跳到、点过的那处（还看得见的话）。
func (d *Diff) barHunk() int {
	if d.lh <= 0 {
		return -1
	}
	if d.hover >= 0 && !d.onBar {
		if h := d.rowHunk[d.hover]; h >= 0 {
			return h
		}
	}
	if cur, inside := d.Current(); inside && cur == d.cur {
		return cur
	}
	return -1
}

// hunkTop 是第 i 处改动的顶在视图里的 y；顶滚出去了就贴着视图顶，直到改动的最后一行（按钮跟着改动走，不悬空）。
func (d *Diff) hunkTop(i int) float32 {
	y0 := float32(d.hunkRow[i])*d.lh - d.ScrollY
	y1 := float32(d.hunkEnd[i])*d.lh - d.ScrollY
	return max(min(0, y1-d.lh), y0)
}

func (d *Diff) geomNow(s *Style) geom {
	g := d.geom(s, d.w, d.h, 1)
	if d.lh > 0 {
		g.lh = d.lh
		g.content = float32(len(d.rows)) * g.lh
	}
	return g
}

func (d *Diff) rowAt(y float32, g geom) int {
	if g.lh <= 0 {
		return -1
	}
	i := int(math.Floor(float64((y + d.ScrollY) / g.lh)))
	if i < 0 || i >= len(d.rows) {
		return -1
	}
	return i
}

func (d *Diff) input(ev wui.InputEvent, s *Style) bool {
	if d.w <= 0 {
		return false
	}
	g := d.geomNow(s)
	inBar := ev.X >= g.ruler.X && g.content+g.lh*3 > g.h
	if ev.Kind != wui.InputScroll && !d.drag && d.barBox.W > 0 && d.barBox.Contains(ev.X, ev.Y) {
		return false // 在改动上那排按钮上：归按钮，指针下的那处改动不变
	}
	switch ev.Kind {
	case wui.InputKeyDown:
		if s.Keys != nil && s.Keys(ev) {
			return true
		}
		return d.onKey(ev, g, s)
	case wui.InputCommand:
		switch ev.Text {
		case "copy":
			if d.sel.empty() || s.Copy == nil {
				return false
			}
			s.Copy(d.Selected())
			return true
		case "selectAll":
			d.SelectAll(d.side)
			return true
		}
		return false
	case wui.InputScroll:
		y, x := d.ScrollY, d.scrollX
		d.ScrollY += ev.DY
		side := 0
		if ev.X >= g.paneX[1] {
			side = 1
		}
		if d.unified {
			side = d.side
		}
		d.scrollX[side] += ev.DX
		d.clamp(g)
		return d.ScrollY != y || d.scrollX != x
	case wui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		if inBar {
			sb := d.bar(g)
			start, n := sb.Thumb()
			if ev.Y >= start && ev.Y < start+n {
				d.dragOff = ev.Y - start
			} else {
				d.dragOff = n / 2
				d.ScrollY = sb.OffsetAt(ev.Y - d.dragOff)
				d.clamp(g)
			}
			d.drag = true
			return true
		}
		d.focus = true
		i := d.rowAt(ev.Y, g)
		if i >= 0 && d.Expand(i) {
			return true
		}
		if i >= 0 && d.rowHunk[i] >= 0 {
			d.cur = d.rowHunk[i]
		}
		side := 0
		if ev.X >= g.paneX[1] {
			side = 1
		}
		if d.unified && i >= 0 {
			side = d.rowSide(d.rows[i])
		}
		d.side = side
		if d.pickLine(ev, i, side, g, s) {
			return true
		}
		p := d.posAt(ev.X, ev.Y, side, g, s)
		switch {
		case ev.Mods&wui.Shift != 0 && !d.sel.empty() && d.sel.side == side: // ⇧ 点：从原来的起点选到这
			d.sel.b = p
		case ev.Clicks == 2: // 双击选一个词
			d.sel = d.wordAt(side, p)
		case ev.Clicks >= 3: // 三击选一行（连着行尾的换行）
			d.sel = selection{side: side, a: pos{p.line, 0}, b: pos{p.line + 1, 0}}
		default:
			d.sel = selection{side: side, a: p, b: p}
		}
		d.selecting = true
		return true
	case wui.InputPointerMove:
		if d.drag {
			d.ScrollY = d.bar(g).OffsetAt(ev.Y - d.dragOff)
			d.clamp(g)
			return true
		}
		if d.picking {
			if ev.Y < 0 {
				d.ScrollY += ev.Y
			} else if ev.Y > g.h {
				d.ScrollY += ev.Y - g.h
			}
			d.clamp(g)
			if i := d.rowAt(min(max(ev.Y, 0), g.h-1), g); i >= 0 && d.rows[i].Kind != Fold {
				if line := lineOf(d.rows[i], d.lsel.side); line >= 0 && d.anchor >= 0 {
					s.SetLines(d.lsel.side, d.anchor, line, true)
				}
			}
			return true
		}
		if d.selecting {
			// 拖出视图上下边时跟着滚（只在指针动的时候滚）
			if ev.Y < 0 {
				d.ScrollY += ev.Y
			} else if ev.Y > g.h {
				d.ScrollY += ev.Y - g.h
			}
			d.clamp(g)
			x := min(max(ev.X, g.paneX[d.sel.side]), g.paneX[d.sel.side]+g.paneW[d.sel.side]-1)
			d.sel.b = d.posAt(x, min(max(ev.Y, 0), g.h-1), d.sel.side, g, s)
			return true
		}
		hv := d.rowAt(ev.Y, g)
		gut := !inBar && d.canPick(s) && d.inGutter(ev.X, g)
		if hv != d.hover || inBar != d.onBar || gut != d.gutter {
			d.hover, d.onBar, d.gutter = hv, inBar, gut
			return true
		}
	case wui.InputPointerUp:
		if d.drag || d.selecting || d.picking {
			d.drag, d.selecting, d.picking = false, false, false
			return true
		}
	}
	return false
}

// canPick 是能不能点行号选行。
func (d *Diff) canPick(s *Style) bool { return s.Lines != nil && s.SetLines != nil }

// inGutter 是 x 在不在那边的行号、+ − 上（正文左边）。
func (d *Diff) inGutter(x float32, g geom) bool {
	side := 0
	if x >= g.paneX[1] {
		side = 1
	}
	return x-g.paneX[side] < g.textX-4
}

// pickLine 是按在行号上：选那一行（⇧ 点从起点选到这，同一边才算），接着拖动扩大。按的不是行号、那边是空行、
// 不能选行时返回 false，照选字走。
func (d *Diff) pickLine(ev wui.InputEvent, i, side int, g geom, s *Style) bool {
	if !d.canPick(s) || i < 0 || d.rows[i].Kind == Fold || !d.inGutter(ev.X, g) {
		return false
	}
	line := lineOf(d.rows[i], side)
	if line < 0 {
		return false
	}
	d.sel = selection{}
	ls, a, b, ok := s.Lines()
	if ev.Mods&wui.Shift != 0 && ok && ls == side {
		if d.anchor < a || d.anchor > b { // 不是在这里选的（mygo 那个 Diff 选的、或者换过）：从原来的头算
			d.anchor = a
		}
	} else {
		d.anchor = line
	}
	d.lsel = lineSel{side: side, a: min(d.anchor, line), b: max(d.anchor, line), ok: true}
	s.SetLines(side, d.anchor, line, true)
	d.picking = true
	return true
}

// LineX 是 side 那边正文从哪开始（画的坐标，上一帧的）；往左是行号，按在上面是选行。给测试用。
func (d *Diff) LineX(side int) float32 { return d.textLeft[side] }

// onKey 是视图自己接的键：↑ ↓ 一行，翻页、空格一屏，Home / End、⌘↑ / ⌘↓ 到头尾，← → 横着滚上次按的那边，
// Esc 取消选中的字，没有选中的字时取消选中的行。
func (d *Diff) onKey(ev wui.InputEvent, g geom, s *Style) bool {
	page := max(g.lh, g.h-2*g.lh)
	cmd := ev.Mods&wui.Super != 0
	switch {
	case ev.Key == wui.KeyEscape && ev.Mods == 0:
		if !d.sel.empty() {
			d.sel = selection{}
			return true
		}
		if d.canPick(s) {
			if _, _, _, ok := s.Lines(); ok {
				s.SetLines(0, 0, 0, false)
				d.anchor = -1
				return true
			}
		}
		return false
	case ev.Key == wui.KeyHome || cmd && ev.Key == wui.KeyUp:
		d.ScrollY = 0
	case ev.Key == wui.KeyEnd || cmd && ev.Key == wui.KeyDown:
		d.ScrollY = g.content
	case ev.Mods&^wui.Shift != 0:
		return false
	case ev.Key == wui.KeyUp:
		d.ScrollY -= g.lh
	case ev.Key == wui.KeyDown:
		d.ScrollY += g.lh
	case ev.Key == wui.KeyPageUp || ev.Key == wui.KeySpace && ev.Mods&wui.Shift != 0:
		d.ScrollY -= page
	case ev.Key == wui.KeyPageDown || ev.Key == wui.KeySpace:
		d.ScrollY += page
	case ev.Key == wui.KeyLeft:
		d.scrollX[d.side] -= 4 * g.cw
	case ev.Key == wui.KeyRight:
		d.scrollX[d.side] += 4 * g.cw
	default:
		return false
	}
	d.clamp(g)
	return true
}

// lineOf 是一行在 side 那边是第几行（Fold 行是折起来的第一行），没有是 -1。
func lineOf(r Row, side int) int {
	if side == 0 {
		return r.Old
	}
	return r.New
}

// posAt 是视图里 (x, y) 在 side 那边对着的位置：对面多出来的空行算下一行的开头，折叠条算折起来的第一行开头。
func (d *Diff) posAt(x, y float32, side int, g geom, s *Style) pos {
	lines := d.lines[side]
	if len(lines) == 0 || len(d.rows) == 0 || g.lh <= 0 {
		return pos{}
	}
	end := func() pos { last := len(lines) - 1; return pos{last, len(clean(lines[last]))} }
	i := int(math.Floor(float64((y + d.ScrollY) / g.lh)))
	if i >= len(d.rows) {
		return end()
	}
	i = max(i, 0)
	row := d.rows[i]
	if row.Kind == Fold {
		return pos{lineOf(row, side), 0}
	}
	line := lineOf(row, side)
	if line < 0 {
		for j := i + 1; j < len(d.rows); j++ {
			if l := lineOf(d.rows[j], side); l >= 0 {
				return pos{l, 0}
			}
		}
		return end()
	}
	l := d.layout(side, line, s)
	tx := x - (g.paneX[side] + g.textX) + d.scrollX[side]
	if tx <= 0 {
		return pos{line, 0}
	}
	_, th := l.l.Size()
	return pos{line, byteAt(l.u16, l.l.IndexAt(wui.Point{X: tx, Y: th / 2}))}
}

// byteAt 是排出来的字里第 idx 个 UTF-16 单元对着原文的第几个字节：落在展开的制表符中间时取近的那头。
func byteAt(u16 []int, idx int) int {
	b := sort.Search(len(u16), func(k int) bool { return u16[k] >= idx })
	if b >= len(u16) {
		return len(u16) - 1
	}
	if u16[b] > idx && b > 0 {
		p := b - 1
		for p > 0 && u16[p-1] == u16[p] { // 退到那个字的开头
			p--
		}
		if idx-u16[p] < u16[b]-idx {
			return p
		}
	}
	return b
}

// wordAt 是 p 所在的那个词（tokens 切的：字母数字连成一个，空白连成一个，别的一个字一个）。
func (d *Diff) wordAt(side int, p pos) selection {
	src := clean(d.lines[side][p.line])
	for _, t := range tokens(src) {
		if p.off >= t.Start && p.off < t.End {
			return selection{side: side, a: pos{p.line, t.Start}, b: pos{p.line, t.End}}
		}
	}
	return selection{side: side, a: p, b: p}
}

func (d *Diff) paint(p *wui.Painter, r wui.Rect, g geom, s *Style) {
	if d.font != s.Font || d.theme != *s.Code { // 换了字体、深浅色：排好的字作废
		d.layouts, d.font, d.theme = nil, s.Font, *s.Code
	}
	d.lsel = lineSel{}
	if d.canPick(s) {
		side, a, b, ok := s.Lines()
		d.lsel = lineSel{side: side, a: min(a, b), b: max(a, b), ok: ok}
	}
	vis := p.Visible()
	top := max(vis.Y, r.Y)
	bottom := min(vis.Y+vis.H, r.Y+r.H)
	first := max(0, int(math.Floor(float64((top-r.Y+d.ScrollY)/g.lh))))
	last := min(len(d.rows), int(math.Ceil(float64((bottom-r.Y+d.ScrollY)/g.lh))))
	for i := first; i < last; i++ {
		y := r.Y + float32(i)*g.lh - d.ScrollY
		row := d.rows[i]
		if row.Kind == Fold {
			d.paintFold(p, r, g, s, row, y, i == d.hover && !d.onBar)
			continue
		}
		if d.unified {
			side := d.rowSide(row)
			line := lineOf(row, side)
			var w []Span
			if o, ok := d.pairs[side][line]; ok { // 配成对的删 / 加：行里改了的字
				pr := Row{Old: line, New: o}
				if side == 1 {
					pr = Row{Old: o, New: line}
				}
				w = d.wordsOf(pr)[side]
			}
			d.paintCell(p, r, g, s, side, row, w, y)
			continue
		}
		var w [2][]Span
		if row.Kind == Changed {
			w = d.wordsOf(row)
		}
		for side := range 2 {
			d.paintCell(p, r, g, s, side, row, w[side], y)
		}
	}
	if !d.unified { // 中间的线
		p.Fill(wui.Rect{X: r.X + g.paneW[0], Y: r.Y, W: 1, H: r.H}, s.Divider, 0)
	}
	d.paintRuler(p, r, g, s)
}

// paintCell 画 row 在 side 那边的格子（统一视图里是整行：两列行号，字是 side 那边的）。
func (d *Diff) paintCell(p *wui.Painter, r wui.Rect, g geom, s *Style, side int, row Row, words []Span, y float32) {
	x0, pw := r.X+g.paneX[side], g.paneW[side]
	cell := wui.Rect{X: x0, Y: y, W: pw, H: g.lh}
	line, kind := lineOf(row, side), row.Kind
	if line < 0 {
		hatch(p, cell, d.ScrollY+y-r.Y, s.Filler)
		return
	}
	changed := kind == Changed || kind == Deleted || kind == Added
	lineBg, wordBg, mark, sign := s.AddedLine, s.AddedWord, s.AddedMark, "+"
	if side == 0 {
		lineBg, wordBg, mark, sign = s.DeletedLine, s.DeletedWord, s.DeletedMark, "−"
	}
	if changed {
		p.Fill(cell, lineBg, 0)
	}
	if d.lsel.has(side, line) || d.unified && kind == Same && d.lsel.has(1-side, lineOf(row, 1-side)) { // 叠在红绿底上看不太出来：左边再画一道竖条
		p.Fill(cell, s.LineSel, 0)
		p.Fill(wui.Rect{X: x0, Y: y, W: 3, H: g.lh}, s.LineMark, 0)
	}
	base := y + (g.lh-(g.asc+g.desc))/2 + g.asc
	nc := s.Code.LineNumber
	if changed {
		nc = mark
	}
	number := func(l int, right float32) {
		if l < 0 {
			return
		}
		if num := wui.Shape(strconv.Itoa(l+1), s.Font); len(num) > 0 {
			last := num[len(num)-1]
			p.Glyphs(num, x0+right-(last.X+last.Advance), base, nc)
		}
	}
	if d.unified { // 两列：旧的行号、新的行号（删掉的行只有旧的，加的只有新的）
		number(row.Old, g.numR)
		number(row.New, g.numR2)
	} else {
		number(line, g.numR)
	}
	if changed {
		p.Glyphs(wui.Shape(sign, s.Font), x0+g.markX, base, mark)
	}
	tx := x0 + g.textX
	clip := wui.Rect{X: tx, Y: y, W: max(0, x0+pw-tx), H: g.lh}
	l := d.layout(side, line, s)
	_, th := l.l.Size()
	ty := y + (g.lh-th)/2
	sx := tx - d.scrollX[side]
	p.Clip(clip, 0, func() {
		for _, w := range words {
			a, b := min(w.Start, len(l.u16)-1), min(w.End, len(l.u16)-1)
			if b <= a {
				continue
			}
			for _, rc := range l.l.SelectionRects(wui.TextInputRange{Start: l.u16[a], End: l.u16[b]}) {
				p.Fill(wui.Rect{X: sx + rc.X, Y: y, W: rc.W, H: g.lh}, wordBg, 2)
			}
		}
		if !d.sel.empty() {
			switch {
			case d.sel.side == side:
				d.paintSel(p, l, line, sx, y, g, s)
			case d.unified && kind == Same: // 统一视图里没改的行两边一样：选的是旧的一边也涂
				d.paintSel(p, l, lineOf(row, d.sel.side), sx, y, g, s)
			}
		}
		p.TextLayout(l.l, sx, ty, s.Code.Text)
	})
}

// paintSel 涂这一行里选中的那段；选到下一行的，行尾多涂半个字宽（像编辑器里选中的换行）。
func (d *Diff) paintSel(p *wui.Painter, l *lineLayout, line int, sx, y float32, g geom, s *Style) {
	a, b := d.sel.span()
	if line < a.line || line > b.line {
		return
	}
	n := len(l.u16) - 1
	from, to := 0, n
	if line == a.line {
		from = min(a.off, n)
	}
	if line == b.line {
		to = min(b.off, n)
	}
	x0 := l.l.Caret(l.u16[from]).X
	x1 := l.l.Caret(l.u16[max(from, to)]).X
	if line < b.line {
		x1 += g.cw / 2
	}
	if x1 > x0 {
		p.Fill(wui.Rect{X: sx + x0, Y: y, W: x1 - x0, H: g.lh}, s.Code.Selection, 0)
	}
}

// hatch 在 r 里画 45° 的斜线（对面多出来的行在这边留的空）。cy 是 r 顶上在整个内容里的 y，斜线按内容对齐，滚动时不跳。
func hatch(p *wui.Painter, r wui.Rect, cy float32, c wui.Color) {
	if c.A == 0 {
		return
	}
	const step = 8
	p.Clip(r, 0, func() {
		// 线是 x + 内容 y = k·step
		k := float32(math.Floor(float64(cy/step))) * step
		for ; k <= r.W+cy+r.H; k += step {
			bx := r.X + k - cy - r.H                                               // 底边上的 x
			p.StrokePolyline([][2]float32{{bx, r.Y + r.H}, {bx + r.H, r.Y}}, 1, c) // Line 只画横竖
		}
	})
}

func (d *Diff) paintFold(p *wui.Painter, r wui.Rect, g geom, s *Style, row Row, y float32, hover bool) {
	bg := s.FoldBack
	if hover {
		bg = s.FoldHover
	}
	p.Fill(wui.Rect{X: r.X, Y: y, W: r.W, H: g.lh}, bg, 0)
	label := strconv.Itoa(row.N)
	if s.FoldLabel != nil {
		label = s.FoldLabel(row.N)
	}
	f := s.UIFont
	if f.Size <= 0 {
		f.Size = 12
	}
	m := f.Metrics()
	base := y + (g.lh-(m.Ascent+m.Descent))/2 + m.Ascent
	gs := wui.Shape("⋯  "+label, f)
	for side := range 2 {
		if d.unified && side == 0 { // 一栏只写一次
			continue
		}
		p.Glyphs(gs, r.X+g.paneX[side]+g.textX, base, s.FoldText)
	}
}

func (d *Diff) paintRuler(p *wui.Painter, r wui.Rect, g geom, s *Style) {
	sb := d.bar(g)
	if !sb.Scrolls() {
		return
	}
	rr := wui.Rect{X: r.X + g.ruler.X, Y: r.Y, W: rulerW, H: r.H}
	p.Fill(rr, s.Ruler, 0)
	// 概览：改动在整个文件里的位置，旧的一边（删）画左半、新的一边（加）画右半
	total := g.content + g.lh*3
	mh := max(2, g.lh*r.H/total)
	for i, row := range d.rows {
		if row.Kind == Same || row.Kind == Fold {
			continue
		}
		y := r.Y + float32(i)*g.lh*r.H/total
		if row.Old >= 0 {
			p.Fill(wui.Rect{X: rr.X + 2, Y: y, W: rulerW/2 - 2, H: mh}, s.DeletedMark, 0)
		}
		if row.New >= 0 {
			p.Fill(wui.Rect{X: rr.X + rulerW/2, Y: y, W: rulerW/2 - 2, H: mh}, s.AddedMark, 0)
		}
	}
	sb.Box = rr
	p.Scrollbar(sb, wui.ScrollbarState{Dragging: d.drag, Hovered: d.onBar})
}
