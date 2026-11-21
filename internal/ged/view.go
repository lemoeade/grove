package ged

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// Style 是 View 的样子，由界面那边按主题给（ged 不认 Grove 的主题）。
type Style struct {
	Code       *editor.Theme // 代码颜色、行号色、底色
	Font       wui.Font      // 等宽字体（Family、Size）
	LineHeight float32       // 行高是字号的几倍，0 为 1.5

	AddedLine, DeletedLine wui.Color // 改了的整行的底
	AddedWord, DeletedWord wui.Color // 行里改了的字的底，叠在整行的底上
	AddedMark, DeletedMark wui.Color // 改了的行的行号和 + −
	Filler                 wui.Color // 对面多出的行在这边留的空：斜线
	FoldBack, FoldText     wui.Color // 折起来的一段
	FoldHover              wui.Color
	Divider                wui.Color // 两边中间的线
	Ruler                  wui.Color // 右边概览条的底

	UIFont    wui.Font           // 折叠条上的字
	FoldLabel func(n int) string // 折叠条上写什么（“12 行未修改”）
	// FoldScope 是折叠条上数字后面写的、下面那处改动在哪（VS Code 那样写所在的函数那一行）：old 是这段之后第一处改动
	// 在旧的一边的第几行（从 0 起；加的行算插在它后面那一行）。返回种类的小字（可空）和那一行的字，text 为空不写；nil 不写。
	FoldScope func(old int) (mark, text string)
	ScopeText wui.Color // FoldScope 那行字的颜色（0 时用 FoldText）
	Label     string    // 无障碍名
	Tab       int       // 制表符几格，0 为 4

	// HunkBar 往指针下（或刚跳到的）那处改动右上角的那排按钮里放东西，nil 不放；i 是第几处改动。
	// 按钮的无障碍名、悬停提示由界面那边给。
	HunkBar func(c *wui.Context, bar wui.Element, i int, h Hunk)
	// Copy 接 ⌘C（编辑菜单的“复制”）复制选中的字；nil 不接。
	Copy func(text string)
	// Keys 在视图有键盘焦点时先拿到按下的键（界面那边接 J / K 跳改动这类），接了返回 true；没接的视图自己接
	// （↑ ↓ 翻页 Home End 滚动、Esc 取消选择）。
	Keys func(ev wui.InputEvent) bool
	// Lines / SetLines 是点行号选的行（行级暂存用）：哪边（0 旧、1 新）、第 start 到第 end 行（两头都算，从 0 起）。
	// 选中的行存在调用方那里（和 mygo 的 Diff 共用一份）。都给了才能选；没给时点行号和点正文一样是选字。
	Lines    func() (side, start, end int, ok bool)
	SetLines func(side, start, end int, ok bool)
	LineSel  wui.Color // 选中的行的底
	LineMark wui.Color // 选中的行左边的竖条
	// Buf 是就地编辑时新的一边的缓冲区（nil 只看）：新的一边能放光标、打字、删、粘贴、撤销，改的直接是它（见 edit.go）。
	// 给了 Buf 时新的一边的内容也要从它来。
	Buf *editor.Buffer
	// Paste 接编辑菜单的“粘贴”：剪贴板里的字；nil 不接。
	Paste func() string
	// FindBar 往查找条（Diff.Find.Open 时在视图右上角）里放东西：输入框绑 Diff.Find.Text，每帧 SetFind；
	// ↩ / ⇧↩ 调 FindStep，Esc 调 CloseFind。nil 不画（也不涂找到的字）。
	FindBar func(c *wui.Context, bar wui.Element)
	// Wrap 是长行折到下一行显示（自动换行）：一行占几行高按两边折出来多的那边算，不横着滚。
	Wrap bool
	// Menu 接右键（或 ⌃点）：按下、按着拖、松开都先给它（调用方开自己的菜单，按住拖到项上松开选它）。按下时视图先记好
	// 按在哪（MenuSpot），按在选中的字外面时选择挪到那里（就地编辑时是放光标），指针下那处改动成了当前那处。nil 不接。
	Menu func(ev wui.InputEvent) bool
}

// Context 是改动上下各留几行不折。
const Context = 3

// Diff 是一个 Diff 视图的状态，跨帧留着（调用方按标签存）：两边的行、对齐的结果、折叠、滚动、排好的字。
type Diff struct {
	key     any
	lines   [2][]string
	lang    *editor.Language
	aligned []Row
	rows    []Row
	open    map[int]Reveal   // 展开了的折叠段（按那段没改的行在旧的一边的第一行）
	folds   map[int]foldInfo // 现在的 Fold 行（按 Old）的来历
	full    bool             // 不折
	trim    bool             // 忽略行首行尾空白（SetIgnoreTrim）
	unified bool             // 统一的一栏（不是并排）
	hunks   []Hunk
	pairs   [2]map[int]int    // 配成对（Changed）的行：旧行号 → 新行号、新行号 → 旧行号（统一视图里找行里改了的字）
	hunkRow []int             // 每处改动的第一行在 rows 里是第几行
	hunkEnd []int             // 每处改动最后一行的下一行在 rows 里是第几行
	rowHunk []int             // rows 的每一行是第几处改动的，-1 不是
	cur     int               // 上次跳到、点过的那处改动，-1 没有
	want    int               // 下一帧要滚到的那行（rows 的下标），-1 没有
	words   map[int][2][]Span // Changed 行按旧行号
	states  [2][]uint64       // 每行开头的分词状态（块注释、多行字符串跨行）
	toks    []editor.Token
	layouts map[lineKey]*lineLayout
	font    wui.Font // layouts 是按这个字体、这套颜色排的
	theme   editor.Theme

	ScrollY float32
	scrollX [2]float32
	widest  [2]float32
	w, h    float32  // 上一帧的大小（本地坐标），输入按它换算
	box     wui.Rect // 上一帧画在哪（画的坐标）
	lh      float32
	drag    bool
	dragOff float32
	hover   int // 指针在第几行（rows 的下标），-1 不在
	hoverZ  int // 指针在折叠条的哪个箭头上（foldZone）
	onBar   bool
	barBox  wui.Rect // 改动上那排按钮在哪（本地坐标；大小是上一帧量的）：按在上面的归按钮

	split     float32 // 并排时左边占多宽（0 到 1，0 是一半）：拖中间那道线改，双击还原
	splitting bool    // 正按着拖中间那道线
	onSplit   bool    // 指针在中间那道线上

	sel       selection
	selecting bool // 正按着拖选
	side      int  // 上次按在哪边（⌘A 全选那边，← → 横着滚那边）
	focus     bool // 按下了：下一帧把键盘焦点给视图

	picking  bool // 正按着在行号上拖着选行
	anchor   int  // 选行的起点（⇧ 点、拖动从这算），-1 没有
	gutter   bool // 指针在行号上（能选行时）
	lsel     lineSel
	textLeft [2]float32 // 两边正文从哪开始（画的坐标，上一帧的），给测试用

	// 就地编辑（edit.go）
	editable     bool     // 这一帧给了 Style.Buf
	focused      bool     // 这一帧视图有键盘焦点（没有时不画光标）
	compose      string   // 输入法正拼着的字（还没进缓冲区，画在光标处）
	composeCaret int      // 拼着的字里光标在第几个字（rune）
	goalX        float32  // 上下挪光标时横着想停在哪（正文里的 x），-1 没有
	reveal       bool     // 下一帧把光标滚进视图
	caretBox     wui.Rect // 上一帧光标画在哪（本地坐标），输入法的候选框跟着
	lastKind     editKind // 上一次改字是哪种（连着打字、连着删合成一步撤销）
	lastEdit     time.Time
	ver          uint64 // 我们上次改完 / 撤销完缓冲区的版本：不一样就是别处改过
	undoGroups   []int  // 我们最近的几步撤销各是几次 Buffer.Edit（连着打的字合成一步）
	redoGroups   []int

	// 自动换行（wrap.go）
	wrap      bool       // 这一帧 Style.Wrap
	wrapW     [2]float32 // 两边折行的宽（layouts 是按它排的）
	tops      []int      // rows 的每一行从第几个“屏幕行”开始（多一个，最后是总数）；不换行时不用
	topsDirty bool       // 行变了、宽变了、新排的行和估的行数不一样：tops 要重算
	wrapAt    int        // 开关换行前顶上的那行（rows 的下标），下一帧滚回它；-1 没有
	style     *Style     // 上一次 View 的样子（估折几行用）

	// 查找（find.go）
	Find       Find
	findQ      string // matches 是按它找的
	matches    []match
	findAt     map[lineKey][]int // 每行里的几处（matches 的下标）
	findCur    int               // 当前那处（matches 的下标），-1 没有
	findReveal bool              // 下一帧把当前那处滚进视图
	findBox    wui.Rect          // 查找条在哪（本地坐标，上一帧量的）
	occ        string            // 这一帧选中的字（occurrence 用，见 occurText）

	// 右键菜单（menu.go）
	spot     Spot // 上一次右键按在哪
	menuHeld bool // 右键按着（按着拖、松开也给 Style.Menu）
}

// findTop 是查找条离视图顶多远。
const findTop = 6

// pos 是一边里的一个位置：第 line 行（从 0 起）第 off 个字节前。
type pos struct{ line, off int }

func (p pos) less(q pos) bool { return p.line < q.line || p.line == q.line && p.off < q.off }

// lineSel 是这一帧选中的行（Style.Lines 给的）。
type lineSel struct {
	side, a, b int
	ok         bool
}

func (l lineSel) has(side, line int) bool {
	return l.ok && l.side == side && line >= l.a && line <= l.b
}

// selection 是一边（side）里选中的字，从 a 到 b（拖的起点和现在的点，不一定 a 在前）。按原文的行算：
// 拖过折叠条时折起来的行也算在里面，展开、折起不影响它。
type selection struct {
	side int
	a, b pos
}

func (s selection) span() (pos, pos) {
	if s.b.less(s.a) {
		return s.b, s.a
	}
	return s.a, s.b
}

func (s selection) empty() bool { return s.a == s.b }

type lineKey struct{ side, line int }

type lineLayout struct {
	l   *wui.TextLayout
	u16 []int // 原文第几个字节在排出来的字里是第几个 UTF-16 单元（制表符展开过）
	w   float32
	n   int     // 折成了几行（不换行时是 1）
	lt  float32 // 排出来的一行多高（画的时候按行高 lh 摆：第 k 行的顶是 k·lh）
}

// NewDiff 建一个空的 Diff。
func NewDiff() *Diff {
	return &Diff{open: map[int]Reveal{}, hover: -1, cur: -1, want: -1, side: 1, anchor: -1, goalX: -1, wrapAt: -1, findCur: -1}
}

// Set 换上两边的内容：key 和上次一样就什么都不做（调用方拿两边的版本号当 key），不一样才调 load，
// 内容变了才重新对齐，返回重新对齐了没有。滚动位置留着；只有新的一边变了时（就地编辑、文件标签里改了）
// 折叠的展开、选中的字、光标也留着，不然折叠的展开状态清掉。
func (d *Diff) Set(key any, load func() (old, new []string), lang *editor.Language) bool {
	if d.aligned != nil && d.key == key && d.lang == lang {
		return false
	}
	if d.lang != lang {
		d.aligned = nil // 换了语言：排好的字、分词状态都得重来
	}
	d.key, d.lang = key, lang
	old, new := load()
	return d.update(old, new)
}

// update 换上两边的行，变了才重新对齐。旧的一边没变时留着折叠的展开、旧的一边排好的字、选中的字（夹到新的行里）。
func (d *Diff) update(old, new []string) bool {
	keep := d.aligned != nil && sameLines(old, d.lines[0])
	if keep && sameLines(new, d.lines[1]) {
		return false
	}
	d.lines = [2][]string{old, new}
	d.align()
	if keep {
		for k := range d.layouts {
			if k.side == 1 {
				delete(d.layouts, k)
			}
		}
		d.states[1] = nil
		d.cur = min(d.cur, len(d.hunks)-1)
		if d.sel.side == 1 {
			d.sel.a, d.sel.b = d.clampPos(d.sel.a), d.clampPos(d.sel.b)
		}
	} else {
		d.open = map[int]Reveal{}
		d.states = [2][]uint64{}
		d.layouts = nil
		d.widest = [2]float32{}
		d.cur = -1
		d.sel, d.selecting = selection{}, false
	}
	d.refold()
	if d.findQ != "" { // 内容变了：重找，当前那处留着（夹到新的个数里）
		d.search()
		d.findCur = min(d.findCur, len(d.matches)-1)
	}
	return true
}

// align 按 lines、trim 重新对齐，跟着重算改动、配对的行、行里改了的字。
func (d *Diff) align() {
	d.aligned = AlignTrim(d.lines[0], d.lines[1], d.trim)
	d.words = map[int][2][]Span{}
	d.hunks = Hunks(d.aligned)
	d.pairs = [2]map[int]int{{}, {}}
	for _, r := range d.aligned {
		if r.Kind == Changed {
			d.pairs[0][r.Old], d.pairs[1][r.New] = r.New, r.Old
		}
	}
}

// SetIgnoreTrim 开、关“忽略行首行尾空白”：只改了缩进、行尾空格的行不算改动（VS Code 的 ignoreTrimWhitespace）。
// 变了就重新对齐：折叠的展开清掉，滚动位置、选中的字留着，当前那处改动夹到新的个数里。
func (d *Diff) SetIgnoreTrim(on bool) {
	if d.trim == on {
		return
	}
	d.trim = on
	if d.aligned == nil {
		return
	}
	d.align()
	d.open = map[int]Reveal{}
	d.cur = min(d.cur, len(d.hunks)-1)
	d.refold()
	if d.findQ != "" {
		d.search()
		d.findCur = min(d.findCur, len(d.matches)-1)
	}
}

// IgnoreTrim 是现在忽不忽略行首行尾空白。
func (d *Diff) IgnoreTrim() bool { return d.trim }

// SetFull 换成显示全文（on）或只看改动附近（没改的长段折起来）。
func (d *Diff) SetFull(on bool) {
	if d.full != on {
		d.full = on
		d.refold()
	}
}

// SetUnified 换成统一的一栏（on：每处改动先列删的、再列加的，左边两列行号）或并排。滚动位置留着，选中的字、行照旧。
func (d *Diff) SetUnified(on bool) {
	if d.unified != on {
		d.unified = on
		d.refold()
	}
}

// Unified 是现在是不是统一的一栏。
func (d *Diff) Unified() bool { return d.unified }

// refold 按 full、open、unified 重排显示的行，跟着重算行和改动的对应。
func (d *Diff) refold() {
	if d.full {
		d.rows = d.aligned
	} else {
		d.rows, d.folds = foldRows(d.aligned, Context, d.open)
	}
	if d.unified {
		d.rows = Unified(d.rows)
	}
	d.topsDirty = true
	// 改了的行折不起来，按改动的顺序挨着：第 k 个改了的行属于哪处改动按个数数（并排时一处占 Row1 - Row0 行，
	// 统一时占两边的行数之和）
	d.rowHunk = make([]int, len(d.rows))
	d.hunkRow = make([]int, len(d.hunks))
	d.hunkEnd = make([]int, len(d.hunks))
	h, left := -1, 0
	for i, r := range d.rows {
		d.rowHunk[i] = -1
		if r.Kind == Same || r.Kind == Fold {
			continue
		}
		for left == 0 && h+1 < len(d.hunks) {
			h++
			hk := d.hunks[h]
			left = hk.Row1 - hk.Row0
			if d.unified {
				left = hk.OldEnd - hk.OldStart + hk.NewEnd - hk.NewStart
			}
			d.hunkRow[h], d.hunkEnd[h] = i, i+left
		}
		d.rowHunk[i] = h
		left--
	}
}

// rowSide 是统一视图里 row 的字是哪边的：删掉的行是旧的，别的（没改的、加的、折叠）按新的。
func (d *Diff) rowSide(r Row) int {
	if r.Kind == Deleted {
		return 0
	}
	return 1
}

// Rows 是现在显示的行（折过的）。
func (d *Diff) Rows() []Row { return d.rows }

// Expand 展开 rows[i] 那段折叠；不是折叠时返回 false。
func (d *Diff) Expand(i int) bool {
	if i < 0 || i >= len(d.rows) || d.rows[i].Kind != Fold {
		return false
	}
	return d.ExpandMore(i, 0)
}

// FoldStep 是折叠条上点一下箭头再露几行（VS Code 也是 20）。
const FoldStep = 20

// ExpandMore 展开 rows[i] 那段折叠：dir 0 整段，-1 接着上面那处改动往下露 FoldStep 行，1 接着下面那处改动往上露；
// 剩的不比 FoldStep 多时整段展开。不是折叠时返回 false。
func (d *Diff) ExpandMore(i, dir int) bool {
	if i < 0 || i >= len(d.rows) || d.rows[i].Kind != Fold {
		return false
	}
	f, ok := d.folds[d.rows[i].Old]
	if !ok {
		f = foldInfo{key: d.rows[i].Old}
	}
	rv := d.open[f.key]
	switch {
	case dir == 0 || d.rows[i].N <= FoldStep+1:
		rv.All = true
	case dir < 0:
		rv.Top += FoldStep
	default:
		rv.Bottom += FoldStep
	}
	if d.open == nil {
		d.open = map[int]Reveal{}
	}
	d.open[f.key] = rv
	d.refold()
	return true
}

// FoldDirs 是 rows[i] 那段折叠能往哪露：up 是上面挨着改动（能接着它往下露），down 是下面挨着改动。不是折叠时都是 false。
func (d *Diff) FoldDirs(i int) (up, down bool) {
	if i < 0 || i >= len(d.rows) || d.rows[i].Kind != Fold {
		return false, false
	}
	f := d.folds[d.rows[i].Old]
	return f.up, f.down
}

// Selected 是选中的字（没选是 ""）。行尾的 \r 不算。
func (d *Diff) Selected() string {
	if d.sel.empty() {
		return ""
	}
	a, b := d.sel.span()
	lines := d.lines[d.sel.side]
	var out strings.Builder
	for l := a.line; l <= b.line && l < len(lines); l++ {
		t := strings.TrimSuffix(lines[l], "\r")
		s, e := 0, len(t)
		if l == a.line {
			s = min(a.off, e)
		}
		if l == b.line {
			e = min(b.off, e)
		}
		out.WriteString(t[s:max(s, e)])
		if l < b.line {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

// Select 选中 side 那边从 (l0, o0) 到 (l1, o1) 的字（行号从 0 起，o 是字节）。
func (d *Diff) Select(side, l0, o0, l1, o1 int) {
	d.sel = selection{side: side, a: pos{l0, o0}, b: pos{l1, o1}}
}

// SelectAll 选中 side 那边的全部。
func (d *Diff) SelectAll(side int) {
	lines := d.lines[side]
	if len(lines) == 0 {
		d.sel = selection{}
		return
	}
	last := len(lines) - 1
	d.Select(side, 0, 0, last, len(strings.TrimSuffix(lines[last], "\r")))
}

// Hunks 是一处处改动。
func (d *Diff) Hunks() []Hunk { return d.hunks }

// GoToHunk 把第 i 处改动滚到离顶上三行的地方，记成当前的那处（Current）。
func (d *Diff) GoToHunk(i int) {
	if i >= 0 && i < len(d.hunks) {
		d.cur = i
		d.ScrollToRow(d.hunkRow[i])
	}
}

// GoToLine 跳到新的一边第 line 行（从 0 起，夹到范围里；新的一边是空的时按旧的一边）：在折起来的段里时展开那段，
// 滚到离顶上三行，选择收成那一行行首（就地编辑时光标在那），键盘给视图。
func (d *Diff) GoToLine(line int) {
	side := 1
	if len(d.lines[1]) == 0 {
		side = 0
	}
	n := len(d.lines[side])
	if n == 0 {
		return
	}
	line = min(max(line, 0), n-1)
	i := d.rowOfLine(side, line)
	if i < 0 {
		return
	}
	if h := d.rowHunk[i]; h >= 0 {
		d.cur = h
	}
	p := pos{line: line}
	d.sel, d.side, d.focus, d.goalX = selection{side: side, a: p, b: p}, side, true, -1
	d.want = i
}

// LineCount 是新的一边有几行（新的一边是空的时旧的一边）：跳行时给的范围。
func (d *Diff) LineCount() int {
	if n := len(d.lines[1]); n > 0 {
		return n
	}
	return len(d.lines[0])
}

// Refocus 把当前的那处改动滚回视图（折起、展开之后）；还没有当前的那处时返回 false。
func (d *Diff) Refocus() bool {
	if d.cur < 0 {
		return false
	}
	d.GoToHunk(d.cur)
	return true
}

// HunkAtLine 是 side（0 旧、1 新）那边第 line 行所在的、或者它后面的第一处改动；都在它前面时是最后一处，没有改动是 -1。
func (d *Diff) HunkAtLine(side, line int) int {
	for i, h := range d.hunks {
		s, e := h.OldStart, h.OldEnd
		if side == 1 {
			s, e = h.NewStart, h.NewEnd
		}
		if e > line || s >= line {
			return i
		}
	}
	return len(d.hunks) - 1
}

// Current 是标签条的上 / 下一处、底栏的“第 i / n 处改动”按的那处：刚跳到、点过的那处还在视图里就是它（inside）；
// 不然按视口，顶上往下三行以前开始的最后一处（inside 是它还看得见），一处都没有是 -1。
func (d *Diff) Current() (cur int, inside bool) {
	if len(d.hunks) == 0 {
		return -1, false
	}
	if d.lh <= 0 || d.h <= 0 {
		return d.cur, d.cur >= 0
	}
	top := d.rowIndex(int(d.ScrollY / d.lh))
	bottom := d.rowIndex(int((d.ScrollY + d.h) / d.lh))
	seen := func(i int) bool { return d.hunkEnd[i] > top && d.hunkRow[i] < bottom }
	if d.cur >= 0 && d.cur < len(d.hunks) && seen(d.cur) {
		return d.cur, true
	}
	cur = -1
	for i, r := range d.hunkRow {
		if r > top+Context {
			break
		}
		cur = i
	}
	return cur, cur >= 0 && seen(cur)
}

// Bounds 是上一帧视图画在哪（窗口里的坐标，测试按它点）。
func (d *Diff) Bounds() wui.Rect { return d.box }

// RowHeight 是上一帧的行高（还没画过是 0）。
func (d *Diff) RowHeight() float32 { return d.lh }

// ScrollToRow 把 rows[i] 滚到离顶上三行的地方（下一帧画的时候生效：那时才知道行高）。
func (d *Diff) ScrollToRow(i int) { d.want = i }

func (d *Diff) wordsOf(r Row) [2][]Span {
	if w, ok := d.words[r.Old]; ok {
		return w
	}
	o, n := Words(d.lines[0][r.Old], d.lines[1][r.New])
	w := [2][]Span{o, n}
	d.words[r.Old] = w
	return w
}

func (d *Diff) stateAt(side, line int) uint64 {
	if d.lang == nil || d.lang.Tokenizer == nil {
		return 0
	}
	st := d.states[side]
	if len(st) == 0 {
		st = append(st, 0)
	}
	for len(st) <= line {
		i := len(st) - 1
		var next uint64
		d.toks, next = d.lang.Tokenizer.Tokenize(clean(d.lines[side][i]), st[i], d.toks[:0])
		st = append(st, next)
	}
	d.states[side] = st
	return st[line]
}

// lineMax 是一行最多排多少字节：再长的（压缩过的 JS）后面看不到也没关系，排它太慢。
const lineMax = 4000

func clean(s string) string {
	s = strings.TrimSuffix(s, "\r")
	if len(s) > lineMax {
		e := lineMax
		for e > 0 && !utf8.RuneStart(s[e]) {
			e--
		}
		s = s[:e]
	}
	return s
}

// layout 排一行字：语法高亮的颜色、制表符展开成空格。排好的留着，留太多了整个清掉重排。
func (d *Diff) layout(side, line int, s *Style) *lineLayout {
	if d.layouts == nil || len(d.layouts) > 4000 {
		d.layouts = map[lineKey]*lineLayout{}
	}
	k := lineKey{side, line}
	composing := side == 1 && d.compose != "" && d.sel.side == 1 && line == d.sel.b.line // 输入法拼着的字排进去，不留
	if l := d.layouts[k]; l != nil && !composing {
		return l
	}
	src := clean(d.lines[side][line])
	if composing {
		o := min(d.sel.b.off, len(src))
		src = src[:o] + d.compose + src[o:]
	}
	ct := s.Code
	type seg struct {
		end  int
		kind editor.TokenKind
		tok  bool
	}
	var segs []seg
	if d.lang != nil && d.lang.Tokenizer != nil {
		var toks []editor.Token
		toks, _ = d.lang.Tokenizer.Tokenize(src, d.stateAt(side, line), d.toks[:0])
		d.toks = toks
		at := 0
		for _, t := range toks {
			if t.End <= at || t.End > len(src) {
				continue
			}
			segs = append(segs, seg{t.End, t.Kind, true})
			at = t.End
		}
	}
	if len(segs) == 0 || segs[len(segs)-1].end < len(src) {
		segs = append(segs, seg{end: len(src)})
	}
	tab := s.Tab
	if tab <= 0 {
		tab = 4
	}
	u16 := make([]int, len(src)+1)
	var spans []wui.Span
	col, units, at := 0, 0, 0
	var b strings.Builder
	for _, sg := range segs {
		b.Reset()
		for i := at; i < sg.end; {
			r, w := utf8.DecodeRuneInString(src[i:])
			for j := range w {
				u16[i+j] = units
			}
			if r == '\t' {
				n := tab - col%tab
				b.WriteString(strings.Repeat(" ", n))
				col, units = col+n, units+n
			} else {
				b.WriteRune(r)
				col++
				units += utf16Len(r)
			}
			i += w
		}
		sp := wui.Span{Text: b.String(), Font: s.Font.Family, Size: s.Font.Size, Color: ct.Text}
		if sg.tok {
			ts := ct.Tokens[sg.kind]
			sp.Color, sp.Italic = ct.TokenColor(sg.kind), ts.Italic
			if ts.Bold {
				sp.Weight = 600
			}
		}
		spans = append(spans, sp)
		at = sg.end
	}
	u16[len(src)] = units
	f := s.Font
	f.LineHeight = s.lineHeight()
	var ww float32
	if d.wrap {
		ww = d.wrapW[side]
	}
	tl := wui.ShapeRichText(spans, f, ww)
	w, th := tl.Size()
	n := max(1, len(tl.Lines()))
	l := &lineLayout{l: tl, u16: u16, w: w, n: n, lt: th / float32(n)}
	if !composing {
		d.layouts[k] = l
		if d.wrap && n != d.estimate(side, line, s) { // 估的不对：tops 按排出来的重算
			d.topsDirty = true
		}
	}
	d.widest[side] = max(d.widest[side], w)
	return l
}

func utf16Len(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

func (s *Style) lineHeight() float32 {
	if s.LineHeight > 0 {
		return s.LineHeight
	}
	return 1.5
}

// geom 是这一帧的版面（本地坐标，左上角是 0, 0）。
type geom struct {
	w, h    float32
	lh      float32
	paneX   [2]float32
	paneW   [2]float32
	numR    float32 // 行号右边对齐到格子里的哪（统一视图：旧的行号）
	numR2   float32 // 统一视图：新的行号右边对齐到哪（并排时和 numR 一样）
	markX   float32 // + − 的位置
	textX   float32 // 正文从格子里的哪开始
	content float32 // 全部行多高
	ruler   wui.Rect
	asc     float32
	desc    float32
	cw      float32 // 一个数字多宽
}

const rulerW = 14

// splitMin 是拖中间那道线时一边至少占多宽（比例）。
const splitMin = 0.15

// nearSplit 是 x 在并排时中间那道线上（左右各 3 pt 的热区）。
func (d *Diff) nearSplit(x float32, g geom) bool {
	return !d.unified && x >= g.paneW[0]-3 && x <= g.paneX[1]+3
}

// Split 是并排时左边占多宽（0 是一半）。给测试用。
func (d *Diff) Split() float32 { return d.split }

func (d *Diff) geom(s *Style, w, h, scale float32) geom {
	size := s.Font.Size
	if size <= 0 {
		size = 13
	}
	scale = max(scale, 1)
	g := geom{w: w, h: h}
	g.lh = float32(math.Round(float64(size*s.lineHeight()*scale))) / scale
	digits := len(strconv.Itoa(max(len(d.lines[0]), len(d.lines[1]), 1)))
	cw := charWidth(s.Font)
	g.cw = cw
	g.numR = 10 + float32(digits)*cw
	g.numR2 = g.numR
	if d.unified { // 两列行号：旧的、新的
		g.numR2 = g.numR + 8 + float32(digits)*cw
	}
	g.markX = g.numR2 + 8
	g.textX = g.markX + cw + 8
	if d.unified { // 一栏：两边的“格子”是同一个
		g.paneW = [2]float32{w, w}
	} else {
		f := float32(0.5)
		if d.split > 0 {
			f = min(max(d.split, splitMin), 1-splitMin)
		}
		g.paneW[0] = float32(math.Floor(float64((w - 1) * f)))
		g.paneX[1] = g.paneW[0] + 1
		g.paneW[1] = w - g.paneX[1]
	}
	if d.wrap {
		d.setWrapWidth(g)
	}
	g.content = float32(d.top(len(d.rows))) * g.lh
	g.ruler = wui.Rect{X: w - rulerW, Y: 0, W: rulerW, H: h}
	m := s.Font.Metrics()
	g.asc, g.desc = m.Ascent, m.Descent
	return g
}

var widthCache sync.Map // wui.Font → 一个数字多宽

func charWidth(f wui.Font) float32 {
	if w, ok := widthCache.Load(f); ok {
		return w.(float32)
	}
	w := float32(7.8)
	if gs := wui.Shape("0", f); len(gs) > 0 {
		w = gs[0].Advance
	}
	widthCache.Store(f, w)
	return w
}

func (d *Diff) clamp(g geom) {
	d.ScrollY = min(d.ScrollY, max(0, g.content-g.h+g.lh*3))
	d.ScrollY = max(d.ScrollY, 0)
	if d.wrap { // 折行了：不横着滚
		d.scrollX = [2]float32{}
		return
	}
	if d.unified { // 一栏横着一起滚：两边的数一样（按上次动的那边），按宽的那边算能滚多远
		room := g.paneW[1] - g.textX - rulerW
		x := max(0, min(d.scrollX[d.side], max(d.widest[0], d.widest[1])-room+g.lh))
		d.scrollX = [2]float32{x, x}
		return
	}
	for side := range 2 {
		room := g.paneW[side] - g.textX - rulerW
		d.scrollX[side] = max(0, min(d.scrollX[side], d.widest[side]-room+g.lh))
	}
}

func (d *Diff) bar(g geom) wui.Scrollbar {
	return wui.Scrollbar{Box: g.ruler, View: g.h, Content: g.content + g.lh*3, Offset: d.ScrollY,
		Style: wui.ScrollbarStyle{TrackWidth: rulerW, ThumbWidth: rulerW, Square: true, Flush: true}}
}

// View 建 Diff 的视图（并排，或 SetUnified 后统一的一栏）：自己管滚动（一个 ScrollY，两边一起走；并排时横着两边各滚各的），
// 只排、只画看得见的行。
// 点折叠条展开那一段。
func View(c *wui.Context, key any, d *Diff, s Style) wui.Element {
	if d.hover >= len(d.rows) {
		d.hover = -1
	}
	if d.cur >= len(d.hunks) {
		d.cur = -1
	}
	d.style = &s
	d.setWrap(s.Wrap)
	d.editable = s.Buf != nil
	if !d.editable {
		d.compose = ""
	}
	if d.w > 0 { // 跳到哪处在建视图时就滚过去：改动上那排按钮按滚好的位置摆（还没画过的等画的时候）
		g := d.geomNow(&s)
		d.applyWant(g)
		d.revealCaret(g, &s)
		d.revealFind(g, &s)
		d.clamp(g)
	}
	el := wui.Box(c.Key(key)).Grow(1).MinHeight(0).MinWidth(0).Clip().Background(s.Code.Background).Label(s.Label).
		Focusable().FocusRing(false)
	if d.focus {
		d.focus = false
		el.Focus(wui.FocusOptions{PreventScroll: true})
	}
	d.focused = el.Focused()
	if _, ok := d.caret(&s); ok { // 就地编辑、有光标：收字和输入法（候选框在光标处）
		el.TextCaret(d.caretBox)
	}
	if !el.Hovered() && !d.drag && !d.splitting { // 指针走了（没有“离开”事件，问 Hovered）
		d.hover, d.onBar, d.onSplit = -1, false, false
	}
	switch {
	case d.onSplit || d.splitting:
		el.Cursor(wui.CursorResizeEW)
	case d.hover >= 0 && d.rows[d.hover].Kind == Fold && !d.onBar:
		el.Cursor(wui.CursorPointer)
	case d.gutter && !d.selecting || d.picking:
		el.Cursor(wui.CursorDefault)
	case d.hover >= 0 && !d.onBar || d.selecting:
		el.Cursor(wui.CursorText)
	}
	el.Draw(func(p *wui.Painter, r wui.Rect) {
		g := d.geom(&s, r.W, r.H, p.Scale())
		d.w, d.h, d.lh, d.box = r.W, r.H, g.lh, r
		d.textLeft = [2]float32{r.X + g.paneX[0] + g.textX, r.X + g.paneX[1] + g.textX}
		d.applyWant(g)
		d.revealCaret(g, &s)
		d.revealFind(g, &s)
		d.clamp(g)
		d.paint(p, r, g, &s)
	})
	el.HandleInput(func(ev wui.InputEvent) bool { return d.input(ev, &s) })
	size, fsize := d.barBox, d.findBox
	d.barBox, d.findBox = wui.Rect{}, wui.Rect{}
	findOpen := d.Find.Open && s.FindBar != nil
	if findOpen {
		el.Children(func() {
			bar := wui.Row(c).Key("find bar").Absolute().Right(rulerW + 10).Top(findTop).Cursor(wui.CursorDefault)
			s.FindBar(c, bar)
			if b := bar.Bounds(); b.W > 0 {
				fsize = b
			} else if fsize.W == 0 {
				fsize = wui.Rect{W: 300, H: 32}
			}
			d.findBox = wui.Rect{X: d.w - rulerW - 10 - fsize.W, Y: findTop, W: fsize.W, H: fsize.H}
		})
	}
	if i := d.barHunk(); i >= 0 && s.HunkBar != nil {
		el.Children(func() {
			top := d.hunkTop(i) + 2
			if findOpen { // 查找条下面
				top = max(top, findTop+fsize.H+4)
			}
			bar := wui.Row(c).Key("hunk bar").Absolute().Right(rulerW + 6).Top(top).
				Cursor(wui.CursorDefault).KeepFocus()
			s.HunkBar(c, bar, i, d.hunks[i])
			// Bounds 是上一帧的（跳了一处改动后位置就不对了），只拿它的大小，位置按这一帧摆的算；还没量过先当 80 × 一行
			if b := bar.Bounds(); b.W > 0 {
				size = b
			} else if size.W == 0 {
				size = wui.Rect{W: 80, H: d.lh}
			}
			d.barBox = wui.Rect{X: d.w - rulerW - 6 - size.W, Y: top, W: size.W, H: size.H}
		})
	}
	return el
}

func (d *Diff) applyWant(g geom) {
	if d.wrapAt >= 0 && g.lh > 0 {
		d.ScrollY, d.wrapAt = float32(d.top(d.wrapAt))*g.lh, -1
	}
	if d.want >= 0 && g.lh > 0 {
		d.ScrollY, d.want = max(0, float32(d.top(d.want)-Context)*g.lh), -1
	}
}

// barHunk 是哪处改动上放那排按钮：指针下的那处；指针不在改动上时是刚跳到、点过的那处（还看得见的话）。
func (d *Diff) barHunk() int {
	if d.lh <= 0 {
		return -1
	}
	if d.hover >= 0 && !d.onBar {
		if h := d.rowHunk[d.hover]; h >= 0 {
			return h
		}
	}
	if cur, inside := d.Current(); inside && cur == d.cur {
		return cur
	}
	return -1
}

// hunkTop 是第 i 处改动的顶在视图里的 y；顶滚出去了就贴着视图顶，直到改动的最后一行（按钮跟着改动走，不悬空）。
func (d *Diff) hunkTop(i int) float32 {
	y0 := float32(d.top(d.hunkRow[i]))*d.lh - d.ScrollY
	y1 := float32(d.top(d.hunkEnd[i]))*d.lh - d.ScrollY
	return max(min(0, y1-d.lh), y0)
}

func (d *Diff) geomNow(s *Style) geom {
	g := d.geom(s, d.w, d.h, 1)
	if d.lh > 0 {
		g.lh = d.lh
		g.content = float32(d.top(len(d.rows))) * g.lh
	}
	return g
}

func (d *Diff) rowAt(y float32, g geom) int {
	if g.lh <= 0 {
		return -1
	}
	v := int(math.Floor(float64((y + d.ScrollY) / g.lh)))
	if v < 0 {
		return -1
	}
	if i := d.rowIndex(v); i < len(d.rows) {
		return i
	}
	return -1
}

func (d *Diff) input(ev wui.InputEvent, s *Style) bool {
	if d.w <= 0 {
		return false
	}
	g := d.geomNow(s)
	inBar := ev.X >= g.ruler.X && g.content+g.lh*3 > g.h
	if took, handled := d.splitInput(ev, g, inBar); handled {
		return took
	}
	if ev.Kind != wui.InputScroll && !d.drag && (d.barBox.W > 0 && d.barBox.Contains(ev.X, ev.Y) ||
		d.findBox.W > 0 && d.findBox.Contains(ev.X, ev.Y)) {
		return false // 在改动上那排按钮上：归按钮，指针下的那处改动不变
	}
	if took, handled := d.menuInput(ev, g, s); handled {
		return took
	}
	if took, handled := d.editInput(ev, g, s); handled {
		return took
	}
	switch ev.Kind {
	case wui.InputKeyDown:
		if _, ok := d.caret(s); ok { // 就地编辑：方向键挪光标、退格删字……打字的键不接，字跟着来
			if d.editKey(ev, g, s) {
				return true
			}
			if ev.Mods&(wui.Super|wui.Ctrl) == 0 && ev.Key != wui.KeyEscape {
				return false
			}
		}
		if s.Keys != nil && s.Keys(ev) {
			return true
		}
		return d.onKey(ev, g, s)
	case wui.InputCommand:
		switch ev.Text {
		case "copy":
			if d.sel.empty() || s.Copy == nil {
				return false
			}
			s.Copy(d.Selected())
			return true
		case "selectAll":
			d.SelectAll(d.side)
			return true
		}
		return false
	case wui.InputScroll:
		y, x := d.ScrollY, d.scrollX
		d.ScrollY += ev.DY
		side := 0
		if ev.X >= g.paneX[1] {
			side = 1
		}
		if d.unified {
			side = d.side
		}
		d.scrollX[side] += ev.DX
		d.clamp(g)
		return d.ScrollY != y || d.scrollX != x
	case wui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		if inBar {
			sb := d.bar(g)
			start, n := sb.Thumb()
			if ev.Y >= start && ev.Y < start+n {
				d.dragOff = ev.Y - start
			} else {
				d.dragOff = n / 2
				d.ScrollY = sb.OffsetAt(ev.Y - d.dragOff)
				d.clamp(g)
			}
			d.drag = true
			return true
		}
		d.focus = true
		d.goalX = -1
		d.endGroup()
		i := d.rowAt(ev.Y, g)
		if i >= 0 && d.ExpandMore(i, d.foldZone(i, ev.X, g)) {
			d.hoverZ = 0
			return true
		}
		if i >= 0 && d.rowHunk[i] >= 0 {
			d.cur = d.rowHunk[i]
		}
		side := 0
		if ev.X >= g.paneX[1] {
			side = 1
		}
		if d.unified && i >= 0 {
			side = d.rowSide(d.rows[i])
		}
		d.side = side
		if d.pickLine(ev, i, side, g, s) {
			return true
		}
		p := d.posAt(ev.X, ev.Y, side, g, s)
		switch {
		case ev.Mods&wui.Shift != 0 && !d.sel.empty() && d.sel.side == side: // ⇧ 点：从原来的起点选到这
			d.sel.b = p
		case ev.Clicks == 2: // 双击选一个词
			d.sel = d.wordAt(side, p)
		case ev.Clicks >= 3: // 三击选一行（连着行尾的换行）
			d.sel = selection{side: side, a: pos{p.line, 0}, b: pos{p.line + 1, 0}}
		default:
			d.sel = selection{side: side, a: p, b: p}
		}
		d.selecting = true
		return true
	case wui.InputPointerMove:
		if d.drag {
			d.ScrollY = d.bar(g).OffsetAt(ev.Y - d.dragOff)
			d.clamp(g)
			return true
		}
		if d.picking {
			if ev.Y < 0 {
				d.ScrollY += ev.Y
			} else if ev.Y > g.h {
				d.ScrollY += ev.Y - g.h
			}
			d.clamp(g)
			if i := d.rowAt(min(max(ev.Y, 0), g.h-1), g); i >= 0 && d.rows[i].Kind != Fold {
				if line := lineOf(d.rows[i], d.lsel.side); line >= 0 && d.anchor >= 0 {
					s.SetLines(d.lsel.side, d.anchor, line, true)
				}
			}
			return true
		}
		if d.selecting {
			// 拖出视图上下边时跟着滚（只在指针动的时候滚）
			if ev.Y < 0 {
				d.ScrollY += ev.Y
			} else if ev.Y > g.h {
				d.ScrollY += ev.Y - g.h
			}
			d.clamp(g)
			x := min(max(ev.X, g.paneX[d.sel.side]), g.paneX[d.sel.side]+g.paneW[d.sel.side]-1)
			d.sel.b = d.posAt(x, min(max(ev.Y, 0), g.h-1), d.sel.side, g, s)
			return true
		}
		hv := d.rowAt(ev.Y, g)
		gut := !inBar && d.canPick(s) && d.inGutter(ev.X, g)
		z := 0
		if hv >= 0 && !inBar {
			z = d.foldZone(hv, ev.X, g)
		}
		if hv != d.hover || inBar != d.onBar || gut != d.gutter || z != d.hoverZ {
			d.hover, d.onBar, d.gutter, d.hoverZ = hv, inBar, gut, z
			return true
		}
	case wui.InputPointerUp:
		if d.drag || d.selecting || d.picking {
			d.drag, d.selecting, d.picking = false, false, false
			return true
		}
	}
	return false
}

// splitInput 管中间那道线：按住拖改两边的宽，双击还原成一半；指针在线上时记 onSplit（换成左右拖的指针）。
func (d *Diff) splitInput(ev wui.InputEvent, g geom, inBar bool) (took, handled bool) {
	switch ev.Kind {
	case wui.InputPointerDown:
		if ev.Button != 0 || inBar || d.drag || !d.nearSplit(ev.X, g) {
			return false, false
		}
		if ev.Clicks == 2 {
			d.split, d.splitting = 0, false
		} else {
			d.splitting = true
		}
		return true, true
	case wui.InputPointerMove:
		if d.splitting {
			if g.w > 1 {
				d.split = min(max(ev.X/(g.w-1), splitMin), 1-splitMin)
			}
			return true, true
		}
		on := !inBar && !d.drag && !d.selecting && !d.picking && d.nearSplit(ev.X, g)
		if on != d.onSplit {
			d.onSplit = on
			return true, on
		}
		if on {
			return false, true
		}
	case wui.InputPointerUp:
		if d.splitting {
			d.splitting = false
			return true, true
		}
	}
	return false, false
}

// canPick 是能不能点行号选行。
func (d *Diff) canPick(s *Style) bool { return s.Lines != nil && s.SetLines != nil }

// inGutter 是 x 在不在那边的行号、+ − 上（正文左边）。
func (d *Diff) inGutter(x float32, g geom) bool {
	side := 0
	if x >= g.paneX[1] {
		side = 1
	}
	return x-g.paneX[side] < g.textX-4
}

// pickLine 是按在行号上：选那一行（⇧ 点从起点选到这，同一边才算），接着拖动扩大。按的不是行号、那边是空行、
// 不能选行时返回 false，照选字走。
func (d *Diff) pickLine(ev wui.InputEvent, i, side int, g geom, s *Style) bool {
	if !d.canPick(s) || i < 0 || d.rows[i].Kind == Fold || !d.inGutter(ev.X, g) {
		return false
	}
	line := lineOf(d.rows[i], side)
	if line < 0 {
		return false
	}
	d.sel = selection{}
	ls, a, b, ok := s.Lines()
	if ev.Mods&wui.Shift != 0 && ok && ls == side {
		if d.anchor < a || d.anchor > b { // 不是在这里选的（mygo 那个 Diff 选的、或者换过）：从原来的头算
			d.anchor = a
		}
	} else {
		d.anchor = line
	}
	d.lsel = lineSel{side: side, a: min(d.anchor, line), b: max(d.anchor, line), ok: true}
	s.SetLines(side, d.anchor, line, true)
	d.picking = true
	return true
}

// LineX 是 side 那边正文从哪开始（画的坐标，上一帧的）；往左是行号，按在上面是选行。给测试用。
func (d *Diff) LineX(side int) float32 { return d.textLeft[side] }

// FoldArrowX 是 rows[i] 那条折叠上 dir 那个箭头的中间在窗口里的 x（新的一边那格）；没有那个箭头时 ok 是 false。给测试用。
func (d *Diff) FoldArrowX(i, dir int) (x float32, ok bool) {
	if d.style == nil {
		return 0, false
	}
	g := d.geomNow(d.style)
	for _, a := range d.foldArrows(i, g) {
		if a.dir == dir {
			return d.box.X + g.paneX[1] + a.x + a.w/2, true
		}
	}
	return 0, false
}

// onKey 是视图自己接的键：↑ ↓ 一行，翻页、空格一屏，Home / End、⌘↑ / ⌘↓ 到头尾，← → 横着滚上次按的那边，
// Esc 取消选中的字，没有选中的字时取消选中的行。
func (d *Diff) onKey(ev wui.InputEvent, g geom, s *Style) bool {
	page := max(g.lh, g.h-2*g.lh)
	cmd := ev.Mods&wui.Super != 0
	switch {
	case ev.Key == wui.KeyEscape && ev.Mods == 0:
		if !d.sel.empty() {
			d.sel = selection{}
			return true
		}
		if d.canPick(s) {
			if _, _, _, ok := s.Lines(); ok {
				s.SetLines(0, 0, 0, false)
				d.anchor = -1
				return true
			}
		}
		return false
	case ev.Key == wui.KeyHome || cmd && ev.Key == wui.KeyUp:
		d.ScrollY = 0
	case ev.Key == wui.KeyEnd || cmd && ev.Key == wui.KeyDown:
		d.ScrollY = g.content
	case ev.Mods&^wui.Shift != 0:
		return false
	case ev.Key == wui.KeyUp:
		d.ScrollY -= g.lh
	case ev.Key == wui.KeyDown:
		d.ScrollY += g.lh
	case ev.Key == wui.KeyPageUp || ev.Key == wui.KeySpace && ev.Mods&wui.Shift != 0:
		d.ScrollY -= page
	case ev.Key == wui.KeyPageDown || ev.Key == wui.KeySpace:
		d.ScrollY += page
	case ev.Key == wui.KeyLeft:
		d.scrollX[d.side] -= 4 * g.cw
	case ev.Key == wui.KeyRight:
		d.scrollX[d.side] += 4 * g.cw
	default:
		return false
	}
	d.clamp(g)
	return true
}

// lineOf 是一行在 side 那边是第几行（Fold 行是折起来的第一行），没有是 -1。
func lineOf(r Row, side int) int {
	if side == 0 {
		return r.Old
	}
	return r.New
}

// posAt 是视图里 (x, y) 在 side 那边对着的位置：对面多出来的空行算下一行的开头，折叠条算折起来的第一行开头。
func (d *Diff) posAt(x, y float32, side int, g geom, s *Style) pos {
	lines := d.lines[side]
	if len(lines) == 0 || len(d.rows) == 0 || g.lh <= 0 {
		return pos{}
	}
	end := func() pos { last := len(lines) - 1; return pos{last, len(clean(lines[last]))} }
	v := int(math.Floor(float64((y + d.ScrollY) / g.lh)))
	i := 0
	if v >= 0 {
		i = d.rowIndex(v)
	}
	if i >= len(d.rows) {
		return end()
	}
	row := d.rows[i]
	if row.Kind == Fold {
		return pos{lineOf(row, side), 0}
	}
	line := lineOf(row, side)
	if line < 0 {
		for j := i + 1; j < len(d.rows); j++ {
			if l := lineOf(d.rows[j], side); l >= 0 {
				return pos{l, 0}
			}
		}
		return end()
	}
	l := d.layout(side, line, s)
	tx := x - (g.paneX[side] + g.textX) + d.scrollX[side]
	k := 0 // 折出来的第几行
	if v >= 0 {
		k = min(v-d.top(i), l.n-1)
	}
	if tx <= 0 && k == 0 {
		return pos{line, 0}
	}
	return pos{line, byteAt(l.u16, l.l.IndexAt(wui.Point{X: max(tx, 0), Y: (float32(k) + 0.5) * l.lt}))}
}

// byteAt 是排出来的字里第 idx 个 UTF-16 单元对着原文的第几个字节：落在展开的制表符中间时取近的那头。
func byteAt(u16 []int, idx int) int {
	b := sort.Search(len(u16), func(k int) bool { return u16[k] >= idx })
	if b >= len(u16) {
		return len(u16) - 1
	}
	if u16[b] > idx && b > 0 {
		p := b - 1
		for p > 0 && u16[p-1] == u16[p] { // 退到那个字的开头
			p--
		}
		if idx-u16[p] < u16[b]-idx {
			return p
		}
	}
	return b
}

// wordAt 是 p 所在的那个词（tokens 切的：字母数字连成一个，空白连成一个，别的一个字一个）。
func (d *Diff) wordAt(side int, p pos) selection {
	src := clean(d.lines[side][p.line])
	for _, t := range tokens(src) {
		if p.off >= t.Start && p.off < t.End {
			return selection{side: side, a: pos{p.line, t.Start}, b: pos{p.line, t.End}}
		}
	}
	return selection{side: side, a: p, b: p}
}

func (d *Diff) paint(p *wui.Painter, r wui.Rect, g geom, s *Style) {
	if d.font != s.Font || d.theme != *s.Code { // 换了字体、深浅色：排好的字作废
		d.layouts, d.font, d.theme = nil, s.Font, *s.Code
		d.topsDirty = true
	}
	d.occ = d.occurText()
	d.lsel = lineSel{}
	if d.canPick(s) {
		side, a, b, ok := s.Lines()
		d.lsel = lineSel{side: side, a: min(a, b), b: max(a, b), ok: ok}
	}
	vis := p.Visible()
	top := max(vis.Y, r.Y)
	bottom := min(vis.Y+vis.H, r.Y+r.H)
	first := d.rowIndex(max(0, int(math.Floor(float64((top-r.Y+d.ScrollY)/g.lh)))))
	for i := first; i < len(d.rows); i++ {
		y := r.Y + float32(d.top(i))*g.lh - d.ScrollY // 排了上一行才知道这一行从哪开始（折行时）
		if y >= bottom {
			break
		}
		row := d.rows[i]
		if row.Kind == Fold {
			z := 2 // 不在这条上
			if i == d.hover && !d.onBar {
				z = d.hoverZ
			}
			d.paintFold(p, r, g, s, i, y, z)
			continue
		}
		if d.unified {
			side := d.rowSide(row)
			line := lineOf(row, side)
			var w []Span
			if o, ok := d.pairs[side][line]; ok { // 配成对的删 / 加：行里改了的字
				pr := Row{Old: line, New: o}
				if side == 1 {
					pr = Row{Old: o, New: line}
				}
				w = d.wordsOf(pr)[side]
			}
			d.paintCell(p, r, g, s, side, row, w, y, d.rowH(i, s, g))
			continue
		}
		var w [2][]Span
		if row.Kind == Changed {
			w = d.wordsOf(row)
		}
		h := d.rowH(i, s, g)
		for side := range 2 {
			d.paintCell(p, r, g, s, side, row, w[side], y, h)
		}
	}
	if !d.unified { // 中间的线
		p.Fill(wui.Rect{X: r.X + g.paneW[0], Y: r.Y, W: 1, H: r.H}, s.Divider, 0)
	}
	d.paintRuler(p, r, g, s)
}

// paintCell 画 row 在 side 那边的格子（统一视图里是整行：两列行号，字是 side 那边的）。
// h 是这一行多高（折行时几行高）。
func (d *Diff) paintCell(p *wui.Painter, r wui.Rect, g geom, s *Style, side int, row Row, words []Span, y, h float32) {
	x0, pw := r.X+g.paneX[side], g.paneW[side]
	cell := wui.Rect{X: x0, Y: y, W: pw, H: h}
	line, kind := lineOf(row, side), row.Kind
	if line < 0 {
		hatch(p, cell, d.ScrollY+y-r.Y, s.Filler)
		return
	}
	changed := kind == Changed || kind == Deleted || kind == Added
	lineBg, wordBg, mark, sign := s.AddedLine, s.AddedWord, s.AddedMark, "+"
	if side == 0 {
		lineBg, wordBg, mark, sign = s.DeletedLine, s.DeletedWord, s.DeletedMark, "−"
	}
	if changed {
		p.Fill(cell, lineBg, 0)
	}
	caretLine := side == 1 && d.editable && d.focused && d.sel.side == 1 && line == d.sel.b.line && d.sel.empty()
	if caretLine { // 光标那一行：垫一层（VS Code 的 current line highlight），行号亮一点
		p.Fill(cell, s.Code.ActiveLine, 0)
	}
	if d.lsel.has(side, line) || d.unified && kind == Same && d.lsel.has(1-side, lineOf(row, 1-side)) { // 叠在红绿底上看不太出来：左边再画一道竖条
		p.Fill(cell, s.LineSel, 0)
		p.Fill(wui.Rect{X: x0, Y: y, W: 3, H: h}, s.LineMark, 0)
	}
	base := y + (g.lh-(g.asc+g.desc))/2 + g.asc
	nc := s.Code.LineNumber
	switch {
	case changed:
		nc = mark
	case caretLine:
		nc = s.Code.ActiveLineNumber
	}
	number := func(l int, right float32) {
		if l < 0 {
			return
		}
		if num := wui.Shape(strconv.Itoa(l+1), s.Font); len(num) > 0 {
			last := num[len(num)-1]
			p.Glyphs(num, x0+right-(last.X+last.Advance), base, nc)
		}
	}
	if d.unified { // 两列：旧的行号、新的行号（删掉的行只有旧的，加的只有新的）
		number(row.Old, g.numR)
		number(row.New, g.numR2)
	} else {
		number(line, g.numR)
	}
	if changed {
		p.Glyphs(wui.Shape(sign, s.Font), x0+g.markX, base, mark)
	}
	tx := x0 + g.textX
	clip := wui.Rect{X: tx, Y: y, W: max(0, x0+pw-tx), H: h}
	l := d.layout(side, line, s)
	ty := y + (g.lh-l.lt)/2
	vy := l.rowY(y, g.lh) // 排出来的 y 在第几个折出来的行：那一行的顶
	sx := tx - d.scrollX[side]
	p.Clip(clip, 0, func() {
		for _, w := range words {
			a, b := min(w.Start, len(l.u16)-1), min(w.End, len(l.u16)-1)
			if b <= a {
				continue
			}
			for _, rc := range l.l.SelectionRects(wui.TextInputRange{Start: l.u16[a], End: l.u16[b]}) {
				p.Fill(wui.Rect{X: sx + rc.X, Y: vy(rc.Y), W: rc.W, H: g.lh}, wordBg, 2)
			}
		}
		for _, o := range d.occurrences(side, line) { // 和选中的字一样的别处：淡淡的一层
			a, b := min(o, len(l.u16)-1), min(o+len(d.occ), len(l.u16)-1)
			for _, rc := range l.l.SelectionRects(wui.TextInputRange{Start: l.u16[a], End: l.u16[b]}) {
				p.Fill(wui.Rect{X: sx + rc.X, Y: vy(rc.Y) + 1, W: rc.W, H: g.lh - 2}, s.Code.Occurrence, 2)
			}
		}
		for _, k := range d.findSpans(side, row) { // 找到的字：当前那处深一点
			m := d.matches[k]
			a, b := min(m.start, len(l.u16)-1), min(m.end, len(l.u16)-1)
			c := s.Code.Match
			if k == d.findCur {
				c = s.Code.CurrentMatch
			}
			for _, rc := range l.l.SelectionRects(wui.TextInputRange{Start: l.u16[a], End: l.u16[b]}) {
				p.Fill(wui.Rect{X: sx + rc.X, Y: vy(rc.Y) + 1, W: rc.W, H: g.lh - 2}, c, 2)
			}
		}
		if !d.sel.empty() {
			switch {
			case d.sel.side == side:
				d.paintSel(p, l, line, sx, vy, g, s)
			case d.unified && kind == Same: // 统一视图里没改的行两边一样：选的是旧的一边也涂
				d.paintSel(p, l, lineOf(row, d.sel.side), sx, vy, g, s)
			}
		}
		p.TextLayout(l.l, sx, ty, s.Code.Text)
		if side == 1 && d.editable && d.focused && d.sel.side == 1 && line == d.sel.b.line {
			d.paintCaret(p, r, l, sx, vy, g, s)
		}
	})
}

// paintCaret 画光标（2 pt 宽、一行高）；输入法拼着字时字下面画一道线，光标在拼着的字里。
func (d *Diff) paintCaret(p *wui.Painter, r wui.Rect, l *lineLayout, sx float32, vy func(float32) float32, g geom, s *Style) {
	n := len(l.u16) - 1
	off := min(d.sel.b.off, n)
	if d.compose != "" {
		e := min(off+len(d.compose), n)
		for _, rc := range l.l.SelectionRects(wui.TextInputRange{Start: l.u16[off], End: l.u16[e]}) {
			p.Fill(wui.Rect{X: sx + rc.X, Y: vy(rc.Y) + (g.lh+l.lt)/2 - 1, W: rc.W, H: 1}, s.Code.Text, 0)
		}
		cb := 0 // 拼着的字里光标前面的字节数
		for k := 0; cb < len(d.compose) && k < d.composeCaret; k++ {
			_, w := utf8.DecodeRuneInString(d.compose[cb:])
			cb += w
		}
		off = min(off+cb, n)
	}
	cr := l.l.Caret(l.u16[off])
	x, y := sx+cr.X, vy(cr.Y)
	c := s.Code.Cursor
	if c.A == 0 {
		c = s.Code.Text
	}
	p.Fill(wui.Rect{X: x - 1, Y: y, W: 2, H: g.lh}, c, 0)
	d.caretBox = wui.Rect{X: x - 1 - r.X, Y: y - r.Y, W: 2, H: g.lh}
}

// paintSel 涂这一行里选中的那段；选到下一行的，行尾多涂半个字宽（像编辑器里选中的换行）。
func (d *Diff) paintSel(p *wui.Painter, l *lineLayout, line int, sx float32, vy func(float32) float32, g geom, s *Style) {
	a, b := d.sel.span()
	if line < a.line || line > b.line {
		return
	}
	n := len(l.u16) - 1
	from, to := 0, n
	if line == a.line {
		from = min(a.off, n)
	}
	if line == b.line {
		to = min(b.off, n)
	}
	to = max(from, to)
	if to > from {
		for _, rc := range l.l.SelectionRects(wui.TextInputRange{Start: l.u16[from], End: l.u16[to]}) {
			p.Fill(wui.Rect{X: sx + rc.X, Y: vy(rc.Y), W: rc.W, H: g.lh}, s.Code.Selection, 0)
		}
	}
	if line < b.line { // 选到了换行
		c := l.l.Caret(l.u16[to])
		p.Fill(wui.Rect{X: sx + c.X, Y: vy(c.Y), W: g.cw / 2, H: g.lh}, s.Code.Selection, 0)
	}
}

// hatch 在 r 里画 45° 的斜线（对面多出来的行在这边留的空）。cy 是 r 顶上在整个内容里的 y，斜线按内容对齐，滚动时不跳。
func hatch(p *wui.Painter, r wui.Rect, cy float32, c wui.Color) {
	if c.A == 0 {
		return
	}
	const step = 8
	p.Clip(r, 0, func() {
		// 线是 x + 内容 y = k·step
		k := float32(math.Floor(float64(cy/step))) * step
		for ; k <= r.W+cy+r.H; k += step {
			bx := r.X + k - cy - r.H                                               // 底边上的 x
			p.StrokePolyline([][2]float32{{bx, r.Y + r.H}, {bx + r.H, r.Y}}, 1, c) // Line 只画横竖
		}
	})
}

// paintFold 画折叠条。z 是指针在哪：2 不在这条上，0 在条上（点了整段展开），-1 / 1 在那个箭头上。
func (d *Diff) paintFold(p *wui.Painter, r wui.Rect, g geom, s *Style, i int, y float32, z int) {
	row := d.rows[i]
	bg := s.FoldBack
	if z == 0 {
		bg = s.FoldHover
	}
	p.Fill(wui.Rect{X: r.X, Y: y, W: r.W, H: g.lh}, bg, 0)
	label := strconv.Itoa(row.N)
	if s.FoldLabel != nil {
		label = s.FoldLabel(row.N)
	}
	f := s.UIFont
	if f.Size <= 0 {
		f.Size = 12
	}
	m := f.Metrics()
	base := y + (g.lh-(m.Ascent+m.Descent))/2 + m.Ascent
	gs := wui.Shape("⋯  "+label, f)
	var mark, scope string
	if s.FoldScope != nil {
		if old := d.changeAfter(i); old >= 0 {
			mark, scope = s.FoldScope(old)
		}
	}
	var ms, ss []wui.Glyph
	var sf wui.Font
	if scope != "" {
		mf := f
		mf.Weight = 600
		ms = wui.Shape(mark, mf)
		sf = s.Font
		sf.Size -= 0.5
		ss = wui.Shape(scope, sf)
	}
	sc := s.ScopeText
	if sc.A == 0 {
		sc = s.FoldText
	}
	for side := range 2 {
		if d.unified && side == 0 { // 一栏只写一次
			continue
		}
		x0 := r.X + g.paneX[side]
		clip := wui.Rect{X: x0, Y: y, W: g.paneW[side] - rulerW*float32(side), H: g.lh} // 右边那格让出概览条
		p.Clip(clip, 0, func() {
			for _, a := range d.foldArrows(i, g) {
				ax := x0 + a.x
				if z == a.dir {
					p.Fill(wui.Rect{X: ax, Y: y + 2, W: a.w, H: g.lh - 4}, s.FoldText.Alpha(0.18), 3)
				}
				paintFoldArrow(p, ax+a.w/2, y+g.lh/2, a.dir, s.FoldText)
			}
			x := x0 + g.textX
			p.Glyphs(gs, x, base, s.FoldText)
			if scope == "" {
				return
			}
			x += glyphsW(gs) + 14
			if len(ms) > 0 {
				p.Glyphs(ms, x, base, s.FoldText)
				x += glyphsW(ms) + 6
			}
			p.Glyphs(ss, x, base, sc)
		})
	}
}

// foldArrow 是折叠条左边行号那格里的一个箭头：dir -1 接着上面那处改动往下露 FoldStep 行，1 接着下面那处往上露；
// x、w 是在那边的格子里从哪到哪（点的热区）。
type foldArrow struct {
	dir  int
	x, w float32
}

// foldArrows 是 rows[i] 那条折叠上画哪几个箭头：剩的不比 FoldStep 多时一个也不画（点哪都是整段展开）。
// 两个都有时往下露的在左、往上露的在右，只有一个时靠右（挨着正文）。
func (d *Diff) foldArrows(i int, g geom) []foldArrow {
	if i < 0 || i >= len(d.rows) || d.rows[i].Kind != Fold || d.rows[i].N <= FoldStep+1 {
		return nil
	}
	up, down := d.FoldDirs(i)
	const aw = 18
	right := g.textX - 6
	var out []foldArrow
	if up && down {
		out = append(out, foldArrow{-1, max(2, right-2*aw), aw})
	}
	switch {
	case down:
		out = append(out, foldArrow{1, max(2, right-aw), aw})
	case up:
		out = append(out, foldArrow{-1, max(2, right-aw), aw})
	}
	return out
}

// foldZone 是 x（本地坐标）在 rows[i] 那条折叠的哪个箭头上（-1 / 1），别处（或不是折叠）是 0。
func (d *Diff) foldZone(i int, x float32, g geom) int {
	side := 1
	if !d.unified && x < g.paneX[1] {
		side = 0
	}
	xx := x - g.paneX[side]
	for _, a := range d.foldArrows(i, g) {
		if xx >= a.x && xx < a.x+a.w {
			return a.dir
		}
	}
	return 0
}

// paintFoldArrow 画一个展开箭头（GitHub 那样：一道横线挨着露出来的那头，尖朝露的方向）：dir -1 朝下，1 朝上。
func paintFoldArrow(p *wui.Painter, cx, cy float32, dir int, c wui.Color) {
	const w, h = 4, 2.5
	f := float32(-dir) // 朝下时 +1
	bar := cy - f*4.5
	p.StrokePolyline([][2]float32{{cx - w - 1, bar}, {cx + w + 1, bar}}, 1.2, c)
	p.StrokePolyline([][2]float32{{cx - w, cy - f*h + f*1.5}, {cx, cy + f*h + f*1.5}, {cx + w, cy - f*h + f*1.5}}, 1.2, c)
}

// glyphsW 是排好的一串字多宽。
func glyphsW(gs []wui.Glyph) float32 {
	if len(gs) == 0 {
		return 0
	}
	l := gs[len(gs)-1]
	return l.X + l.Advance
}

// changeAfter 是第 i 行（折叠行）之后第一处改动在旧的一边的第几行（加的行算插在前一行后面，即前一行 + 1）；后面没有改动了是 -1。
func (d *Diff) changeAfter(i int) int {
	row := d.rows[i]
	next := row.Old + row.N // 折起来的之后旧的一边的下一行
	for _, r := range d.rows[i+1:] {
		switch r.Kind {
		case Same, Fold:
			next = r.Old + max(r.N, 1)
		case Added:
			return next
		default:
			return r.Old
		}
	}
	return -1
}

func (d *Diff) paintRuler(p *wui.Painter, r wui.Rect, g geom, s *Style) {
	sb := d.bar(g)
	if !sb.Scrolls() {
		return
	}
	rr := wui.Rect{X: r.X + g.ruler.X, Y: r.Y, W: rulerW, H: r.H}
	p.Fill(rr, s.Ruler, 0)
	// 概览：改动在整个文件里的位置，旧的一边（删）画左半、新的一边（加）画右半
	total := g.content + g.lh*3
	mh := max(2, g.lh*r.H/total)
	for i, row := range d.rows {
		if row.Kind == Same || row.Kind == Fold {
			continue
		}
		y := r.Y + float32(d.top(i))*g.lh*r.H/total
		if row.Old >= 0 {
			p.Fill(wui.Rect{X: rr.X + 2, Y: y, W: rulerW/2 - 2, H: mh}, s.DeletedMark, 0)
		}
		if row.New >= 0 {
			p.Fill(wui.Rect{X: rr.X + rulerW/2, Y: y, W: rulerW/2 - 2, H: mh}, s.AddedMark, 0)
		}
	}
	sb.Box = rr
	p.Scrollbar(sb, wui.ScrollbarState{Dragging: d.drag, Hovered: d.onBar})
}
