package gmd

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	ui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/css"
)

// hctx 是摆 HTML 时往下传的：在哪个视图、哪一块，字底下是什么颜色。
type hctx struct {
	st    *viewState
	c     *ui.Context
	hs    *htmlState
	block int      // 块号（复制用），-1 是 HTMLView
	bg    ui.Color // 字底下的颜色：作者的字色和它几乎看不出时换一个
	ink   ui.Color // 作者给了底色、没给字色时字的颜色（A == 0 是照样式的）
	rootC string   // 根的 color：字色是它就是作者没写过
	rem   float32
}

// parentKind 是元素在什么里面摆：块的流、flex、grid，或者行内（一行会折的 Row 里的一项）。
type parentKind uint8

const (
	pkBlock parentKind = iota
	pkFlex
	pkGrid
	pkInline
)

func (st *viewState) renderHTML(c *ui.Context, hs *htmlState, block int) ui.Element {
	r := &hctx{st: st, c: c, hs: hs, block: block, bg: st.uiTh.Background, rootC: hs.root.Get("color"), rem: st.th.FontSize}
	if hs.runs == nil {
		hs.runs = map[*hnode]*irun{}
	}
	root := &hnode{tag: "html", kids: hs.roots, st: hs.root}
	col := ui.Column(c).AlignSelf(ui.Stretch).AlignItems(ui.Stretch)
	return col.Children(func() { r.flow(root) })
}

// ---- 长度、颜色

func (r *hctx) fontSize(s *css.Style) float32 {
	if l, ok := css.ParseLength(s.Get("font-size")); ok && l.U == css.Px && l.V > 0 {
		return l.V
	}
	return r.st.th.FontSize
}

// length 是 s 的 p（px），百分数对着 pct；没写、auto、读不出是 ok = false。
func (r *hctx) length(s *css.Style, p string, pct float32) (float32, bool) {
	l, ok := css.ParseLength(s.Get(p))
	if !ok || l.U == css.Auto {
		return 0, false
	}
	return l.Resolve(r.fontSize(s), r.rem, pct), true
}

func (r *hctx) px(s *css.Style, p string) float32 {
	v, _ := r.length(s, p, 0)
	return v
}

func toUI(c css.Color) ui.Color { return ui.RGBA(c.R, c.G, c.B, c.A) }

func (r *hctx) color(v string) ui.Color {
	c, ok := css.ParseColor(v)
	if !ok {
		return ui.Color{}
	}
	return toUI(c)
}

// luminance 是相对亮度（WCAG）。
func luminance(c ui.Color) float64 {
	ch := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.R) + 0.7152*ch(c.G) + 0.0722*ch(c.B)
}

func contrast(a, b ui.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// over 是 c 盖在 bg 上的颜色。
func over(c, bg ui.Color) ui.Color {
	if c.A == 255 || bg.A == 0 {
		return c
	}
	a := float64(c.A) / 255
	mix := func(x, y uint8) uint8 { return uint8(math.Round(float64(x)*a + float64(y)*(1-a))) }
	return ui.RGB(mix(c.R, bg.R), mix(c.G, bg.G), mix(c.B, bg.B))
}

// textColor 是 s 的字色：作者没写过、又在作者的底色上时用 ink；和底下的颜色几乎分不出时（暗色主题里写死的深灰字）换成看得清的。
func (r *hctx) textColor(s *css.Style) ui.Color {
	v := s.Get("color")
	col := r.color(v)
	if v == r.rootC && r.ink.A > 0 {
		return r.ink
	}
	if col.A > 0 && r.bg.A > 0 && contrast(over(col, r.bg), r.bg) < 1.5 {
		return readable(r.bg)
	}
	return col
}

// readable 是 bg 上看得清的字色。
func readable(bg ui.Color) ui.Color {
	if luminance(bg) > 0.4 {
		return ui.Hex("#1f2328")
	}
	return ui.Hex("#e6edf3")
}

// sized 是 s 写了 p（不是 auto）。
func sized(s *css.Style, p string) bool {
	v := s.Get(p)
	return v != "" && v != "auto"
}

func num(v string) (float32, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 32)
	return float32(f), err == nil
}

func atoi(v string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}

func alignOf(v string) (ui.Align, bool) {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "flex-start", "start", "left", "self-start":
		return ui.Start, true
	case "center", "safe center":
		return ui.Center, true
	case "flex-end", "end", "right", "self-end":
		return ui.End, true
	case "stretch", "normal":
		return ui.Stretch, true
	case "baseline", "first baseline", "last baseline":
		return ui.Baseline, true
	case "space-between":
		return ui.SpaceBetween, true
	case "space-around":
		return ui.SpaceAround, true
	case "space-evenly":
		return ui.SpaceEvenly, true
	}
	return 0, false
}

func textAlign(v string) ui.Align {
	switch v {
	case "center":
		return ui.Center
	case "right", "end":
		return ui.End
	}
	return ui.Start
}

// ---- 分类

func (h *hnode) display() string {
	if h.tag == "" {
		return "inline"
	}
	d := strings.ToLower(strings.TrimSpace(h.get("display")))
	if d == "" {
		return "inline"
	}
	return d
}

func (h *hnode) outOfFlow() bool {
	p := h.get("position")
	return h.tag != "" && (p == "absolute" || p == "fixed")
}

// blockLevel 是在块的流里自己占一块的。
func (h *hnode) blockLevel() bool {
	switch h.display() {
	case "inline", "inline-block", "inline-flex", "inline-grid", "inline-table", "contents", "ruby":
		return false
	}
	return true
}

// blank 是只有空白的文字。
func (h *hnode) blank() bool {
	return h.tag == "" && strings.TrimSpace(h.text) == ""
}

// ---- 块

// node 画一个元素：块、flex、grid、表格，或者 flex / grid 里被当成块的行内元素。
func (r *hctx) node(h *hnode, pk parentKind) ui.Element {
	c := r.c
	d := h.display()
	if h.tag == "img" {
		el := r.img(h)
		r.place(el, h, pk)
		return el
	}
	var el ui.Element
	kind := pkBlock
	switch d {
	case "flex", "inline-flex":
		if strings.HasPrefix(h.get("flex-direction"), "column") {
			el = ui.Column(c)
		} else {
			el = ui.Row(c)
		}
		kind = pkFlex
		if w := h.get("flex-wrap"); w == "wrap" || w == "wrap-reverse" {
			el.Wrap()
		}
		if a, ok := alignOf(h.get("justify-content")); ok && a != ui.Stretch {
			el.Justify(a)
		}
		ai := ui.Stretch
		if a, ok := alignOf(h.get("align-items")); ok {
			ai = a
		}
		el.AlignItems(ai)
		if a, ok := alignOf(h.get("align-content")); ok && a != ui.Stretch {
			el.AlignContent(a)
		}
	case "grid", "inline-grid":
		el = ui.Grid(c)
		kind = pkGrid
		fs := r.fontSize(h.st)
		if ts, ok := css.ParseTracks(h.get("grid-template-columns")); ok {
			el.ColumnTracks(r.tracks(ts, fs)...)
		}
		if a, ok := alignOf(h.get("justify-items")); ok {
			el.JustifyItems(a)
		}
		if a, ok := alignOf(h.get("align-items")); ok {
			el.AlignItems(a)
		}
		if a, ok := alignOf(h.get("justify-content")); ok && a != ui.Stretch {
			el.Justify(a)
		}
	case "table", "inline-table":
		el = ui.Grid(c)
		kind = pkGrid
	default:
		el = ui.Column(c).AlignItems(ui.Stretch)
	}
	if kind != pkBlock {
		gy, gx := r.px(h.st, "row-gap"), r.px(h.st, "column-gap")
		if gx > 0 {
			el.GapX(gx)
		}
		if gy > 0 {
			el.GapY(gy)
		}
	}
	rr := r.box(el, h)
	r.place(el, h, pk)
	if kind == pkGrid && (d == "table" || d == "inline-table") && pk == pkBlock && !sized(h.st, "width") {
		el.AlignSelf(ui.Start) // 表格照内容宽，不铺满（和浏览器一样）
	}
	el.Children(func() {
		if d == "list-item" {
			rr.marker(h)
		}
		switch {
		case d == "table" || d == "inline-table":
			rr.table(el, h)
		case kind == pkBlock:
			rr.flow(h)
		default:
			rr.items(h, kind)
		}
	})
	return el
}

// items 画 flex / grid 容器的孩子：每个元素一项，连着的字是一项。
func (r *hctx) items(h *hnode, kind parentKind) {
	var run []*hnode
	flush := func() {
		if len(run) > 0 {
			r.inline(h, run, kind)
			run = run[:0]
		}
	}
	for _, k := range h.kids {
		switch {
		case k.tag == "":
			run = append(run, k)
		case k.display() == "none":
		default:
			flush()
			el := r.node(k, kind)
			if k.outOfFlow() {
				continue
			}
			_ = el
		}
	}
	flush()
}

// flow 画块的流：块级的孩子一个一个往下，连着的行内内容一段一个 RichText；上下外边距照 CSS 合并。
func (r *hctx) flow(h *hnode) {
	type item struct {
		el         ui.Element
		h          *hnode
		mt, mb     float32
		ml, mr     float32
		autoL, aut bool
	}
	var items []item
	var run []*hnode
	flush := func() {
		if len(run) == 0 {
			return
		}
		if el, ok := r.inline(h, run, pkBlock); ok {
			items = append(items, item{el: el})
		}
		run = run[:0]
	}
	for _, k := range h.kids {
		if k.tag != "" && k.display() == "none" {
			continue
		}
		if k.tag != "" && k.outOfFlow() {
			r.node(k, pkBlock)
			continue
		}
		if !k.blockLevel() {
			run = append(run, k)
			continue
		}
		flush()
		el := r.node(k, pkBlock)
		s := k.st
		it := item{el: el, h: k, mt: max(0, r.px(s, "margin-top")), mb: max(0, r.px(s, "margin-bottom")),
			ml: r.px(s, "margin-left"), mr: r.px(s, "margin-right")}
		it.autoL, it.aut = s.Get("margin-left") == "auto", s.Get("margin-right") == "auto"
		items = append(items, it)
	}
	flush()
	// 第一个的上外边距、最后一个的下外边距：父元素有上 / 下边框或内边距才留（不然在 CSS 里和外面的合并了）
	s := h.st
	keepTop := h.tag != "html" && (r.px(s, "padding-top") > 0 || r.borderW(s, "top") > 0 || isFlowRoot(h))
	keepBot := h.tag != "html" && (r.px(s, "padding-bottom") > 0 || r.borderW(s, "bottom") > 0 || isFlowRoot(h))
	prev := float32(0)
	for k := range items {
		it := &items[k]
		top := max(prev, it.mt)
		if k == 0 && !keepTop {
			top = 0
		}
		bot := float32(0)
		if k == len(items)-1 && keepBot {
			bot = it.mb
		}
		if it.h == nil && top == 0 && bot == 0 {
			prev = 0
			continue
		}
		l, rt := it.ml, it.mr
		if it.autoL {
			l = ui.Auto
		}
		if it.aut {
			rt = ui.Auto
		}
		it.el.Margin(top, rt, bot, l)
		if it.autoL || it.aut {
			switch {
			case it.autoL && it.aut:
				it.el.AlignSelf(ui.Center)
			case it.autoL:
				it.el.AlignSelf(ui.End)
			}
		}
		prev = it.mb
	}
}

// isFlowRoot 是自己开一个块格式上下文的（外边距不往外合并）：flex / grid 的项、inline-block、overflow 不是 visible、表格的格。
func isFlowRoot(h *hnode) bool {
	switch h.display() {
	case "flow-root", "inline-block", "table-cell", "flex", "grid", "inline-flex", "inline-grid", "list-item":
		return true
	}
	if o := h.get("overflow"); o != "" && o != "visible" {
		return true
	}
	if p := h.parent; p != nil {
		switch p.display() {
		case "flex", "grid", "inline-flex", "inline-grid":
			return true
		}
	}
	return false
}

func (r *hctx) borderW(s *css.Style, side string) float32 {
	switch s.Get("border-" + side + "-style") {
	case "", "none", "hidden":
		return 0
	}
	switch v := s.Get("border-" + side + "-width"); v {
	case "thin":
		return 1
	case "medium", "":
		return 3
	case "thick":
		return 5
	default:
		return r.px(s, "border-"+side+"-width")
	}
}

// box 给 el 上 h 的盒子样式（尺寸、边框、底色、圆角、内边距、字），返回往孩子传的 hctx（底色变了的话）。
func (r *hctx) box(el ui.Element, h *hnode) *hctx {
	s := h.st
	fs := r.fontSize(s)
	sides := [4]string{"top", "right", "bottom", "left"}
	var bw [4]float32
	var bc [4]ui.Color
	var dashed [4]bool
	for k, sd := range sides {
		bw[k] = r.borderW(s, sd)
		if bw[k] > 0 {
			bc[k] = r.color(s.Get("border-" + sd + "-color"))
			if bc[k].A == 0 {
				bw[k] = 0 // 透明的边：只占地方（CSS 三角形靠它）
				bc[k] = ui.Color{}
				bw[k] = r.borderW(s, sd)
			}
			st := s.Get("border-" + sd + "-style")
			dashed[k] = st == "dashed" || st == "dotted"
		}
	}
	var pad [4]float32
	for k, sd := range sides {
		pad[k] = max(0, r.px(s, "padding-"+sd))
	}
	contentBox := s.Get("box-sizing") != "border-box"
	extraW, extraH := float32(0), float32(0)
	if contentBox {
		extraW = pad[1] + pad[3] + bw[1] + bw[3]
		extraH = pad[0] + pad[2] + bw[0] + bw[2]
	}
	knownW := float32(0)
	if l, ok := css.ParseLength(s.Get("width")); ok && l.U != css.Auto {
		if l.U == css.Percent {
			el.WidthPercent(l.V)
		} else {
			knownW = l.Resolve(fs, r.rem, 0) + extraW
			el.Width(knownW)
		}
	}
	if l, ok := css.ParseLength(s.Get("height")); ok && l.U != css.Auto {
		if l.U == css.Percent {
			el.HeightPercent(l.V)
		} else {
			el.Height(l.Resolve(fs, r.rem, 0) + extraH)
		}
	}
	if l, ok := css.ParseLength(s.Get("min-width")); ok && l.U != css.Auto && l.V > 0 {
		if l.U == css.Percent {
			el.MinWidthPercent(l.V)
		} else {
			el.MinWidth(l.Resolve(fs, r.rem, 0) + extraW)
		}
	}
	if l, ok := css.ParseLength(s.Get("max-width")); ok && l.U != css.Auto && s.Get("max-width") != "none" {
		if l.U == css.Percent {
			el.MaxWidthPercent(l.V)
		} else {
			el.MaxWidth(l.Resolve(fs, r.rem, 0) + extraW)
		}
	}
	if v, ok := r.length(s, "min-height", 0); ok && v > 0 {
		el.MinHeight(v + extraH)
	}
	if l, ok := css.ParseLength(s.Get("max-height")); ok && l.U != css.Auto && l.U != css.Percent && s.Get("max-height") != "none" {
		el.MaxHeight(l.Resolve(fs, r.rem, 0) + extraH)
	}
	// 圆角
	var rad [4]float32
	anyRad := false
	for k, corner := range [4]string{"top-left", "top-right", "bottom-right", "bottom-left"} {
		v := s.Get("border-" + corner + "-radius")
		if f := css.Fields(v); len(f) > 0 {
			v = f[0]
		}
		l, ok := css.ParseLength(v)
		if !ok {
			continue
		}
		if l.U == css.Percent {
			if knownW > 0 {
				rad[k] = knownW * min(l.V, 50) / 100
			} else if l.V >= 50 {
				rad[k] = 9999
			}
		} else {
			rad[k] = l.Resolve(fs, r.rem, 0)
		}
		anyRad = anyRad || rad[k] > 0
	}
	// 底色
	out := r
	bg := r.color(s.Get("background-color"))
	if g := s.Get("background-image"); strings.HasPrefix(strings.ToLower(g), "linear-gradient(") {
		if from, to, angle, ok := r.gradient(g); ok {
			el.Gradient(from, to, angle)
			if bg.A == 0 {
				bg = from
			}
		}
	}
	if bg.A > 0 {
		el.Background(bg)
		eff := over(bg, r.bg)
		cp := *r
		cp.bg = eff
		cp.ink = ui.Color{}
		if s.Get("color") == r.rootC {
			if col := r.color(r.rootC); contrast(col, eff) < 3 {
				cp.ink = readable(eff)
			}
		} else if r.ink.A > 0 && s.Get("color") == r.rootC {
			cp.ink = r.ink
		}
		out = &cp
	}
	// 边框
	visible := 0
	var col ui.Color
	uniform := true
	for k := range bw {
		if bw[k] > 0 && bc[k].A > 0 {
			if visible > 0 && bc[k] != col {
				uniform = false
			}
			col = bc[k]
			visible++
		}
	}
	anyBorder := bw[0]+bw[1]+bw[2]+bw[3] > 0
	if anyBorder {
		allVisible := true
		for k := range bw {
			if bw[k] > 0 && bc[k].A == 0 {
				allVisible = false
			}
		}
		if uniform && allVisible {
			if bw[0] == bw[1] && bw[1] == bw[2] && bw[2] == bw[3] {
				el.Border(bw[0], col)
			} else {
				el.BorderWidth(bw[0], bw[1], bw[2], bw[3]).BorderColor(col)
			}
			if dashed[0] || dashed[1] || dashed[2] || dashed[3] {
				el.BorderStyle(ui.BorderDashed)
			}
		} else {
			// 几条边颜色不一样（或有透明的边，CSS 的三角形）：边框当内边距占地方，自己画梯形
			for k := range pad {
				pad[k] += bw[k]
			}
			w, c := bw, bc
			el.Draw(func(p *ui.Painter, rc ui.Rect) { paintBorders(p, rc, w, c) })
		}
	}
	if pad != [4]float32{} {
		el.Padding(pad[0], pad[1], pad[2], pad[3])
	}
	if anyRad {
		el.Radius(rad[0], rad[1], rad[2], rad[3])
	}
	if sh := s.Get("box-shadow"); sh != "" && sh != "none" {
		if x, y, blur, spread, c, ok := r.shadow(sh, fs); ok {
			el.Shadow(x, y, blur, spread, c)
		}
	}
	if v, ok := num(s.Get("opacity")); ok && v < 1 {
		el.Opacity(max(v, 0))
	}
	if s.Get("visibility") == "hidden" {
		el.Opacity(0)
	}
	if o := s.Get("overflow"); o == "hidden" || o == "clip" || o == "auto" || o == "scroll" || s.Get("overflow-x") == "hidden" {
		el.Clip()
	}
	// 位置
	switch s.Get("position") {
	case "absolute", "fixed":
		el.Absolute()
		r.offsets(el, s)
	case "relative", "sticky":
		r.offsets(el, s)
	}
	if z := s.Get("z-index"); z != "" && z != "auto" {
		el.ZIndex(atoi(z, 0))
	}
	// 字（孩子里的 Text、列表的记号用）
	el.TextColor(out.textColor(s)).FontSize(fs)
	if w := atoi(s.Get("font-weight"), 0); w > 0 && w != 400 {
		el.FontWeight(w)
	}
	if fi := s.Get("font-style"); fi == "italic" || fi == "oblique" {
		el.Italic()
	}
	if f := r.family(s); f != "" {
		el.Font(f)
	}
	if lh := r.lineHeight(s, fs); lh > 0 {
		el.LineHeight(lh)
	}
	return out
}

// place 照元素在什么里面摆，上 flex / grid 项的属性。
func (r *hctx) place(el ui.Element, h *hnode, pk parentKind) {
	s := h.st
	switch pk {
	case pkFlex:
		if v, ok := num(s.Get("flex-grow")); ok && v > 0 {
			el.Grow(v)
		}
		sh := float32(1)
		if v, ok := num(s.Get("flex-shrink")); ok {
			sh = v
		}
		el.Shrink(sh)
		if l, ok := css.ParseLength(s.Get("flex-basis")); ok && l.U != css.Auto && s.Get("flex-basis") != "content" {
			if l.U == css.Percent {
				el.BasisPercent(l.V)
			} else {
				el.Basis(l.Resolve(r.fontSize(s), r.rem, 0))
			}
		}
		if a, ok := alignOf(s.Get("align-self")); ok && s.Get("align-self") != "auto" {
			el.AlignSelf(a)
		}
		r.margins(el, s)
	case pkGrid:
		if cs, span := gridLine(s, "column"); cs != 0 || span != 0 {
			if cs != 0 {
				el.ColumnStart(cs)
			}
			if span != 0 && span != 1 {
				el.ColumnSpan(span)
			}
		}
		if rs, span := gridLine(s, "row"); rs != 0 || span != 0 {
			if rs != 0 {
				el.RowStart(rs)
			}
			if span != 0 && span != 1 {
				el.RowSpan(span)
			}
		}
		if a, ok := alignOf(s.Get("justify-self")); ok && s.Get("justify-self") != "auto" {
			el.JustifySelf(a)
		}
		if a, ok := alignOf(s.Get("align-self")); ok && s.Get("align-self") != "auto" {
			el.AlignSelf(a)
		}
		r.margins(el, s)
	case pkInline:
		r.margins(el, s)
	}
}

func (r *hctx) margins(el ui.Element, s *css.Style) {
	var m [4]float32
	any := false
	for k, sd := range [4]string{"top", "right", "bottom", "left"} {
		if s.Get("margin-"+sd) == "auto" {
			m[k] = ui.Auto
			any = true
			continue
		}
		m[k] = r.px(s, "margin-"+sd)
		any = any || m[k] != 0
	}
	if any {
		el.Margin(m[0], m[1], m[2], m[3])
	}
}

// gridLine 读 grid-column / grid-row：起始线（0 是自动）和跨几格（负的是到最后一条线）。
func gridLine(s *css.Style, axis string) (start, span int) {
	a, b := s.Get("grid-"+axis+"-start"), s.Get("grid-"+axis+"-end")
	if a == "" && b == "" {
		v := s.Get("grid-" + axis)
		if v == "" {
			return 0, 0
		}
		parts := strings.SplitN(v, "/", 2)
		a = strings.TrimSpace(parts[0])
		if len(parts) == 2 {
			b = strings.TrimSpace(parts[1])
		}
	}
	spanOf := func(v string) (int, bool) {
		if f := strings.Fields(v); len(f) == 2 && f[0] == "span" {
			return atoi(f[1], 1), true
		}
		return 0, false
	}
	if n, ok := spanOf(a); ok {
		span = n
	} else if a != "auto" {
		start = atoi(a, 0)
	}
	if n, ok := spanOf(b); ok {
		span = n
	} else if b != "" && b != "auto" {
		end := atoi(b, 0)
		switch {
		case end < 0 && start > 0:
			span = end // 到最后一条线
		case end > start && start > 0:
			span = end - start
		}
	}
	return start, span
}

func (r *hctx) offsets(el ui.Element, s *css.Style) {
	for _, p := range [4]string{"top", "right", "bottom", "left"} {
		l, ok := css.ParseLength(s.Get(p))
		if !ok || l.U == css.Auto {
			continue
		}
		pct := l.U == css.Percent
		v := l.Resolve(r.fontSize(s), r.rem, 0)
		switch p {
		case "top":
			if pct {
				el.TopPercent(l.V)
			} else {
				el.Top(v)
			}
		case "right":
			if pct {
				el.RightPercent(l.V)
			} else {
				el.Right(v)
			}
		case "bottom":
			if pct {
				el.BottomPercent(l.V)
			} else {
				el.Bottom(v)
			}
		case "left":
			if pct {
				el.LeftPercent(l.V)
			} else {
				el.Left(v)
			}
		}
	}
}

// tracks 把 CSS 的列换成 mygo 的。
func (r *hctx) tracks(ts []css.Track, fs float32) []ui.Track {
	out := make([]ui.Track, 0, len(ts))
	for _, t := range ts {
		switch {
		case t.Max.U == css.Fr:
			tr := ui.Fr(t.Max.V)
			switch {
			case t.Min.U == css.Auto:
				tr = tr.MinContent()
			case t.Min.U == css.Px || t.Min.U == css.Em || t.Min.U == css.Rem:
				tr = tr.Min(t.Min.Resolve(fs, r.rem, 0))
			}
			out = append(out, tr)
		case t.Max.U == css.Auto:
			out = append(out, ui.FitContent())
		case t.Max.U == css.Percent:
			out = append(out, ui.Fr(t.Max.V))
		case t.Min.U == css.Auto: // fit-content(x)
			out = append(out, ui.FitContent())
		default:
			out = append(out, ui.Fixed(t.Max.Resolve(fs, r.rem, 0)))
		}
	}
	return out
}

func (r *hctx) family(s *css.Style) string {
	f := strings.ToLower(s.Get("font-family"))
	for _, m := range []string{"mono", "menlo", "consolas", "courier", "code"} {
		if strings.Contains(f, m) {
			return r.st.th.CodeFont
		}
	}
	return ""
}

// lineHeight 是行高的倍数（0 是照外面的）。
func (r *hctx) lineHeight(s *css.Style, fs float32) float32 {
	v := s.Get("line-height")
	switch v {
	case "", "normal":
		return 0
	}
	if f, ok := num(v); ok {
		return f
	}
	if l, ok := css.ParseLength(v); ok && fs > 0 {
		if l.U == css.Percent {
			return l.V / 100
		}
		return l.Resolve(fs, r.rem, 0) / fs
	}
	return 0
}

// gradient 读 linear-gradient(角度?, 颜色, …, 颜色)：取头尾两个颜色。
func (r *hctx) gradient(v string) (from, to ui.Color, angle float32, ok bool) {
	i := strings.IndexByte(v, '(')
	if i < 0 || !strings.HasSuffix(v, ")") {
		return
	}
	args := css.SplitComma(v[i+1 : len(v)-1])
	angle = 180
	var cols []ui.Color
	for k, a := range args {
		a = strings.TrimSpace(a)
		if k == 0 {
			la := strings.ToLower(a)
			if strings.HasSuffix(la, "deg") {
				if f, ok := num(la[:len(la)-3]); ok {
					angle = f
					continue
				}
			}
			if strings.HasPrefix(la, "to ") {
				switch la {
				case "to right":
					angle = 90
				case "to left":
					angle = 270
				case "to top":
					angle = 0
				case "to bottom":
					angle = 180
				}
				continue
			}
		}
		f := css.Fields(a)
		if len(f) == 0 {
			continue
		}
		if c := r.color(f[0]); c.A > 0 || strings.EqualFold(f[0], "transparent") {
			cols = append(cols, c)
		}
	}
	if len(cols) < 2 {
		return
	}
	return cols[0], cols[len(cols)-1], angle, true
}

// shadow 读 box-shadow 的第一个（inset 的不画）。
func (r *hctx) shadow(v string, fs float32) (x, y, blur, spread float32, c ui.Color, ok bool) {
	first := css.SplitComma(v)[0]
	var ls []float32
	c = ui.RGBA(0, 0, 0, 0.2)
	for _, f := range css.Fields(first) {
		if strings.EqualFold(f, "inset") {
			return
		}
		if l, ok := css.ParseLength(f); ok {
			ls = append(ls, l.Resolve(fs, r.rem, 0))
			continue
		}
		if col := r.color(f); col.A > 0 {
			c = col
		}
	}
	if len(ls) < 2 {
		return
	}
	x, y = ls[0], ls[1]
	if len(ls) > 2 {
		blur = ls[2]
	}
	if len(ls) > 3 {
		spread = ls[3]
	}
	return x, y, blur, spread, c, true
}

// paintBorders 画四条边：每条是一个梯形（两头斜着接），颜色可以各不一样，透明的不画——CSS 的三角形就是这样画出来的。
func paintBorders(p *ui.Painter, r ui.Rect, w [4]float32, c [4]ui.Color) {
	x0, y0, x1, y1 := r.X, r.Y, r.X+r.W, r.Y+r.H
	t, rt, b, l := w[0], w[1], w[2], w[3]
	quads := [4][4][2]float32{
		{{x0, y0}, {x1, y0}, {x1 - rt, y0 + t}, {x0 + l, y0 + t}},
		{{x1, y0}, {x1, y1}, {x1 - rt, y1 - b}, {x1 - rt, y0 + t}},
		{{x1, y1}, {x0, y1}, {x0 + l, y1 - b}, {x1 - rt, y1 - b}},
		{{x0, y1}, {x0, y0}, {x0 + l, y0 + t}, {x0 + l, y1 - b}},
	}
	for k, q := range quads {
		if w[k] <= 0 || c[k].A == 0 {
			continue
		}
		path := &ui.Path{}
		path.MoveTo(q[0][0], q[0][1])
		for _, pt := range q[1:] {
			path.LineTo(pt[0], pt[1])
		}
		path.Close()
		p.FillPath(path, c[k])
	}
}

// ---- 列表、表格、图片

// marker 画列表项前面的记号：放在项的左边外面（ul / ol 的左内边距里）。
func (r *hctx) marker(h *hnode) {
	t := strings.ToLower(h.get("list-style-type"))
	if t == "" {
		t = "disc"
	}
	if t == "none" {
		return
	}
	n := 1
	if p := h.parent; p != nil && p.tag == "ol" {
		n = atoi(p.attr("start"), 1)
	}
	for k := h.prev; k != nil; k = k.prev {
		if k.display() == "list-item" {
			n++
		}
	}
	if v := h.attr("value"); v != "" {
		n = atoi(v, n)
	}
	var m string
	switch t {
	case "disc":
		m = "•"
	case "circle":
		m = "◦"
	case "square":
		m = "▪"
	case "decimal":
		m = itoa(n) + "."
	case "lower-alpha", "lower-latin":
		m = string(rune('a'+(n-1)%26)) + "."
	case "upper-alpha", "upper-latin":
		m = string(rune('A'+(n-1)%26)) + "."
	case "lower-roman":
		m = strings.ToLower(roman(n)) + "."
	case "upper-roman":
		m = roman(n) + "."
	default:
		if len(t) >= 2 && (t[0] == '"' || t[0] == '\'') {
			m = unescapeCSS(t[1 : len(t)-1])
		} else {
			m = "•"
		}
	}
	fs := r.fontSize(h.st)
	w := fs * 1.6
	ui.Text(r.c, m).Absolute().Left(-w).Top(0).Width(w - fs*0.4).TextAlign(ui.End).Unselectable().FontFeatures("tnum")
}

func roman(n int) string {
	if n <= 0 || n >= 4000 {
		return itoa(n)
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var b strings.Builder
	for k, v := range vals {
		for n >= v {
			b.WriteString(syms[k])
			n -= v
		}
	}
	return b.String()
}

// table 画 <table>：一个 Grid，列数是最多的那一行，colspan 跨列。
func (r *hctx) table(el ui.Element, h *hnode) {
	var rows []*hnode
	var caption *hnode
	var walk func(list []*hnode)
	walk = func(list []*hnode) {
		for _, k := range list {
			switch k.display() {
			case "table-row":
				rows = append(rows, k)
			case "table-row-group", "table-header-group", "table-footer-group":
				walk(k.kids)
			case "table-caption":
				caption = k
			}
		}
	}
	walk(h.kids)
	cols := 0
	for _, row := range rows {
		n := 0
		for _, cell := range row.kids {
			if cell.tag != "" && cell.display() == "table-cell" {
				n += max(1, atoi(cell.attr("colspan"), 1))
			}
		}
		cols = max(cols, n)
	}
	if cols == 0 {
		return
	}
	tracks := make([]ui.Track, cols)
	for k := range tracks {
		tracks[k] = ui.FitContent()
	}
	el.ColumnTracks(tracks...)
	if sp := r.px(h.st, "border-spacing"); sp > 0 && h.get("border-collapse") != "collapse" {
		el.Gap(sp)
	}
	if caption != nil {
		r.node(caption, pkGrid).ColumnSpan(-1)
	}
	for _, row := range rows {
		rowBg := r.color(row.get("background-color"))
		for _, cell := range row.kids {
			if cell.tag == "" || cell.display() != "table-cell" {
				continue
			}
			cr := r
			if rowBg.A > 0 && r.color(cell.get("background-color")).A == 0 {
				cp := *r
				cp.bg = over(rowBg, r.bg)
				cr = &cp
			}
			ce := cr.node(cell, pkGrid)
			if rowBg.A > 0 && r.color(cell.get("background-color")).A == 0 {
				ce.Background(rowBg)
			}
			if n := atoi(cell.attr("colspan"), 1); n > 1 {
				ce.ColumnSpan(n)
			}
		}
	}
}

// img 画 <img>：读好了照 width / height（属性或 CSS）或原大小，最宽和容器一样；没读好显示 alt。
func (r *hctx) img(h *hnode) ui.Element {
	c := r.c
	th := &r.st.th
	src := r.st.image(c, h.attr("src"))
	alt := h.attr("alt")
	s := h.st
	fs := r.fontSize(s)
	w, wok := r.length(s, "width", 0)
	hh, hok := r.length(s, "height", 0)
	if !wok {
		if v, ok := num(strings.TrimSuffix(h.attr("width"), "px")); ok {
			w, wok = v, true
		}
	}
	if !hok {
		if v, ok := num(strings.TrimSuffix(h.attr("height"), "px")); ok {
			hh, hok = v, true
		}
	}
	if src == nil {
		label := alt
		if label == "" {
			label = r.st.o.Strings.or(r.st.o.Strings.Image, "Image")
		}
		ph := ui.Row(c).Padding(em(th, 0.3), em(th, 0.5)).Radius(em(th, 0.3)).Background(th.CodeBackground).Role(ui.RoleImage).Label(alt)
		ph.Children(func() { ui.Text(c, label).TextColor(th.Muted).FontSize(fs * 0.875) })
		return ph
	}
	img := ui.Image(c, src).MaxWidthPercent(100).Label(alt)
	iw, ih := imageSize(src)
	switch {
	case wok && hok:
		img.Size(w, hh)
	case wok:
		img.Width(w)
		if iw > 0 && ih > 0 {
			img.AspectRatio(iw / ih)
		}
	case hok:
		img.Height(hh)
		if iw > 0 && ih > 0 {
			img.AspectRatio(iw / ih)
		}
	default:
		if iw > 0 && ih > 0 {
			img.Width(iw).AspectRatio(iw / ih)
		}
	}
	if v := s.Get("border-top-left-radius"); v != "" {
		if rad := r.px(s, "border-top-left-radius"); rad > 0 {
			img.Radius(rad)
		}
	}
	if t := h.attr("title"); t != "" {
		img.Tooltip(t)
	}
	return img
}

// ---- 行内

// irun 是一段行内内容建好的字：一个 RichText 的几段（链接是单独的一段）和字里的块。
type irun struct {
	pieces  []hpiece
	objects []mathObject
	align   ui.Align
	nowrap  bool
	empty   bool
}

type hpiece struct {
	link        int // -1 不是链接；别的是 links 的下标
	href, title string
	spans       []ui.Span
}

// ib 建一段行内内容的字。
type ib struct {
	r     *hctx
	run   *irun
	cur   *hpiece
	runes int
	space bool // 前面是空白（或行首）：压掉接着的空白
}

// ictx 是行内元素往里传的：装饰线、行内盒子的底色和圆角、在哪个链接里。
type ictx struct {
	underline, strike bool
	bg                ui.Color
	radius, padX      float32
	padY              float32
	link              int
	href, title       string
	linkColor         bool
}

// atomic 是行内内容里画不进字的：图片、里面有块的 inline-block。
func atomic(h *hnode) bool {
	if h.tag == "" {
		return false
	}
	if h.tag == "img" || h.tag == "video" || h.tag == "svg" || h.tag == "iframe" || h.tag == "picture" && hasImg(h) {
		return true
	}
	switch h.display() {
	case "inline-flex", "inline-grid", "inline-table":
		return true
	case "inline-block":
		if len(h.kids) == 0 || allBlank(h.kids) {
			return false // 空的：字里的一块
		}
		for _, k := range h.kids {
			if k.tag != "" && (k.blockLevel() || atomic(k)) {
				return true
			}
		}
		if sized(h.st, "width") || sized(h.st, "height") {
			return true
		}
		return false
	}
	for _, k := range h.kids {
		if atomic(k) || k.tag != "" && k.blockLevel() && k.display() != "none" {
			return true
		}
	}
	return false
}

func hasImg(h *hnode) bool {
	for _, k := range h.kids {
		if k.tag == "img" || hasImg(k) {
			return true
		}
	}
	return false
}

func allBlank(list []*hnode) bool {
	for _, k := range list {
		if !k.blank() {
			return false
		}
	}
	return true
}

// inline 画 parent 里连着的一段行内内容；全是空白时什么也不画（ok = false）。
func (r *hctx) inline(parent *hnode, run []*hnode, pk parentKind) (ui.Element, bool) {
	if allBlank(run) {
		return ui.Element{}, false
	}
	c := r.c
	for _, k := range run {
		if atomic(k) {
			row := ui.Row(c).Wrap().AlignItems(ui.Baseline)
			if a := textAlign(parent.get("text-align")); a != ui.Start {
				row.Justify(a)
			}
			if pk == pkFlex || pk == pkGrid {
				row.Shrink(1)
			}
			row.Children(func() { r.mixed(parent, run) })
			return row, true
		}
	}
	ir := r.hs.runs[run[0]]
	if ir == nil {
		ir = r.buildRun(parent, run)
		r.hs.runs[run[0]] = ir
	}
	if ir.empty {
		return ui.Element{}, false
	}
	el := r.richText(ir)
	el.TextAlign(ir.align)
	if pk == pkFlex {
		el.Shrink(1)
	}
	return el, true
}

// mixed 画有图片、复杂 inline-block 的一段：一行会折的 Row，字一段一个 RichText，图片和盒子各一项。
func (r *hctx) mixed(parent *hnode, run []*hnode) {
	var text []*hnode
	flush := func() {
		if len(text) > 0 && !allBlank(text) {
			ir := r.hs.runs[text[0]]
			if ir == nil {
				ir = r.buildRun(parent, text)
				r.hs.runs[text[0]] = ir
			}
			if !ir.empty {
				r.richText(ir)
			}
		}
		text = nil
	}
	for _, k := range run {
		if !atomic(k) {
			text = append(text, k)
			continue
		}
		flush()
		if k.tag == "a" || k.tag == "picture" || k.display() == "inline" {
			// 里面有图片的链接：链接里一行
			if href := k.attr("href"); k.tag == "a" && href != "" {
				r.st.link(r.c, href, k.attr("title"), false, func() {
					ui.Row(r.c).Wrap().AlignItems(ui.Baseline).Children(func() { r.mixed(k, k.kids) })
				})
				continue
			}
			ui.Row(r.c).Wrap().AlignItems(ui.Baseline).Children(func() { r.mixed(k, k.kids) })
			continue
		}
		r.node(k, pkInline)
	}
	flush()
}

func (r *hctx) richText(ir *irun) ui.Element {
	c := r.c
	ps := ir.pieces
	var el ui.Element
	if ps[0].link == -1 {
		el = ui.RichText(c, ps[0].spans...)
		ps = ps[1:]
	} else {
		el = ui.RichText(c)
	}
	if len(ps) > 0 {
		el.Children(func() {
			for k := range ps {
				p := &ps[k]
				if p.link == -1 {
					ui.RichText(c, p.spans...)
					continue
				}
				r.st.link(c, p.href, p.title, false, func() { ui.RichText(c, p.spans...) })
			}
		})
	}
	if ir.nowrap {
		el.NoWrap()
	}
	if objects := ir.objects; len(objects) > 0 {
		el.DrawObjects(func(pa *ui.Painter, at int, rc ui.Rect) {
			for k := range objects {
				if objects[k].rune == at && objects[k].draw != nil {
					objects[k].draw(pa, rc)
				}
			}
		})
	}
	if r.block >= 0 {
		r.st.note(el.ID(), r.block, -2)
	}
	return el
}

func (r *hctx) buildRun(parent *hnode, run []*hnode) *irun {
	ir := &irun{align: textAlign(parent.get("text-align"))}
	ir.nowrap = parent.get("white-space") == "nowrap" || parent.get("white-space") == "pre"
	b := &ib{r: r, run: ir, space: true}
	for _, k := range run {
		b.node(k, ictx{link: -1})
	}
	if len(ir.pieces) == 0 {
		ir.empty = true
		return ir
	}
	// 一段的最后一个空白不要
	last := &ir.pieces[len(ir.pieces)-1]
	if n := len(last.spans); n > 0 {
		last.spans[n-1].Text = strings.TrimRight(last.spans[n-1].Text, " ")
	}
	ir.empty = true
	for _, p := range ir.pieces {
		for _, sp := range p.spans {
			if sp.Text != "" {
				ir.empty = false
			}
		}
	}
	return ir
}

func (b *ib) piece(x ictx) *hpiece {
	if b.cur == nil || b.cur.link != x.link {
		b.run.pieces = append(b.run.pieces, hpiece{link: x.link, href: x.href, title: x.title})
		b.cur = &b.run.pieces[len(b.run.pieces)-1]
	}
	return b.cur
}

func (b *ib) node(h *hnode, x ictx) {
	r := b.r
	if h.tag == "" {
		b.text(h.text, h.st, x)
		return
	}
	if h.display() == "none" {
		return
	}
	switch h.tag {
	case "br":
		b.add("\n", h.st, x)
		b.space = true
		return
	case "wbr":
		b.add("​", h.st, x)
		return
	}
	s := h.st
	if h.tag == "a" {
		if href := h.attr("href"); href != "" && x.link == -1 {
			x.link, x.href, x.title = len(b.run.pieces), href, h.attr("title")
			// 作者没给链接上色：用主题的链接色
			x.linkColor = h.parent == nil || s.Get("color") == h.parent.get("color")
			b.cur = nil
		}
	}
	if d := s.Get("text-decoration-line"); d != "" {
		x.underline = x.underline || strings.Contains(d, "underline")
		x.strike = x.strike || strings.Contains(d, "line-through")
	} else if d := s.Get("text-decoration"); d != "" {
		x.underline = x.underline || strings.Contains(d, "underline")
		x.strike = x.strike || strings.Contains(d, "line-through")
	}
	if bg := r.color(s.Get("background-color")); bg.A > 0 {
		x.bg = bg
		x.radius = r.px(s, "border-top-left-radius")
		if v := s.Get("border-top-left-radius"); strings.HasSuffix(v, "%") {
			x.radius = 9999
		}
		x.padX = max(0, r.px(s, "padding-left"))
		x.padY = max(0, r.px(s, "padding-top"))
	}
	if h.display() == "inline-block" && allBlank(h.kids) {
		b.object(h, x)
		return
	}
	// 行内元素左右的外边距、内边距（没底色时）：用空白顶上，大概齐
	if ml := r.px(s, "margin-left") + map0(x.bg.A == 0, r.px(s, "padding-left")); ml >= r.fontSize(s)*0.2 && len(b.run.pieces) > 0 {
		b.add(" ", s, ictx{link: x.link, href: x.href, title: x.title})
	}
	for _, k := range h.kids {
		if k.tag != "" && k.outOfFlow() {
			continue // 行内元素里绝对定位的（提示框）：不画
		}
		b.node(k, x)
	}
	if mr := r.px(s, "margin-right") + map0(x.bg.A == 0, r.px(s, "padding-right")); mr >= r.fontSize(s)*0.2 {
		b.add(" ", s, ictx{link: x.link, href: x.href, title: x.title})
	}
}

func map0(ok bool, v float32) float32 {
	if ok {
		return v
	}
	return 0
}

// text 按 white-space 处理一段字再加进去。
func (b *ib) text(t string, s *css.Style, x ictx) {
	ws := s.Get("white-space")
	switch ws {
	case "pre", "pre-wrap", "break-spaces":
		t = strings.ReplaceAll(t, "\r\n", "\n")
		if t == "" {
			return
		}
		b.add(t, s, x)
		b.space = strings.HasSuffix(t, "\n")
		return
	}
	var out strings.Builder
	for _, ch := range t {
		switch ch {
		case '\n', '\r':
			if ws == "pre-line" {
				out.WriteByte('\n')
				b.space = true
				continue
			}
			fallthrough
		case ' ', '\t', '\f':
			if !b.space {
				if ws == "nowrap" {
					out.WriteRune(' ')
				} else {
					out.WriteByte(' ')
				}
				b.space = true
			}
		default:
			out.WriteRune(ch)
			b.space = false
		}
	}
	if out.Len() == 0 {
		return
	}
	text := out.String()
	switch s.Get("text-transform") {
	case "uppercase":
		text = strings.ToUpper(text)
	case "lowercase":
		text = strings.ToLower(text)
	}
	b.add(text, s, x)
}

func (b *ib) span(text string, s *css.Style, x ictx) ui.Span {
	r := b.r
	th := &r.st.th
	sp := ui.Span{Text: text, Font: th.Font, Size: r.fontSize(s), Weight: atoi(s.Get("font-weight"), 400),
		Underline: x.underline, Strikethrough: x.strike}
	if f := r.family(s); f != "" {
		sp.Font = f
	}
	if fi := s.Get("font-style"); fi == "italic" || fi == "oblique" {
		sp.Italic = true
	}
	cr := r
	if x.bg.A > 0 {
		cp := *r
		cp.bg = over(x.bg, r.bg)
		if s.Get("color") == r.rootC {
			if col := r.color(r.rootC); contrast(col, cp.bg) < 3 {
				cp.ink = readable(cp.bg)
			}
		}
		cr = &cp
		sp.Background, sp.Radius, sp.PaddingX, sp.PaddingY = x.bg, x.radius, x.padX, x.padY
	}
	sp.Color = cr.textColor(s)
	if x.linkColor {
		sp.Color = th.Link
	}
	if v := s.Get("letter-spacing"); v != "" && v != "normal" {
		sp.LetterSpacing = r.px(s, "letter-spacing")
	}
	switch s.Get("vertical-align") {
	case "sub", "super":
		sp.Size *= 0.8
	}
	return sp
}

func (b *ib) add(text string, s *css.Style, x ictx) {
	p := b.piece(x)
	p.spans = append(p.spans, b.span(text, s, x))
	b.runes += utf8.RuneCountInString(text)
}

// object 把一个空的 inline-block（色块、圆点）放进字里：一块照它的大小、外边距，画底色和边框。
func (b *ib) object(h *hnode, x ictx) {
	r := b.r
	s := h.st
	fs := r.fontSize(s)
	pad := [4]float32{r.px(s, "padding-top"), r.px(s, "padding-right"), r.px(s, "padding-bottom"), r.px(s, "padding-left")}
	bw := [4]float32{r.borderW(s, "top"), r.borderW(s, "right"), r.borderW(s, "bottom"), r.borderW(s, "left")}
	w := r.px(s, "width") + pad[1] + pad[3] + bw[1] + bw[3]
	hh := r.px(s, "height") + pad[0] + pad[2] + bw[0] + bw[2]
	if s.Get("box-sizing") == "border-box" {
		w, hh = r.px(s, "width"), r.px(s, "height")
	}
	if w <= 0 || hh <= 0 {
		return
	}
	ml, mr := max(0, r.px(s, "margin-left")), max(0, r.px(s, "margin-right"))
	bg := r.color(s.Get("background-color"))
	var bc [4]ui.Color
	for k, sd := range [4]string{"top", "right", "bottom", "left"} {
		bc[k] = r.color(s.Get("border-" + sd + "-color"))
	}
	rad := r.px(s, "border-top-left-radius")
	if strings.HasSuffix(s.Get("border-top-left-radius"), "%") {
		rad = min(w, hh) / 2
	}
	descent := float32(0)
	if s.Get("vertical-align") == "middle" {
		descent = max(0, hh/2-fs*0.3)
	}
	b.run.objects = append(b.run.objects, mathObject{rune: b.runes, draw: func(p *ui.Painter, rc ui.Rect) {
		box := ui.Rect{X: rc.X + ml, Y: rc.Y, W: w, H: hh}
		if bg.A > 0 {
			p.Fill(box, bg, rad)
		}
		if bw[0] > 0 && bw[0] == bw[1] && bw[1] == bw[2] && bw[2] == bw[3] && bc[0] == bc[1] && bc[1] == bc[2] && bc[2] == bc[3] {
			p.Stroke(box, bc[0], rad, bw[0])
		} else if bw != [4]float32{} {
			paintBorders(p, box, bw, bc)
		}
	}})
	pc := b.piece(x)
	pc.spans = append(pc.spans, ui.Span{Text: "￼", Size: fs, ObjectWidth: ml + w + mr, ObjectHeight: hh - descent, ObjectDescent: descent})
	b.runes++
	b.space = false
}
