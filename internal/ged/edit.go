package ged

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// 这个文件是就地编辑：Style.Buf 给了时新的一边能放光标、打字、删、粘贴、撤销，改的直接是那个缓冲区
// （和文件标签共用一个，撤销、保存都是它的）。光标就是新的一边选中的字的 b 头（a == b 时是一个点），
// 所以点、拖、⇧ 点、双击照旧，选中的字打字时被替换。旧的一边只能选、复制。

// 连着打的字、连着删的字合成一步撤销（mygo 的 Buffer.Edit 每次都是一步，打字合并它没导出）：
// 隔了这么久不再合。
const typeGroup = time.Second

type editKind uint8

const (
	editOther editKind = iota
	editTyping
	editDeleting
)

// caret 是光标（新的一边的位置）；不在编辑、选的是旧的一边、新的一边是空的时 ok 为 false。
func (d *Diff) caret(s *Style) (pos, bool) {
	if s.Buf == nil || d.sel.side != 1 || len(d.lines[1]) == 0 {
		return pos{}, false
	}
	return d.sel.b, true
}

// Caret 是光标在新的一边第几行（从 0 起）第几个字节前；没有光标时 ok 为 false。给测试用。
func (d *Diff) Caret() (line, off int, ok bool) {
	if d.sel.side != 1 || !d.sel.empty() || !d.editable {
		return 0, 0, false
	}
	return d.sel.b.line, d.sel.b.off, true
}

// PlaceCaret 把光标放到新的一边第 line 行第 off 个字节前（就地编辑时），滚到看得见。
func (d *Diff) PlaceCaret(line, off int) {
	p := d.clampPos(pos{line, off})
	d.sel = selection{side: 1, a: p, b: p}
	d.side, d.goalX, d.reveal = 1, -1, true
	d.endGroup()
}

// linesOf 是缓冲区的每一行（不带换行）。
func linesOf(b *editor.Buffer) []string {
	out := make([]string, b.LineCount())
	for i := range out {
		out[i] = b.Line(i)
	}
	return out
}

func (d *Diff) clampPos(p pos) pos {
	lines := d.lines[1]
	if len(lines) == 0 {
		return pos{}
	}
	p.line = min(max(p.line, 0), len(lines)-1)
	t := lines[p.line]
	p.off = min(max(p.off, 0), len(t))
	for p.off > 0 && p.off < len(t) && !utf8.RuneStart(t[p.off]) {
		p.off--
	}
	return p
}

// moveTo 把光标挪到 p；extend 时起点不动（⇧ 加方向键扩大选中的字）。
func (d *Diff) moveTo(p pos, extend bool) {
	p = d.clampPos(p)
	if extend {
		d.sel.b = p
	} else {
		d.sel = selection{side: 1, a: p, b: p}
	}
	d.side, d.goalX, d.reveal = 1, -1, true
	d.endGroup()
}

func (d *Diff) endGroup() { d.lastKind = editOther }

// left / right 是往左 / 右一个字（行头往上一行的行尾，行尾往下一行的行头）。
func (d *Diff) left(p pos) pos {
	if p.off > 0 {
		_, w := utf8.DecodeLastRuneInString(d.lines[1][p.line][:p.off])
		return pos{p.line, p.off - w}
	}
	if p.line > 0 {
		return pos{p.line - 1, len(d.lines[1][p.line-1])}
	}
	return p
}

func (d *Diff) right(p pos) pos {
	t := d.lines[1][p.line]
	if p.off < len(t) {
		_, w := utf8.DecodeRuneInString(t[p.off:])
		return pos{p.line, p.off + w}
	}
	if p.line+1 < len(d.lines[1]) {
		return pos{p.line + 1, 0}
	}
	return p
}

// wordLeft / wordRight 是往左 / 右一个词（跳过中间的空白，按 tokens 切的词），行头行尾跨到上下一行。
func (d *Diff) wordLeft(p pos) pos {
	if p.off == 0 {
		return d.left(p)
	}
	ts := tokens(d.lines[1][p.line])
	for i := len(ts) - 1; i >= 0; i-- {
		t := ts[i]
		if t.Start >= p.off {
			continue
		}
		if isSpace(d.lines[1][p.line][t.Start:t.End]) && i > 0 && t.Start > 0 {
			continue
		}
		return pos{p.line, t.Start}
	}
	return pos{p.line, 0}
}

func (d *Diff) wordRight(p pos) pos {
	t := d.lines[1][p.line]
	if p.off >= len(t) {
		return d.right(p)
	}
	for _, tk := range tokens(t) {
		if tk.End <= p.off {
			continue
		}
		if isSpace(t[tk.Start:tk.End]) && tk.End < len(t) {
			continue
		}
		return pos{p.line, tk.End}
	}
	return pos{p.line, len(t)}
}

func isSpace(s string) bool { return strings.TrimLeft(s, " \t") == "" }

// home 是这一行“行头”：先到缩进后面，已经在那就到最前（和 VS Code 一样）。
func (d *Diff) home(p pos) int {
	t := d.lines[1][p.line]
	ind := len(t) - len(strings.TrimLeft(t, " \t"))
	if p.off == ind {
		return 0
	}
	return ind
}

// vertical 上下挪 n 行，横着尽量停在 goalX（第一次上下挪时记下的光标的 x）。
func (d *Diff) vertical(c pos, n int, extend bool, s *Style) {
	lines := d.lines[1]
	if d.goalX < 0 {
		l := d.layout(1, c.line, s)
		d.goalX = l.l.Caret(l.u16[min(c.off, len(l.u16)-1)]).X
	}
	goal := d.goalX
	if n == 1 || n == -1 { // 折行了：先在这一行折出来的几行里上下走
		l := d.layout(1, c.line, s)
		if k := l.caretRow(c.off) + n; k >= 0 && k < l.n {
			d.moveTo(pos{c.line, l.indexOnRow(k, goal)}, extend)
			d.goalX = goal
			return
		}
	}
	t := c.line + n
	switch {
	case t < 0:
		d.moveTo(pos{0, 0}, extend)
	case t >= len(lines):
		d.moveTo(pos{len(lines) - 1, len(lines[len(lines)-1])}, extend)
	default:
		l := d.layout(1, t, s)
		k := 0
		if n < 0 { // 往上走到上一行折出来的最后一行
			k = l.n - 1
		}
		d.moveTo(pos{t, l.indexOnRow(k, goal)}, extend)
	}
	d.goalX = goal
}

// editKey 是就地编辑、有光标时视图自己接的键；没接的（打字的键）返回 false，字跟着以 InputText 来。
func (d *Diff) editKey(ev wui.InputEvent, g geom, s *Style) bool {
	c, ok := d.caret(s)
	if !ok || d.compose != "" { // 输入法正拼着字：键归它
		return false
	}
	mods := ev.Mods & (wui.Shift | wui.Ctrl | wui.Alt | wui.Super)
	shift, alt, cmd := mods&wui.Shift != 0, mods&wui.Alt != 0, mods&wui.Super != 0
	if mods&wui.Ctrl != 0 {
		return false
	}
	lines := d.lines[1]
	page := max(1, int((g.h-2*g.lh)/max(g.lh, 1)))
	switch ev.Key {
	case wui.KeyLeft:
		switch {
		case !shift && !d.sel.empty() && !cmd && !alt: // 有选中的字：收到前头
			a, _ := d.sel.span()
			d.moveTo(a, false)
		case cmd:
			d.moveTo(pos{c.line, d.home(c)}, shift)
		case alt:
			d.moveTo(d.wordLeft(c), shift)
		default:
			d.moveTo(d.left(c), shift)
		}
	case wui.KeyRight:
		switch {
		case !shift && !d.sel.empty() && !cmd && !alt:
			_, b := d.sel.span()
			d.moveTo(b, false)
		case cmd:
			d.moveTo(pos{c.line, len(lines[c.line])}, shift)
		case alt:
			d.moveTo(d.wordRight(c), shift)
		default:
			d.moveTo(d.right(c), shift)
		}
	case wui.KeyUp:
		if cmd {
			d.moveTo(pos{0, 0}, shift)
		} else if !alt {
			d.vertical(c, -1, shift, s)
		}
	case wui.KeyDown:
		if cmd {
			d.moveTo(pos{len(lines) - 1, len(lines[len(lines)-1])}, shift)
		} else if !alt {
			d.vertical(c, 1, shift, s)
		}
	case wui.KeyPageUp:
		d.ScrollY -= float32(page) * g.lh
		d.vertical(c, -page, shift, s)
	case wui.KeyPageDown:
		d.ScrollY += float32(page) * g.lh
		d.vertical(c, page, shift, s)
	case wui.KeyHome:
		d.moveTo(pos{c.line, d.home(c)}, shift)
	case wui.KeyEnd:
		d.moveTo(pos{c.line, len(lines[c.line])}, shift)
	case wui.KeyBackspace:
		switch {
		case !d.sel.empty():
			d.typeText(s, "", editOther)
		case cmd:
			d.replace(s, pos{c.line, 0}, c, "", editOther)
		case alt:
			d.replace(s, d.wordLeft(c), c, "", editOther)
		default:
			d.replace(s, d.left(c), c, "", editDeleting)
		}
	case wui.KeyDelete:
		switch {
		case !d.sel.empty():
			d.typeText(s, "", editOther)
		case cmd:
			d.replace(s, c, pos{c.line, len(lines[c.line])}, "", editOther)
		case alt:
			d.replace(s, c, d.wordRight(c), "", editOther)
		default:
			d.replace(s, c, d.right(c), "", editDeleting)
		}
	case wui.KeyEnter:
		if mods&^wui.Shift != 0 {
			return false
		}
		a, _ := d.sel.span()
		t := lines[a.line][:a.off]
		d.typeText(s, "\n"+t[:len(t)-len(strings.TrimLeft(t, " \t"))], editOther) // 接着这一行的缩进
	case wui.KeyTab:
		if mods != 0 {
			return false
		}
		d.typeText(s, "\t", editTyping)
	case wui.KeyEscape:
		if mods != 0 || d.sel.empty() {
			return false
		}
		d.moveTo(d.sel.b, false)
	default:
		return false
	}
	d.clamp(g)
	return true
}

// typeText 把选中的字换成 text（没选就是在光标处插进去）。
func (d *Diff) typeText(s *Style, text string, kind editKind) {
	a, b := d.sel.span()
	d.replace(s, a, b, text, kind)
}

// replace 把新的一边 [a, b) 换成 text，光标放到换进去的字后面。改的是 Style.Buf，改完马上重新对齐
// （一帧里可能连着来好几个键）。
func (d *Diff) replace(s *Style, a, b pos, text string, kind editKind) {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	a, b = d.clampPos(a), d.clampPos(b)
	if b.less(a) {
		a, b = b, a
	}
	if a == b && text == "" {
		return
	}
	off := func(p pos) int { return s.Buf.Offset(editor.Pos{Line: p.line, Col: p.off}) }
	if kind == editTyping && strings.Contains(text, "\n") {
		kind = editOther
	}
	now := time.Now()
	merge := kind != editOther && kind == d.lastKind && now.Sub(d.lastEdit) < typeGroup && s.Buf.Version() == d.ver
	if s.Buf.Version() != d.ver { // 别处（文件标签）改过：我们记的几步已经不是缓冲区最近的了
		d.undoGroups = nil
	}
	s.Buf.Edit(editor.Edit{Start: off(a), End: off(b), Text: text})
	if merge && len(d.undoGroups) > 0 {
		d.undoGroups[len(d.undoGroups)-1]++
	} else {
		d.undoGroups = append(d.undoGroups, 1)
	}
	d.redoGroups = nil
	d.lastKind, d.lastEdit, d.ver = kind, now, s.Buf.Version()
	c := after(a, text)
	d.reload(s)
	d.sel = selection{side: 1, a: c, b: c}
	d.side, d.goalX, d.reveal = 1, -1, true
}

// after 是从 p 插进 text 以后光标在哪。
func after(p pos, text string) pos {
	i := strings.LastIndexByte(text, '\n')
	if i < 0 {
		return pos{p.line, p.off + len(text)}
	}
	return pos{p.line + strings.Count(text, "\n"), len(text) - i - 1}
}

// reload 按缓冲区现在的字重新对齐（旧的一边不变）。
func (d *Diff) reload(s *Style) { d.update(d.lines[0], linesOf(s.Buf)) }

// undo 撤销（redo 为真时重做）缓冲区上一步：连着打的字一起撤（undoGroups 记着我们这几步各合了几次 Edit），
// 光标放到变了的地方。
func (d *Diff) undo(s *Style, redo bool) {
	if s.Buf.Version() != d.ver { // 别处改过：只撤一步
		d.undoGroups, d.redoGroups = nil, nil
	}
	from, to := &d.undoGroups, &d.redoGroups
	step := s.Buf.Undo
	if redo {
		from, to, step = to, from, s.Buf.Redo
	}
	n := 1
	if k := len(*from); k > 0 {
		n = (*from)[k-1]
		*from = (*from)[:k-1]
	}
	before := d.lines[1]
	done := 0
	for range n {
		if !step() {
			break
		}
		done++
	}
	if done == 0 {
		return
	}
	*to = append(*to, done)
	d.ver, d.lastKind = s.Buf.Version(), editOther
	now := linesOf(s.Buf)
	c := changedEnd(before, now)
	d.update(d.lines[0], now)
	d.sel = selection{side: 1, a: c, b: c}
	d.side, d.goalX, d.reveal = 1, -1, true
}

// changedEnd 是从 before 变成 now 时，now 里变了的那段的末尾（撤销、重做后光标放这）。
func changedEnd(before, now []string) pos {
	p := 0
	for p < len(before) && p < len(now) && before[p] == now[p] {
		p++
	}
	q := 0
	for q < len(before)-p && q < len(now)-p && before[len(before)-1-q] == now[len(now)-1-q] {
		q++
	}
	last := len(now) - 1 - q // now 里变了的最后一行
	if last < p {            // now 里没有变了的行（整行删掉）：放到那一行的开头
		return pos{min(p, max(len(now)-1, 0)), 0}
	}
	t := now[last]
	off := len(t)
	if bl := len(before) - 1 - q; bl >= p { // 去掉和原来那行一样的尾巴
		o := before[bl]
		k := 0
		for k < len(t) && k < len(o) && t[len(t)-1-k] == o[len(o)-1-k] {
			k++
		}
		off = len(t) - k
		if last == p && bl == p { // 同一行里改的：至少在一样的开头后面
			h := 0
			for h < len(t) && h < len(o) && t[h] == o[h] {
				h++
			}
			off = max(off, min(h, len(t)))
		}
	}
	for off > 0 && off < len(t) && !utf8.RuneStart(t[off]) {
		off--
	}
	return pos{last, off}
}

// editInput 接就地编辑时的字、输入法、编辑菜单的剪切 / 粘贴 / 撤销 / 重做；不是这些（或不在编辑）返回 handled 为 false。
func (d *Diff) editInput(ev wui.InputEvent, g geom, s *Style) (took, handled bool) {
	if s.Buf == nil {
		return false, false
	}
	_, ok := d.caret(s)
	switch ev.Kind {
	case wui.InputText:
		if !ok {
			return false, true
		}
		d.compose = ""
		d.typeText(s, ev.Text, editTyping)
	case wui.InputCompose:
		if !ok {
			return false, true
		}
		if ev.Text != "" && d.compose == "" && !d.sel.empty() { // 开始拼字：先删掉选中的字
			d.typeText(s, "", editOther)
		}
		d.compose, d.composeCaret = ev.Text, ev.Caret
		d.reveal = true
	case wui.InputCommand:
		switch ev.Text {
		case "cut":
			if !ok || d.sel.empty() {
				return false, true
			}
			if s.Copy != nil {
				s.Copy(d.Selected())
			}
			d.typeText(s, "", editOther)
		case "paste", "pasteAndMatchStyle":
			if !ok || s.Paste == nil {
				return false, true
			}
			if t := s.Paste(); t != "" {
				d.typeText(s, t, editOther)
			}
		case "delete":
			if !ok || d.sel.empty() {
				return false, true
			}
			d.typeText(s, "", editOther)
		case "undo", "redo":
			d.undo(s, ev.Text == "redo")
		default:
			return false, false
		}
	default:
		return false, false
	}
	d.clamp(g)
	return true, true
}

// revealCaret 在光标挪过、改过字以后把它滚进视图（光标在折起来的段里时先展开那段）。
func (d *Diff) revealCaret(g geom, s *Style) {
	if !d.reveal || g.lh <= 0 || d.h <= 0 {
		return
	}
	d.reveal = false
	c, ok := d.caret(s)
	if !ok {
		return
	}
	i := d.rowOfLine(1, c.line)
	if i < 0 {
		return
	}
	l := d.layout(1, c.line, s)
	g.content = float32(d.top(len(d.rows))) * g.lh
	y := float32(d.top(i)+l.caretRow(c.off)) * g.lh
	if y < d.ScrollY {
		d.ScrollY = y
	} else if y+g.lh > d.ScrollY+g.h {
		d.ScrollY = y + g.lh - g.h
	}
	if d.unified {
		d.side = 1
	}
	if d.wrap {
		return
	}
	x := l.l.Caret(l.u16[min(c.off, len(l.u16)-1)]).X
	room := g.paneW[1] - g.textX - rulerW
	if x < d.scrollX[1] {
		d.scrollX[1] = max(0, x-4*g.cw)
	} else if x > d.scrollX[1]+room-g.cw {
		d.scrollX[1] = x - room + 4*g.cw
	}
}

// rowOfLine 是 side 那边第 line 行在 rows 里是第几行；在折起来的段里时展开那段。没有是 -1。
func (d *Diff) rowOfLine(side, line int) int {
	for i, r := range d.rows {
		l := lineOf(r, side)
		switch {
		case r.Kind == Fold && l <= line && line < l+r.N:
			d.Expand(i)
			return d.rowOfLine(side, line)
		case r.Kind != Fold && l == line:
			return i
		}
	}
	return -1
}

// sameLines 是两份行一样不一样。
func sameLines(a, b []string) bool { return slices.Equal(a, b) }
