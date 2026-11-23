package ged

import (
	"slices"
	"strings"
	"testing"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

var goSrc = strings.Split(`package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Println(os.Args[1])
	}
	fmt.Println("hi") // {
}

def:
    a
    b

tail`, "\n")

func plainEd(t *testing.T, lines []string) *Diff {
	t.Helper()
	d := NewEditor()
	d.SetText(1, func() []string { return lines }, editor.LanguageFor("main.go"))
	return d
}

// 单栏：每行一个 Same，Old == New，没有改动。
func TestEditorRows(t *testing.T) {
	d := plainEd(t, goSrc)
	if len(d.Rows()) != len(goSrc) || len(d.Hunks()) != 0 {
		t.Fatalf("行 %d（应 %d）、改动 %d", len(d.Rows()), len(goSrc), len(d.Hunks()))
	}
	for i, r := range d.Rows() {
		if r.Kind != Same || r.Old != i || r.New != i {
			t.Fatalf("第 %d 行：%+v", i, r)
		}
	}
}

// 折到哪：行尾开括号折到闭括号前一行；按缩进的折到缩进深的最后一行；不能折的不折。
func TestEditorFoldEnd(t *testing.T) {
	d := plainEd(t, goSrc)
	for _, c := range []struct {
		line, end int
		ok        bool
	}{
		{2, 4, true},   // import ( → ) 前
		{7, 11, true},  // func main() { → } 前
		{8, 9, true},   // if … { 只有一行在里面
		{11, 0, false}, // 行尾注释里的 { 不算
		{14, 16, true}, // def: 按缩进
		{0, 0, false},
		{1, 0, false}, // 空行
	} {
		end, ok := d.foldEnd(c.line)
		if ok != c.ok || ok && end != c.end {
			t.Errorf("第 %d 行：折到 %d（%v），应为 %d（%v）", c.line, end, ok, c.end, c.ok)
		}
		if f := d.foldable(c.line); f != c.ok {
			t.Errorf("第 %d 行 foldable = %v", c.line, f)
		}
	}
}

// 折起、展开、光标挪出藏起来的行、上下挪跳过折着的行、改了上面的行折叠跟着挪。
func TestEditorFolding(t *testing.T) {
	d := plainEd(t, goSrc)
	d.PlaceCaret(9, 3)
	if !d.Fold(7) || !d.IsFolded(7) {
		t.Fatal("折不起 func main")
	}
	if len(d.Rows()) != len(goSrc)-4 {
		t.Fatalf("折了 4 行，还剩 %d 行", len(d.Rows()))
	}
	if d.sel.b != (pos{7, len(goSrc[7])}) {
		t.Errorf("光标该挪到折起来那行的行尾：%v", d.sel.b)
	}
	if got := d.stepLine(7, 1); got != 12 {
		t.Errorf("从折着的行往下一行该到 12（闭括号）：%d", got)
	}
	if got := d.stepLine(12, -1); got != 7 {
		t.Errorf("从 12 往上一行该到 7：%d", got)
	}
	if d.plainRow(9) != -1 || d.plainRow(12) != 8 {
		t.Errorf("plainRow：%d %d", d.plainRow(9), d.plainRow(12))
	}
	// 上面加两行：折叠跟着挪到 9
	more := append([]string{"// a", "// b"}, goSrc...)
	d.SetText(2, func() []string { return more }, d.lang)
	if !slices.Equal(d.Folded(), []int{9}) || d.folded[9] != 13 {
		t.Fatalf("加了两行后折叠：%v %v", d.Folded(), d.folded)
	}
	// 光标去藏着的行：展开
	if d.rowOfLine(1, 11) < 0 || d.IsFolded(9) {
		t.Errorf("挪到藏着的行该展开")
	}
	d.FoldAll() // 里面的 if 也折（展开外层时它还折着）
	if !slices.Equal(d.Folded(), []int{4, 9, 10, 16}) {
		t.Errorf("全部折起：%v", d.Folded())
	}
	if len(d.Rows()) != len(more)-2-4-2 { // import 里两行、func main 里四行、def 下两行
		t.Errorf("全部折起后 %d 行", len(d.Rows()))
	}
	d.foldLevel(2) // 第二层：只有 if
	if !slices.Equal(d.Folded(), []int{10}) {
		t.Errorf("折叠级别 2：%v", d.Folded())
	}
	d.UnfoldAll()
	d.PlaceCaret(10, 0)
	if !d.FoldRecursive() || !slices.Equal(d.Folded(), []int{10}) {
		t.Errorf("光标在 if 上递归折叠：%v", d.Folded())
	}
	d.PlaceCaret(12, 0)
	d.UnfoldAll()
	d.PlaceCaret(12, 0) // func main 里、if 外
	if !d.FoldRecursive() || !slices.Equal(d.Folded(), []int{9, 10}) {
		t.Errorf("递归折叠 func main：%v", d.Folded())
	}
	d.PlaceCaret(9, 0)
	if !d.UnfoldRecursive() || len(d.Folded()) != 0 {
		t.Errorf("递归展开：%v", d.Folded())
	}
	d.UnfoldAll()
	if len(d.Folded()) != 0 || len(d.Rows()) != len(more) {
		t.Errorf("全部展开：%v", d.Folded())
	}
	// 头一行变得不能折了：丢掉
	d.Fold(16)
	gone := slices.Clone(more)
	gone[16] = "x"
	d.SetText(3, func() []string { return gone }, d.lang)
	if d.IsFolded(16) {
		t.Errorf("不能折了的还折着")
	}
}

// 括号按层上色：跨行接着数，字符串、注释里的括号不算，多出来的右括号不让层数变负。
func TestBracketLevels(t *testing.T) {
	d := NewEditor()
	lines := []string{"func f() {", "\ts := \"(\" // )", "\tg(a[0])", "}", "}"}
	d.SetText(1, func() []string { return lines }, editor.LanguageFor("main.go"))
	type mark struct{ off, level int }
	got := func(line int) []mark {
		var out []mark
		src := lines[line]
		toks, _ := d.lang.Tokenizer.Tokenize(src, d.stateAt(1, line), nil)
		d.walkBrackets(src, toks, d.depthAt(1, line), func(off, level int) { out = append(out, mark{off, level}) })
		return out
	}
	if m := got(0); !slices.Equal(m, []mark{{6, 0}, {7, 0}, {9, 0}}) {
		t.Errorf("第 1 行 %v", m)
	}
	if m := got(1); len(m) != 0 {
		t.Errorf("字符串、注释里的括号也算了：%v", m)
	}
	if m := got(2); !slices.Equal(m, []mark{{2, 1}, {4, 2}, {6, 2}, {7, 1}}) {
		t.Errorf("第 3 行 %v", m)
	}
	if d.depthAt(1, 3) != 1 || d.depthAt(1, 4) != 0 || d.walkBrackets("}", nil, 0, nil) != 0 {
		t.Errorf("层数 %d %d", d.depthAt(1, 3), d.depthAt(1, 4))
	}
}

// 小地图：只有单栏 + Minimap + 够宽时有，在概览条左边、正文让出它；行按词的颜色成条（空白不画），
// 按在滑块外把滑块挪到指针下、拖着滚，滚到底时小地图也到底。
func TestMinimap(t *testing.T) {
	var lines []string
	for i := range 2000 {
		lines = append(lines, "\tfmt.Println(\"x\", "+strings.Repeat("a", i%50)+")")
	}
	d := plainEd(t, lines)
	s := &Style{Code: editor.DefaultTheme(wui.LightTheme()), Font: wui.Font{Family: "monospace", Size: 13}}
	if g := d.geom(s, 800, 600, 1); g.mm.W != 0 || g.right != rulerW {
		t.Fatalf("没开 Minimap 也有小地图：%+v", g.mm)
	}
	s.Minimap = true
	if g := d.geom(s, 300, 600, 1); g.mm.W != 0 {
		t.Fatal("太窄也画了小地图")
	}
	g := d.geom(s, 800, 600, 1)
	if g.mm.W != minimapW || g.mm.X != 800-rulerW-minimapW || g.right != rulerW+minimapW {
		t.Fatalf("小地图的位置 %+v right %v", g.mm, g.right)
	}
	d.style, d.w, d.h, d.lh = s, 800, 600, g.lh

	runs := d.mmRuns(0, s)
	if len(runs) < 2 || runs[0].c0 != 4 { // Tab 展开成 4 格，后面有不同颜色的词
		t.Fatalf("第 1 行的条：%+v", runs)
	}
	for i := 1; i < len(runs); i++ {
		if runs[i].c0 < runs[i-1].c1 {
			t.Fatalf("条叠在一起：%+v", runs)
		}
	}

	// 按在小地图中间（滑块外）：滑块挪到指针下，接着拖
	y := float32(300)
	d.input(wui.InputEvent{Kind: wui.InputPointerDown, X: g.mm.X + 10, Y: y, Clicks: 1}, s)
	if !d.mm.drag || d.ScrollY <= 0 {
		t.Fatalf("按下后 drag=%v ScrollY=%v", d.mm.drag, d.ScrollY)
	}
	top, h := d.mmSlider(g)
	if y < top || y > top+h {
		t.Fatalf("按下的点 %v 不在滑块 [%v, %v] 里", y, top, top+h)
	}
	before := d.ScrollY
	d.input(wui.InputEvent{Kind: wui.InputPointerMove, X: g.mm.X + 10, Y: y + 50}, s)
	if d.ScrollY <= before {
		t.Fatalf("往下拖没往下滚：%v → %v", before, d.ScrollY)
	}
	d.input(wui.InputEvent{Kind: wui.InputPointerMove, X: g.mm.X + 10, Y: 10000}, s)
	d.input(wui.InputEvent{Kind: wui.InputPointerUp, X: g.mm.X + 10, Y: 10000}, s)
	if d.mm.drag {
		t.Fatal("松开后还在拖")
	}
	most := g.content - g.h + d.tail(g)
	if d.ScrollY < most-1 {
		t.Fatalf("拖到底没滚到底：%v（应 %v）", d.ScrollY, most)
	}
	if sc, total := d.mmScroll(g), g.content*mmLH/g.lh; sc < total-g.h-1 {
		t.Fatalf("编辑器到底了小地图没到底：%v（应 %v）", sc, total-g.h)
	}
	if !d.sel.empty() {
		t.Fatal("在小地图上按、拖动了选择")
	}
}

// RowOf / LineAtRow 在源码行和屏幕行之间换（并排的 Markdown 预览按屏幕行对齐）：折起来藏着的行不占屏幕行，
// 折成几行的行占几行，小数按它占的屏幕行摊；两个互为反函数，也不展开折叠。
func TestEditorRowOf(t *testing.T) {
	d := plainEd(t, goSrc)
	d.Fold(7) // 藏起 8–11
	for _, c := range []struct{ line, row float64 }{{0, 0}, {7, 7}, {9, 8}, {12, 8}, {12.5, 8.5}} {
		if got := d.RowOf(c.line); got != c.row {
			t.Errorf("折起后 RowOf(%v) = %v，应为 %v", c.line, got, c.row)
		}
	}
	if got := d.LineAtRow(8.5); got != 12.5 {
		t.Errorf("折起后 LineAtRow(8.5) = %v，应为 12.5", got)
	}
	if !d.IsFolded(7) {
		t.Error("RowOf 不该展开折叠")
	}
	d.UnfoldAll()
	// 第 0 行估的折成三个屏幕行；排出来是两行（tops）也不改 RowOf：排出来的数是滚到眼前才有的，
	// 拿它算的话往下滚时一块里的屏幕行数一帧一帧地变，预览一跳一跳
	d.wrap, d.topsDirty, d.etopsDirty = true, false, false
	d.tops, d.etops = d.tops[:0], d.etops[:0]
	for i := range len(d.rows) + 1 {
		d.etops = append(d.etops, i+min(i, 1)*2)
		d.tops = append(d.tops, i+min(i, 1))
	}
	for _, c := range []struct{ line, row float64 }{{0, 0}, {0.5, 1.5}, {1, 3}, {2.25, 4.25}} {
		if got := d.RowOf(c.line); got != c.row {
			t.Errorf("折行后 RowOf(%v) = %v，应为 %v", c.line, got, c.row)
		}
		if got := d.LineAtRow(c.row); got != c.line {
			t.Errorf("折行后 LineAtRow(%v) = %v，应为 %v", c.row, got, c.line)
		}
	}
	// TopLine 还是按排出来的：ScrollY 是真的屏幕行
	d.lh, d.h, d.ScrollY = 10, 100, 10
	if got, _ := d.TopLine(); got != 0.5 {
		t.Errorf("排出来两行时滚了一行 TopLine = %v，应为 0.5", got)
	}
}

// 文件短（不用上下滚，概览条不铺底）、有一行很长：正文的字不画到小地图和概览条底下，
// 概览条那一列整列还是底色（之前字一直画到窗口最右边，从概览条那列露出半截字）。
func TestMinimapClipsText(t *testing.T) {
	long := strings.Repeat("WWWW 应用平台工程规范 ", 40)
	d := plainEd(t, []string{"a: 1", "b: 2", long, "c: 3", ""})
	th := editor.DefaultTheme(wui.LightTheme())
	const w, h = 600, 400
	view := func(c *wui.Context) {
		wui.Column(c).Width(w).Height(h).Children(func() {
			View(c, "ed", d, Style{Code: th, Font: wui.Font{Family: "monospace", Size: 13}, Minimap: true})
		})
	}
	tt := wui.NewTester(view, w, h)
	tt.Frame()
	if d.lh <= 0 {
		t.Fatal("没画出来")
	}
	img := tt.Image()
	k := float32(img.Bounds().Dx()) / w
	px := func(x, y float32) [4]uint8 {
		c := img.RGBAAt(int(x*k), int(y*k))
		return [4]uint8{c.R, c.G, c.B, c.A}
	}
	empty := 4*d.lh + d.lh/2 // 第 5 行是空的：同一列的底色
	for x := float32(w - rulerW); x < w; x++ {
		for y := 2 * d.lh; y < 3*d.lh; y++ {
			if got, want := px(x, y), px(x, empty); got != want {
				t.Fatalf("概览条那列 (%v, %v) 画上了字：%v（底色 %v）", x, y, got, want)
			}
		}
	}
}
