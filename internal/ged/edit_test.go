package ged

import "testing"

func TestAfter(t *testing.T) {
	for _, c := range []struct {
		p    pos
		text string
		want pos
	}{
		{pos{2, 3}, "ab", pos{2, 5}},
		{pos{2, 3}, "a\nbc", pos{3, 2}},
		{pos{0, 0}, "\n\n", pos{2, 0}},
		{pos{1, 4}, "", pos{1, 4}},
		{pos{0, 1}, "你", pos{0, 4}},
	} {
		if got := after(c.p, c.text); got != c.want {
			t.Errorf("after(%v, %q) = %v，应为 %v", c.p, c.text, got, c.want)
		}
	}
}

// 撤销、重做后光标放到变了的那段末尾。
func TestChangedEnd(t *testing.T) {
	for _, c := range []struct {
		name        string
		before, now []string
		want        pos
	}{
		{"行尾打的字撤掉", []string{"a", "return 100", "}"}, []string{"a", "return 1", "}"}, pos{1, 8}},
		{"重做回来", []string{"a", "return 1", "}"}, []string{"a", "return 100", "}"}, pos{1, 10}},
		{"行中间加字", []string{"foo()"}, []string{"foo(x, y)"}, pos{0, 8}},
		{"整行删掉", []string{"a", "b", "c"}, []string{"a", "c"}, pos{1, 0}},
		{"加一行", []string{"a", "c"}, []string{"a", "b", "c"}, pos{1, 1}},
		{"汉字不切半", []string{"你好"}, []string{"你们好"}, pos{0, 6}},
		{"全删空", []string{"x"}, []string{""}, pos{0, 0}},
	} {
		if got := changedEnd(c.before, c.now); got != c.want {
			t.Errorf("%s：changedEnd = %v，应为 %v", c.name, got, c.want)
		}
	}
}

func TestWordMoves(t *testing.T) {
	d := NewDiff()
	d.lines[1] = []string{"\treturn fmt.Errorf(x)  // 注释", "next"}
	line := d.lines[1][0]
	at := func(s string) int { // s 在第一行第一次出现的字节位置
		for i := range len(line) - len(s) + 1 {
			if line[i:i+len(s)] == s {
				return i
			}
		}
		t.Fatalf("没有 %q", s)
		return 0
	}
	// 从 return 开头往右：return 尾、fmt 尾（跳过空格）、“.” 尾、Errorf 尾
	p := pos{0, 1}
	for _, want := range []int{at("return") + 6, at("fmt") + 3, at(".") + 1, at("Errorf") + 6} {
		if p = d.wordRight(p); p.off != want {
			t.Fatalf("⌥→ 到了 %d，应为 %d", p.off, want)
		}
	}
	if got := d.wordRight(pos{0, len(line)}); got != (pos{1, 0}) {
		t.Errorf("行尾 ⌥→ 该到下一行行首：%v", got)
	}
	if got := d.wordLeft(pos{0, at("fmt") + 2}); got != (pos{0, at("fmt")}) {
		t.Errorf("词中间 ⌥← 到词头：%v", got)
	}
	if got := d.wordLeft(pos{0, at("return")}); got != (pos{0, 0}) {
		t.Errorf("缩进后 ⌥← 到行首：%v", got)
	}
	if got := d.wordLeft(pos{0, at("//")}); got.off >= at("//") || got.off < at(")") {
		t.Errorf("跳过空白：%v", got)
	}
	if got := d.wordLeft(pos{1, 0}); got != (pos{0, len(line)}) {
		t.Errorf("行首 ⌥← 该到上一行行尾：%v", got)
	}
}
