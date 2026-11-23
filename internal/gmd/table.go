package gmd

import (
	"unicode"
	"unicode/utf8"

	ui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown/parser"
)

// tableState keeps a table's column widths, which only grow while the
// table is being written, so that its columns do not jump as its cells
// fill, and whether it scrolled sideways the last frame.
type tableState struct {
	widths, mins []float32
	seen         bool
	scroll       bool
}

// table builds a table the way a browser's automatic table layout does (VS
// Code's preview is one): each column asks for the width of its widest cell
// on one line (max-content) and gives way down to its longest piece that
// cannot wrap (min-content). A table whose columns fit is as wide as they
// ask, not the view's width; one that does not fit is the view's width, the
// columns giving way in proportion to what they ask, so a long column wraps
// more than a short one and none collapses to a letter a line; only a
// table whose min-content widths do not fit scrolls sideways.
func (st *viewState) table(c *ui.Context, i int, base spanStyle) ui.Element {
	d := st.doc
	th := &st.th
	b := d.Block(i)
	if st.tables == nil {
		st.tables = map[int]*tableState{}
	}
	ts := st.tables[b.Start]
	if ts == nil {
		ts = &tableState{}
		st.tables[b.Start] = ts
	}
	ts.seen = true
	cols := len(b.Align)
	padX, padY := em(th, th.TableCellPadding[0]), em(th, th.TableCellPadding[1])
	// The widths of the columns, from their cells' text.
	widths, mins := make([]float32, cols), make([]float32, cols)
	if b.Open {
		copy(widths, ts.widths)
		copy(mins, ts.mins)
	}
	for r := i + 1; r < d.Block(i).Next; r = d.Block(r).Next {
		row := d.Block(r)
		k := 0
		for cell := r + 1; cell < d.Block(r).Next; cell = d.Block(cell).Next {
			if k >= cols {
				break
			}
			s := base
			if row.Header {
				s.weight = th.StrongWeight
			}
			p := st.para(cell, s)
			if !p.measured {
				p.width, p.minWidth, p.measured = 0, 0, true
				for _, pc := range p.pieces {
					pw, _ := c.MeasureText(0, pc.spans...)
					p.width += pw
					p.minWidth = max(p.minWidth, minContent(c, pc.spans))
				}
				p.minWidth = min(p.minWidth, p.width)
			}
			widths[k] = max(widths[k], p.width+2*padX+1)
			mins[k] = max(mins[k], p.minWidth+2*padX+1)
			k++
		}
	}
	ts.widths = append(ts.widths[:0], widths...)
	ts.mins = append(ts.mins[:0], mins...)
	var sumMax, sumMin float32 = 2, 2 // 表格的边框
	for k := range widths {
		sumMax += widths[k]
		sumMin += mins[k]
	}
	// 放不下最窄的排法才横着滚。看上一帧这一栏多宽；这一帧排出来换了（窗口改了宽）就再要一帧
	wrap := ui.Column(c).AlignSelf(ui.Stretch)
	if w := wrap.Bounds().W; w > 0 {
		ts.scroll = sumMin > w+0.5
	}
	scroll := ts.scroll
	wrap.Draw(func(p *ui.Painter, r ui.Rect) {
		if r.W > 0 && (sumMin > r.W+0.5) != scroll {
			c.Invalidate()
		}
	})
	return wrap.Children(func() {
		body := func() {
			grid := ui.Column(c).Border(1, th.TableBorder).Radius(em(th, 0.3)).Clip().Shrink(0).Role(ui.RoleTable)
			if scroll {
				grid.Width(sumMin)
			} else {
				grid.AlignSelf(ui.Start).FillWidth().MaxWidth(sumMax).MinWidth(sumMin)
			}
			st.tableRows(c, grid, i, widths, mins, padX, padY, base)
		}
		if scroll {
			ui.ScrollHorizontal(c).AlignSelf(ui.Stretch).Children(body)
		} else {
			body()
		}
	})
}

// tableRows fills a table's grid with its rows: a cell asks for its column's
// max-content width and shrinks, with the others in its row, down to the
// column's min-content width, so the columns line up row to row.
func (st *viewState) tableRows(c *ui.Context, grid ui.Element, i int, widths, mins []float32, padX, padY float32, base spanStyle) {
	d := st.doc
	th := &st.th
	b := d.Block(i)
	cols := len(b.Align)
	grid.Children(func() {
		n := 0
		for r := i + 1; r < d.Block(i).Next; r = d.Block(r).Next {
			row := d.Block(r)
			rowEl := ui.Row(c).Role(ui.RoleRow).AlignItems(ui.Stretch)
			if row.Header {
				rowEl.Background(th.TableHeader)
			} else if n%2 == 1 && th.TableStripe.A > 0 {
				rowEl.Background(th.TableStripe)
			}
			if n > 0 {
				rowEl.BorderWidth(1, 0, 0, 0).BorderColor(th.TableBorder)
			}
			header := row.Header
			rowEl.Children(func() {
				k := 0
				for cell := r + 1; cell < d.Block(r).Next; cell = d.Block(cell).Next {
					if k >= cols {
						break
					}
					s := base
					role := ui.RoleCell
					if header {
						s.weight = th.StrongWeight
						role = ui.RoleColumnHeader
					}
					box := ui.Column(c).Width(widths[k]).Shrink(1).MinWidth(mins[k]).Padding(padY, padX).Role(role)
					if k > 0 {
						box.BorderWidth(0, 0, 0, 1).BorderColor(th.TableBorder)
					}
					align := ui.Start
					switch b.Align[k] {
					case parser.AlignCenter:
						align = ui.Center
					case parser.AlignRight:
						align = ui.End
					}
					box.Children(func() { st.paragraph(c, cell, s).TextAlign(align) })
					k++
				}
			})
			n++
		}
	})
}

// minContent is the width of the widest piece of spans' text that cannot
// wrap, as a browser breaks lines: at spaces, after a hyphen, and around
// each CJK character. The text shaper would break a long word between any
// two letters where it must, which is no measure of it. Only the longest
// few pieces are measured.
func minContent(c *ui.Context, spans []ui.Span) float32 {
	type seg struct {
		span  int
		text  string
		runes int
	}
	var segs []seg
	most := 0
	for k, sp := range spans {
		t := sp.Text
		start := -1
		cut := func(end int) {
			if start >= 0 && end > start {
				n := utf8.RuneCountInString(t[start:end])
				segs = append(segs, seg{k, t[start:end], n})
				most = max(most, n)
			}
			start = -1
		}
		for i, r := range t {
			switch {
			case unicode.IsSpace(r):
				cut(i)
			case isCJK(r):
				cut(i)
				start = i
				cut(i + utf8.RuneLen(r))
			default:
				if start < 0 {
					start = i
				}
				if r == '-' {
					cut(i + 1)
				}
			}
		}
		cut(len(t))
	}
	var w float32
	measured := 0
	for _, sg := range segs {
		if sg.runes*5 < most*3 || measured >= 8 {
			continue
		}
		sp := spans[sg.span]
		sp.Text = sg.text
		sw, _ := c.MeasureText(0, sp)
		w = max(w, sw)
		measured++
	}
	return w
}

// isCJK is a character a line may break before and after: Han, kana,
// Hangul and the CJK punctuation and full-width forms.
func isCJK(r rune) bool {
	return r >= 0x2E80 && (unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r) || r >= 0x3000 && r <= 0x303F || r >= 0xFF00 && r <= 0xFFEF)
}
