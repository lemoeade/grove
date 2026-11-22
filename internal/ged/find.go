package ged

import "strings"

// 查找：在 Diff 里找字（不分大小写，只折 ASCII 的大小写：中日韩没有大小写，别的折了字节数会变）。找的是看得见的字各一遍：
// 新的一边的每一行，加上旧的一边删掉、改掉的行（没改的行两边一样，算新的一边的；并排时左边也一样涂）。
// 按对齐的顺序排，同一行旧的在前。查找条由调用方画（Style.FindBar），输入框绑在 Diff.Find.Text 上。

// Find 是查找条的状态。
type Find struct {
	Open  bool   // 查找条开着
	Text  string // 输入框里的字
	Focus bool   // 下一帧把键盘给输入框、全选里面的字（调用方画输入框时用掉）
	// Replace 是查找条下面多一行替换（就地编辑时才有），With 是替换框里的字；FocusWith 下一帧把键盘给替换框。
	Replace   bool
	With      string
	FocusWith bool
}

type match struct {
	side, line int
	start, end int // 那一行里的字节
	nl         int // 在新的一边的哪一行（旧的一边的行算它后面那一行）：从视口往下找第一处时按它比
}

// findMax 是最多记几处：再多数字也只写到这，免得一个字母把整个大文件找一遍卡住。
const findMax = 20000

// OpenFind 开查找条、把键盘给输入框。选中了一行之内的字时拿它当要找的。
func (d *Diff) OpenFind() {
	if t := d.Selected(); t != "" && !strings.Contains(t, "\n") {
		d.Find.Text = t
	}
	d.Find.Open, d.Find.Focus, d.Find.Replace = true, true, false
	d.findQ = "\x00" // 下一次 SetFind 重新找、跳到视口往下的第一处
}

// OpenReplace 开查找条和下面的替换（不在就地编辑时只开查找）。查找框里已经有字时键盘给替换框。
func (d *Diff) OpenReplace() {
	d.OpenFind()
	if !d.editable {
		return
	}
	d.Find.Replace = true
	if d.Find.Text != "" {
		d.Find.Focus, d.Find.FocusWith = false, true
	}
}

// CanReplace 是能替换：就地编辑着、查找条开着替换。
func (d *Diff) CanReplace() bool { return d.editable && d.buf != nil && d.Find.Open && d.Find.Replace }

// ReplaceOne 把当前那处换成 Find.With，跳到下一处（VS Code 的“替换”）：当前那处不在新的一边（旧的一边只能看）
// 或还没跳过时，先跳到下一处新的一边的，不换。换了返回 true。
func (d *Diff) ReplaceOne() bool {
	if !d.CanReplace() || len(d.matches) == 0 {
		return false
	}
	if d.findCur < 0 || d.matches[d.findCur].side != 1 {
		d.stepNew(d.findCur)
		return false
	}
	m := d.matches[d.findCur]
	s := &Style{Buf: d.buf}
	d.replace(s, pos{m.line, m.start}, pos{m.line, m.end}, d.Find.With, editOther)
	// replace 重新对齐时已经重找过：从换进去的字后面找下一处新的一边的
	at := pos{m.line, m.start + len(d.Find.With)}
	d.findCur = -1
	for i, n := range d.matches {
		if n.side == 1 && !(pos{n.line, n.start}).less(at) {
			d.findCur = i
			break
		}
	}
	if d.findCur < 0 {
		d.stepNew(-1)
	}
	d.revealMatch()
	return true
}

// stepNew 从 from 往后（到尾绕回）跳到下一处新的一边的。
func (d *Diff) stepNew(from int) {
	n := len(d.matches)
	for k := 1; k <= n; k++ {
		i := ((from+k)%n + n) % n
		if d.matches[i].side == 1 {
			d.findCur = i
			d.revealMatch()
			return
		}
	}
}

// ReplaceAll 把新的一边找到的都换成 Find.With（一步撤销），返回换了几处。
func (d *Diff) ReplaceAll() int {
	if !d.CanReplace() {
		return 0
	}
	q := foldASCII(d.findQ)
	if q == "" || q == "\x00" {
		return 0
	}
	lines := d.lines[1]
	out := make([]string, len(lines))
	n, first, last := 0, -1, -1
	for i, t := range lines {
		src := foldASCII(t)
		var b strings.Builder
		at := 0
		for {
			k := strings.Index(src[at:], q)
			if k < 0 {
				break
			}
			b.WriteString(t[at : at+k])
			b.WriteString(d.Find.With)
			at += k + len(q)
			n++
		}
		if at == 0 {
			out[i] = t
			continue
		}
		b.WriteString(t[at:])
		out[i] = b.String()
		if first < 0 {
			first = i
		}
		last = i
	}
	if n == 0 {
		return 0
	}
	c := d.sel.b
	s := &Style{Buf: d.buf}
	d.rewriteLines(s, first, last, out[first:last+1], nil)
	p := d.clampPos(c)
	d.sel = selection{side: 1, a: p, b: p}
	return n
}

// CloseFind 关查找条，键盘还给视图。
func (d *Diff) CloseFind() {
	d.Find.Open = false
	d.focus = true
}

// SetFind 按 q 找（和上次一样不重找）；q 变了时当前那处换成视口顶上往下的第一处，滚过去。
func (d *Diff) SetFind(q string) {
	if q == d.findQ {
		return
	}
	d.findQ = q
	d.search()
	d.findCur = -1
	if len(d.matches) == 0 {
		return
	}
	top := d.topNewLine()
	d.findCur = 0
	for i, m := range d.matches {
		if m.nl >= top {
			d.findCur = i
			break
		}
	}
	d.revealMatch()
}

// FindStep 跳到下一处（dir 1）/ 上一处（-1），到头了绕回去；在折起来的段里时展开那段。没有找到的返回 false。
func (d *Diff) FindStep(dir int) bool {
	n := len(d.matches)
	if n == 0 {
		return false
	}
	switch {
	case d.findCur < 0 && dir < 0:
		d.findCur = n - 1
	case d.findCur < 0:
		d.findCur = 0
	default:
		d.findCur = ((d.findCur+dir)%n + n) % n
	}
	d.revealMatch()
	return true
}

// FindStatus 是第几处（从 1 起，还没跳过是 0）、一共几处；more 是多到没记全。
func (d *Diff) FindStatus() (cur, total int, more bool) {
	return d.findCur + 1, len(d.matches), len(d.matches) >= findMax
}

// search 按 findQ 重新找一遍。
func (d *Diff) search() {
	d.matches, d.findAt = d.matches[:0], map[lineKey][]int{}
	q := foldASCII(d.findQ)
	if q == "" || q == "\x00" {
		return
	}
	add := func(side, line, nl int) {
		src := foldASCII(clean(d.lines[side][line]))
		for at := 0; len(d.matches) < findMax; {
			k := strings.Index(src[at:], q)
			if k < 0 {
				return
			}
			s := at + k
			d.findAt[lineKey{side, line}] = append(d.findAt[lineKey{side, line}], len(d.matches))
			d.matches = append(d.matches, match{side: side, line: line, start: s, end: s + len(q), nl: nl})
			at = s + len(q)
		}
	}
	next := 0 // 新的一边的下一行
	for _, r := range d.aligned {
		if r.Old >= 0 && (r.Kind != Same || !d.sameText(r)) {
			add(0, r.Old, next)
		}
		if r.New >= 0 {
			add(1, r.New, r.New)
			next = r.New + 1
		}
	}
}

// foldASCII 把 ASCII 的大写字母换成小写（字节数不变）。
func foldASCII(s string) string {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if b[j] >= 'A' && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

// topNewLine 是视口顶上那一行在新的一边是第几行（那行是旧的一边才有的，往下找第一行新的）。
func (d *Diff) topNewLine() int {
	i := 0
	if d.lh > 0 {
		i = d.rowIndex(int(d.ScrollY / d.lh))
	}
	for ; i < len(d.rows); i++ {
		if l := d.rows[i].New; l >= 0 {
			return l
		}
	}
	return len(d.lines[1])
}

// revealMatch 展开当前那处所在的折叠段，下一帧把它滚进视图（revealFind）。
func (d *Diff) revealMatch() {
	if d.findCur < 0 || d.findCur >= len(d.matches) {
		return
	}
	m := d.matches[d.findCur]
	if i := d.rowOfLine(m.side, m.line); i >= 0 && d.rowHunk[i] >= 0 {
		d.cur = d.rowHunk[i]
	}
	d.findReveal = true
}

// revealFind 把当前那处滚进视图：竖着在视口外时滚到离顶上三行，横着滚那一边（不换行时）。
func (d *Diff) revealFind(g geom, s *Style) {
	if !d.findReveal || g.lh <= 0 || d.h <= 0 {
		return
	}
	d.findReveal = false
	if d.findCur < 0 || d.findCur >= len(d.matches) {
		return
	}
	m := d.matches[d.findCur]
	i := d.rowOfLine(m.side, m.line)
	if i < 0 {
		return
	}
	l := d.layout(m.side, m.line, s)
	y := float32(d.top(i)+l.caretRow(m.start)) * g.lh
	if y < d.ScrollY || y+g.lh > d.ScrollY+g.h {
		d.ScrollY = max(0, y-Context*g.lh)
	}
	if d.wrap {
		return
	}
	side := m.side
	if d.unified {
		side, d.side = 0, m.side
	}
	x0 := l.l.Caret(l.u16[min(m.start, len(l.u16)-1)]).X
	x1 := l.l.Caret(l.u16[min(m.end, len(l.u16)-1)]).X
	room := g.paneW[m.side] - g.textX - g.right
	if x0 < d.scrollX[side] || x1 > d.scrollX[side]+room {
		x := max(0, x0-room/3)
		if d.unified {
			d.scrollX = [2]float32{x, x}
		} else {
			d.scrollX[side] = x
		}
	}
}

// findSpans 是 side 那边第 line 行里找到的几处（matches 的下标）；没改的行旧的一边借新的一边的。
func (d *Diff) findSpans(side int, row Row) []int {
	if !d.Find.Open || len(d.matches) == 0 {
		return nil
	}
	if side == 0 && row.Kind == Same && d.sameText(row) {
		side = 1
	}
	return d.findAt[lineKey{side, lineOf(row, side)}]
}

// sameText 是没改的行两边的字一模一样（忽略行首行尾空白时，只改了空白的行也算没改，但字不一样）。
func (d *Diff) sameText(r Row) bool {
	return r.Old >= 0 && r.New >= 0 && r.Old < len(d.lines[0]) && r.New < len(d.lines[1]) && d.lines[0][r.Old] == d.lines[1][r.New]
}

// occurMax 是选中的字多长以内才涂别处一样的（再长就不像是在看“这个词还在哪用了”）。
const occurMax = 200

// occurText 是要涂别处一样的字：选中了一行之内的、不全是空白的字（VS Code 的 selection highlight，分大小写）。
func (d *Diff) occurText() string {
	if d.sel.empty() {
		return ""
	}
	a, b := d.sel.span()
	if a.line != b.line || b.off-a.off > occurMax {
		return ""
	}
	t := d.Selected()
	if strings.TrimSpace(t) == "" {
		return ""
	}
	return t
}

// occurrences 是 side 那边第 line 行里和选中的字一样的几处（字节偏移，选中的那处自己不算）。
func (d *Diff) occurrences(side, line int) []int {
	if d.occ == "" || line < 0 || line >= len(d.lines[side]) {
		return nil
	}
	src := clean(d.lines[side][line])
	var out []int
	a, _ := d.sel.span()
	for at := 0; ; {
		k := strings.Index(src[at:], d.occ)
		if k < 0 {
			return out
		}
		o := at + k
		if side != d.sel.side || line != a.line || o != a.off {
			out = append(out, o)
		}
		at = o + len(d.occ)
	}
}

// Occurrences 是 side 那边第 line 行里涂了的几处（上一帧算的选中的字）。给测试用。
func (d *Diff) Occurrences(side, line int) []int { return d.occurrences(side, line) }
