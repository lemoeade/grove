package ged

import (
	"math/rand/v2"
	"reflect"
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
