package ged

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// 单栏的文件编辑器：和 Diff 共用一套排字、画、光标、编辑、查找、换行、粘性滚动、缩进参考线（view.go、edit.go、keys.go……），
// 只是没有旧的一边——不对齐、不折没改的段、没有 + −、一列行号。NewEditor 建的 Diff 是“纯的”（plain），
// 两边的行是同一份，行都是 Same（Old == New）；就地编辑的 Style.Buf 就是文件的缓冲区。
//
// 折叠照 VS Code 没有语法树时的做法（和 mygo 的编辑器一样）：行尾是开括号的，折到配对的闭括号前一行（闭括号那行留着）；
// 别的折到后面缩进比它深的行（末尾的空行不算）。折起来时那一行后面画一个“⋯”，点它或点行号右边的箭头展开；
// 光标挪进折起来的行里时自动展开。

// NewEditor 建一个空的单栏编辑器（见上）。
func NewEditor() *Diff {
	d := NewDiff()
	d.plain, d.unified, d.full = true, true, true
	d.folded = map[int]int{}
	return d
}

// Plain 是单栏的文件编辑器（NewEditor 建的）。
func (d *Diff) Plain() bool { return d.plain }

// SetText 换上内容（单栏编辑器用）：key 和上次一样就什么都不做（拿缓冲区的版本号当 key），返回内容变了没有。
// 光标、选中的字、折叠、滚动都留着（夹到新的行里）。
func (d *Diff) SetText(key any, load func() []string, lang *editor.Language) bool {
	return d.Set(key, func() (old, new []string) { l := load(); return l, l }, lang)
}

// Lines 是现在的每一行（单栏编辑器的内容）。
func (d *Diff) Lines() []string { return d.lines[1] }

// plainRows 是单栏的行：每行一个 Same，折起来的段藏掉（头一行留着）。
func (d *Diff) plainRows() []Row {
	n := len(d.lines[1])
	rows := make([]Row, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, Row{Kind: Same, Old: i, New: i})
		if end, ok := d.folded[i]; ok && end > i {
			i = end
		}
	}
	return rows
}

// foldEnd 是第 line 行能折到第几行（含），不能折 ok 为 false。
func (d *Diff) foldEnd(line int) (int, bool) {
	lines := d.lines[1]
	n := len(lines)
	if line < 0 || line >= n-1 {
		return 0, false
	}
	s := lines[line]
	if strings.TrimSpace(s) == "" {
		return 0, false
	}
	if open, ok := d.trailingOpener(line, s); ok {
		if _, m, ok := d.matchBracket(1, pos{line, open}); ok {
			switch {
			case m.line > line+1:
				return m.line - 1, true
			case m.line == line+1:
				return 0, false
			}
		}
	}
	tab := d.tabSize()
	in := indentWidth(s, tab)
	end := line
	for i := line + 1; i < n; i++ {
		t := lines[i]
		if strings.TrimSpace(t) == "" {
			continue
		}
		if indentWidth(t, tab) <= in {
			break
		}
		end = i
	}
	return end, end > line
}

// foldable 是第 line 行能不能折（画行号右边的箭头用，只看下一行非空行缩进深不深、行尾是不是开括号，便宜）。
func (d *Diff) foldable(line int) bool {
	lines := d.lines[1]
	if line < 0 || line >= len(lines)-1 || strings.TrimSpace(lines[line]) == "" {
		return false
	}
	if _, ok := d.trailingOpener(line, lines[line]); ok {
		_, ok := d.foldEnd(line)
		return ok
	}
	tab := d.tabSize()
	in := indentWidth(lines[line], tab)
	for i := line + 1; i < min(len(lines), line+50); i++ {
		if t := lines[i]; strings.TrimSpace(t) != "" {
			return indentWidth(t, tab) > in
		}
	}
	return false
}

// trailingOpener 是 s 末尾（去掉空白和行尾注释）的开括号在第几个字节。
func (d *Diff) trailingOpener(line int, s string) (int, bool) {
	t := strings.TrimRight(s, " \t")
	if d.lang != nil && d.lang.LineComment != "" {
		if i := strings.LastIndex(t, d.lang.LineComment); i >= 0 && d.tokenAt(pos{line, i}) == editor.TokenComment {
			t = strings.TrimRight(t[:i], " \t")
		}
	}
	if t == "" {
		return 0, false
	}
	c := t[len(t)-1]
	for _, p := range d.brackets() {
		if len(p.Open) == 1 && p.Open[0] == c && c != '<' {
			if k := d.tokenAt(pos{line, len(t) - 1}); k == editor.TokenString || k == editor.TokenComment {
				return 0, false
			}
			return len(t) - 1, true
		}
	}
	return 0, false
}

func (d *Diff) tabSize() int {
	if d.style != nil && d.style.Tab > 0 {
		return d.style.Tab
	}
	return 4
}

func indentWidth(s string, tab int) int {
	w := 0
	for _, c := range s {
		switch c {
		case ' ':
			w++
		case '\t':
			w += tab - w%tab
		default:
			return w
		}
	}
	return w
}

// IsFolded 是第 line 行折起来了没有（它后面的行藏着）。
func (d *Diff) IsFolded(line int) bool { _, ok := d.folded[line]; return ok }

// Folded 是折起来的行（头一行，从小到大）。
func (d *Diff) Folded() []int {
	out := make([]int, 0, len(d.folded))
	for l := range d.folded {
		out = append(out, l)
	}
	slices.Sort(out)
	return out
}

// Fold 折起第 line 行（能折的话），返回折了没有。光标在藏起来的行里时挪到这一行的行尾。
func (d *Diff) Fold(line int) bool {
	if !d.plain || d.IsFolded(line) {
		return false
	}
	end, ok := d.foldEnd(line)
	if !ok {
		return false
	}
	d.folded[line] = end
	d.afterFold(line, end)
	return true
}

// Unfold 展开第 line 行（折着的话），返回展开了没有。
func (d *Diff) Unfold(line int) bool {
	if _, ok := d.folded[line]; !ok {
		return false
	}
	delete(d.folded, line)
	d.refold()
	return true
}

// ToggleFold 折起或展开第 line 行。
func (d *Diff) ToggleFold(line int) bool {
	if d.IsFolded(line) {
		return d.Unfold(line)
	}
	return d.Fold(line)
}

// FoldAll 把能折的都折起（里面的也折，展开外层时里面的还折着，像 VS Code），UnfoldAll 都展开。
func (d *Diff) FoldAll() { d.foldLevel(0) }

func (d *Diff) UnfoldAll() {
	if len(d.folded) == 0 {
		return
	}
	clear(d.folded)
	d.refold()
}

// foldRange 是能折的一段：第 start 行折到第 end 行（含），depth 是第几层（最外层是 1）。
type foldRange struct{ start, end, depth int }

// foldRanges 是能折的每一段，按头一行从小到大。
func (d *Diff) foldRanges() []foldRange {
	var out []foldRange
	var stack []int // 包着现在这行的那几段的末行
	for i := range d.lines[1] {
		for len(stack) > 0 && stack[len(stack)-1] < i {
			stack = stack[:len(stack)-1]
		}
		if end, ok := d.foldEnd(i); ok {
			out = append(out, foldRange{i, end, len(stack) + 1})
			stack = append(stack, end)
		}
	}
	return out
}

// foldLevel 折起第 level 层的（0 是每一层都折）；别的折叠先都展开（VS Code 的“折叠级别 n”）。
func (d *Diff) foldLevel(level int) {
	if !d.plain {
		return
	}
	clear(d.folded)
	for _, f := range d.foldRanges() {
		if level == 0 || f.depth == level {
			d.folded[f.start] = f.end
		}
	}
	d.afterFold(-1, -1)
}

// blockAt 是包着第 line 行的、最里面那段能折的（line 自己能折就是它）：头一行和末行。
func (d *Diff) blockAt(line int) (start, end int, ok bool) {
	for l := line; l >= 0; l-- {
		if e, can := d.foldEnd(l); can && e >= line {
			return l, e, true
		}
		if line-l > 5000 { // 太远了不找（大文件里一直往上扫）
			break
		}
	}
	return 0, 0, false
}

// FoldRecursive 折起光标那一块和它里面的每一块；UnfoldRecursive 展开光标那一块和它里面的。
func (d *Diff) FoldRecursive() bool {
	h, end, ok := d.blockAt(d.sel.b.line)
	if !ok {
		return false
	}
	for i := h; i <= end; i++ {
		if e, can := d.foldEnd(i); can {
			d.folded[i] = e
		}
	}
	d.afterFold(h, end)
	return true
}

func (d *Diff) UnfoldRecursive() bool {
	line := d.sel.b.line
	h, end, ok := d.blockAt(line)
	if f, folded := d.folded[line]; folded {
		h, end, ok = line, f, true
	}
	if !ok {
		return false
	}
	changed := false
	for l := range d.folded {
		if l >= h && l <= end {
			delete(d.folded, l)
			changed = true
		}
	}
	if changed {
		d.refold()
	}
	return changed
}

// foldCommand 做折叠的命令（单栏编辑器），ok 是 cmd 是折叠的命令；done 是做了什么。
func (d *Diff) foldCommand(cmd editor.Command) (done, ok bool) {
	line := d.sel.b.line
	switch cmd {
	case editor.Fold:
		return d.FoldAt(), true
	case editor.Unfold:
		if !d.UnfoldAt() {
			if h, ok := d.hiddenBy(line); ok {
				return d.Unfold(h), true
			}
			return false, true
		}
		return true, true
	case editor.ToggleFold:
		if d.UnfoldAt() {
			return true, true
		}
		return d.FoldAt(), true
	case editor.FoldAll:
		d.FoldAll()
		return true, true
	case editor.UnfoldAll:
		d.UnfoldAll()
		return true, true
	case editor.FoldRecursive:
		return d.FoldRecursive(), true
	case editor.UnfoldRecursive:
		return d.UnfoldRecursive(), true
	case editor.FoldSelection:
		a, b := d.sel.span()
		if b.line > a.line {
			d.folded[a.line] = b.line
			d.afterFold(a.line, b.line)
		}
		return true, true
	}
	if cmd >= editor.FoldLevel1 && cmd <= editor.FoldLevel9 {
		d.foldLevel(int(cmd-editor.FoldLevel1) + 1)
		return true, true
	}
	return false, false
}

// FoldAt 折起光标所在的那一块：光标那一行能折就折它，不然往上找包着它的那一行。
func (d *Diff) FoldAt() bool {
	line := d.sel.b.line
	for l := line; l >= 0; l-- {
		if end, ok := d.foldEnd(l); ok && end >= line && !d.IsFolded(l) {
			return d.Fold(l)
		}
	}
	return false
}

// UnfoldAt 展开光标那一行（折着的话）。
func (d *Diff) UnfoldAt() bool { return d.Unfold(d.sel.b.line) }

// afterFold 折了以后重排行，光标、选中的字在藏起来的行里就挪到头一行的行尾。
func (d *Diff) afterFold(line, end int) {
	d.refold()
	for _, p := range []*pos{&d.sel.a, &d.sel.b} {
		if h, ok := d.hiddenBy(p.line); ok {
			*p = pos{h, len(d.lines[1][h])}
		}
	}
	_ = line
	_ = end
}

// hiddenBy 是第 line 行藏在哪一行的折叠里（最外层的头一行）。
func (d *Diff) hiddenBy(line int) (int, bool) {
	best, ok := 0, false
	for h, end := range d.folded {
		if line > h && line <= end && (!ok || h < best) {
			best, ok = h, true
		}
	}
	return best, ok
}

// fixFolds 在内容变了以后丢掉不再成立的折叠（头一行没了、变成不能折了），折到哪按现在的字重算。
func (d *Diff) fixFolds() {
	for h := range d.folded {
		end, ok := d.foldEnd(h)
		if !ok {
			delete(d.folded, h)
			continue
		}
		d.folded[h] = end
	}
}

// revealLine 让第 line 行露出来（展开藏着它的折叠），返回展开了没有。
func (d *Diff) revealLine(line int) bool {
	changed := false
	for {
		h, ok := d.hiddenBy(line)
		if !ok {
			break
		}
		delete(d.folded, h)
		changed = true
	}
	if changed {
		d.refold()
	}
	return changed
}

// foldGutterW 是单栏编辑器行号右边放折叠箭头的那一格多宽。
const foldGutterW = 16

// inFoldGutter 是 x（本地坐标）在折叠箭头那一格里。
func (d *Diff) inFoldGutter(x float32, g geom) bool {
	return d.plain && x >= g.numR+4 && x < g.numR+4+foldGutterW
}

// paintFoldMark 画第 line 行行号右边的折叠箭头：折着的一直画（›），能折的指针在行号那块上时才画（⌄），像 VS Code。
func (d *Diff) paintFoldMark(p *wui.Painter, x0, y float32, line int, g geom, s *Style) {
	folded := d.IsFolded(line)
	if !folded && !(d.gutterHover && d.foldable(line)) {
		return
	}
	c := s.Code.LineNumber
	cx, cy := x0+g.numR+4+foldGutterW/2, y+g.lh/2
	const k = 3.5
	if folded {
		p.StrokePolyline([][2]float32{{cx - k/2, cy - k}, {cx + k/2, cy}, {cx - k/2, cy + k}}, 1.3, c)
		return
	}
	p.StrokePolyline([][2]float32{{cx - k, cy - k/2}, {cx, cy + k/2}, {cx + k, cy - k/2}}, 1.3, c)
}

// paintEllipsis 在折起来的那一行的字后面画一个“⋯”的小框（点了展开），记下它在哪（本地坐标）。
func (d *Diff) paintEllipsis(p *wui.Painter, r wui.Rect, l *lineLayout, line int, sx, y float32, g geom, s *Style) {
	if !d.IsFolded(line) {
		return
	}
	x := sx + l.w + g.cw
	box := wui.Rect{X: x, Y: y + 3, W: 2.4 * g.cw, H: g.lh - 6}
	p.Fill(box, s.Code.LineNumber.Alpha(0.18), 3)
	f := s.Font
	gs := wui.Shape("⋯", f)
	base := y + (g.lh-(g.asc+g.desc))/2 + g.asc
	p.Glyphs(gs, box.X+(box.W-glyphsW(gs))/2, base, s.Code.LineNumber)
	if d.ellipses == nil {
		d.ellipses = map[int]wui.Rect{}
	}
	d.ellipses[line] = wui.Rect{X: box.X - r.X, Y: box.Y - r.Y, W: box.W, H: box.H}
}

// ellipsisAt 是 (x, y)（本地坐标）点在哪一行的“⋯”上。
func (d *Diff) ellipsisAt(x, y float32) (int, bool) {
	for line, b := range d.ellipses {
		if b.Contains(x, y) && d.IsFolded(line) {
			return line, true
		}
	}
	return 0, false
}

// plainNumbersW 是单栏编辑器行号那一格的右边（按行数的位数）。
func plainNumbersW(n int, cw float32) float32 {
	return 18 + float32(len(strconv.Itoa(max(n, 1))))*cw
}

// plainRow 是第 line 行在 rows 里是第几行（单栏，藏着的返回 -1）：rows 按行号从小到大，二分找。
func (d *Diff) plainRow(line int) int {
	i, ok := slices.BinarySearchFunc(d.rows, line, func(r Row, l int) int { return r.New - l })
	if !ok {
		return -1
	}
	return i
}

// stepLine 是从第 line 行往下（n > 0）/ 往上走 n 个看得见的行到第几行：单栏编辑器跳过折起来藏着的行，别的就是 line + n。
func (d *Diff) stepLine(line, n int) int {
	if !d.plain || len(d.folded) == 0 {
		return line + n
	}
	i := d.plainRow(line)
	if i < 0 {
		return line + n
	}
	j := i + n
	switch {
	case j < 0:
		return -1
	case j >= len(d.rows):
		return len(d.lines[1])
	}
	return d.rows[j].New
}

// shiftFolds 在内容从 before 变成 now 以后挪折叠：改的那段前面的不动，后面的跟着挪行号，头一行在改的那段里的丢掉；
// 然后按现在的字重算每个折到哪。
func (d *Diff) shiftFolds(before, now []string) {
	if len(d.folded) == 0 {
		return
	}
	p := 0
	for p < len(before) && p < len(now) && before[p] == now[p] {
		p++
	}
	q := 0
	for q < len(before)-p && q < len(now)-p && before[len(before)-1-q] == now[len(now)-1-q] {
		q++
	}
	delta := len(now) - len(before)
	out := map[int]int{}
	for h, end := range d.folded {
		switch {
		case h < p:
			out[h] = end
		case h >= len(before)-q:
			out[h+delta] = end + delta
		}
	}
	d.folded = out
	d.fixFolds()
}

// firstDiff 是 a、b 从第几行开始不一样（一样长且全一样时是长度）。
func firstDiff(a, b []string) int {
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	return p
}

// Selection 是选择（新的一边）：起点 (AL, AO)、光标那头 (BL, BO)。
func (d *Diff) Selection() Range {
	return Range{d.sel.a.line, d.sel.a.off, d.sel.b.line, d.sel.b.off}
}

// SetSelection 设选择（夹到现在的行里；在折起来的行里的展开）。reveal 时把光标滚进视图：本来看不见的滚到离顶上三行
// （跳到定义、搜索结果那样跳得远的），看得见的不动。键盘焦点不动。
func (d *Diff) SetSelection(r Range, reveal bool) {
	a, b := d.clampPos(pos{r.AL, r.AO}), d.clampPos(pos{r.BL, r.BO})
	if d.plain {
		d.revealLine(a.line)
		d.revealLine(b.line)
	}
	d.sel = selection{side: 1, a: a, b: b}
	d.side, d.goalX = 1, -1
	d.endGroup()
	if !reveal {
		return
	}
	if first, last := d.VisibleLines(); d.lh > 0 && b.line >= first && b.line <= last {
		d.reveal = true
		return
	}
	if i := d.rowOfLine(1, b.line); i >= 0 {
		d.want = i
	}
}

// VisibleLines 是上一帧看得见的第一行和最后一行（新的一边，从 0 起）；还没画过是 0, 0。
func (d *Diff) VisibleLines() (first, last int) {
	if d.lh <= 0 || d.h <= 0 || len(d.rows) == 0 {
		return 0, 0
	}
	top := min(d.rowIndex(int(d.ScrollY/d.lh)), len(d.rows)-1)
	bottom := min(d.rowIndex(int((d.ScrollY+d.h-1)/d.lh)), len(d.rows)-1)
	line := func(i int) int {
		for ; i >= 0; i-- {
			if l := d.rows[i].New; l >= 0 {
				return l
			}
		}
		return 0
	}
	return line(top), line(bottom)
}

// ScrollToLine 把第 line 行（新的一边，从 0 起）滚到顶上（下一帧画的时候生效）。
func (d *Diff) ScrollToLine(line int) { d.ScrollToLineFrac(float64(line)) }

// ScrollToLineFrac 和 ScrollToLine 一样，line 可以带小数：2.5 是第 2 行滚上去一半（折行的行按它占的屏幕行算）。
func (d *Diff) ScrollToLineFrac(line float64) {
	l := min(max(int(line), 0), max(len(d.lines[1])-1, 0))
	if i := d.rowOfLine(1, l); i >= 0 {
		d.wantTop, d.wantFrac = i+1, float32(min(max(line-float64(l), 0), 1))
	}
}

// TopLine 是上一帧顶上那一行（新的一边，从 0 起）带小数：2.5 是第 2 行滚上去了一半；还没画过 ok 为 false。
// 并排的 Markdown 预览按它在块里按比例对齐，不是整块整块地跳。
func (d *Diff) TopLine() (line float64, ok bool) {
	if d.lh <= 0 || d.h <= 0 || len(d.rows) == 0 {
		return 0, false
	}
	return d.lineAtRow(float64(d.ScrollY/d.lh), d.top, d.rowIndex), true
}

// RowOf 是第 line 行（新的一边，从 0 起，可带小数，小数同 TopLine 按它占的屏幕行算）从第几个屏幕行起（带小数）：
// 折行的行占几个屏幕行，折起来藏着的行不占。并排的 Markdown 预览按屏幕行和编辑器对齐：只按行数对齐的话，
// 折成三行的一段和它后面一个空行各算一行，编辑器匀速滚时预览一下快一下慢。
// 折成几行按估的算（只看字数和宽，不看排没排过）：排出来的数是滚到眼前才有的，用它的话一块里的屏幕行数
// 会随着往下滚一帧一帧地变，预览跟着一跳一跳。只读，不展开折叠；还没排过时原样返回 line。
func (d *Diff) RowOf(line float64) float64 {
	if len(d.rows) == 0 {
		return line
	}
	l := max(int(line), 0)
	frac := max(line-float64(l), 0)
	i := len(d.rows)
	if d.plain {
		i, _ = slices.BinarySearchFunc(d.rows, l, func(r Row, l int) int { return r.New - l })
	} else {
		for k, r := range d.rows {
			if r.New >= l {
				i = k
				break
			}
		}
	}
	if i >= len(d.rows) || d.rows[i].New != l {
		frac = 0 // 藏着的行、过了最后一行：算到下一个看得见的行的开头
	}
	t0, t1 := d.etop(i), d.etop(min(i+1, len(d.rows)))
	return float64(t0) + min(frac, 1)*float64(t1-t0)
}

// LineAtRow 是 RowOf 反过来：第 v 个屏幕行（带小数，同 RowOf 按估的折行数）对着新的一边的第几行（带小数）；
// 还没排过时原样返回 v。
func (d *Diff) LineAtRow(v float64) float64 {
	if len(d.rows) == 0 {
		return v
	}
	return d.lineAtRow(v, d.etop, func(v int) int {
		if !d.wrap {
			return d.rowIndex(v)
		}
		d.ensureETops()
		return sort.Search(len(d.rows), func(k int) bool { return d.etops[k+1] > v })
	})
}

// lineAtRow 是第 v 个屏幕行对着第几行（带小数）：top / index 是排出来的（TopLine）或估的（LineAtRow）。
func (d *Diff) lineAtRow(v float64, top, index func(int) int) float64 {
	v = max(v, 0)
	i := min(index(int(v)), len(d.rows)-1)
	frac := 0.0
	if t0, t1 := top(i), top(i+1); t1 > t0 {
		frac = min(max((v-float64(t0))/float64(t1-t0), 0), 1)
	}
	for k := i; k >= 0; k-- {
		if l := d.rows[k].New; l >= 0 {
			if k != i {
				frac = 0 // 那里是旧的一边才有的行：算到它上面那一行的开头
			}
			return float64(l) + frac
		}
	}
	return 0
}

// PointAt 是视图里 (x, y)（本地坐标）对着第几行第几个字节（新的一边）；不在正文上时 ok 为 false。
func (d *Diff) PointAt(x, y float32) (line, off int, ok bool) {
	if d.style == nil || d.w <= 0 {
		return 0, 0, false
	}
	g := d.geomNow(d.style)
	if d.rowAt(y, g) < 0 || x < g.paneX[1]+g.textX {
		return 0, 0, false
	}
	p := d.posAt(x, y, 1, g, d.style)
	return p.line, p.off, true
}

// CharBox 是第 line 行第 off 个字节那个字在视图里的框（本地坐标，按上一帧的滚动）：悬停提示、补全列表跟着它摆。
// 那一行藏着、还没画过时 ok 为 false。
func (d *Diff) CharBox(line, off int) (wui.Rect, bool) {
	if d.style == nil || d.lh <= 0 || line < 0 || line >= len(d.lines[1]) {
		return wui.Rect{}, false
	}
	i := -1
	if d.plain {
		i = d.plainRow(line)
	} else {
		for k, r := range d.rows {
			if r.Kind != Fold && r.New == line {
				i = k
				break
			}
		}
	}
	if i < 0 {
		return wui.Rect{}, false
	}
	g := d.geomNow(d.style)
	l := d.layout(1, line, d.style)
	off = min(max(off, 0), len(l.u16)-1)
	cr := l.l.Caret(l.u16[off])
	x := g.paneX[1] + g.textX - d.scrollX[1] + cr.X
	y := float32(d.top(i)+l.caretRow(off))*g.lh - d.ScrollY
	return wui.Rect{X: x, Y: y, W: g.cw, H: g.lh}, true
}

// Focused 是上一帧视图有键盘焦点。
func (d *Diff) Focused() bool { return d.focused }

// Focus 让视图下一帧拿到键盘焦点。
func (d *Diff) Focus() { d.focus = true }

// trackHover 记指针停在哪个字上（Style.Hover）：换了字重新计时，出了正文（行号、过了行尾、概览条）算走了。
func (d *Diff) trackHover(x, y float32, i int, inBar bool, g geom, s *Style) {
	on := d.plain && !inBar && i >= 0 && x >= g.paneX[1]+g.textX && !d.selecting
	var p pos
	if on {
		p = d.posAt(x, y, 1, g, s)
		if r, ok := d.CharBox(p.line, len(d.lines[1][p.line])); !ok || x > r.X+g.cw/2 && !d.wrap {
			on = false // 过了行尾
		}
	}
	if !on {
		d.endHover(s)
		return
	}
	if !d.hoverOn || p != d.hoverPos {
		d.hoverOn, d.hoverPos, d.hoverAt = true, p, time.Time{}
		d.hoverSent = false
	}
}

// endHover 是指针走了、按了键：停稳过的报一声 Hover(-1, 0)。
func (d *Diff) endHover(s *Style) {
	if !d.hoverOn {
		return
	}
	d.hoverOn = false
	if d.hoverSent && s.Hover != nil {
		d.hoverSent = false
		s.Hover(-1, 0)
	}
}

// hoverTick 每帧看指针停够了没有（够了报 Style.Hover，不够排一帧到点再看）。
func (d *Diff) hoverTick(c *wui.Context, hovered bool, s *Style) {
	if s.Hover == nil {
		return
	}
	if !hovered {
		d.endHover(s)
		return
	}
	if !d.hoverOn || d.hoverSent {
		return
	}
	now := c.Now()
	if d.hoverAt.IsZero() {
		d.hoverAt = now
	}
	if wait := HoverDelay - now.Sub(d.hoverAt); wait > 0 {
		c.After(wait)
		return
	}
	d.hoverSent = true
	s.Hover(d.hoverPos.line, d.hoverPos.off)
}

// Command 执行一个编辑器命令（菜单、命令面板来的，和按了它的键一样），做了返回 true；ged 不做的交给 Style.Fallback。
// 还没画过（没有 Style）时什么都不做。
func (d *Diff) Command(cmd editor.Command) bool {
	if d.style == nil || d.w <= 0 {
		return false
	}
	s := d.style
	g := d.geomNow(s)
	c, ok := d.caret(s)
	if !ok {
		c = d.sel.b
	}
	if !d.runEdit(cmd, c, g, s) {
		return false
	}
	d.clamp(g)
	if s.Changed != nil {
		s.Changed()
	}
	return true
}

// FoldImports 折起前 200 行里第一段能折的导入（import ( / import { / use {……），折了返回 true。
func (d *Diff) FoldImports() bool {
	lines := d.lines[1]
	for i := 0; i < len(lines) && i < 200; i++ {
		t := strings.TrimSpace(lines[i])
		if (strings.HasPrefix(t, "import") || strings.HasPrefix(t, "use ") || strings.HasPrefix(t, "from ")) && d.foldable(i) {
			return d.Fold(i)
		}
	}
	return false
}

// Lookup 是 ev（按键）在键位表里对应的命令（和视图自己查的一样，和弦也认）：eaten 是键被和弦吃掉了
// （和弦的第一个键，或没查到的第二个键），这时 cmd 是 NoCommand。给 Style.Intercept 用：多光标时键交给别人做。
func (d *Diff) Lookup(ev wui.InputEvent) (cmd editor.Command, eaten bool) {
	if d.style == nil || ev.Kind != wui.InputKeyDown {
		return editor.NoCommand, false
	}
	switch cmd := d.command(ev, d.style); cmd {
	case chordPending, chordDropped:
		return editor.NoCommand, true
	default:
		return cmd, false
	}
}
