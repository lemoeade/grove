package ged

import (
	"strings"
	"testing"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// editState 拿一段字造一个就地编辑着的 Diff：“|” 是光标，“^” 是选中的字的起点（没有就是没选）。
func editState(t *testing.T, text, file string) (*Diff, *Style) {
	t.Helper()
	var a, b pos
	hasA := false
	var lines []string
	for i, l := range strings.Split(text, "\n") {
		for {
			ka, kb := strings.IndexByte(l, '^'), strings.IndexByte(l, '|')
			switch {
			case ka >= 0 && (kb < 0 || ka < kb):
				a, hasA = pos{i, ka}, true
				l = l[:ka] + l[ka+1:]
				continue
			case kb >= 0:
				b = pos{i, kb}
				l = l[:kb] + l[kb+1:]
				continue
			}
			break
		}
		lines = append(lines, l)
	}
	if !hasA {
		a = b
	}
	buf := editor.NewBuffer(strings.Join(lines, "\n"))
	s := &Style{Buf: buf}
	d := NewDiff()
	if file != "" {
		d.lang = editor.LanguageFor(file)
	}
	d.update(lines, linesOf(buf))
	d.editable = true
	d.sel = selection{side: 1, a: a, b: b}
	return d, s
}

// editText 是 Diff 新的一边现在的字，光标、选中的起点照 editState 的写法标回去。
func editText(d *Diff) string {
	lines := append([]string(nil), d.lines[1]...)
	mark := func(p pos, m string) {
		lines[p.line] = lines[p.line][:p.off] + m + lines[p.line][p.off:]
	}
	a, b := d.sel.a, d.sel.b
	if a == b {
		mark(b, "|")
	} else if b.less(a) {
		mark(a, "^")
		mark(b, "|")
	} else {
		mark(b, "|")
		mark(a, "^")
	}
	return strings.Join(lines, "\n")
}

func TestEditCommands(t *testing.T) {
	g := geom{h: 400, lh: 20}
	for _, c := range []struct {
		name, file string
		cmd        editor.Command
		in, want   string
	}{
		{"⌥↓ 挪行", "a.go", editor.MoveLinesDown, "a|\nb\nc", "b\na|\nc"},
		{"⌥↑ 挪几行", "a.go", editor.MoveLinesUp, "x\n^a\nb|\nc", "^a\nb|\nx\nc"},
		{"⌥↑ 到顶不动", "a.go", editor.MoveLinesUp, "a|\nb", "a|\nb"},
		{"选到下一行行头不算那行", "a.go", editor.MoveLinesDown, "^a\n|b\nc", "b\n^a\n|c"},
		{"⇧⌥↓ 复制行", "a.go", editor.DuplicateLinesDown, "a|b\nc", "ab\na|b\nc"},
		{"⇧⌥↑ 复制行光标不动", "a.go", editor.DuplicateLinesUp, "a|b\nc", "a|b\nab\nc"},
		{"⇧⌘K 删行", "a.go", editor.DeleteLine, "a\nb|cd\ne", "a\ne|"},
		{"⇧⌘K 删最后一行", "a.go", editor.DeleteLine, "a\nb|", "a|"},
		{"⌘↩ 下面开一行", "a.go", editor.NewlineBelow, "\tfo|o\nx", "\tfoo\n\t|\nx"},
		{"⇧⌘↩ 上面开一行", "a.go", editor.NewlineAbove, "x\n\tfo|o", "x\n\t|\n\tfoo"},
		{"⌘] 缩进", "a.go", editor.Indent, "^a\nb|", "\t^a\n\tb|"},
		{"⌘[ 减缩进", "a.go", editor.Outdent, "\ta|\n  b", "a|\n  b"},
		{"空格缩进的文件按空格", "a.py", editor.Indent, "def f():\n  re|turn 1", "def f():\n    re|turn 1"},
		{"Tab 插到下一格", "a.py", editor.Tab, "x = 1\n  y|", "x = 1\n  y |"},
		{"Tab 选了几行是缩进", "a.go", editor.Tab, "^a\nb|", "\t^a\n\tb|"},
		{"⇧Tab 减缩进", "a.go", editor.Backtab, "\t\tx|", "\tx|"},
		{"⌘/ 注释", "a.go", editor.ToggleComment, "\t^a\n\t\tb|", "\t// ^a\n\t// \tb|"},
		{"⌘/ 取消注释", "a.go", editor.ToggleComment, "\t// a|\n\t// b", "\ta|\n\t// b"},
		{"⌘/ CSS 用块注释", "a.css", editor.ToggleComment, "a { co|lor: red }", "/* a { co|lor: red } */"},
		{"⌘/ 纯文本不接", "", editor.ToggleComment, "a|", "a|"},
		{"⌃J 接行", "a.go", editor.JoinLines, "fo|o(\n\tx)", "foo(|x)"},
		{"⌃J 合并行注释", "a.go", editor.JoinLines, "// a|\n// b", "// a| b"},
		{"⌃J 两边都有字加空格", "a.go", editor.JoinLines, "a|\n   b", "a| b"},
		{"⌃T 对调", "a.go", editor.Transpose, "a|bc", "ba|c"},
		{"⌃T 行尾对调前两个", "a.go", editor.Transpose, "ab|", "ba|"},
		{"⌃K 剪到行尾", "a.go", editor.CutToLineEnd, "a|bc\nd", "a|\nd"},
		{"⌃K 行尾剪换行", "a.go", editor.CutToLineEnd, "abc|\nd", "abc|d"},
		{"⌘⌫ 删到行头", "a.go", editor.DeleteToLineStart, "abc|d", "|d"},
		{"⌥⌫ 删一个词", "a.go", editor.DeleteWordLeft, "foo bar|", "foo |"},
		{"退格删一对空括号", "a.go", editor.Backspace, "f(|)", "f|"},
		{"↩ 括号里拆成三行", "a.go", editor.Newline, "\tif x {|}", "\tif x {\n\t\t|\n\t}"},
		{"↩ 开括号后多缩一级", "a.go", editor.Newline, "func f() {|", "func f() {\n\t|"},
		{"↩ Python 冒号后缩进", "a.py", editor.Newline, "def f():|", "def f():\n    |"},
		{"↩ 接着缩进", "a.go", editor.Newline, "\tx := 1|", "\tx := 1\n\t|"},
		{"⌘L 选行", "a.go", editor.SelectLine, "a\nb|c\nd", "a\n^bc\n|d"},
		{"⌘L 再按多选一行", "a.go", editor.SelectLine, "a\n^bc\n|d\ne", "a\n^bc\nd\n|e"},
		{"⌘D 选光标处的词", "a.go", editor.SelectNext, "foo ba|r bar", "foo ^bar| bar"},
		{"⌘D 选下一处", "a.go", editor.SelectNext, "foo ^bar| bar", "foo bar ^bar|"},
		{"⌘D 到尾绕回", "a.go", editor.SelectNext, "bar\nfoo ^bar|", "^bar|\nfoo bar"},
		{"⇧⌘\\ 到配对括号", "a.go", editor.MoveToBracket, "f|(a(b))", "f(a(b)|)"},
		{"⌃↓ 下一段", "a.go", editor.MoveParagraphDown, "a|\nb\n\nc", "a\nb\n|\nc"},
		{"去掉行尾空白", "a.go", editor.TrimTrailingWhitespace, "a  \nb|\t", "a\nb|"},
		{"⌘← 到缩进后", "a.go", editor.MoveLineStart, "\tab|c", "\t|abc"},
		{"⇧⌘→ 选到行尾", "a.go", editor.SelectLineEnd, "a|bc", "a^bc|"},
	} {
		d, s := editState(t, c.in, c.file)
		d.runEdit(c.cmd, d.sel.b, g, s)
		if got := editText(d); got != c.want {
			t.Errorf("%s：\n得到 %q\n应为 %q", c.name, got, c.want)
		}
	}
}

// 打括号、引号自动补上另一半，打右边那个时跳过去，选了字时括起来。
func TestAutoClose(t *testing.T) {
	for _, c := range []struct {
		name, file, typed string
		in, want          string
	}{
		{"补右括号", "a.go", "(", "f|", "f(|)"},
		{"跳过右括号", "a.go", ")", "f(|)", "f()|"},
		{"字前面不补", "a.go", "(", "|x", "(|x"},
		{"补引号", "a.go", `"`, "x := |", `x := "|"`},
		{"词后面的引号不补", "a.go", "'", "don|", "don'|"},
		{"字符串里的引号不补", "a.go", "'", `s := "a |"`, `s := "a '|"`},
		{"括起选中的字", "a.go", "[", "^ab|", "[^ab|]"},
		{"右括号对齐到左括号那一行", "a.go", "}", "\tif x {\n\t\ty()\n\t\t|", "\tif x {\n\t\ty()\n\t}|"},
		{"纯文本也补括号", "", "{", "|", "{|}"},
	} {
		d, s := editState(t, c.in, c.file)
		d.typeChar(s, c.typed)
		if got := editText(d); got != c.want {
			t.Errorf("%s：\n得到 %q\n应为 %q", c.name, got, c.want)
		}
	}
}

// 一个命令改了好几处（挪行、注释）也只是一步撤销。
func TestEditCommandUndo(t *testing.T) {
	d, s := editState(t, "a|\nb\nc", "a.go")
	g := geom{h: 400, lh: 20}
	d.runEdit(editor.MoveLinesDown, d.sel.b, g, s)
	d.runEdit(editor.ToggleComment, d.sel.b, g, s)
	if got := editText(d); got != "b\n// a|\nc" {
		t.Fatalf("改完：%q", got)
	}
	d.undo(s, false)
	if got := strings.Join(d.lines[1], "\n"); got != "b\na\nc" {
		t.Fatalf("撤一步：%q", got)
	}
	d.undo(s, false)
	if got := strings.Join(d.lines[1], "\n"); got != "a\nb\nc" {
		t.Fatalf("撤两步：%q", got)
	}
}

// 键位表：改过的键先查，绑到 NoCommand 的键算拿走了，没改的按 mygo 的默认键。
func TestCommandLookup(t *testing.T) {
	s := &Style{}
	d := NewDiff()
	command := d.command
	if got := command(wui.InputEvent{Key: wui.KeyDown, Mods: wui.Alt}, s); got != editor.MoveLinesDown {
		t.Errorf("⌥↓ = %v，应为挪行", got)
	}
	if got := command(wui.InputEvent{Key: wui.KeySlash, Mods: wui.Super}, s); got != editor.ToggleComment {
		t.Errorf("⌘/ = %v，应为注释", got)
	}
	s.Bindings = []editor.Binding{
		{Mods: wui.Alt, Key: wui.KeyDown},                                       // 拿走
		{Mods: wui.Ctrl | wui.Shift, Key: wui.KeyD, Command: editor.DeleteLine}, // 换了键
	}
	if got := command(wui.InputEvent{Key: wui.KeyDown, Mods: wui.Alt}, s); got != editor.NoCommand {
		t.Errorf("拿走的键还是 %v", got)
	}
	if got := command(wui.InputEvent{Key: wui.KeyD, Mods: wui.Ctrl | wui.Shift}, s); got != editor.DeleteLine {
		t.Errorf("改过的键 = %v", got)
	}
}

// 配对括号：光标挨着的括号（先看后面再看前面），中间成对的跳过。
func TestMatchBracket(t *testing.T) {
	d, _ := editState(t, "f(a, g(b)) {\n\tx[1]\n}|", "a.go")
	for _, c := range []struct {
		p, at, m pos
		ok       bool
	}{
		{pos{0, 1}, pos{0, 1}, pos{0, 9}, true},
		{pos{0, 10}, pos{0, 9}, pos{0, 1}, true},
		{pos{0, 11}, pos{0, 11}, pos{2, 0}, true},
		{pos{2, 1}, pos{2, 0}, pos{0, 11}, true},
		{pos{1, 1}, pos{}, pos{}, false},
	} {
		at, m, ok := d.matchBracket(1, c.p)
		if ok != c.ok || ok && (at != c.at || m != c.m) {
			t.Errorf("%v：得到 %v %v %v，应为 %v %v %v", c.p, at, m, ok, c.at, c.m, c.ok)
		}
	}
}

// 查找条上的替换：只换新的一边的；当前那处在旧的一边时先跳到新的一边的；全部替换一步撤销。
func TestReplace(t *testing.T) {
	d, s := editState(t, "foo := Foo|\nbar(foo)", "a.go")
	d.lines[0] = []string{"foo := 1", "bar(foo)"} // 旧的一边第一行不一样：那行里的 foo 也算找到的
	d.align()
	d.buf = s.Buf
	d.editable = true
	d.OpenReplace()
	if !d.Find.Replace || !d.CanReplace() {
		t.Fatal("就地编辑时 OpenReplace 该开替换")
	}
	d.Find.With = "x"
	d.SetFind("foo")
	if _, total, _ := d.FindStatus(); total != 4 { // 旧的 foo、新的 foo、Foo、第二行的 foo
		t.Fatalf("找到 %d 处", total)
	}
	d.findCur = 0 // 旧的一边那处
	if d.ReplaceOne() {
		t.Fatal("当前那处在旧的一边：该先跳过去，不换")
	}
	if m := d.matches[d.findCur]; m.side != 1 || m.line != 0 || m.start != 0 {
		t.Fatalf("跳到了 %+v", m)
	}
	if !d.ReplaceOne() {
		t.Fatal("没换")
	}
	if got := strings.Join(d.lines[1], "\n"); got != "x := Foo\nbar(foo)" {
		t.Fatalf("换一处：%q", got)
	}
	if m := d.matches[d.findCur]; m.side != 1 || m.line != 0 || m.start != len("x := ") {
		t.Fatalf("换完该到下一处：%+v", m)
	}
	if n := d.ReplaceAll(); n != 2 {
		t.Fatalf("全部替换换了 %d 处", n)
	}
	if got := strings.Join(d.lines[1], "\n"); got != "x := x\nbar(x)" {
		t.Fatalf("全部替换：%q", got)
	}
	d.undo(s, false)
	if got := strings.Join(d.lines[1], "\n"); got != "x := Foo\nbar(foo)" {
		t.Fatalf("全部替换该一步撤销：%q", got)
	}
	d.OpenFind()
	if d.Find.Replace {
		t.Error("⌘F 收起替换")
	}
}

// 和弦：单栏编辑器里 ⌘K 先记着，⌘L 一起认成折叠 / 展开；Diff 里 ⌘K 不认。
func TestChord(t *testing.T) {
	s := &Style{}
	d := NewEditor()
	k := wui.InputEvent{Key: wui.KeyK, Mods: wui.Super}
	if got := d.command(k, s); got != chordPending {
		t.Fatalf("⌘K = %v，应为等第二个键", got)
	}
	if got := d.command(wui.InputEvent{Key: wui.KeyL, Mods: wui.Super}, s); got != editor.ToggleFold {
		t.Errorf("⌘K ⌘L = %v，应为折叠 / 展开", got)
	}
	d.command(k, s)
	if got := d.command(wui.InputEvent{Key: wui.KeyQ}, s); got != chordDropped {
		t.Errorf("⌘K Q = %v，应为吃掉", got)
	}
	if got := d.command(wui.InputEvent{Key: wui.KeyDown, Mods: wui.Alt}, s); got != editor.MoveLinesDown {
		t.Errorf("和弦以后 ⌥↓ = %v", got)
	}
	if got := NewDiff().command(k, s); got == chordPending {
		t.Errorf("Diff 里 ⌘K 不该等第二个键")
	}
}
