package ged

import wui "github.com/egoist/mygo/ui"

// 右键菜单：菜单本身由调用方画（Style.Menu 接右键的按下、拖、松开），视图只记按在哪（Spot），
// 再给几个菜单项要用的动作（Run 编辑菜单的命令、SpotLine 按在的那一行的字、FileLine 跳到文件的哪一行）。
// 键盘开菜单（⇧F10 / 菜单键）时调用方先调 KeySpot，按光标 / 当前那处改动定 Spot。

// Spot 是右键按在哪。
type Spot struct {
	Side int // 0 旧的一边、1 新的一边
	Line int // 那边第几行（从 0 起）；按在折叠条上、那边是空的（对面多出的行）时 -1
	Hunk int // 第几处改动；不在改动上 -1
	New  int // 跳到文件时去新的一边第几行：那一行有新的一边就是它，不然是它下面最近的一行新的
	Fold int // 按在折叠条上时是第几行（rows 的下标，给 ExpandMore），不然 -1
}

// MenuSpot 是上一次右键（或 KeySpot）记下的位置。
func (d *Diff) MenuSpot() Spot { return d.spot }

// menuInput 把右键的按下、按着拖、松开交给 Style.Menu；按下时先记好按在哪、把选择挪过去。
func (d *Diff) menuInput(ev wui.InputEvent, g geom, s *Style) (took, handled bool) {
	if s.Menu == nil {
		return false, false
	}
	switch ev.Kind {
	case wui.InputPointerDown:
		if !(ev.Button == 1 || ev.Button == 0 && ev.Mods&wui.Ctrl != 0) || ev.X >= g.ruler.X {
			return false, false
		}
		d.menuAt(ev, g, s)
		d.menuHeld = true
		s.Menu(ev)
		return true, true
	case wui.InputPointerMove, wui.InputPointerUp:
		if !d.menuHeld {
			return false, false
		}
		if ev.Kind == wui.InputPointerUp {
			d.menuHeld = false
		}
		s.Menu(ev)
		return true, true
	}
	return false, false
}

// menuAt 记下右键按在哪；按在正文上、不在选中的字里时选择挪到那里（只看时是取消选择，就地编辑时放光标），
// VS Code 也是这样：右键菜单对着的总是指针下的东西。
func (d *Diff) menuAt(ev wui.InputEvent, g geom, s *Style) {
	d.focus = true
	d.goalX = -1
	d.endGroup()
	side := 0
	if ev.X >= g.paneX[1] {
		side = 1
	}
	i := d.rowAt(ev.Y, g)
	if d.unified && i >= 0 {
		side = d.rowSide(d.rows[i])
	}
	d.spot = Spot{Side: side, Line: -1, Hunk: -1, New: d.newLineFrom(i), Fold: -1}
	if i >= 0 && d.rows[i].Kind == Fold {
		d.spot.Fold = i
	}
	if i < 0 || d.rows[i].Kind == Fold {
		return
	}
	d.side = side
	if h := d.rowHunk[i]; h >= 0 {
		d.cur, d.spot.Hunk = h, h
	}
	line := lineOf(d.rows[i], side)
	d.spot.Line = line
	if line < 0 || d.inGutter(ev.X, g) {
		return
	}
	p := d.posAt(ev.X, ev.Y, side, g, s)
	if !d.sel.empty() && d.sel.side == side {
		if a, b := d.sel.span(); !p.less(a) && !b.less(p) {
			return
		}
	}
	d.sel = selection{side: side, a: p, b: p}
}

// newLineFrom 是 rows[i] 往下第一行新的一边的行号（rows[i] 自己有就是它）；到底了是最后一行。
func (d *Diff) newLineFrom(i int) int {
	for k := max(i, 0); k < len(d.rows); k++ {
		r := d.rows[k]
		if r.Kind == Fold {
			continue
		}
		if r.New >= 0 {
			return r.New
		}
	}
	return max(len(d.lines[1])-1, 0)
}

// KeySpot 是键盘开菜单（⇧F10 / 菜单键）时对着哪：就地编辑有光标时按光标，不然按选中的字的头、当前那处改动、视口顶上那一行；
// x、y 是菜单开在哪（本地坐标，那一行下面、正文的左边）。不改状态：调用方开菜单时再 SetSpot。
func (d *Diff) KeySpot() (sp Spot, x, y float32) {
	side, line := d.side, -1
	switch {
	case d.editable && d.sel.side == 1:
		side, line = 1, d.sel.b.line
	case !d.sel.empty():
		a, _ := d.sel.span()
		side, line = d.sel.side, a.line
	}
	i := -1
	if line >= 0 {
		i = d.rowOfLine(side, line)
	}
	if i < 0 && d.cur >= 0 && d.cur < len(d.hunkRow) {
		i = d.hunkRow[d.cur]
	}
	if i < 0 && d.lh > 0 {
		i = d.rowIndex(int(d.ScrollY / d.lh))
	}
	side = min(max(side, 0), 1)
	if i < 0 || i >= len(d.rows) {
		return Spot{Side: side, Line: -1, Hunk: -1, Fold: -1}, d.textLeft[side] - d.box.X, d.lh
	}
	if d.unified {
		side = d.rowSide(d.rows[i])
	}
	sp = Spot{Side: side, Line: lineOf(d.rows[i], side), Hunk: d.rowHunk[i], New: d.newLineFrom(i), Fold: -1}
	if d.rows[i].Kind == Fold {
		sp.Line, sp.Fold = -1, i
	}
	y = float32(d.top(i+1))*d.lh - d.ScrollY
	return sp, d.textLeft[side] - d.box.X, min(max(y, d.lh), max(d.h-d.lh, d.lh))
}

// SetSpot 定菜单对着哪（键盘开菜单时用 KeySpot 给的）。
func (d *Diff) SetSpot(sp Spot) { d.spot = sp }

// SpotLine 是 Spot 那一行的字（没有那一行时 ok 为 false）。
func (d *Diff) SpotLine() (string, bool) {
	sp := d.spot
	if sp.Line < 0 || sp.Side < 0 || sp.Side > 1 || sp.Line >= len(d.lines[sp.Side]) {
		return "", false
	}
	return clean(d.lines[sp.Side][sp.Line]), true
}

// Run 执行编辑菜单的命令（"copy" "cut" "paste" "selectAll" "undo" "redo"），和菜单栏的一样；没接返回 false。
func (d *Diff) Run(cmd string) bool {
	if d.style == nil || d.w <= 0 {
		return false
	}
	d.focus = true
	return d.input(wui.InputEvent{Kind: wui.InputCommand, Text: cmd}, d.style)
}

// CanEdit 是这一帧能就地编辑（给了 Style.Buf）。
func (d *Diff) CanEdit() bool { return d.editable }

// HasSelection 是选着字（不是光标）。
func (d *Diff) HasSelection() bool { return !d.sel.empty() }

// FoldSize 是 rows[i] 那条折叠折了几行（不是折叠是 0）。
func (d *Diff) FoldSize(i int) int {
	if i < 0 || i >= len(d.rows) || d.rows[i].Kind != Fold {
		return 0
	}
	return d.rows[i].N
}
