package css

import (
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// node 是测试用的小 DOM。
type node struct {
	tag    string
	attrs  map[string]string
	parent *node
	kids   []*node
}

func (n *node) Tag() string { return n.tag }

func (n *node) Attr(name string) (string, bool) {
	v, ok := n.attrs[name]
	return v, ok
}

func (n *node) Parent() Element {
	if n.parent == nil {
		return nil
	}
	return n.parent
}

func (n *node) sibling(d int) Element {
	if n.parent == nil {
		return nil
	}
	i := slices.Index(n.parent.kids, n) + d
	if i < 0 || i >= len(n.parent.kids) {
		return nil
	}
	return n.parent.kids[i]
}

func (n *node) Prev() Element { return n.sibling(-1) }
func (n *node) Next() Element { return n.sibling(1) }

// el 造一个元素：attrs 是 "k=v k2=v2"（值里不能有空格，要空格的直接改 attrs）。
func el(tag, attrs string, kids ...*node) *node {
	n := &node{tag: tag, attrs: map[string]string{}}
	for _, kv := range strings.Fields(attrs) {
		k, v, _ := strings.Cut(kv, "=")
		n.attrs[k] = v
	}
	for _, k := range kids {
		k.parent = n
		n.kids = append(n.kids, k)
	}
	return n
}

// styleOf 从根往下一层层算，返回 target 的样式。
func styleOf(sh *Sheet, target *node, pseudo string) *Style {
	var chain []*node
	for n := target; n != nil; n = n.parent {
		chain = append(chain, n)
	}
	var st *Style
	for i := len(chain) - 1; i >= 0; i-- {
		n := chain[i]
		st = Compute(sh, n, "", ParseDecls(n.attrs["style"]), st, 16)
	}
	if pseudo != "" {
		// 伪元素的父样式是元素自己。
		st = Compute(sh, target, pseudo, nil, st, 16)
	}
	return st
}

func TestParseColor(t *testing.T) {
	cases := []struct {
		in   string
		want Color
		ok   bool
	}{
		{"#fff", Color{255, 255, 255, 1}, true},
		{"#F00", Color{255, 0, 0, 1}, true},
		{"#11223344", Color{0x11, 0x22, 0x33, float32(0x44) / 255}, true},
		{"#1234", Color{0x11, 0x22, 0x33, float32(0x44) / 255}, true},
		{"rgb(1, 2, 3)", Color{1, 2, 3, 1}, true},
		{"rgba(10,20,30,.5)", Color{10, 20, 30, 0.5}, true},
		{"rgb(10 20 30 / 25%)", Color{10, 20, 30, 0.25}, true},
		{"rgb(100%, 0%, 50%)", Color{255, 0, 128, 1}, true},
		{"hsl(0, 100%, 50%)", Color{255, 0, 0, 1}, true},
		{"hsl(120deg 100% 25%)", Color{0, 128, 0, 1}, true},
		{"hsla(240, 100%, 50%, 0.5)", Color{0, 0, 255, 0.5}, true},
		{"RebeccaPurple", Color{0x66, 0x33, 0x99, 1}, true},
		{"transparent", Color{}, true},
		{"currentcolor", Color{}, false},
		{"#ggg", Color{}, false},
		{"rgb(1,2)", Color{}, false},
		{"", Color{}, false},
		{"notacolor", Color{}, false},
	}
	for _, c := range cases {
		got, ok := ParseColor(c.in)
		if ok != c.ok || ok && (got.R != c.want.R || got.G != c.want.G || got.B != c.want.B || math.Abs(float64(got.A-c.want.A)) > 0.01) {
			t.Errorf("ParseColor(%q) = %v %v，要 %v %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseLength(t *testing.T) {
	cases := []struct {
		in   string
		want Length
		ok   bool
	}{
		{"12px", Length{12, Px}, true},
		{"12pt", Length{16, Px}, true},
		{"1.5em", Length{1.5, Em}, true},
		{"2rem", Length{2, Rem}, true},
		{"50%", Length{50, Percent}, true},
		{"1fr", Length{1, Fr}, true},
		{"auto", Length{0, Auto}, true},
		{"0", Length{0, Px}, true},
		{"1.45", Length{1.45, None}, true},
		{"-4px", Length{-4, Px}, true},
		{".5em", Length{0.5, Em}, true},
		{"10vw", Length{10, Vw}, true},
		{"1e1px", Length{10, Px}, true},
		{"calc(1px + 2em)", Length{}, false},
		{"px", Length{}, false},
		{"12parsecs", Length{}, false},
		{"", Length{}, false},
	}
	for _, c := range cases {
		got, ok := ParseLength(c.in)
		if ok != c.ok || ok && (got.U != c.want.U || math.Abs(float64(got.V-c.want.V)) > 1e-4) {
			t.Errorf("ParseLength(%q) = %v %v，要 %v %v", c.in, got, ok, c.want, c.ok)
		}
	}
	if v := (Length{2, Em}).Resolve(10, 16, 300); v != 20 {
		t.Errorf("2em @10 = %v", v)
	}
	if v := (Length{2, Rem}).Resolve(10, 16, 300); v != 32 {
		t.Errorf("2rem = %v", v)
	}
	if v := (Length{50, Percent}).Resolve(10, 16, 300); v != 150 {
		t.Errorf("50%% of 300 = %v", v)
	}
	if v := (Length{1.5, None}).Resolve(10, 16, 300); v != 1.5 {
		t.Errorf("unitless = %v", v)
	}
	if v := (Length{U: Auto}).Resolve(10, 16, 300); v != 0 {
		t.Errorf("auto = %v", v)
	}
}

func TestParseTracks(t *testing.T) {
	tr, ok := ParseTracks("200px repeat(2, 1fr) minmax(100px, 2fr) auto fit-content(40%)")
	if !ok || len(tr) != 6 {
		t.Fatalf("got %v %v", tr, ok)
	}
	if tr[0].Min != (Length{200, Px}) || tr[0].Max != (Length{200, Px}) {
		t.Errorf("200px → %v", tr[0])
	}
	if tr[1].Min.U != Auto || tr[1].Max != (Length{1, Fr}) || tr[2] != tr[1] {
		t.Errorf("repeat(2, 1fr) → %v %v", tr[1], tr[2])
	}
	if tr[3].Min != (Length{100, Px}) || tr[3].Max != (Length{2, Fr}) {
		t.Errorf("minmax → %v", tr[3])
	}
	if tr[4].Min.U != Auto || tr[4].Max.U != Auto {
		t.Errorf("auto → %v", tr[4])
	}
	if tr[5].Min.U != Auto || tr[5].Max != (Length{40, Percent}) {
		t.Errorf("fit-content → %v", tr[5])
	}
	for _, bad := range []string{"", "repeat(auto-fill, 100px)", "1.5", "minmax(1fr, 2fr)", "calc(1px)"} {
		if _, ok := ParseTracks(bad); ok {
			t.Errorf("ParseTracks(%q) 应该读不出", bad)
		}
	}
}

func TestFieldsAndComma(t *testing.T) {
	if got := Fields(" 1px  solid rgb(0, 0, 0) "); !slices.Equal(got, []string{"1px", "solid", "rgb(0, 0, 0)"}) {
		t.Errorf("Fields = %q", got)
	}
	if got := Fields(`"Fira Code", monospace`); !slices.Equal(got, []string{`"Fira Code",`, "monospace"}) {
		t.Errorf("Fields 引号 = %q", got)
	}
	if got := SplitComma("a, rgb(1,2,3) , 'x,y'"); !slices.Equal(got, []string{"a", "rgb(1,2,3)", "'x,y'"}) {
		t.Errorf("SplitComma = %q", got)
	}
}

func TestParseDecls(t *testing.T) {
	ds := ParseDecls(`color: red; /* c */ Margin : 1px 2px !important ; --Brand: #abc; bad; :x; a{b}: c; width:; x: {y}`)
	want := []Decl{{"color", "red", false}, {"margin", "1px 2px", true}, {"--Brand", "#abc", false}}
	if !slices.Equal(ds, want) {
		t.Errorf("ParseDecls = %#v", ds)
	}
	// 引号里的 ; 不切。
	ds = ParseDecls(`content: "a;b"; color: blue`)
	if len(ds) != 2 || ds[0].Value != `"a;b"` {
		t.Errorf("引号里的分号 = %#v", ds)
	}
}

func TestParseSheet(t *testing.T) {
	sh := ParseSheet(`
		@import url(x.css);
		@media (max-width: 600px) { .a { color: red } }
		/* 注释 { } */
		.a, :hover, p::bogus-elem, .b > .c { color: blue }
		}
		.d { }
		@font-face { font-family: x; src: url(y) }
		.e { color: green
	`)
	if sh.Empty() {
		t.Fatal("空表")
	}
	// .a、:hover（永不中，但读得出）、p::bogus-elem（同上）、.b > .c、.e（没收尾也认）
	if len(sh.rules) != 5 {
		var sels []int
		for _, r := range sh.rules {
			sels = append(sels, len(r.sel.parts))
		}
		t.Errorf("规则数 = %d %v", len(sh.rules), sels)
	}
	var nilSheet *Sheet
	if !nilSheet.Empty() || !(&Sheet{}).Empty() {
		t.Error("nil / 零值应该是空表")
	}
	nilSheet.Add(sh) // 不 panic
	a := ParseSheet(".x{color:red}")
	a.Add(ParseSheet(".x{color:blue}"))
	x := el("div", "class=x")
	if got := styleOf(a, x, "").Get("color"); got != "blue" {
		t.Errorf("Add 后来的压过先来的：%q", got)
	}
}

func TestSelectors(t *testing.T) {
	// <div id=root class="wrap">
	//   <p class="note first">  <span class=x/> <b/> </p>
	//   <p class="note" data-k="hello-world" lang="en-US"/>
	//   <ul> <li/> <li class=odd/> <li/> <li/> <li/> </ul>
	//   <a href="https://e.com/a.pdf"/>
	// </div>
	span := el("span", "class=x")
	b := el("b", "")
	p1 := el("p", "", span, b)
	p1.attrs["class"] = "note first"
	p2 := el("p", "class=note data-k=hello-world lang=en-US")
	lis := []*node{el("li", ""), el("li", "class=odd"), el("li", ""), el("li", ""), el("li", "")}
	ul := el("ul", "", lis...)
	a := el("a", "href=https://e.com/a.pdf")
	root := el("DIV", "id=root class=wrap", p1, p2, ul, a)

	cases := []struct {
		sel  string
		e    *node
		want bool
	}{
		{"*", span, true},
		{"div", root, true}, // 标签不分大小写
		{"P", p1, true},
		{".note", p1, true},
		{".note.first", p1, true},
		{".note.first", p2, false},
		{"#root", root, true},
		{"#ROOT", root, false},
		{"[data-k]", p2, true},
		{"[data-k=hello-world]", p2, true},
		{`[data-k="hello"]`, p2, false},
		{"[data-k^=hello]", p2, true},
		{"[data-k$=world]", p2, true},
		{"[data-k*=o-w]", p2, true},
		{"[class~=first]", p1, true},
		{"[class~=fir]", p1, false},
		{"[lang|=en]", p2, true},
		{`[href$=".PDF" i]`, a, true},
		{".wrap span", span, true},
		{".wrap > span", span, false},
		{"p > span", span, true},
		{"#root > p.note > .x", span, true},
		{"span + b", b, true},
		{"p + p", p2, true},
		{"p ~ a", a, true},
		{"p + a", a, false},
		{"li:first-child", lis[0], true},
		{"li:first-child", lis[1], false},
		{"li:last-child", lis[4], true},
		{"li:only-child", lis[0], false},
		{"span:only-child", span, false},
		{"li:nth-child(2)", lis[1], true},
		{"li:nth-child(odd)", lis[2], true},
		{"li:nth-child(odd)", lis[1], false},
		{"li:nth-child(even)", lis[3], true},
		{"li:nth-child(3n+1)", lis[3], true},
		{"li:nth-child(-n+2)", lis[1], true},
		{"li:nth-child(-n+2)", lis[2], false},
		{"li:nth-last-child(1)", lis[4], true},
		{"p:first-of-type", p1, true},
		{"p:last-of-type", p2, true},
		{"ul:only-of-type", ul, true},
		{"li:not(.odd)", lis[0], true},
		{"li:not(.odd)", lis[1], false},
		{"li:not(.odd, :first-child)", lis[0], false},
		{":is(p, ul) > li", lis[2], true},
		{":where(.wrap) .x", span, true},
		{":root", root, true},
		{":root", p1, false},
		{"a:link", a, true},
		{"a:hover", a, false},
		{"p:lang(en)", p2, false},
		{"li:nth-child(2n of .odd)", lis[1], false},
		{".md\\:flex", el("i", "class=md:flex"), true},
	}
	for _, c := range cases {
		sel, ok := parseSelector(c.sel)
		if !ok {
			t.Errorf("parseSelector(%q) 读不出", c.sel)
			continue
		}
		if got := sel.match(c.e); got != c.want {
			t.Errorf("%q 对 <%s %v> = %v，要 %v", c.sel, c.e.tag, c.e.attrs, got, c.want)
		}
	}
	for _, bad := range []string{"", ">", "p >", "p..x", "[", "[a=", "p::after .x", ":not(", "#", "a,b"} {
		if _, ok := parseSelector(bad); ok {
			t.Errorf("parseSelector(%q) 应该读不出", bad)
		}
	}
}

func TestSpecificity(t *testing.T) {
	spec := func(s string) int {
		sel, ok := parseSelector(s)
		if !ok {
			t.Fatalf("读不出 %q", s)
		}
		return sel.spec()
	}
	abc := func(a, b, c int) int { return a<<20 | b<<10 | c }
	cases := map[string]int{
		"*":                       0,
		"li":                      abc(0, 0, 1),
		"ul li":                   abc(0, 0, 2),
		".a":                      abc(0, 1, 0),
		"[x]":                     abc(0, 1, 0),
		"#x":                      abc(1, 0, 0),
		"div#x.a.b > p:hover":     abc(1, 3, 2),
		"p::after":                abc(0, 0, 2),
		":not(#x)":                abc(1, 0, 0),
		":is(.a, #b) p":           abc(1, 0, 1),
		":where(#a .b) p":         abc(0, 0, 1),
		"li:nth-child(2n+1)":      abc(0, 1, 1),
		"a[href^='http']:visited": abc(0, 2, 1),
	}
	for s, want := range cases {
		if got := spec(s); got != want {
			t.Errorf("spec(%q) = %x，要 %x", s, got, want)
		}
	}
}

func TestCascadeOrder(t *testing.T) {
	sh := ParseSheet(`
		p { color: red; margin: 1px }
		.c { color: green }
		#i { color: blue }
		p { color: purple }
		.imp { color: orange !important }
	`)
	cases := []struct {
		attrs, style, want string
	}{
		{"", "", "purple"},           // 同优先级后来的赢
		{"class=c", "", "green"},     // 类压过标签
		{"class=c id=i", "", "blue"}, // id 压过类
		{"id=i", "color: teal", "teal"},
		{"class=imp", "color: teal", "orange"},              // author !important 压过 inline
		{"class=imp", "color: teal !important", "teal"},     // inline !important 压过一切
		{"class=imp id=i", "color: teal", "orange"},         // !important 不看优先级的层
		{"", "color: teal; color: navy", "navy"},            // inline 里后写的赢
		{"", "color: teal !important; color: navy", "teal"}, // inline 里 important 赢
	}
	for _, c := range cases {
		e := el("p", c.attrs)
		if c.style != "" {
			e.attrs["style"] = c.style
		}
		if got := styleOf(sh, e, "").Get("color"); got != c.want {
			t.Errorf("<p %s style=%q> color = %q，要 %q", c.attrs, c.style, got, c.want)
		}
	}
	// 表现属性在 UA 后面、author 前面。
	td := el("td", "align=right bgcolor=#eee")
	el("table", "", el("tr", "", td))
	if got := styleOf(nil, td, "").Get("text-align"); got != "right" {
		t.Errorf("align=right → %q", got)
	}
	if got := styleOf(ParseSheet("td{text-align:left}"), td, "").Get("text-align"); got != "left" {
		t.Errorf("author 压过表现属性 → %q", got)
	}
	if got := styleOf(nil, td, "").Get("background-color"); got != "#eee" {
		t.Errorf("bgcolor → %q", got)
	}
	// th 的 UA 居中被表现属性盖掉。
	th := el("th", "align=left")
	if got := styleOf(nil, th, "").Get("text-align"); got != "left" {
		t.Errorf("th align=left → %q", got)
	}
}

func TestInheritance(t *testing.T) {
	sh := ParseSheet(`
		.box { color: #123456; font-family: Georgia; border: 1px solid red; padding: 4px; text-align: center; --gap: 8px }
		.reset { color: initial; text-align: unset; padding: inherit; border-left-color: inherit }
	`)
	inner := el("span", "")
	reset := el("i", "class=reset")
	box := el("div", "class=box", inner, reset)
	st := styleOf(sh, inner, "")
	if st.Get("color") != "#123456" || st.Get("font-family") != "Georgia" || st.Get("text-align") != "center" {
		t.Errorf("继承的属性：%v", st.dump())
	}
	if st.Has("padding-top") || st.Has("border-top-width") {
		t.Errorf("不继承的属性跟下来了：%v", st.dump())
	}
	if st.Get("--gap") != "8px" {
		t.Errorf("自定义属性要继承：%q", st.Get("--gap"))
	}
	rs := styleOf(sh, reset, "")
	if rs.Has("color") {
		t.Errorf("color: initial 应该没有（调用方的默认）：%q", rs.Get("color"))
	}
	if rs.Get("text-align") != "center" {
		t.Errorf("text-align: unset 要继承：%q", rs.Get("text-align"))
	}
	if rs.Get("padding-left") != "4px" {
		t.Errorf("padding: inherit：%q", rs.Get("padding-left"))
	}
	if rs.Get("border-left-color") != "red" {
		t.Errorf("border-left-color: inherit：%q", rs.Get("border-left-color"))
	}
	_ = box
}

func TestVar(t *testing.T) {
	sh := ParseSheet(`
		:root { --c: #f00; --w: 2px; --b: var(--w) solid var(--c); --loop1: var(--loop2); --loop2: var(--loop1) }
		.a { border: var(--b); color: var(--missing, var(--c)); margin: var(--nope) }
		.b { --c: blue; color: var(--c) }
		.cyc { color: var(--loop1, green); background: var(--loop2) }
		.inh { --c: inherit; color: var(--c) }
	`)
	a := el("div", "class=a")
	bb := el("div", "class=b", el("i", ""))
	cyc := el("div", "class=cyc")
	inh := el("div", "class=inh")
	el("html", "", a, bb, cyc, inh)

	st := styleOf(sh, a, "")
	if st.Get("border-top-width") != "2px" || st.Get("border-top-style") != "solid" || st.Get("border-top-color") != "#f00" {
		t.Errorf("var() 套 var() 再展开简写：%v", st.dump())
	}
	if st.Get("color") != "#f00" {
		t.Errorf("后备里的 var()：%q", st.Get("color"))
	}
	if st.Get("margin-top") != "0" {
		t.Errorf("引用不到又没后备的当 unset（margin 不继承，是初始值 0）：%q", st.Get("margin-top"))
	}
	if got := styleOf(sh, bb.kids[0], "").Get("color"); got != "blue" {
		t.Errorf("子元素继承覆盖后的 --c：%q", got)
	}
	cs := styleOf(sh, cyc, "")
	if cs.Get("color") != "green" {
		t.Errorf("成环的变量作废、用后备：%q", cs.Get("color"))
	}
	if cs.Has("--loop1") || cs.Has("--loop2") {
		t.Errorf("成环的变量应该作废：%v", cs.dump())
	}
	if got := styleOf(sh, inh, "").Get("color"); got != "#f00" {
		t.Errorf("--c: inherit：%q", got)
	}
}

func TestShorthands(t *testing.T) {
	sh := ParseSheet(`
		.m { margin: 1px 2px 3px; padding: 5px 6px; inset: 0 }
		.b { border: 2px dashed; border-left: thick double #0f0; border-radius: 4px 8px / 2px; border-color: red blue }
		.bg1 { background: #fafafa url(x.png) no-repeat }
		.bg2 { background: linear-gradient(90deg, rgba(0,0,0,.1) 0%, #fff 100%) }
		.bg3 { background: url(a.png) }
		.f { font: italic bold 12px/1.5 "Helvetica Neue", Arial, sans-serif }
		.f2 { font: 14px }
		.g { display: grid; grid-template-columns: 1fr 2fr; gap: 4px 8px }
		.fx1 { flex: 1 } .fx2 { flex: none } .fx3 { flex: 2 3 10% } .fx4 { flex: 200px } .fx5 { flex: auto }
		.ff { flex-flow: column wrap; place-items: center start }
		.td { text-decoration: underline wavy red }
		.ls { list-style: square inside }
		.ls2 { list-style: none }
		.o { overflow: hidden auto }
		.gc { grid-column: 1 / 3 }
		.ww { word-wrap: break-word }
	`)
	get := func(cls string) *Style { return styleOf(sh, el("div", "class="+cls), "") }
	expect := func(st *Style, kv ...string) {
		t.Helper()
		for i := 0; i < len(kv); i += 2 {
			if got := st.Get(kv[i]); got != kv[i+1] {
				t.Errorf("%s = %q，要 %q（%v）", kv[i], got, kv[i+1], st.dump())
			}
		}
	}
	expect(get("m"), "margin-top", "1px", "margin-right", "2px", "margin-bottom", "3px", "margin-left", "2px",
		"padding-top", "5px", "padding-left", "6px", "top", "0", "left", "0")
	expect(get("b"),
		"border-top-width", "2px", "border-top-style", "dashed", "border-top-color", "red", "border-right-color", "blue",
		"border-left-width", "thick", "border-left-style", "double", "border-left-color", "blue",
		"border-top-left-radius", "4px 2px", "border-top-right-radius", "8px 2px", "border-bottom-right-radius", "4px 2px")
	expect(get("bg1"), "background-color", "#fafafa", "background-image", "url(x.png)")
	expect(get("bg2"), "background-color", "rgba(0,0,0,.1)")
	expect(get("bg3"), "background-color", "transparent")
	expect(get("f"), "font-style", "italic", "font-weight", "700", "font-size", "12px", "line-height", "1.5",
		"font-family", `"Helvetica Neue", Arial, sans-serif`)
	if get("f2").Has("font-family") {
		t.Error("没有字体名的 font 简写要整条不认")
	}
	expect(get("g"), "display", "grid", "grid-template-columns", "1fr 2fr", "row-gap", "4px", "column-gap", "8px")
	expect(get("fx1"), "flex-grow", "1", "flex-shrink", "1", "flex-basis", "0%")
	expect(get("fx2"), "flex-grow", "0", "flex-shrink", "0", "flex-basis", "auto")
	expect(get("fx3"), "flex-grow", "2", "flex-shrink", "3", "flex-basis", "10%")
	expect(get("fx4"), "flex-grow", "1", "flex-basis", "200px")
	expect(get("fx5"), "flex-grow", "1", "flex-basis", "auto")
	expect(get("ff"), "flex-direction", "column", "flex-wrap", "wrap", "align-items", "center", "justify-items", "start")
	expect(get("td"), "text-decoration-line", "underline", "text-decoration-style", "wavy", "text-decoration-color", "red")
	expect(get("ls"), "list-style-type", "square", "list-style-position", "inside")
	expect(get("ls2"), "list-style-type", "none")
	expect(get("o"), "overflow-x", "hidden", "overflow-y", "auto")
	expect(get("gc"), "grid-column-start", "1", "grid-column-end", "3")
	expect(get("ww"), "overflow-wrap", "break-word")
}

func TestShorthandLonghandOrder(t *testing.T) {
	sh := ParseSheet(`.a { padding-left: 9px; padding: 1px } .b { padding: 1px; padding-left: 9px } .c { border: 1px solid; border: bogus 3px }`)
	if got := styleOf(sh, el("div", "class=a"), "").Get("padding-left"); got != "1px" {
		t.Errorf("简写在后：%q", got)
	}
	if got := styleOf(sh, el("div", "class=b"), "").Get("padding-left"); got != "9px" {
		t.Errorf("长写在后：%q", got)
	}
	if got := styleOf(sh, el("div", "class=c"), "").Get("border-top-width"); got != "1px" {
		t.Errorf("读不出的简写整条不要：%q", got)
	}
}

func TestFontComputed(t *testing.T) {
	sh := ParseSheet(`
		.base { font-size: 20px; font-weight: 300; line-height: 1.5em }
		.em { font-size: 1.5em }
		.pct { font-size: 50% }
		.rem { font-size: 2rem }
		.kw { font-size: x-large }
		.sm { font-size: smaller }
		.pt { font-size: 12pt }
		.bolder { font-weight: bolder }
		.lighter { font-weight: lighter }
		.lh { line-height: 150% }
		.lhn { line-height: 1.2 }
		.bad { font-size: calc(1px + 1em); font-weight: heavy }
	`)
	base := el("div", "class=base")
	kid := func(cls string) *Style {
		k := el("span", "class="+cls)
		k.parent = base
		base.kids = []*node{k}
		return styleOf(sh, k, "")
	}
	cases := []struct{ cls, prop, want string }{
		{"em", "font-size", "30px"},
		{"pct", "font-size", "10px"},
		{"rem", "font-size", "32px"},
		{"kw", "font-size", "24px"},
		{"pt", "font-size", "16px"},
		{"none", "font-size", "20px"},
		{"none", "font-weight", "300"},
		{"bolder", "font-weight", "400"},
		{"lighter", "font-weight", "100"},
		{"none", "line-height", "30px"}, // 父元素的 1.5em 换成 px 再继承
		{"lh", "line-height", "30px"},
		{"lhn", "line-height", "1.2"},
		{"bad", "font-size", "20px"},
		{"bad", "font-weight", "300"},
	}
	for _, c := range cases {
		if got := kid(c.cls).Get(c.prop); got != c.want {
			t.Errorf(".%s %s = %q，要 %q", c.cls, c.prop, got, c.want)
		}
	}
	if got := kid("sm").FontPx(); math.Abs(float64(got-20/1.2)) > 0.01 {
		t.Errorf("smaller = %v", got)
	}
	// 根元素：没有父样式时按 rootPx。
	if got := Compute(nil, el("div", ""), "", nil, nil, 0).Get("font-size"); got != "16px" {
		t.Errorf("根字号默认 16：%q", got)
	}
	if got := Compute(nil, el("h1", ""), "", nil, nil, 14).Get("font-size"); got != "28px" {
		t.Errorf("h1 在 14px 根下：%q", got)
	}
	// 调用方造的根样式。
	var root Style
	root.Set("font-size", "15px")
	root.Set("color", "#333")
	st := Compute(nil, el("b", ""), "", nil, &root, 16)
	if st.Get("font-size") != "15px" || st.Get("font-weight") != "700" || st.Get("color") != "#333" {
		t.Errorf("根样式 + <b>：%v", st.dump())
	}
}

func TestCurrentColor(t *testing.T) {
	sh := ParseSheet(`
		.p { color: navy }
		.k { color: currentColor; border-bottom: 1px solid; background-color: currentcolor; outline: 2px dotted currentcolor }
		.w { color: green; border-top-width: 3px }
		.n { border: 1px solid }
	`)
	k := el("span", "class=k")
	el("div", "class=p", k)
	st := styleOf(sh, k, "")
	if st.Get("color") != "navy" || st.Get("border-bottom-color") != "navy" || st.Get("background-color") != "navy" || st.Get("outline-color") != "navy" {
		t.Errorf("currentcolor：%v", st.dump())
	}
	if got := styleOf(sh, el("div", "class=w"), "").Get("border-top-color"); got != "green" {
		t.Errorf("只写了宽的边框用 color：%q", got)
	}
	if got := styleOf(sh, el("div", "class=n"), "").Get("border-top-color"); got != "currentcolor" {
		t.Errorf("没有 color 时留着 currentcolor：%q", got)
	}
}

func TestPseudoElements(t *testing.T) {
	sh := ParseSheet(`
		.tag::after { content: "→"; color: red; margin-left: 4px }
		.tag:before { content: "★" }
		.tag { color: blue }
		.tag::first-line { color: green }
	`)
	e := el("span", "class=tag")
	e.attrs["style"] = "color: purple"
	after := styleOf(sh, e, "after")
	if after.Get("content") != `"→"` || after.Get("color") != "red" || after.Get("margin-left") != "4px" {
		t.Errorf("::after：%v", after.dump())
	}
	before := styleOf(sh, e, "before")
	if before.Get("content") != `"★"` || before.Get("color") != "purple" {
		// before 没写 color：从元素（inline 的 purple）继承。
		t.Errorf(":before：%v", before.dump())
	}
	main := styleOf(sh, e, "")
	if main.Has("content") || main.Get("color") != "purple" {
		t.Errorf("元素本身不吃伪元素的规则：%v", main.dump())
	}
}

func TestStyleNil(t *testing.T) {
	var s *Style
	if s.Get("color") != "" || s.Has("color") || s.Props() != nil || s.FontPx() != 0 {
		t.Error("nil Style 应该什么都没有")
	}
}

func TestUA(t *testing.T) {
	get := func(tag, prop string) string { return Compute(nil, el(tag, ""), "", nil, nil, 16).Get(prop) }
	cases := []struct{ tag, prop, want string }{
		{"div", "display", "block"},
		{"li", "display", "list-item"},
		{"span", "display", ""},
		{"td", "display", "table-cell"},
		{"script", "display", "none"},
		{"strong", "font-weight", "700"},
		{"em", "font-style", "italic"},
		{"code", "font-family", "monospace"},
		{"pre", "white-space", "pre"},
		{"pre", "margin-top", "1em"},
		{"small", "font-size", "13.333333px"},
		{"sup", "vertical-align", "super"},
		{"u", "text-decoration-line", "underline"},
		{"del", "text-decoration-line", "line-through"},
		{"mark", "background-color", "yellow"},
		{"center", "text-align", "center"},
		{"th", "text-align", "center"},
		{"th", "font-weight", "700"},
		{"h1", "font-size", "32px"},
		{"h1", "margin-top", "0.67em"},
		{"h2", "font-size", "24px"},
		{"h6", "font-weight", "700"},
		{"p", "margin-bottom", "1em"},
		{"ul", "padding-left", "40px"},
		{"ol", "list-style-type", "decimal"},
		{"blockquote", "margin-left", "40px"},
		{"hr", "border-top-style", "inset"},
		{"hr", "border-top-color", "gray"},
	}
	for _, c := range cases {
		if got := get(c.tag, c.prop); got != c.want {
			t.Errorf("<%s> %s = %q，要 %q", c.tag, c.prop, got, c.want)
		}
	}
	hid := el("p", "hidden")
	if got := Compute(nil, hid, "", nil, nil, 16).Get("display"); got != "none" {
		t.Errorf("[hidden] → %q", got)
	}
	inner := el("li", "")
	el("ul", "", el("li", "", el("ul", "", inner)))
	if got := styleOf(nil, inner, "").Get("list-style-type"); got != "circle" {
		t.Errorf("套着的 ul 用 circle：%q", got)
	}
}

func TestPresentational(t *testing.T) {
	td := el("td", "valign=top nowrap width=120")
	tbl := el("table", "border=2 cellpadding=6 cellspacing=0 width=80% align=center", el("tbody", "", el("tr", "", td)))
	ds := Presentational(tbl)
	want := map[string]string{"margin-left": "auto", "margin-right": "auto", "width": "80%", "border": "2px outset gray", "border-spacing": "0px"}
	got := map[string]string{}
	for _, d := range ds {
		got[d.Prop] = d.Value
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("table %s = %q，要 %q（%v）", k, got[k], v, ds)
		}
	}
	st := styleOf(nil, td, "")
	expect := map[string]string{
		"vertical-align": "top", "white-space": "nowrap", "width": "120px",
		"border-top-width": "1px", "border-top-style": "inset", "border-top-color": "gray", "padding-left": "6px",
	}
	for k, v := range expect {
		if st.Get(k) != v {
			t.Errorf("td %s = %q，要 %q（%v）", k, st.Get(k), v, st.dump())
		}
	}
	f := el("font", "color=red size=+2 face=Courier")
	fs := styleOf(nil, f, "")
	if fs.Get("color") != "red" || fs.Get("font-family") != "Courier" || fs.Get("font-size") != "24px" {
		t.Errorf("<font>：%v", fs.dump())
	}
	p := el("p", "align=center")
	if got := styleOf(nil, p, "").Get("text-align"); got != "center" {
		t.Errorf("<p align=center>：%q", got)
	}
	img := el("img", "width=50 height=abc align=right")
	is := styleOf(nil, img, "")
	if is.Get("width") != "50px" || is.Has("height") || is.Get("float") != "right" {
		t.Errorf("<img>：%v", is.dump())
	}
	if Presentational(nil) != nil {
		t.Error("nil 元素")
	}
}

func TestComputeNilElement(t *testing.T) {
	st := Compute(ParseSheet("*{color:red}"), nil, "", []Decl{{"color", "blue", false}}, nil, 16)
	if st.Get("color") != "blue" || st.Get("font-size") != "16px" {
		t.Errorf("nil 元素只用 inline：%v", st.dump())
	}
}

// TestGarbage 喂各种坏 CSS，不许 panic。
func TestGarbage(t *testing.T) {
	samples := []string{
		"", "{", "}", "{{{{", "}}}}", ";;;", ":", "a{", "a{b", "a{b:", "a{b:c", "@", "@media{", "@media (x{y}",
		"/*", "/* unterminated", "\"", "'", "a[", "a[b", "a[b=", "a[b='", "a:not(", "a:nth-child(", "a:nth-child()",
		":nth-child(n+)", ":nth-child(99999999999999999999n)", "a::", "a:::b", "#{}", ".{}", "\\", "a\\", ".\\",
		"*{color:var(}", "*{color:var(--x}", "*{color:var(--a,var(--b,var(--c,var(--d,var(--e)))))}",
		"*{--a:var(--a)}", "*{font:}", "*{font:/}", "*{font:12px/}", "*{border-radius:/}", "*{border-radius:1px/}",
		"*{background:linear-gradient(}", "*{grid-template-columns:repeat(999999999,1fr)}", "*{flex:a b c d}",
		"a > > b{}", "a + {}", "~ a{}", "a ~{}", ":is(){}", ":not(){}", ":where(,,,){}", "[a~=]{}", "[=a]{}",
		"*{color:rgb(1,2,3,4,5)}", "*{color:hsl(nan,1%,1%)}", "*{color:#}", "*{width:1e999px}", "*{width:-1e-999px}",
		"*{font-size:1e30px}", "*{font-size:-5px}", "*{font-weight:1e99}", "a{color:red!}", "a{color:!important}",
		"\x00\xff\xfe{\x00:\x00}", "😀{😀:😀}", "a,{}", ",a{}", ",{}",
	}
	r := rand.New(rand.NewPCG(1, 2))
	alphabet := []byte("{}[]():;,.#*>+~!@/\\\"' -_azAZ09%\n")
	for range 300 {
		b := make([]byte, r.IntN(60))
		for i := range b {
			b[i] = alphabet[r.IntN(len(alphabet))]
		}
		samples = append(samples, string(b))
	}
	child := el("span", "class=a id=b style=x")
	root := el("div", "class=a", el("p", ""), child)
	for _, s := range samples {
		func() {
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("%q panic：%v", s, p)
				}
			}()
			sh := ParseSheet(s)
			ds := ParseDecls(s)
			ParseColor(s)
			ParseLength(s)
			ParseTracks(s)
			Fields(s)
			SplitComma(s)
			for _, ps := range []string{"", "before", "after"} {
				st := Compute(sh, child, ps, ds, Compute(sh, root, "", ds, nil, 16), 16)
				st.FontPx()
			}
			e := el("td", "")
			e.attrs = map[string]string{"align": s, "width": s, "bgcolor": s, "valign": s, "border": s, "size": s, "color": s, "cellpadding": s}
			el("table", "", e).attrs["border"] = s
			Presentational(e)
			f := el("font", "")
			f.attrs["size"] = s
			Presentational(f)
		}()
	}
}

func TestDeepDescendantBudget(t *testing.T) {
	// 很深的树 + 一串后代组合符：到了步数上限就算不中，不卡住。
	leaf := el("i", "")
	n := leaf
	for range 300 {
		n = el("b", "", n)
	}
	sel, ok := parseSelector("a a a a a a a a a a a a a a a a a a a a b b b b b b b b i")
	if !ok {
		t.Fatal("读不出")
	}
	if sel.match(leaf) {
		t.Error("不该中")
	}
}
