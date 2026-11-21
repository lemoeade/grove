// Package ged 是 Grove 自己的编辑器部件，只用 mygo 公开的接口（ui.ShapeRichText / Painter / HandleInput），
// 不走 mygo 的 ui/editor：那个编辑器的底色、行间的块、装订线都改不了（见 docs/mygo-pitfalls.md）。
//
// 现在只有只读的并排 Diff（DiffView）；这个文件是它的模型：两边按行对齐、改了的行里改了哪几个字、
// 没改的长段折起来。不碰界面，单独测。
package ged

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// RowKind 是并排 Diff 里一行的种类。
type RowKind uint8

const (
	Same    RowKind = iota // 两边一样
	Changed                // 两边都有、内容不同（一处改动里删几行加几行，前面配成对的那几行）
	Deleted                // 只有旧的一边有，新的一边留空
	Added                  // 只有新的一边有，旧的一边留空
	Fold                   // 折起来的一段没改的行
)

// Row 是并排 Diff 的一行。Old / New 是两边的行号（从 0 起），那一边没有时是 -1；
// Fold 行的 Old / New 是折起来的第一行，N 是折了几行。
type Row struct {
	Kind     RowKind
	Old, New int
	N        int
}

// Span 是一行里的一段字节 [Start, End)。
type Span struct{ Start, End int }

// diffMax 是最多算到多少处不同；再多（几乎整个都换了）就把中间整段当成一处删加。回溯要留 diffMax² 个数，1000 是 8 MB。
const diffMax = 1000

// Align 把两边的行对齐成并排的行：一样的一行对一行；一处改动删 d 行、加 a 行时，前 min(d, a) 行配成对（Changed），
// 多出来的那几行对面留空（Deleted / Added），和 VS Code、GitHub 的并排视图一样。
func Align(old, new []string) []Row {
	a, b := intern(old, new)
	pairs := matches(a, b)
	rows := make([]Row, 0, max(len(old), len(new)))
	i, j := 0, 0
	gap := func(ti, tj int) { // [i, ti) 删、[j, tj) 加
		for i < ti || j < tj {
			switch {
			case i < ti && j < tj:
				rows = append(rows, Row{Kind: Changed, Old: i, New: j})
				i, j = i+1, j+1
			case i < ti:
				rows = append(rows, Row{Kind: Deleted, Old: i, New: -1})
				i++
			default:
				rows = append(rows, Row{Kind: Added, Old: -1, New: j})
				j++
			}
		}
	}
	for _, p := range pairs {
		gap(p[0], p[1])
		rows = append(rows, Row{Kind: Same, Old: i, New: j})
		i, j = i+1, j+1
	}
	gap(len(a), len(b))
	return rows
}

// Folded 把 rows 里离改动超过 context 行的没改的段折成一行 Fold（段要比折起来那一行多才折）；
// open 里的 Fold（按折起来的第一行的 Old 认）不折。没有改动时整个文件不折。
func Folded(rows []Row, context int, open map[int]bool) []Row {
	context = max(context, 0)
	changed := false
	for _, r := range rows {
		if r.Kind != Same {
			changed = true
			break
		}
	}
	if !changed {
		return rows
	}
	out := make([]Row, 0, len(rows))
	for i := 0; i < len(rows); {
		if rows[i].Kind != Same {
			out = append(out, rows[i])
			i++
			continue
		}
		e := i
		for e < len(rows) && rows[e].Kind == Same {
			e++
		}
		lo, hi := i, e // 折 [lo, hi)
		if i > 0 {
			lo += context
		}
		if e < len(rows) {
			hi -= context
		}
		if hi-lo > 1 && !open[rows[lo].Old] {
			out = append(out, rows[i:lo]...)
			out = append(out, Row{Kind: Fold, Old: rows[lo].Old, New: rows[lo].New, N: hi - lo})
			out = append(out, rows[hi:e]...)
		} else {
			out = append(out, rows[i:e]...)
		}
		i = e
	}
	return out
}

// Words 是改了的一对行里两边各自改了的字（按词比：字母数字下划线连成一个词，空白连成一段，别的一个字符一个），
// 挨着的合成一段。两行几乎全不一样时（一样的不到两成）两边都整行算改了：一个个零碎的字看着更乱。
func Words(old, new string) (o, n []Span) {
	ta, tb := tokens(old), tokens(new)
	sa, sb := make([]string, len(ta)), make([]string, len(tb))
	for i, t := range ta {
		sa[i] = old[t.Start:t.End]
	}
	for i, t := range tb {
		sb[i] = new[t.Start:t.End]
	}
	a, b := intern(sa, sb)
	pairs := matches(a, b)
	same := 0
	for _, p := range pairs {
		same += ta[p[0]].End - ta[p[0]].Start
	}
	if len(old)+len(new) > 0 && same*2*5 < len(old)+len(new) {
		return whole(old), whole(new)
	}
	i, j := 0, 0
	add := func(dst []Span, ts []Span, from, to int) []Span {
		if from >= to {
			return dst
		}
		s := Span{ts[from].Start, ts[to-1].End}
		if k := len(dst) - 1; k >= 0 && dst[k].End == s.Start {
			dst[k].End = s.End
			return dst
		}
		return append(dst, s)
	}
	for _, p := range pairs {
		o, n = add(o, ta, i, p[0]), add(n, tb, j, p[1])
		i, j = p[0]+1, p[1]+1
	}
	o, n = add(o, ta, i, len(ta)), add(n, tb, j, len(tb))
	return o, n
}

// whole 是整行算改了时涂的那段：去掉行首的缩进和行尾的空白（VS Code 也不涂它们）。
func whole(s string) []Span {
	a := len(s) - len(strings.TrimLeft(s, " \t"))
	b := len(strings.TrimRight(s, " \t\r"))
	if b <= a {
		return nil
	}
	return []Span{{a, b}}
}

// tokens 把一行切成词：字母数字下划线连成一个，空白连成一个，别的每个字符一个。
func tokens(s string) []Span {
	var out []Span
	class := func(r rune) int {
		switch {
		case r == '_' || unicode.IsLetter(r) && r < 0x2E80 || unicode.IsDigit(r):
			return 1
		case unicode.IsSpace(r):
			return 2
		}
		return 0 // 标点和中日韩的字：一个字一个词
	}
	for i := 0; i < len(s); {
		r, w := utf8.DecodeRuneInString(s[i:])
		c, e := class(r), i+w
		if c != 0 {
			for e < len(s) {
				r2, w2 := utf8.DecodeRuneInString(s[e:])
				if class(r2) != c {
					break
				}
				e += w2
			}
		}
		out = append(out, Span{i, e})
		i = e
	}
	return out
}

// intern 把两边的字符串换成整数（一样的一个数），比起来快。
func intern(a, b []string) ([]int, []int) {
	ids := make(map[string]int, len(a)+len(b))
	conv := func(ss []string) []int {
		out := make([]int, len(ss))
		for i, s := range ss {
			id, ok := ids[s]
			if !ok {
				id = len(ids)
				ids[s] = id
			}
			out[i] = id
		}
		return out
	}
	return conv(a), conv(b)
}

// matches 是 a、b 里配上的位置（按顺序），先去掉头尾一样的，中间用 Myers；不同超过 diffMax 时中间一个都不配。
func matches(a, b []int) [][2]int {
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	out := make([][2]int, 0, pre+suf)
	for i := range pre {
		out = append(out, [2]int{i, i})
	}
	for _, p := range myers(a[pre:len(a)-suf], b[pre:len(b)-suf]) {
		out = append(out, [2]int{p[0] + pre, p[1] + pre})
	}
	for k := suf; k > 0; k-- {
		out = append(out, [2]int{len(a) - k, len(b) - k})
	}
	return out
}

// myers 是 a、b 的最长公共子序列（Myers 的 O(ND)），配上的位置按顺序；超过 diffMax 步返回 nil。
func myers(a, b []int) [][2]int {
	n, m := len(a), len(b)
	if n == 0 || m == 0 {
		return nil
	}
	limit := min(n+m, diffMax)
	off := limit + 1
	v := make([]int, 2*limit+2)
	var trace [][]int // trace[d] 是走第 d 步之前 k 在 [-d, d] 上的 v
	for d := 0; d <= limit; d++ {
		trace = append(trace, append([]int(nil), v[off-d:off+d+1]...))
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || k != d && v[off+k-1] < v[off+k+1] {
				x = v[off+k+1]
			} else {
				x = v[off+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x, y = x+1, y+1
			}
			v[off+k] = x
			if x >= n && y >= m {
				return backtrack(trace, a, b)
			}
		}
	}
	return nil
}

func backtrack(trace [][]int, a, b []int) [][2]int {
	x, y := len(a), len(b)
	var out [][2]int
	for d := len(trace) - 1; d > 0; d-- {
		v := trace[d] // 下标 k+d
		k := x - y
		var pk int
		if k == -d || k != d && v[k-1+d] < v[k+1+d] {
			pk = k + 1
		} else {
			pk = k - 1
		}
		px := v[pk+d]
		py := px - pk
		for x > px && y > py {
			x, y = x-1, y-1
			out = append(out, [2]int{x, y})
		}
		x, y = px, py
	}
	for x > 0 && y > 0 {
		x, y = x-1, y-1
		out = append(out, [2]int{x, y})
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// Hunk 是一处改动：对齐后的行里连着的不是 Same 的那几行，Row0、Row1 是它们在 Align 结果里的 [Row0, Row1)。
// OldStart / OldEnd、NewStart / NewEnd 是两边的行 [Start, End)（从 0 起，和 editor.DiffHunk 一样）；
// 一边没有行时 Start = End，是插在那一边的哪一行前面。
type Hunk struct {
	Row0, Row1                         int
	OldStart, OldEnd, NewStart, NewEnd int
}

// Hunks 是 Align 结果里的一处处改动，按顺序。
func Hunks(aligned []Row) []Hunk {
	var out []Hunk
	oi, ni := 0, 0 // 下一行旧的、新的是第几行
	for i := 0; i < len(aligned); {
		r := aligned[i]
		if r.Kind == Same || r.Kind == Fold {
			oi, ni = oi+1, ni+1
			if r.Kind == Fold {
				oi, ni = r.Old+r.N, r.New+r.N
			}
			i++
			continue
		}
		h := Hunk{Row0: i, OldStart: oi, NewStart: ni}
		for i < len(aligned) && aligned[i].Kind != Same && aligned[i].Kind != Fold {
			if aligned[i].Old >= 0 {
				oi++
			}
			if aligned[i].New >= 0 {
				ni++
			}
			i++
		}
		h.Row1, h.OldEnd, h.NewEnd = i, oi, ni
		out = append(out, h)
	}
	return out
}
