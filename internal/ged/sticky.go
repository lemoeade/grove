package ged

import (
	"strconv"

	wui "github.com/egoist/mygo/ui"
)

// 粘性滚动（VS Code 的 editor.stickyScroll，Diff 里两边各吸各的）：滚动时把包着每边最上面那几行的函数 / 类 / 标题
// 那一行吸在那边的顶上，最多 stickyMax 行（视图矮时不超过三分之一），点一行跳过去。哪几行由 Style.Sticky 给
// （调用方按自己的大纲算），这里只管排、画、点。统一视图只有一栏，按新的一边吸。

// stickyMax 是最多吸几行。
const stickyMax = 5

// stickyRows 是 side 那边这一帧该吸的行（从外到里）。
func (d *Diff) stickyRows(side int, g geom, s *Style) []int {
	if s.Sticky == nil || g.lh <= 0 || len(d.lines[side]) == 0 {
		return nil
	}
	n := min(stickyMax, int(g.h/g.lh)/3)
	if n <= 0 {
		return nil
	}
	rowLine := func(k int) int { // 顶上吸了 k 行时，紧挨着它们下面那一行在 side 那边是第几行（对面多的空行往下找）
		v := int((d.ScrollY + float32(k)*g.lh) / g.lh)
		for i := d.rowIndex(v); i < len(d.rows); i++ {
			if l := lineOf(d.rows[i], side); l >= 0 {
				return l
			}
		}
		return 1 << 30
	}
	var out []int
	for _, l := range s.Sticky(side, rowLine, n) {
		if l >= 0 && l < len(d.lines[side]) {
			out = append(out, l)
		}
	}
	return out
}

// prepSticky 每帧画之前算两边吸着的行。
func (d *Diff) prepSticky(g geom, s *Style) {
	d.stuck = [2][]int{}
	if d.unified {
		d.stuck[1] = d.stickyRows(1, g, s)
		return
	}
	for side := range 2 {
		d.stuck[side] = d.stickyRows(side, g, s)
	}
}

// stickyPane 是 x 在哪一栏（统一视图只有新的一边那一栏）。
func (d *Diff) stickyPane(x float32, g geom) int {
	if d.unified || x >= g.paneX[1] {
		return 1
	}
	return 0
}

// paintSticky 把吸着的行画在每一栏的顶上：编辑器的底、行号、高亮过的字（只画第一行，折行的不展开），下面一道淡阴影。
func (d *Diff) paintSticky(p *wui.Painter, r wui.Rect, g geom, s *Style) {
	for side := range 2 {
		rows := d.stuck[side]
		if len(rows) == 0 {
			continue
		}
		pane := side
		if d.unified {
			pane = 0
		}
		x0 := r.X + g.paneX[pane]
		pw := min(g.paneW[pane], g.ruler.X-g.paneX[pane]) // 不盖右边的概览条
		h := float32(len(rows)) * g.lh
		p.Fill(wui.Rect{X: x0, Y: r.Y, W: pw, H: h}, s.Code.Background, 0)
		for k, line := range rows {
			y := r.Y + float32(k)*g.lh
			if d.stickyHover == (stickyHit{side, k, true}) {
				p.Fill(wui.Rect{X: x0, Y: y, W: pw, H: g.lh}, s.FoldHover, 0)
			}
			base := y + (g.lh-(g.asc+g.desc))/2 + g.asc
			if num := wui.Shape(strconv.Itoa(line+1), s.Font); len(num) > 0 {
				right := g.numR
				if d.unified {
					right = g.numR2
				}
				last := num[len(num)-1]
				p.Glyphs(num, x0+right-(last.X+last.Advance), base, s.Code.LineNumber)
			}
			tx := x0 + g.textX
			l := d.layout(side, line, s)
			p.Clip(wui.Rect{X: tx, Y: y, W: max(0, x0+pw-tx), H: g.lh}, 0, func() {
				p.TextLayout(l.l, tx-d.scrollX[side], y+(g.lh-l.lt)/2, s.Code.Text)
			})
		}
		for i, a := range []float32{1, 0.5, 0.2} {
			p.Fill(wui.Rect{X: x0, Y: r.Y + h + float32(i), W: pw, H: 1}, s.StickyShadow.Alpha(a), 0)
		}
	}
}

// stickyHit 是指针在哪一栏吸着的第几行上。
type stickyHit struct {
	side, k int
	ok      bool
}

// stickyAt 是视图里 (x, y) 在不在吸着的行上。
func (d *Diff) stickyAt(x, y float32, g geom) stickyHit {
	if g.lh <= 0 || y < 0 || x >= g.ruler.X {
		return stickyHit{}
	}
	side := d.stickyPane(x, g)
	k := int(y / g.lh)
	if k >= len(d.stuck[side]) {
		return stickyHit{}
	}
	return stickyHit{side, k, true}
}

// stickyInput 接吸着的行上的指针（不在拖滚动条、拖选、拖选行的时候）：悬停垫一层，按下跳到那一行（滚到它刚好露在还吸着的那几行下面）。
func (d *Diff) stickyInput(ev wui.InputEvent, g geom) (took, handled bool) {
	switch ev.Kind {
	case wui.InputPointerMove:
		h := d.stickyAt(ev.X, ev.Y, g)
		changed := h != d.stickyHover
		d.stickyHover = h
		if h.ok {
			return changed, true
		}
		return changed, false
	case wui.InputPointerDown:
		h := d.stickyAt(ev.X, ev.Y, g)
		if !h.ok || ev.Button != 0 {
			return false, false
		}
		line := d.stuck[h.side][h.k]
		if i := d.rowOfLine(h.side, line); i >= 0 {
			d.ScrollY = float32(max(0, d.top(i)-h.k)) * g.lh
			d.clamp(g)
		}
		d.stickyHover = stickyHit{}
		d.focus = true
		return true, true
	}
	return false, false
}

// Stuck 是这一帧 side 那边吸在顶上的行（从外到里；统一视图只有新的一边）。
func (d *Diff) Stuck(side int) []int {
	if side < 0 || side > 1 {
		return nil
	}
	return d.stuck[side]
}
