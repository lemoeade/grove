package ged

import (
	"math/rand/v2"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// Myers 找到的是最长公共子序列：和动态规划算的一样长，配上的位置递增、两边相等。
func TestMyersLCS(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for range 500 {
		a, b := make([]int, r.IntN(30)), make([]int, r.IntN(30))
		for i := range a {
			a[i] = r.IntN(4)
		}
		for i := range b {
			b[i] = r.IntN(4)
		}
		got := matches(a, b)
		pi, pj := -1, -1
		for _, p := range got {
			if p[0] <= pi || p[1] <= pj || a[p[0]] != b[p[1]] {
				t.Fatalf("%v %v：配对不对 %v", a, b, got)
			}
			pi, pj = p[0], p[1]
		}
		if want := lcs(a, b); len(got) != want {
			t.Fatalf("%v %v：配了 %d 个，最长公共子序列是 %d", a, b, len(got), want)
		}
	}
}

func lcs(a, b []int) int {
	dp := make([][]int, len(a)+1)
	for i := range dp {
		dp[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else {
				dp[i][j] = max(dp[i+1][j], dp[i][j+1])
			}
		}
	}
	return dp[0][0]
}

func TestAlign(t *testing.T) {
	old := strings.Split("a b c d e", " ")
	new := strings.Split("a B c X Y e f", " ")
	want := []Row{
		{Kind: Same, Old: 0, New: 0},
		{Kind: Changed, Old: 1, New: 1},
		{Kind: Same, Old: 2, New: 2},
		{Kind: Changed, Old: 3, New: 3}, // d → X Y：配一对，多的一行对面留空
		{Kind: Added, Old: -1, New: 4},
		{Kind: Same, Old: 4, New: 5},
		{Kind: Added, Old: -1, New: 6},
	}
	if got := Align(old, new); !reflect.DeepEqual(got, want) {
		t.Errorf("Align =\n%v\n应为\n%v", got, want)
	}
	if got := Align([]string{"x", "y"}, nil); !reflect.DeepEqual(got, []Row{{Deleted, 0, -1, 0}, {Deleted, 1, -1, 0}}) {
		t.Errorf("整个删掉：%v", got)
	}
}

func TestFolded(t *testing.T) {
	var old, new []string
	for i := range 20 {
		old = append(old, string(rune('a'+i)))
	}
	new = append(new, old...)
	new[10] = "改"
	rows := Align(old, new)
	got := Folded(rows, 3, nil)
	// 0..6 折（7 行），7 8 9 留着，10 改，11 12 13 留着，14..19 折（6 行）
	want := []Row{{Fold, 0, 0, 7}, {Same, 7, 7, 0}, {Same, 8, 8, 0}, {Same, 9, 9, 0}, {Changed, 10, 10, 0},
		{Same, 11, 11, 0}, {Same, 12, 12, 0}, {Same, 13, 13, 0}, {Fold, 14, 14, 6}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Folded =\n%v\n应为\n%v", got, want)
	}
	// 展开开头那段
	if got := Folded(rows, 3, map[int]bool{0: true}); got[0] != (Row{Same, 0, 0, 0}) || len(got) != 15 {
		t.Errorf("展开开头：%v", got)
	}
	// 没有改动不折；只多一行的不折
	if got := Folded(Align(old, old), 3, nil); len(got) != 20 {
		t.Errorf("没有改动也折了：%v", got)
	}
	if got := Folded(Align(old[:5], append(append([]string{}, old[:4]...), "改")), 3, nil); got[0].Kind != Same {
		t.Errorf("只多一行的折了：%v", got)
	}
}

func TestWords(t *testing.T) {
	o, n := Words("\tn += 20 // 第 20 步", "\tn *= 20 // 第 20 步，翻倍")
	if want := []Span{{3, 4}}; !reflect.DeepEqual(o, want) {
		t.Errorf("旧：%v", o)
	}
	// 新：* 和 “，翻倍”
	if len(n) != 2 || "\tn *= 20 // 第 20 步，翻倍"[n[0].Start:n[0].End] != "*" ||
		"\tn *= 20 // 第 20 步，翻倍"[n[1].Start:n[1].End] != "，翻倍" {
		t.Errorf("新：%v", n)
	}
	// 几乎全不一样：整行
	if o, n := Words("alpha beta", "gamma delta"); !reflect.DeepEqual(o, []Span{{0, 10}}) || !reflect.DeepEqual(n, []Span{{0, 11}}) {
		t.Errorf("全不一样：%v %v", o, n)
	}
	if o, n := Words("", "x"); o != nil || !reflect.DeepEqual(n, []Span{{0, 1}}) {
		t.Errorf("空行变 x：%v %v", o, n)
	}
	// 整行算改了时不涂缩进
	if o, n := Words("\talpha", "\tgamma delta "); !reflect.DeepEqual(o, []Span{{1, 6}}) || !reflect.DeepEqual(n, []Span{{1, 12}}) {
		t.Errorf("整行不涂缩进：%v %v", o, n)
	}
}

// 很大、几乎全换了的也不卡：超过 diffMax 后中间不配，照样一行行对齐。
func TestAlignHuge(t *testing.T) {
	var a, b []string
	for i := range 20000 {
		a = append(a, "a"+strings.Repeat("x", i%7))
		b = append(b, "b"+strings.Repeat("y", i%5))
	}
	rows := Align(a, b)
	if len(rows) != 20000 || rows[0].Kind != Changed {
		t.Fatalf("%d 行，第一行 %v", len(rows), rows[0])
	}
}

func TestHunks(t *testing.T) {
	old := strings.Split("a b c d e", " ")
	new := strings.Split("a B c X Y e f", " ")
	got := Hunks(Align(old, new))
	want := []Hunk{
		{Row0: 1, Row1: 2, OldStart: 1, OldEnd: 2, NewStart: 1, NewEnd: 2}, // b → B
		{Row0: 3, Row1: 5, OldStart: 3, OldEnd: 4, NewStart: 3, NewEnd: 5}, // d → X Y
		{Row0: 6, Row1: 7, OldStart: 5, OldEnd: 5, NewStart: 6, NewEnd: 7}, // 末尾加 f：旧的一边插在最后
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Hunks =\n%v\n应为\n%v", got, want)
	}
	if got := Hunks(Align([]string{"x", "y", "z"}, []string{"x", "z"})); !reflect.DeepEqual(got, []Hunk{{1, 2, 1, 2, 1, 1}}) {
		t.Errorf("删一行：%v", got)
	}
}

func TestUnified(t *testing.T) {
	old := strings.Split("a b c d e", " ")
	new := strings.Split("a B c X Y e f", " ")
	want := []Row{
		{Kind: Same, Old: 0, New: 0},
		{Kind: Deleted, Old: 1, New: -1}, {Kind: Added, Old: -1, New: 1}, // b → B
		{Kind: Same, Old: 2, New: 2},
		{Kind: Deleted, Old: 3, New: -1}, {Kind: Added, Old: -1, New: 3}, {Kind: Added, Old: -1, New: 4}, // d → X Y：先删后加
		{Kind: Same, Old: 4, New: 5},
		{Kind: Added, Old: -1, New: 6},
	}
	if got := Unified(Align(old, new)); !reflect.DeepEqual(got, want) {
		t.Errorf("Unified =\n%v\n应为\n%v", got, want)
	}
	// 折叠照旧
	var a, b []string
	for i := range 20 {
		a = append(a, string(rune('a'+i)))
	}
	b = append(b, a...)
	b[10] = "改"
	got := Unified(Folded(Align(a, b), 3, nil))
	if len(got) != 10 || got[0].Kind != Fold || got[4] != (Row{Deleted, 10, -1, 0}) || got[5] != (Row{Added, -1, 10, 0}) || got[9].Kind != Fold {
		t.Errorf("折过的再排成一栏：%v", got)
	}
}

// 折叠条往下 / 往上露几行：开头那段只能往上露（接着下面的改动），中间的两头都能，露到剩的不多时整段展开。
func TestFoldReveal(t *testing.T) {
	var old []string
	for i := range 100 {
		old = append(old, strconv.Itoa(i))
	}
	new := append([]string{}, old...)
	new[30], new[70] = "改", "又改"
	rows := Align(old, new)
	out, infos := foldRows(rows, 3, nil)
	var folds []Row
	for _, r := range out {
		if r.Kind == Fold {
			folds = append(folds, r)
		}
	}
	// 0..26 折（27 行），34..66 折（33 行），74..99 折（26 行）
	if len(folds) != 3 || folds[0].N != 27 || folds[1].Old != 34 || folds[1].N != 33 || folds[2].N != 26 {
		t.Fatalf("折出来的段：%v", folds)
	}
	if f := infos[0]; f.up || !f.down || f.key != 0 {
		t.Errorf("开头那段：%+v", f)
	}
	if f := infos[34]; !f.up || !f.down || f.key != 31 {
		t.Errorf("中间那段：%+v", f)
	}
	if f := infos[74]; !f.up || f.down || f.key != 71 {
		t.Errorf("结尾那段：%+v", f)
	}
	// 中间那段接着上面的改动往下露 20 行：折的从 54 起、剩 13 行
	out, _ = foldRows(rows, 3, map[int]Reveal{31: {Top: 20}})
	for _, r := range out {
		if r.Kind == Fold && r.Old >= 31 && r.Old < 71 && (r.Old != 54 || r.N != 13) {
			t.Errorf("往下露 20 行后：%v", r)
		}
	}
	// 再从下面往上露 20 行：剩的不到两行，整段展开
	out, _ = foldRows(rows, 3, map[int]Reveal{31: {Top: 20, Bottom: 20}})
	for _, r := range out {
		if r.Kind == Fold && r.Old >= 31 && r.Old < 71 {
			t.Errorf("两头都露了还折着：%v", r)
		}
	}
}

// ExpandMore：点箭头露 FoldStep 行，剩的不比它多时整段展开；FoldDirs 跟着段走。
func TestExpandMore(t *testing.T) {
	var old []string
	for i := range 100 {
		old = append(old, strconv.Itoa(i))
	}
	new := append([]string{}, old...)
	new[60] = "改"
	d := NewDiff()
	d.Set(1, func() ([]string, []string) { return old, new }, nil)
	if r := d.Rows()[0]; r.Kind != Fold || r.N != 57 {
		t.Fatalf("开头：%v", r)
	}
	if up, down := d.FoldDirs(0); up || !down {
		t.Fatalf("开头那段能往哪露：up=%v down=%v", up, down)
	}
	d.ExpandMore(0, 1)
	if r := d.Rows()[0]; r.Kind != Fold || r.N != 37 || d.Rows()[1].Old != 37 {
		t.Fatalf("往上露 20 行后：%v %v", r, d.Rows()[1])
	}
	d.ExpandMore(0, 1)
	if r := d.Rows()[0]; r.Kind != Fold || r.N != 17 {
		t.Fatalf("再露 20 行后：%v", r)
	}
	d.ExpandMore(0, 1) // 剩 17 行，不比 20 多：整段展开
	if r := d.Rows()[0]; r.Kind != Same || r.Old != 0 {
		t.Fatalf("剩的不多时该整段展开：%v", r)
	}
	last := len(d.Rows()) - 1
	if up, down := d.FoldDirs(last); !up || down {
		t.Fatalf("结尾那段能往哪露：up=%v down=%v", up, down)
	}
	if !d.ExpandMore(last, 0) || d.Rows()[len(d.Rows())-1].Kind == Fold {
		t.Fatal("整段展开结尾那段")
	}
	if d.ExpandMore(0, 0) {
		t.Fatal("不是折叠的行不该展开")
	}
}

// 忽略行首行尾空白：只改了缩进、行尾空格的行算没改，真改了的照旧。
func TestAlignTrim(t *testing.T) {
	old := []string{"func f() {", "if x {", "return 1", "}", "}"}
	new := []string{"func f() {", "\tif x {  ", "\t\treturn 2", "\t}", "}"}
	kinds := func(rows []Row) (n int) {
		for _, r := range rows {
			if r.Kind != Same {
				n++
			}
		}
		return n
	}
	if n := kinds(Align(old, new)); n != 3 {
		t.Errorf("不忽略空白时改了 3 行，算出 %d", n)
	}
	rows := AlignTrim(old, new, true)
	if n := kinds(rows); n != 1 || rows[2].Kind != Changed {
		t.Errorf("忽略空白时只改了 return 那行：%v", rows)
	}
	d := NewDiff()
	d.Set(1, func() ([]string, []string) { return old, new }, nil)
	if len(d.Hunks()) != 1 || d.Hunks()[0].NewEnd-d.Hunks()[0].NewStart != 3 {
		t.Fatalf("不忽略：%+v", d.Hunks())
	}
	d.SetIgnoreTrim(true)
	if h := d.Hunks(); len(h) != 1 || h[0].NewStart != 2 || h[0].NewEnd != 3 {
		t.Fatalf("忽略后只剩 return 那一处：%+v", h)
	}
	d.SetIgnoreTrim(false)
	if h := d.Hunks(); h[0].NewEnd-h[0].NewStart != 3 {
		t.Fatalf("关掉后还原：%+v", h)
	}
}

func TestOccurrences(t *testing.T) {
	old := []string{"foo := bar(foo)", "x"}
	new := []string{"foo := bar(foo)", "y foo", "  "}
	d := NewDiff()
	d.Set(1, func() ([]string, []string) { return old, new }, nil)
	d.sel = selection{side: 1, a: pos{0, 0}, b: pos{0, 3}} // 选中新的一边第一行的 foo
	d.occ = d.occurText()
	if d.occ != "foo" {
		t.Fatalf("occurText = %q", d.occ)
	}
	if got := d.Occurrences(1, 0); len(got) != 1 || got[0] != 11 {
		t.Errorf("同一行里另一处 foo：%v", got)
	}
	if got := d.Occurrences(1, 1); len(got) != 1 || got[0] != 2 {
		t.Errorf("下一行的 foo：%v", got)
	}
	if got := d.Occurrences(0, 0); len(got) != 2 {
		t.Errorf("旧的一边两处都涂：%v", got)
	}
	d.sel = selection{side: 1, a: pos{2, 0}, b: pos{2, 2}} // 全是空白：不涂
	if d.occurText() != "" {
		t.Error("全是空白的不该涂别处")
	}
	d.sel = selection{side: 1, a: pos{0, 0}, b: pos{1, 2}} // 跨行：不涂
	if d.occurText() != "" {
		t.Error("跨行的不该涂别处")
	}
}
