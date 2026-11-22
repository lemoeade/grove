package ged

import (
	"sort"
	"unicode"
	"unicode/utf8"

	wui "github.com/egoist/mygo/ui"
)

// 单栏编辑器的改动竖条（Style.Changes）和诊断的波浪线（Style.Squiggles）。

// changeW 是改动竖条多宽：紧挨着行号右边、折叠箭头左边。
const changeW = 3

// paintChange 画第 line 行行号右边的改动竖条；删的画在删掉的地方下面那一行的顶上（最后一行后面删的画在最后一行底下）。
func (d *Diff) paintChange(p *wui.Painter, x0, y, h float32, line int, g geom, s *Style) {
	cs := s.Changes
	if len(cs) == 0 {
		return
	}
	x := x0 + g.numR + 4
	k := sort.Search(len(cs), func(i int) bool { return cs[i].End > line || cs[i].Start >= line })
	for ; k < len(cs) && cs[k].Start <= line+1; k++ {
		c := cs[k]
		switch {
		case c.Kind == ChangeDeleted && c.Start == line:
			deletedMark(p, x, y, g.lh, s.Code.Deleted)
		case c.Kind == ChangeDeleted && c.Start == line+1 && line+1 == len(d.lines[1]):
			deletedMark(p, x, y+h, g.lh, s.Code.Deleted)
		case c.Kind != ChangeDeleted && c.Start <= line && line < c.End:
			col := s.Code.Added
			if c.Kind == ChangeModified {
				col = s.Code.Modified
			}
			p.Fill(wui.Rect{X: x, Y: y, W: changeW, H: h}, col, 0)
		}
	}
}

// deletedMark 是删掉的行留下的记号：行与行之间朝右的红三角（VS Code 那样）。
func deletedMark(p *wui.Painter, x, y, lh float32, c wui.Color) {
	k := min(lh*0.25, 4)
	p.StrokePolyline([][2]float32{{x, y - k}, {x + k + 1, y}, {x, y + k}, {x, y - k}}, 1.5, c)
	p.Fill(wui.Rect{X: x, Y: y - k/2, W: k * 0.6, H: k}, c, 0)
}

// paintSquiggles 画第 line 行上诊断的波浪线（在字的下沿）。
func (d *Diff) paintSquiggles(p *wui.Painter, l *lineLayout, line int, sx float32, vy func(float32) float32, g geom, s *Style) {
	if len(s.Squiggles) == 0 || len(l.u16) == 0 {
		return
	}
	text := clean(d.lines[1][line])
	n := len(l.u16) - 1
	for _, q := range s.Squiggles {
		if q.AL > line || q.BL < line {
			continue
		}
		a, b := 0, len(text)
		if q.AL == line {
			a = q.AO
		}
		if q.BL == line {
			b = q.BO
		}
		a, b = min(max(a, 0), len(text)), min(max(b, 0), len(text))
		if b <= a { // 一个点：画到这个词的末尾，至少一个字（行尾时往前退一个）
			b = a
			for b < len(text) {
				r, w := utf8.DecodeRuneInString(text[b:])
				if b > a && !(r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
					break
				}
				b += w
			}
			if b == a && a > 0 {
				_, w := utf8.DecodeLastRuneInString(text[:a])
				a -= w
			}
			if b == a {
				b = a + 1 // 空行：画一个字宽
			}
		}
		lo, hi := l.u16[min(a, n)], l.u16[min(b, n)]
		var rects []wui.Rect
		if hi > lo {
			rects = l.l.SelectionRects(wui.TextInputRange{Start: lo, End: hi})
		} else {
			cr := l.l.Caret(lo)
			rects = []wui.Rect{{X: cr.X, Y: cr.Y, W: g.cw}}
		}
		for _, rc := range rects {
			squiggle(p, sx+rc.X, sx+rc.X+max(rc.W, 2), vy(rc.Y)+g.lh-2.5, q.Color)
		}
	}
}

// squiggle 是 x0 到 x1 之间、以 y 为中线的一道波浪线（每 4 pt 一个来回）。
func squiggle(p *wui.Painter, x0, x1, y float32, c wui.Color) {
	const step, amp = 2, 1.2
	pts := make([][2]float32, 0, int((x1-x0)/step)+2)
	up := true
	for x := x0; ; x += step {
		if x > x1 {
			x = x1
		}
		dy := float32(amp)
		if up {
			dy = -amp
		}
		pts = append(pts, [2]float32{x, y + dy})
		up = !up
		if x >= x1 {
			break
		}
	}
	p.StrokePolyline(pts, 1, c)
}

// paintNotes 画第 line 行行尾后面的淡字（Style.Notes）。
func (d *Diff) paintNotes(p *wui.Painter, l *lineLayout, line int, sx float32, vy func(float32) float32, g geom, s *Style) {
	if len(l.u16) == 0 {
		return
	}
	for _, n := range s.Notes {
		if n.Line != line || n.Text == "" {
			continue
		}
		cr := l.l.Caret(l.u16[len(l.u16)-1])
		base := vy(cr.Y) + (g.lh-(g.asc+g.desc))/2 + g.asc
		p.Glyphs(wui.Shape(n.Text, s.Font), sx+cr.X, base, s.NoteColor)
	}
}
