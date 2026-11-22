package ged

import wui "github.com/egoist/mygo/ui"

// 缩进参考线（VS Code 的 editor.guides.indentation）：每一级缩进的开头一道竖线，空行接着上下两行的；
// 就地编辑时光标所在的那一块（光标那一行最里面那级缩进管着的连续几行）的那道线深一点（guides.highlightActiveIndentation）。

// guideScan 是空行往上 / 往下找有字的行最多找几行。
const guideScan = 200

// guides 是这一帧算好的：一级缩进几格、光标那一块的线。
type guides struct {
	on       bool
	unit     int // 一级几格（Tab 按 Style.Tab 算）
	tab      int
	act      bool // 有光标那一块
	actSide  int
	actCol   int // 那道线在第几格
	actLines [2]int
}

// indentCols 是这一行开头的空白占几格（Tab 补到下一个 tab 的整数倍）；整行空白（或空行）返回 -1。
func indentCols(s string, tab int) int {
	col := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ':
			col++
		case '\t':
			col += tab - col%tab
		case '\r', '\n':
		default:
			return col
		}
	}
	return -1
}

// guideCols 是 side 那边第 line 行画到第几格（这一格之前每级一道线）。空行看上下最近的有字的行：
// 两边一样深取它，不一样时取浅的那边再深一级、但不超过深的那边（VS Code 一样：块里的空行线不断，块后面的空行不多画）。
func (d *Diff) guideCols(side, line int) int {
	lines := d.lines[side]
	if line < 0 || line >= len(lines) {
		return 0
	}
	if c := indentCols(lines[line], d.guide.tab); c >= 0 {
		return c
	}
	above, below := 0, 0
	for i := line - 1; i >= 0 && i >= line-guideScan; i-- {
		if c := indentCols(lines[i], d.guide.tab); c >= 0 {
			above = c
			break
		}
	}
	for i := line + 1; i < len(lines) && i <= line+guideScan; i++ {
		if c := indentCols(lines[i], d.guide.tab); c >= 0 {
			below = c
			break
		}
	}
	return min(max(above, below), min(above, below)+d.guide.unit)
}

// prepGuides 每帧画之前算一级缩进几格和光标那一块。
func (d *Diff) prepGuides(s *Style) {
	d.guide = guides{on: s.Guides && s.Code.IndentGuide.A > 0}
	if !d.guide.on {
		return
	}
	tab := s.Tab
	if tab <= 0 {
		tab = 4
	}
	d.guide.tab = tab
	if u := d.indentUnit(s); u == "\t" {
		d.guide.unit = tab
	} else {
		d.guide.unit = max(len(u), 1)
	}
	if !d.editable || !d.focused || d.sel.side != 1 || len(d.lines[1]) == 0 {
		return
	}
	line := min(d.sel.b.line, len(d.lines[1])-1)
	c := d.guideCols(1, line)
	if c <= 0 {
		return
	}
	col := (c - 1) / d.guide.unit * d.guide.unit // 光标那一行最里面那道线
	a, b := line, line
	for a > 0 && d.guideCols(1, a-1) > col {
		a--
	}
	for b < len(d.lines[1])-1 && d.guideCols(1, b+1) > col {
		b++
	}
	d.guide.act, d.guide.actSide, d.guide.actCol, d.guide.actLines = true, 1, col, [2]int{a, b}
}

// paintGuides 在 side 那边第 line 行的格子里画缩进参考线：sx 是正文第 0 格的 x，y、h 是格子的顶和高（折行时几行高）。
func (d *Diff) paintGuides(p *wui.Painter, side, line int, sx, y, h float32, g geom, s *Style) {
	if !d.guide.on || g.cw <= 0 {
		return
	}
	c := d.guideCols(side, line)
	act := d.guide.act && side == d.guide.actSide && line >= d.guide.actLines[0] && line <= d.guide.actLines[1]
	for col := 0; col < c; col += d.guide.unit {
		color := s.Code.IndentGuide
		if act && col == d.guide.actCol {
			color = s.Code.ActiveIndentGuide
		}
		p.Fill(wui.Rect{X: sx + float32(col)*g.cw, Y: y, W: 1, H: h}, color, 0)
	}
}
