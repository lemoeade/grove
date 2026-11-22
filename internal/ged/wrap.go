package ged

import (
	"math"
	"sort"
	"unicode/utf8"

	wui "github.com/egoist/mygo/ui"
)

// 自动换行（Style.Wrap）：长行按那边正文的宽折成几行，一个 Row 占几个“屏幕行”（lh 高的一条）按两边折出来多的那边算
// （统一视图按那一行自己）。tops 是每个 Row 从第几个屏幕行开始的前缀和；没排过的行按字数估（等宽字体，宽字算两格），
// 排出来和估的不一样就重算 tops（看得见的行画的时候都排了，所以看得见的那段是准的；上面没看过的按估的，滚过去时会挪一点）。
// 不换行时 top(i) 就是 i，别的都和原来一样。

// setWrap 开、关自动换行：排好的字作废，滚动位置留在顶上那一行。
func (d *Diff) setWrap(on bool) {
	if d.wrap == on {
		return
	}
	at := -1
	if d.lh > 0 {
		at = d.rowIndex(int(d.ScrollY / d.lh))
	}
	d.wrap = on
	d.layouts, d.widest, d.wrapW = nil, [2]float32{}, [2]float32{}
	d.topsDirty = true
	d.scrollX = [2]float32{}
	if at >= 0 && at < len(d.rows) && d.want < 0 {
		d.wrapAt = at // 知道宽了再滚过去（applyWant）
	}
}

// setWrapWidth 按这一帧的版面定两边折行的宽：变了（窗口拉宽、拉窄）排好的字作废。
func (d *Diff) setWrapWidth(g geom) {
	var ww [2]float32
	for side := range 2 {
		room := g.paneW[side] - g.textX - 8
		if side == 1 || d.unified { // 右边那格让出概览条
			room = g.paneW[side] - g.textX - g.right - 4
		}
		ww[side] = max(room, 10*g.cw)
	}
	if ww != d.wrapW {
		d.wrapW = ww
		d.layouts, d.widest = nil, [2]float32{}
		d.topsDirty = true
	}
}

// estimate 是 side 那边第 line 行大概折成几行：按显示的格数（制表符展开，宽字两格）乘一个数字的宽。
func (d *Diff) estimate(side, line int, s *Style) int {
	if !d.wrap || d.wrapW[side] <= 0 {
		return 1
	}
	tab := s.Tab
	if tab <= 0 {
		tab = 4
	}
	src := clean(d.lines[side][line])
	col := 0
	for _, r := range src {
		switch {
		case r == '\t':
			col += tab - col%tab
		case wide(r):
			col += 2
		default:
			col++
		}
	}
	cw := charWidth(s.Font)
	return max(1, int(math.Ceil(float64(float32(col)*cw/d.wrapW[side]))))
}

// wide 是 East Asian 宽字（中日韩、全角标点、表情）：等宽字体里占两格。
func wide(r rune) bool {
	return r >= 0x1100 && (r <= 0x115f || r >= 0x2e80 && r <= 0xa4cf || r >= 0xac00 && r <= 0xd7a3 ||
		r >= 0xf900 && r <= 0xfaff || r >= 0xfe30 && r <= 0xfe4f || r >= 0xff00 && r <= 0xff60 ||
		r >= 0xffe0 && r <= 0xffe6 || r >= 0x1f300 && r <= 0x1faff || r >= 0x20000 && r < utf8.MaxRune)
}

// lineCount 是 side 那边第 line 行折成几行：排过的按排的，没排过的估。
func (d *Diff) lineCount(side, line int, s *Style) int {
	if l := d.layouts[lineKey{side, line}]; l != nil {
		return l.n
	}
	return d.estimate(side, line, s)
}

// rowLines 是 row 占几个屏幕行。
func (d *Diff) rowLines(r Row, s *Style) int {
	if r.Kind == Fold {
		return 1
	}
	n := 1
	for side := range 2 {
		if d.unified && side != d.rowSide(r) {
			continue
		}
		if l := lineOf(r, side); l >= 0 && l < len(d.lines[side]) {
			n = max(n, d.lineCount(side, l, s))
		}
	}
	return n
}

// buildTops 重算 tops。s 只用来估（制表符几格、字体）；排过的行不用它。
func (d *Diff) buildTops(s *Style) {
	d.tops = d.tops[:0]
	t := 0
	for _, r := range d.rows {
		d.tops = append(d.tops, t)
		t += d.rowLines(r, s)
	}
	d.tops = append(d.tops, t)
	d.topsDirty = false
}

// top 是 rows[i] 从第几个屏幕行开始（i 可以是 len(rows)：总共几个屏幕行）。
func (d *Diff) top(i int) int {
	if !d.wrap {
		return i
	}
	d.ensureTops()
	return d.tops[max(0, min(i, len(d.tops)-1))]
}

// rowIndex 是第 v 个屏幕行在哪个 Row 里；过了最后一行是 len(rows)。
func (d *Diff) rowIndex(v int) int {
	if !d.wrap {
		return max(0, min(v, len(d.rows)))
	}
	d.ensureTops()
	return sort.Search(len(d.rows), func(k int) bool { return d.tops[k+1] > v })
}

func (d *Diff) ensureTops() {
	if d.topsDirty || len(d.tops) != len(d.rows)+1 {
		s := d.style
		if s == nil {
			s = &Style{}
		}
		d.buildTops(s)
	}
}

// rowH 是 rows[i] 画出来多高：折行时先把它两边排了（排了才知道折成几行）。
func (d *Diff) rowH(i int, s *Style, g geom) float32 {
	if !d.wrap {
		return g.lh
	}
	r := d.rows[i]
	for side := range 2 {
		if d.unified && side != d.rowSide(r) {
			continue
		}
		if l := lineOf(r, side); l >= 0 && r.Kind != Fold {
			d.layout(side, l, s)
		}
	}
	return float32(d.top(i+1)-d.top(i)) * g.lh
}

// rowY 把排出来的 y（第几行的顶）换成画的 y：第 k 个折出来的行画在 y + k·lh。
func (l *lineLayout) rowY(y, lh float32) func(float32) float32 {
	return func(ly float32) float32 {
		if l.lt <= 0 {
			return y
		}
		return y + float32(math.Round(float64(ly/l.lt)))*lh
	}
}

// caretRow 是第 off 个字节前的光标在折出来的第几行。
func (l *lineLayout) caretRow(off int) int {
	if l.n <= 1 || l.lt <= 0 {
		return 0
	}
	c := l.l.Caret(l.u16[max(0, min(off, len(l.u16)-1))])
	return min(l.n-1, int(math.Round(float64(c.Y/l.lt))))
}

// indexOnRow 是折出来的第 k 行里横着在 x 的位置（第几个字节前）。
func (l *lineLayout) indexOnRow(k int, x float32) int {
	return byteAt(l.u16, l.l.IndexAt(wui.Point{X: max(x, 0), Y: (float32(k) + 0.5) * l.lt}))
}

// RowTop 是 rows[i] 的顶在整个内容里的 y（上一帧的行高；i 可以是 len(rows)）。给测试用。
func (d *Diff) RowTop(i int) float32 { return float32(d.top(i)) * d.lh }
