package ged

import (
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// 就地编辑时的键：和 Grove 的编辑器（mygo 的 editor）用一张键位表（Style.Bindings 叠在 editor.DefaultBindings 上，
// 改过的键两处一起变），认出是哪个命令以后 ged 自己做。ged 只有一个光标：⌘D 没选字时选中光标处的词，选了字时
// 选中下一处一样的（Diff 里不是多加一个光标）。ged 不做的命令（多光标、补全……）交给 Style.Fallback（单栏编辑器
// 交给 mygo 的 State 做，ged 只画结果），没给 Fallback 时键交回去。和弦（⌘K ⌘L 这种）、折叠只在单栏编辑器里认。

var defaultBindings = sync.OnceValue(editor.DefaultBindings)

// command 是 ev 在键位表里对应的命令。改过的键（Style.Bindings）先查，绑到 NoCommand 的键算拿走了。
// 和弦（⌘K ⌘L 这种）：第一个键只是记下来（返回 chordPending），下一个键和它一起查；查不到就算了（两个键都吃掉）。
func (d *Diff) command(ev wui.InputEvent, s *Style) editor.Command {
	mods := ev.Mods & (wui.Shift | wui.Ctrl | wui.Alt | wui.Super)
	if ev.Key == wui.KeyUnknown { // 只按了修饰键
		return editor.NoCommand
	}
	if d.chordOn {
		d.chordOn = false
		for _, list := range [][]editor.Binding{s.Bindings, defaultBindings()} {
			for _, b := range list {
				if b.ThenKey != 0 && b.Mods == d.chord.Mods && b.Key == d.chord.Key && b.ThenMods == mods && b.ThenKey == ev.Key {
					return b.Command
				}
			}
		}
		return chordDropped
	}
	prefix := false
	for _, list := range [][]editor.Binding{s.Bindings, defaultBindings()} {
		for _, b := range list {
			if b.Mods != mods || b.Key != ev.Key {
				continue
			}
			if b.ThenKey == 0 {
				return b.Command
			}
			prefix = true
		}
		if prefix {
			break
		}
	}
	if prefix && d.plain { // 和弦只在单栏编辑器里认（Diff 里 ⌘K 没有要接的）
		d.chord, d.chordOn = editor.Binding{Mods: mods, Key: ev.Key}, true
		return chordPending
	}
	return editor.NoCommand
}

// 和弦的第一个键、和弦没查到：command 返回的假命令（键吃掉，什么都不做）。
const (
	chordPending editor.Command = 0xFFFF - iota
	chordDropped
)

// editKey 是就地编辑、有光标时视图自己接的键；没接的（打字的键、ged 没有的命令）返回 false。
func (d *Diff) editKey(ev wui.InputEvent, g geom, s *Style) bool {
	c, ok := d.caret(s)
	if !ok || d.compose != "" { // 输入法正拼着字：键归它
		return false
	}
	if !d.runEdit(d.command(ev, s), c, g, s) {
		return false
	}
	d.clamp(g)
	return true
}

// runEdit 做一个编辑器命令；ged 不做的返回 false。
func (d *Diff) runEdit(cmd editor.Command, c pos, g geom, s *Style) bool {
	lines := d.lines[1]
	page := max(1, int((g.h-2*g.lh)/max(g.lh, 1)))
	last := pos{len(lines) - 1, len(lines[len(lines)-1])}
	a, b := d.sel.span()
	if d.plain {
		if done, ok := d.foldCommand(cmd); ok {
			return done
		}
		switch cmd { // 单栏编辑器的查找条是 ged 的（Diff 的查找由 App 的命令开）
		case editor.Find:
			d.OpenFind()
			return true
		case editor.FindReplace:
			d.OpenReplace()
			return true
		case editor.FindNext, editor.FindPrevious:
			if !d.Find.Open || d.Find.Text == "" {
				d.OpenFind()
			} else if cmd == editor.FindNext {
				d.FindStep(1)
			} else {
				d.FindStep(-1)
			}
			return true
		}
	}
	switch cmd {
	case chordPending, chordDropped:
		return true
	case editor.MoveLeft:
		if !d.sel.empty() { // 有选中的字：收到前头
			d.moveTo(a, false)
		} else {
			d.moveTo(d.left(c), false)
		}
	case editor.SelectLeft:
		d.moveTo(d.left(c), true)
	case editor.MoveRight:
		if !d.sel.empty() {
			d.moveTo(b, false)
		} else {
			d.moveTo(d.right(c), false)
		}
	case editor.SelectRight:
		d.moveTo(d.right(c), true)
	case editor.MoveUp, editor.SelectUp:
		d.vertical(c, -1, cmd == editor.SelectUp, s)
	case editor.MoveDown, editor.SelectDown:
		d.vertical(c, 1, cmd == editor.SelectDown, s)
	case editor.MoveWordLeft, editor.MoveSubwordLeft, editor.SelectWordLeft, editor.SelectSubwordLeft:
		d.moveTo(d.wordLeft(c), cmd == editor.SelectWordLeft || cmd == editor.SelectSubwordLeft)
	case editor.MoveWordRight, editor.MoveSubwordRight, editor.SelectWordRight, editor.SelectSubwordRight:
		d.moveTo(d.wordRight(c), cmd == editor.SelectWordRight || cmd == editor.SelectSubwordRight)
	case editor.MoveLineStart, editor.SelectLineStart:
		d.moveTo(pos{c.line, d.home(c)}, cmd == editor.SelectLineStart)
	case editor.MoveLineEnd, editor.SelectLineEnd:
		d.moveTo(pos{c.line, len(lines[c.line])}, cmd == editor.SelectLineEnd)
	case editor.MoveDocStart, editor.SelectDocStart:
		d.moveTo(pos{}, cmd == editor.SelectDocStart)
	case editor.MoveDocEnd, editor.SelectDocEnd:
		d.moveTo(last, cmd == editor.SelectDocEnd)
	case editor.MoveParagraphUp, editor.SelectParagraphUp:
		d.moveTo(d.paragraph(c, -1), cmd == editor.SelectParagraphUp)
	case editor.MoveParagraphDown, editor.SelectParagraphDown:
		d.moveTo(d.paragraph(c, 1), cmd == editor.SelectParagraphDown)
	case editor.MovePageUp, editor.SelectPageUp:
		d.ScrollY -= float32(page) * g.lh
		d.vertical(c, -page, cmd == editor.SelectPageUp, s)
	case editor.MovePageDown, editor.SelectPageDown:
		d.ScrollY += float32(page) * g.lh
		d.vertical(c, page, cmd == editor.SelectPageDown, s)
	case editor.MoveToBracket, editor.SelectToBracket:
		at, m, ok := d.matchBracket(1, c)
		if !ok {
			return true
		}
		if cmd == editor.MoveToBracket {
			d.moveTo(m, false)
			break
		}
		lo, hi := at, m // 连着两个括号一起选
		if hi.less(lo) {
			lo, hi = hi, lo
		}
		d.sel = selection{side: 1, a: lo, b: d.right(hi)}
		d.reveal = true
	case editor.SelectAll:
		d.SelectAll(1)
	case editor.SelectLine: // 选中光标所在的行（连着换行）；已经是整行时往下多选一行
		if d.sel.empty() || a.off != 0 || b.off != 0 || b.line == a.line {
			a = pos{a.line, 0}
		}
		end := pos{b.line + 1, 0}
		if end.line >= len(lines) {
			end = last
		}
		d.sel = selection{side: 1, a: a, b: end}
		d.reveal = true
	case editor.SelectNext:
		if !d.sel.empty() && s.Fallback != nil { // 单栏编辑器：⌘D 再按是多加一个光标（交给 Fallback）
			return s.Fallback(cmd)
		}
		if d.sel.empty() {
			d.sel = d.wordAt(1, c)
			d.sel.side = 1
		} else if n, ok := d.nextOccurrence(d.Selected(), b); ok {
			d.sel = n
		}
		d.reveal = true
	case editor.Cancel:
		if d.sel.empty() {
			return false
		}
		d.moveTo(d.sel.b, false)
	case editor.Backspace:
		switch {
		case !d.sel.empty():
			d.typeText(s, "", editOther)
		case d.inPair(c):
			d.replace(s, d.left(c), d.right(c), "", editOther)
		default:
			d.replace(s, d.left(c), c, "", editDeleting)
		}
	case editor.Delete:
		if !d.sel.empty() {
			d.typeText(s, "", editOther)
		} else {
			d.replace(s, c, d.right(c), "", editDeleting)
		}
	case editor.DeleteWordLeft, editor.DeleteSubwordLeft:
		d.deleteTo(s, d.wordLeft(c))
	case editor.DeleteWordRight, editor.DeleteSubwordRight:
		d.deleteTo(s, d.wordRight(c))
	case editor.DeleteToLineStart:
		d.deleteTo(s, pos{c.line, 0})
	case editor.DeleteToLineEnd:
		d.deleteTo(s, pos{c.line, len(lines[c.line])})
	case editor.CutToLineEnd: // ⌃K：剪到行尾，已经在行尾时剪掉换行
		e := pos{c.line, len(lines[c.line])}
		if c.off == e.off {
			e = d.right(c)
		}
		if !d.sel.empty() {
			c, e = a, b
		}
		if s.Copy != nil && c != e {
			d.sel = selection{side: 1, a: c, b: e}
			s.Copy(d.Selected())
		}
		d.replace(s, c, e, "", editOther)
	case editor.DeleteLine:
		d.deleteLines(s)
	case editor.Newline:
		d.newline(s)
	case editor.NewlineBelow:
		ind := indentOf(lines[c.line])
		d.replace(s, pos{c.line, len(lines[c.line])}, pos{c.line, len(lines[c.line])}, "\n"+ind, editOther)
	case editor.NewlineAbove:
		ind := indentOf(lines[c.line])
		d.replace(s, pos{c.line, 0}, pos{c.line, 0}, ind+"\n", editOther)
		d.sel = selection{side: 1, a: pos{c.line, len(ind)}, b: pos{c.line, len(ind)}}
	case editor.MoveLinesUp, editor.MoveLinesDown:
		d.moveLines(s, map[bool]int{true: -1, false: 1}[cmd == editor.MoveLinesUp])
	case editor.DuplicateLinesUp, editor.DuplicateLinesDown:
		d.duplicateLines(s, cmd == editor.DuplicateLinesDown)
	case editor.JoinLines:
		d.joinLines(s)
	case editor.Transpose:
		d.transpose(s, c)
	case editor.Tab:
		if a.line != b.line {
			d.indentLines(s, 1)
			break
		}
		d.typeText(s, d.indentAt(s, a), editTyping)
	case editor.Indent:
		d.indentLines(s, 1)
	case editor.Outdent, editor.Backtab:
		d.indentLines(s, -1)
	case editor.ToggleComment:
		return d.toggleComment(s)
	case editor.ToggleBlockComment:
		return d.toggleBlockComment(s)
	case editor.TrimTrailingWhitespace:
		d.trimTrailing(s)
	case editor.ScrollCursorCenter:
		if i := d.rowOfLine(1, c.line); i >= 0 {
			d.ScrollY = max(0, float32(d.top(i))*g.lh-g.h/2)
		}
	case editor.ScrollLineUp, editor.ScrollLineDown:
		d.ScrollY += map[bool]float32{true: -g.lh, false: g.lh}[cmd == editor.ScrollLineUp]
	default:
		if cmd != editor.NoCommand && s.Fallback != nil {
			return s.Fallback(cmd)
		}
		return false
	}
	return true
}

// deleteTo 删光标到 p 之间的字（有选中的字时删选中的）。
func (d *Diff) deleteTo(s *Style, p pos) {
	if !d.sel.empty() {
		d.typeText(s, "", editOther)
		return
	}
	d.replace(s, d.sel.b, p, "", editOther)
}

// paragraph 是往上 / 下（dir）到下一个空行（没有就到头）。
func (d *Diff) paragraph(c pos, dir int) pos {
	lines := d.lines[1]
	for i := c.line + dir; i >= 0 && i < len(lines); i += dir {
		if strings.TrimSpace(lines[i]) == "" && strings.TrimSpace(lines[i-dir]) != "" {
			return pos{i, 0}
		}
	}
	if dir < 0 {
		return pos{}
	}
	return pos{len(lines) - 1, len(lines[len(lines)-1])}
}

// lineSpan 是选中的字占的几行（从 l0 到 l1，两头都算）：选到下一行行头的，那一行不算。
func (d *Diff) lineSpan() (l0, l1 int) {
	a, b := d.sel.span()
	l1 = b.line
	if b.line > a.line && b.off == 0 {
		l1--
	}
	return a.line, l1
}

// rewriteLines 把新的一边第 l0 到 l1 行换成 out（一步撤销）；选中的字两头按那一行在 at[k] 处多出 / 少了的字挪
// （at 为 nil 时光标放在 out 最后一行的行尾）。
func (d *Diff) rewriteLines(s *Style, l0, l1 int, out []string, at []int) {
	old := slices.Clone(d.lines[1][l0 : l1+1])
	sel := d.sel
	d.replace(s, pos{l0, 0}, pos{l1, len(d.lines[1][l1])}, strings.Join(out, "\n"), editOther)
	if at == nil || len(out) != len(old) {
		return
	}
	fix := func(p pos) pos {
		if p.line < l0 || p.line > l1 {
			return p
		}
		k := p.line - l0
		if p.off >= at[k] {
			p.off = max(at[k], p.off+len(out[k])-len(old[k]))
		}
		return d.clampPos(p)
	}
	d.sel = selection{side: 1, a: fix(sel.a), b: fix(sel.b)}
}

// indentOf 是一行开头的空白。
func indentOf(t string) string { return t[:len(t)-len(strings.TrimLeft(t, " \t"))] }

// indentUnit 是缩进一级打什么：Style.Indent 定了按它，没定（自动）时看这个文件里用的是 Tab 还是几个空格，
// 都没有按语言的习惯（Go 是 Tab），再没有是 4 个空格。
func (d *Diff) indentUnit(s *Style) string {
	switch s.Indent {
	case "tab":
		return "\t"
	case "", "auto":
	default:
		if n, err := strconv.Atoi(s.Indent); err == nil && n > 0 {
			return strings.Repeat(" ", n)
		}
	}
	tabs, spaces, least := 0, 0, 0
	for i, t := range d.lines[1] {
		if i > 2000 {
			break
		}
		switch {
		case strings.HasPrefix(t, "\t"):
			tabs++
		case strings.HasPrefix(t, " ") && strings.TrimSpace(t) != "":
			n := len(t) - len(strings.TrimLeft(t, " "))
			if n >= 2 && (least == 0 || n < least) {
				least = n
			}
			spaces++
		}
	}
	switch {
	case tabs > spaces:
		return "\t"
	case spaces > 0 && least > 0:
		return strings.Repeat(" ", min(least, 8))
	case d.lang != nil && d.lang.HardTabs:
		return "\t"
	case d.lang != nil && d.lang.TabSize > 0:
		return strings.Repeat(" ", d.lang.TabSize)
	}
	return "    "
}

// indentAt 是在 p 按 Tab 打进去的：用 Tab 缩进时是一个 Tab，用空格时补到下一格。
func (d *Diff) indentAt(s *Style, p pos) string {
	u := d.indentUnit(s)
	if u == "\t" {
		return u
	}
	tab := max(s.Tab, 1)
	if s.Tab <= 0 {
		tab = 4
	}
	col := 0
	for _, r := range d.lines[1][p.line][:p.off] {
		if r == '\t' {
			col += tab - col%tab
		} else {
			col++
		}
	}
	return strings.Repeat(" ", len(u)-col%len(u))
}

// indentLines 给选中的几行（没选是光标那一行）加（dir 1）/ 减（-1）一级缩进；空行不加。
func (d *Diff) indentLines(s *Style, dir int) {
	l0, l1 := d.lineSpan()
	u := d.indentUnit(s)
	lines := d.lines[1]
	out := make([]string, 0, l1-l0+1)
	at := make([]int, 0, l1-l0+1)
	for _, t := range lines[l0 : l1+1] {
		at = append(at, 0)
		switch {
		case dir > 0 && t != "":
			t = u + t
		case dir < 0 && strings.HasPrefix(t, "\t"):
			t = t[1:]
		case dir < 0:
			n := len(t) - len(strings.TrimLeft(t, " "))
			t = t[min(n, max(len(u), 1)):]
		}
		out = append(out, t)
	}
	if slices.Equal(out, lines[l0:l1+1]) {
		return
	}
	d.rewriteLines(s, l0, l1, out, at)
}

// deleteLines 删掉选中的几行（没选是光标那一行），光标留在原来那一格、下一行（删的是最后几行时上一行）。
func (d *Diff) deleteLines(s *Style) {
	l0, l1 := d.lineSpan()
	lines := d.lines[1]
	col := d.sel.b.off
	switch {
	case l1+1 < len(lines):
		d.replace(s, pos{l0, 0}, pos{l1 + 1, 0}, "", editOther)
	case l0 > 0:
		d.replace(s, pos{l0 - 1, len(lines[l0-1])}, pos{l1, len(lines[l1])}, "", editOther)
		l0--
	default:
		d.replace(s, pos{}, pos{l1, len(lines[l1])}, "", editOther)
	}
	p := d.clampPos(pos{l0, col})
	d.sel = selection{side: 1, a: p, b: p}
}

// moveLines 把选中的几行（没选是光标那一行）往上（dir -1）/ 下挪一行，选中的字跟着。
func (d *Diff) moveLines(s *Style, dir int) {
	l0, l1 := d.lineSpan()
	lines := d.lines[1]
	if dir < 0 && l0 == 0 || dir > 0 && l1+1 >= len(lines) {
		return
	}
	sel := d.sel
	block := slices.Clone(lines[l0 : l1+1])
	if dir < 0 {
		d.replace(s, pos{l0 - 1, 0}, pos{l1, len(lines[l1])}, strings.Join(append(block, lines[l0-1]), "\n"), editOther)
	} else {
		d.replace(s, pos{l0, 0}, pos{l1 + 1, len(lines[l1+1])}, strings.Join(append([]string{lines[l1+1]}, block...), "\n"), editOther)
	}
	sel.a.line += dir
	sel.b.line += dir
	d.sel = selection{side: 1, a: d.clampPos(sel.a), b: d.clampPos(sel.b)}
}

// duplicateLines 把选中的几行（没选是光标那一行）复制一份放在下面；down 时选中的字跟到下面那份。
func (d *Diff) duplicateLines(s *Style, down bool) {
	l0, l1 := d.lineSpan()
	lines := d.lines[1]
	sel := d.sel
	d.replace(s, pos{l1, len(lines[l1])}, pos{l1, len(lines[l1])}, "\n"+strings.Join(lines[l0:l1+1], "\n"), editOther)
	if down {
		n := l1 - l0 + 1
		sel.a.line += n
		sel.b.line += n
	}
	d.sel = selection{side: 1, a: d.clampPos(sel.a), b: d.clampPos(sel.b)}
}

// joinLines 把光标那一行和下一行（选了几行时是选中的几行）接成一行，和文件标签的编辑器一样：下一行开头的空白
// 换成一个空格（右括号前、左括号后不加），两行都是行注释时去掉下一行的注释记号。光标放在接缝处。
func (d *Diff) joinLines(s *Style) {
	l0, l1 := d.lineSpan()
	lines := d.lines[1]
	l1 = max(l1, l0+1)
	if l1 >= len(lines) {
		return
	}
	comment := ""
	if d.lang != nil {
		comment = d.lang.LineComment
	}
	t := lines[l0]
	at := 0
	for _, next := range lines[l0+1 : l1+1] {
		rest := strings.TrimLeft(next, " \t")
		cur := strings.TrimRight(t, " \t")
		if comment != "" && strings.HasPrefix(strings.TrimLeft(cur, " \t"), comment) && strings.HasPrefix(rest, comment) {
			rest = strings.TrimLeft(rest[len(comment):], " \t")
		}
		sep := " "
		if rest == "" || strings.HasPrefix(rest, ")") || strings.HasPrefix(rest, "]") || strings.HasSuffix(cur, "(") || strings.HasSuffix(cur, "[") {
			sep = ""
		}
		if at == 0 {
			at = len(cur)
		}
		t = cur + sep + rest
	}
	d.replace(s, pos{l0, 0}, pos{l1, len(lines[l1])}, t, editOther)
	p := d.clampPos(pos{l0, at})
	d.sel = selection{side: 1, a: p, b: p}
}

// transpose 把光标两边的字对调（在行尾时对调前面两个），光标往后一个。
func (d *Diff) transpose(s *Style, c pos) {
	t := d.lines[1][c.line]
	if !d.sel.empty() || utf8.RuneCountInString(t) < 2 || c.off == 0 {
		return
	}
	if c.off == len(t) {
		c = d.left(c)
	}
	l := d.left(c)
	r := d.right(c)
	d.replace(s, l, r, t[c.off:r.off]+t[l.off:c.off], editOther)
}

// newline 换行：接着这一行的缩进；前面是开括号（或语言说这行之后要缩进）时多缩一级，光标正夹在一对括号中间时
// 右括号挪到再下一行、和这一行对齐。
func (d *Diff) newline(s *Style) {
	a, b := d.sel.span()
	lines := d.lines[1]
	before, after := lines[a.line][:a.off], lines[b.line][b.off:]
	ind := indentOf(before)
	text := "\n" + ind
	if d.opensIndent(before) {
		u := d.indentUnit(s)
		if d.closesFirst(after) {
			d.replace(s, a, b, "\n"+ind+u+"\n"+ind, editOther)
			p := pos{a.line + 1, len(ind + u)}
			d.sel = selection{side: 1, a: p, b: p}
			return
		}
		text += u
	}
	d.typeText(s, text, editOther)
}

// opensIndent 是光标前面这段字之后下一行该多缩一级：以开括号结尾，或语言的 IncreaseIndent 认。
func (d *Diff) opensIndent(before string) bool {
	t := strings.TrimRight(before, " \t")
	if t == "" {
		return false
	}
	if d.lang != nil && d.lang.IncreaseIndent != nil && d.lang.IncreaseIndent.MatchString(t) {
		return true
	}
	for _, p := range d.brackets() {
		if strings.HasSuffix(t, p.Open) {
			return true
		}
	}
	return false
}

// closesFirst 是光标后面这段字以右括号开头。
func (d *Diff) closesFirst(after string) bool {
	t := strings.TrimLeft(after, " \t")
	for _, p := range d.brackets() {
		if strings.HasPrefix(t, p.Close) {
			return true
		}
	}
	return false
}

var plainBrackets = []editor.BracketPair{{Open: "(", Close: ")"}, {Open: "[", Close: "]"}, {Open: "{", Close: "}"}}

// brackets 是这种语言配对的括号（不认识的语言是 () [] {}）。
func (d *Diff) brackets() []editor.BracketPair {
	if d.lang != nil && len(d.lang.Brackets) > 0 {
		return d.lang.Brackets
	}
	return plainBrackets
}

// autoPairs 是打了左边那个自动补上右边那个的几对（引号也在里面）。
func (d *Diff) autoPairs() []editor.BracketPair {
	if d.lang != nil && len(d.lang.AutoClose) > 0 {
		return d.lang.AutoClose
	}
	return plainBrackets
}

// runeAround 是光标前、后挨着的那个字（没有是 ""）。
func (d *Diff) runeAround(c pos) (prev, next string) {
	t := d.lines[1][c.line]
	if c.off > 0 {
		_, w := utf8.DecodeLastRuneInString(t[:c.off])
		prev = t[c.off-w : c.off]
	}
	if c.off < len(t) {
		_, w := utf8.DecodeRuneInString(t[c.off:])
		next = t[c.off : c.off+w]
	}
	return prev, next
}

// inPair 是光标正夹在一对空的括号 / 引号中间（退格两个一起删）。
func (d *Diff) inPair(c pos) bool {
	prev, next := d.runeAround(c)
	for _, p := range d.autoPairs() {
		if prev == p.Open && next == p.Close {
			return true
		}
	}
	return false
}

// typeChar 打一个字：打的是右括号 / 引号、光标后面正好是它时跳过去；打的是左括号 / 引号时补上右边那个
// （选了字时把选中的字括起来）；别的照常打。
func (d *Diff) typeChar(s *Style, text string) {
	if utf8.RuneCountInString(text) != 1 {
		d.typeText(s, text, editTyping)
		return
	}
	a, b := d.sel.span()
	prev, next := d.runeAround(b)
	pairs := d.autoPairs()
	if d.sel.empty() && next == text {
		for _, p := range pairs {
			if p.Close == text {
				d.moveTo(d.right(b), false)
				return
			}
		}
	}
	for _, p := range pairs {
		if p.Open != text {
			continue
		}
		if !d.sel.empty() { // 括起选中的字，括完还选着里面的
			inner := d.Selected()
			d.replace(s, a, b, p.Open+inner+p.Close, editOther)
			e := after(pos{a.line, a.off + len(p.Open)}, inner)
			d.sel = selection{side: 1, a: pos{a.line, a.off + len(p.Open)}, b: e}
			return
		}
		quote := p.Open == p.Close
		if next != "" && !isSpace(next) && !d.isCloser(next) {
			break
		}
		if quote && prev != "" && (wordRune(prev) || prev == p.Open) {
			break
		}
		if len(p.NotIn) > 0 && slices.Contains(p.NotIn, d.tokenAt(b)) { // 字符串、注释里打引号不补
			break
		}
		d.replace(s, b, b, p.Open+p.Close, editTyping)
		c := pos{b.line, b.off + len(p.Open)}
		d.sel = selection{side: 1, a: c, b: c}
		return
	}
	d.typeText(s, text, editTyping)
	if d.isCloser(text) { // 一行里头一个打的是右括号：和配对的左括号那一行一样缩进
		d.alignCloser(s)
	}
}

// tokenAt 是新的一边 p 前面那个字在哪种词里（字符串、注释……）；没有语法高亮是 TokenText。
func (d *Diff) tokenAt(p pos) editor.TokenKind {
	if d.lang == nil || d.lang.Tokenizer == nil || p.off == 0 {
		return editor.TokenText
	}
	src := clean(d.lines[1][p.line])
	toks, _ := d.lang.Tokenizer.Tokenize(src, d.stateAt(1, p.line), nil)
	for _, t := range toks { // 词一个挨一个，只记着到哪
		if p.off <= t.End {
			return t.Kind
		}
	}
	return editor.TokenText
}

func (d *Diff) isCloser(t string) bool {
	for _, p := range d.autoPairs() {
		if p.Close == t && p.Open != p.Close {
			return true
		}
	}
	return false
}

func wordRune(t string) bool {
	r, _ := utf8.DecodeRuneInString(t)
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// alignCloser 在刚打了右括号、它前面只有空白时，把这一行的缩进改成配对的左括号那一行的。
func (d *Diff) alignCloser(s *Style) {
	c := d.sel.b
	t := d.lines[1][c.line]
	if c.off == 0 || strings.TrimSpace(t[:c.off-1]) != "" {
		return
	}
	at := d.left(c)
	_, m, ok := d.matchBracket(1, at)
	if !ok || m.line == c.line {
		return
	}
	want := indentOf(d.lines[1][m.line])
	have := indentOf(t)
	if want == have {
		return
	}
	d.replace(s, pos{c.line, 0}, pos{c.line, len(have)}, want, editOther)
	p := pos{c.line, c.off - len(have) + len(want)}
	d.sel = selection{side: 1, a: p, b: p}
}

// toggleComment 注释 / 取消注释选中的几行（没选是光标那一行）：都注释了就都去掉，不然在最浅的缩进处都加上。
// 语言没有行注释时用块注释；都没有（纯文本）不接。
func (d *Diff) toggleComment(s *Style) bool {
	if d.lang == nil {
		return false
	}
	tok := d.lang.LineComment
	if tok == "" {
		return d.toggleBlockComment(s)
	}
	l0, l1 := d.lineSpan()
	lines := d.lines[1]
	least, all, any := -1, true, false
	for _, t := range lines[l0 : l1+1] {
		if strings.TrimSpace(t) == "" {
			continue
		}
		any = true
		n := len(indentOf(t))
		if least < 0 || n < least {
			least = n
		}
		if !strings.HasPrefix(t[n:], tok) {
			all = false
		}
	}
	if !any {
		return true
	}
	out := make([]string, 0, l1-l0+1)
	at := make([]int, 0, l1-l0+1)
	for _, t := range lines[l0 : l1+1] {
		n := len(indentOf(t))
		switch {
		case strings.TrimSpace(t) == "":
			at = append(at, len(t))
		case all:
			rest := strings.TrimPrefix(t[n+len(tok):], " ")
			t = t[:n] + rest
			at = append(at, n)
		default:
			t = t[:least] + tok + " " + t[least:]
			at = append(at, least)
		}
		out = append(out, t)
	}
	d.rewriteLines(s, l0, l1, out, at)
	return true
}

// toggleBlockComment 用块注释括起 / 去掉选中的几行（没选是光标那一行）。语言没有块注释不接。
func (d *Diff) toggleBlockComment(s *Style) bool {
	if d.lang == nil || d.lang.BlockComment[0] == "" {
		return false
	}
	open, close := d.lang.BlockComment[0], d.lang.BlockComment[1]
	l0, l1 := d.lineSpan()
	out := slices.Clone(d.lines[1][l0 : l1+1])
	first, end := out[0], out[len(out)-1]
	n := len(indentOf(first))
	sel := d.sel
	shift := len(open) + 1 // 第一行缩进后面的字挪了几个字节
	if strings.HasPrefix(first[n:], open) && strings.HasSuffix(strings.TrimRight(end, " \t"), close) {
		rest := first[n+len(open):]
		out[0] = first[:n] + strings.TrimPrefix(rest, " ")
		shift = -(len(first) - n - len(strings.TrimPrefix(rest, " ")))
		e := strings.TrimRight(out[len(out)-1], " \t")
		out[len(out)-1] = strings.TrimSuffix(strings.TrimSuffix(e, close), " ")
	} else {
		out[0] = first[:n] + open + " " + first[n:]
		out[len(out)-1] += " " + close
	}
	d.rewriteLines(s, l0, l1, out, nil)
	fix := func(p pos) pos {
		if p.line == l0 && p.off > n {
			p.off = max(n, p.off+shift)
		}
		return d.clampPos(p)
	}
	d.sel = selection{side: 1, a: fix(sel.a), b: fix(sel.b)}
	return true
}

// trimTrailing 去掉每一行行尾的空白（一步撤销）。
func (d *Diff) trimTrailing(s *Style) {
	lines := d.lines[1]
	out := make([]string, len(lines))
	changed := false
	for i, t := range lines {
		out[i] = strings.TrimRight(t, " \t")
		changed = changed || out[i] != t
	}
	if !changed {
		return
	}
	c := d.sel.b
	d.rewriteLines(s, 0, len(lines)-1, out, nil)
	p := d.clampPos(c)
	d.sel = selection{side: 1, a: p, b: p}
}

// nextOccurrence 是 from 往后（到尾绕回开头）下一处 t（新的一边）。
func (d *Diff) nextOccurrence(t string, from pos) (selection, bool) {
	if t == "" || strings.Contains(t, "\n") {
		return selection{}, false
	}
	lines := d.lines[1]
	for k := 0; k <= len(lines); k++ {
		l := (from.line + k) % len(lines)
		start := 0
		if k == 0 {
			start = from.off
		}
		if start > len(lines[l]) {
			continue
		}
		if i := strings.Index(lines[l][start:], t); i >= 0 {
			o := start + i
			return selection{side: 1, a: pos{l, o}, b: pos{l, o + len(t)}}, true
		}
	}
	return selection{}, false
}

// bracketScan 是找配对括号最多看多少行。
const bracketScan = 5000

// matchBracket 是 side 那边 p 挨着的括号（先看后面那个字，再看前面那个）和配对的那个括号（字节位置）。
// 只认单个字的括号；字符串、注释里的括号也算（不看高亮），找不到、太远返回 false。
func (d *Diff) matchBracket(side int, p pos) (at, match pos, ok bool) {
	lines := d.lines[side]
	if p.line < 0 || p.line >= len(lines) {
		return
	}
	t := lines[p.line]
	try := func(off int) bool {
		if off < 0 || off >= len(t) {
			return false
		}
		ch := t[off : off+1]
		for _, bp := range d.brackets() {
			if len(bp.Open) != 1 || len(bp.Close) != 1 || bp.Open == bp.Close {
				continue
			}
			switch ch {
			case bp.Open:
				if m, found := d.scanBracket(side, pos{p.line, off}, bp.Open[0], bp.Close[0], 1); found {
					at, match, ok = pos{p.line, off}, m, true
					return true
				}
			case bp.Close:
				if m, found := d.scanBracket(side, pos{p.line, off}, bp.Close[0], bp.Open[0], -1); found {
					at, match, ok = pos{p.line, off}, m, true
					return true
				}
			}
		}
		return false
	}
	if !try(p.off) {
		try(p.off - 1)
	}
	return
}

// scanBracket 从 from 那个括号（self）往 dir 找配对的 other（中间同样的括号成对跳过）。
func (d *Diff) scanBracket(side int, from pos, self, other byte, dir int) (pos, bool) {
	lines := d.lines[side]
	depth := 0
	for l, n := from.line, 0; l >= 0 && l < len(lines) && n < bracketScan; l, n = l+dir, n+1 {
		t := clean(lines[l])
		i := len(t) - 1
		if dir > 0 {
			i = 0
		}
		if l == from.line {
			i = from.off
		}
		for ; i >= 0 && i < len(t); i += dir {
			switch t[i] {
			case self:
				depth++
			case other:
				depth--
				if depth == 0 {
					return pos{l, i}, true
				}
			}
		}
	}
	return pos{}, false
}
