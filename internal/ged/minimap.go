package ged

import (
	"unicode/utf8"

	wui "github.com/egoist/mygo/ui"
)

// 小地图：单栏编辑器右边、概览条左边的一条缩略图（照 VS Code 的 minimap）。每行 mmLH 高，每个字 mmCW 宽，
// 按词的颜色画成细条（空白不画）；整个文件比小地图高时它跟着编辑器按比例滚（编辑器滚到底它也到底）。
// 当前一屏的那块（滑块）一直淡淡地画着，指针在上面时深一档；按在滑块上拖着滚，按在别处先把滑块挪到指针下、接着拖。
const (
	minimapW    = 100 // 多宽（VS Code 默认一行最多画 120 个字，常见的宽差不多这么多）
	minimapMinW = 480 // 编辑器比这窄就不画
	mmLH        = 2   // 一行多高
	mmCW        = 1   // 一个字多宽（VS Code 的 minimap.scale 1）
	mmPad       = 2   // 左边空多少
)

type mmState struct {
	on, drag bool
	off      float32 // 拖的时候指针离滑块顶多远
	lines    map[int]mmLine
}

type mmLine struct {
	src   string
	state uint64
	runs  []mmRun
}

type mmRun struct {
	c0, c1 float32 // 从第几格到第几格（Tab 展开后）
	col    wui.Color
}

// mmScroll 是小地图自己滚了多少；ratio 是小地图的一点对编辑器多少点。
func (d *Diff) mmScroll(g geom) float32 {
	total := g.content * mmLH / g.lh
	most := g.content - g.h + d.tail(g)
	if total <= g.h || most <= 0 {
		return 0
	}
	return min(max(d.ScrollY/most, 0), 1) * (total - g.h)
}

// mmK 是滑块顶跟着编辑器滚多快：滑块顶 = ScrollY × mmK。
func (d *Diff) mmK(g geom) float32 {
	r := mmLH / g.lh
	total := g.content * r
	most := g.content - g.h + d.tail(g)
	if total <= g.h || most <= 0 {
		return r
	}
	return r - (total-g.h)/most
}

func (d *Diff) mmSlider(g geom) (top, h float32) {
	r := mmLH / g.lh
	return d.ScrollY*r - d.mmScroll(g), g.h * r
}

// mmRuns 是第 line 行在小地图上的细条；按行的字和行首的分词状态缓存。
func (d *Diff) mmRuns(line int, s *Style) []mmRun {
	src := clean(d.lines[1][line])
	var st uint64
	tk := d.lang != nil && d.lang.Tokenizer != nil
	if tk {
		st = d.stateAt(1, line)
	}
	if c, ok := d.mm.lines[line]; ok && c.src == src && c.state == st {
		return c.runs
	}
	if d.mm.lines == nil || len(d.mm.lines) > 20000 {
		d.mm.lines = map[int]mmLine{}
	}
	ct := s.Code
	var runs []mmRun
	if len(src) > 0 {
		var toks []tokenEnd
		if tk {
			ts, _ := d.lang.Tokenizer.Tokenize(src, st, d.toks[:0])
			d.toks = ts
			for _, t := range ts {
				toks = append(toks, tokenEnd{t.End, ct.TokenColor(t.Kind)})
			}
		}
		tab := s.Tab
		if tab <= 0 {
			tab = 4
		}
		maxCols := float32(minimapW-mmPad) / mmCW
		col, ti := 0, 0
		for i := 0; i < len(src) && float32(col) < maxCols; {
			r, w := utf8.DecodeRuneInString(src[i:])
			switch r {
			case '\t':
				col += tab - col%tab
			case ' ':
				col++
			default:
				for ti < len(toks) && toks[ti].end <= i {
					ti++
				}
				c := ct.Text
				if ti < len(toks) {
					c = toks[ti].col
				}
				c = c.Alpha(0.75)
				if n := len(runs); n > 0 && runs[n-1].c1 == float32(col) && runs[n-1].col == c {
					runs[n-1].c1++
				} else {
					runs = append(runs, mmRun{float32(col), float32(col + 1), c})
				}
				col++
			}
			i += w
		}
	}
	d.mm.lines[line] = mmLine{src, st, runs}
	return runs
}

type tokenEnd struct {
	end int
	col wui.Color
}

func (d *Diff) paintMinimap(p *wui.Painter, r wui.Rect, g geom, s *Style) {
	if g.mm.W == 0 || g.lh <= 0 {
		return
	}
	box := wui.Rect{X: r.X + g.mm.X, Y: r.Y, W: g.mm.W, H: r.H}
	p.Fill(box, s.Code.Background, 0)
	scroll := d.mmScroll(g)
	right := box.X + box.W
	for i := d.rowIndex(max(0, int(scroll/mmLH))); i < len(d.rows); i++ {
		y := box.Y + float32(d.top(i))*mmLH - scroll
		if y >= box.Y+box.H {
			break
		}
		row := d.rows[i]
		if row.Kind == Fold || row.New < 0 || row.New >= len(d.lines[1]) {
			continue
		}
		for _, ru := range d.mmRuns(row.New, s) {
			x := box.X + mmPad + ru.c0*mmCW
			if x >= right {
				break
			}
			p.Fill(wui.Rect{X: x, Y: y, W: min((ru.c1-ru.c0)*mmCW, right-x), H: mmLH - 0.5}, ru.col, 0)
		}
	}
	// 滑块一直画（照 VS Code 的 minimap.showSlider: always），不然看不出这一屏在文件的哪儿；指针在上面深一档，拖着再深一档
	top, h := d.mmSlider(g)
	a := float32(0.12)
	switch {
	case d.mm.drag:
		a = 0.3
	case d.mm.on:
		a = 0.2
	}
	p.Fill(wui.Rect{X: box.X, Y: box.Y + top, W: box.W, H: h}, wui.RGBA(121, 121, 121, a), 0)
}

// minimapInput 管小地图上的按、拖、悬停；滚轮照常滚编辑器（不在这接）。
func (d *Diff) minimapInput(ev wui.InputEvent, g geom) (took, handled bool) {
	if g.mm.W == 0 {
		d.mm.on, d.mm.drag = false, false
		return false, false
	}
	in := ev.X >= g.mm.X && ev.X < g.mm.X+g.mm.W && ev.Y >= 0 && ev.Y < g.h
	switch ev.Kind {
	case wui.InputPointerDown:
		if !in {
			return false, false
		}
		if ev.Button != 0 {
			return true, true // 右键在小地图上不开编辑器的菜单
		}
		top, h := d.mmSlider(g)
		if ev.Y < top || ev.Y >= top+h { // 按在滑块外：滑块的中间挪到指针下（小地图自己也跟着滚，所以按滑块算）
			if k := d.mmK(g); k > 0 {
				d.ScrollY = (ev.Y - h/2) / k
			}
			d.clamp(g)
			top, _ = d.mmSlider(g)
		}
		d.mm.off = ev.Y - top
		d.mm.drag, d.mm.on = true, true
		return true, true
	case wui.InputPointerMove:
		if d.mm.drag {
			if k := d.mmK(g); k > 0 {
				d.ScrollY = (ev.Y - d.mm.off) / k
			}
			d.clamp(g)
			return true, true
		}
		if in {
			changed := !d.mm.on || d.hover != -1 || d.gutterHover
			d.mm.on, d.hover, d.gutterHover = true, -1, false
			return changed, true
		}
		d.mm.on = false
	case wui.InputPointerUp:
		if d.mm.drag {
			d.mm.drag = false
			return true, true
		}
	}
	return false, false
}

// Minimap 是小地图在本地坐标里的位置（没有时 W 为 0）。给测试用。
func (d *Diff) Minimap() wui.Rect {
	if d.style == nil || d.w <= 0 {
		return wui.Rect{}
	}
	return d.geomNow(d.style).mm
}
