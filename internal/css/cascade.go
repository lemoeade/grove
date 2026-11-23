package css

import (
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// Style 是算好的样式：属性名 → 值（都是展开后的长写法）。nil 的 Style 什么都没有。
//
// 算过的：font-size 总是 px（"14px"），font-weight 总是数（"700"），line-height 写了 em / % / rem 的换成 px，
// *-color 里的 currentcolor 换成 color 的值（元素没有 color 时留着 currentcolor，调用方用自己的字色）。
// 别的长度原样留着，调用方用 ParseLength + Resolve(FontPx(), 根字号, 百分比的基数) 换。
type Style struct {
	props map[string]string
}

// Get 拿一个属性的值，没有是 ""。
func (s *Style) Get(prop string) string {
	if s == nil {
		return ""
	}
	return s.props[prop]
}

// Has 看有没有这个属性。
func (s *Style) Has(prop string) bool {
	if s == nil {
		return false
	}
	_, ok := s.props[prop]
	return ok
}

// Set 设一个属性（调用方造根样式时用：比如根元素的 color、font-family 跟着主题）。
func (s *Style) Set(prop, v string) {
	if s.props == nil {
		s.props = map[string]string{}
	}
	s.props[prop] = v
}

// Props 是全部属性名，排好序。
func (s *Style) Props() []string {
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.props))
	for k := range s.props {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// FontPx 是字号（px）；没有 font-size 时是 0。
func (s *Style) FontPx() float32 {
	l, ok := ParseLength(s.Get("font-size"))
	if !ok || l.U != Px {
		return 0
	}
	return l.V
}

// inherited 是会继承的属性（自定义属性 --x 也继承，另外看）。
var inherited = map[string]bool{
	"color": true, "font-family": true, "font-size": true, "font-style": true, "font-weight": true,
	"font-variant": true, "line-height": true, "letter-spacing": true, "word-spacing": true, "text-align": true,
	"text-indent": true, "text-transform": true, "white-space": true, "visibility": true, "list-style-type": true,
	"list-style-position": true, "list-style-image": true, "word-break": true, "overflow-wrap": true, "cursor": true,
	"border-collapse": true, "border-spacing": true, "direction": true, "tab-size": true, "quotes": true,
}

// initial 是 initial / unset（不继承的）时的值；表里没有的属性就当没写，调用方用默认的。
var initial = map[string]string{
	"display": "inline", "font-style": "normal", "font-weight": "400", "font-variant": "normal",
	"font-size": "medium", "line-height": "normal", "text-align": "start", "white-space": "normal",
	"visibility": "visible", "text-decoration-line": "none", "vertical-align": "baseline",
	"background-color": "transparent", "background-image": "none", "opacity": "1",
	"width": "auto", "height": "auto", "min-width": "auto", "min-height": "auto", "max-width": "none", "max-height": "none",
	"margin-top": "0", "margin-right": "0", "margin-bottom": "0", "margin-left": "0",
	"padding-top": "0", "padding-right": "0", "padding-bottom": "0", "padding-left": "0",
	"border-top-width": "medium", "border-right-width": "medium", "border-bottom-width": "medium", "border-left-width": "medium",
	"border-top-style": "none", "border-right-style": "none", "border-bottom-style": "none", "border-left-style": "none",
	"border-top-left-radius": "0", "border-top-right-radius": "0", "border-bottom-right-radius": "0", "border-bottom-left-radius": "0",
	"flex-grow": "0", "flex-shrink": "1", "flex-basis": "auto", "flex-direction": "row", "flex-wrap": "nowrap",
	"row-gap": "normal", "column-gap": "normal", "list-style-type": "disc", "list-style-position": "outside",
	"overflow-x": "visible", "overflow-y": "visible", "position": "static", "float": "none", "box-sizing": "content-box",
	"letter-spacing": "normal", "word-spacing": "normal", "text-indent": "0", "text-transform": "none",
	"word-break": "normal", "overflow-wrap": "normal", "content": "normal",
}

// level 是层叠的先后：后面的压过前面的。
const (
	levelUA = iota
	levelHint
	levelAuthor
	levelInline
	levelAuthorImportant
	levelInlineImportant
)

type entry struct {
	level, spec int
	d           Decl
}

// Compute 算元素 e 的样式（pseudo 是 "before" / "after" 时算那个伪元素的）。author 是文档里的 <style>，
// inline 是 style= 的声明（伪元素不用），parent 是父元素算好的样式（根元素给 nil 或调用方造的根样式），
// rootPx 是根字号（rem 和字号关键字按它，≤ 0 当 16）。
//
// 层叠：UA → 表现属性（align=、bgcolor= 这些）→ author → inline；!important 反过来：author 的 !important 压过 inline，
// inline 的 !important 压过一切。同一层按优先级，再按先后。
func Compute(author *Sheet, e Element, pseudo string, inline []Decl, parent *Style, rootPx float32) *Style {
	if rootPx <= 0 {
		rootPx = 16
	}
	var es []entry
	add := func(sh *Sheet, normal, important int) {
		if sh == nil || e == nil {
			return
		}
		for _, r := range sh.rules {
			if r.sel.elem() != pseudo || !r.sel.match(e) {
				continue
			}
			for _, d := range r.decls {
				lv := normal
				if d.Important {
					lv = important
				}
				es = append(es, entry{lv, r.spec, d})
			}
		}
	}
	add(UA, levelUA, levelUA)
	if pseudo == "" && e != nil {
		for _, d := range Presentational(e) {
			es = append(es, entry{levelHint, 0, d})
		}
	}
	add(author, levelAuthor, levelAuthorImportant)
	if pseudo == "" {
		for _, d := range inline {
			lv := levelInline
			if d.Important {
				lv = levelInlineImportant
			}
			es = append(es, entry{lv, 0, d})
		}
	}
	sort.SliceStable(es, func(i, j int) bool {
		if es[i].level != es[j].level {
			return es[i].level < es[j].level
		}
		return es[i].spec < es[j].spec
	})

	vars := customProps(es, parent)
	lookup := func(name string) (string, bool) {
		v, ok := vars[name]
		return v, ok
	}

	spec := map[string]string{}
	set := func(p, v string) { spec[p] = v }
	for _, en := range es {
		if strings.HasPrefix(en.d.Prop, "--") {
			continue
		}
		v := en.d.Value
		if hasVar(v) {
			sv, ok := substitute(v, lookup, 0)
			if !ok {
				sv = "unset"
			}
			v = strings.TrimSpace(sv)
		}
		expand(en.d.Prop, v, set)
	}
	return finish(spec, vars, parent, rootPx)
}

// customProps 层叠自定义属性：父元素的继承下来，元素自己的盖上去，再把里面的 var() 换掉（成环的、引用不到又没后备的作废）。
func customProps(es []entry, parent *Style) map[string]string {
	vars := map[string]string{}
	if parent != nil {
		for k, v := range parent.props {
			if strings.HasPrefix(k, "--") {
				vars[k] = v
			}
		}
	}
	own := map[string]string{}
	for _, en := range es {
		if strings.HasPrefix(en.d.Prop, "--") {
			own[en.d.Prop] = en.d.Value
		}
	}
	if len(own) == 0 {
		return vars
	}
	inheritedVars := maps.Clone(vars)
	for k, v := range own {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "inherit", "unset", "revert", "revert-layer":
			if pv, ok := inheritedVars[k]; ok {
				own[k] = pv
			} else {
				delete(own, k)
				delete(vars, k)
			}
		case "initial":
			delete(own, k)
			delete(vars, k)
		}
	}
	const (
		todo = iota
		busy
		done
	)
	state := map[string]int{}
	var resolve func(name string) (string, bool)
	resolve = func(name string) (string, bool) {
		raw, mine := own[name]
		if !mine {
			v, ok := vars[name]
			return v, ok
		}
		switch state[name] {
		case busy:
			return "", false
		case done:
			v, ok := vars[name]
			return v, ok
		}
		state[name] = busy
		v, ok := raw, true
		if hasVar(raw) {
			v, ok = substitute(raw, resolve, 0)
		}
		state[name] = done
		if !ok {
			delete(vars, name)
			return "", false
		}
		vars[name] = v
		return v, true
	}
	for k := range own {
		resolve(k)
	}
	return vars
}

func hasVar(v string) bool { return strings.Contains(strings.ToLower(v), "var(") }

// substitute 把 v 里的 var(--x, 后备) 换掉。引用不到又没有后备的、套得太深的给 false。
func substitute(v string, lookup func(string) (string, bool), depth int) (string, bool) {
	if depth > 16 {
		return "", false
	}
	var b strings.Builder
	lower := strings.ToLower(v)
	i := 0
	for {
		j := strings.Index(lower[i:], "var(")
		if j < 0 {
			b.WriteString(v[i:])
			return b.String(), true
		}
		j += i
		if j > 0 && isNameChar(lower[j-1]) {
			// 不是 var(：比如 xvar(
			b.WriteString(v[i : j+4])
			i = j + 4
			continue
		}
		b.WriteString(v[i:j])
		p := &selParser{s: v, i: j + 3}
		arg, ok := p.paren()
		if !ok {
			return "", false
		}
		name, fallback, hasFallback := strings.Cut(arg, ",")
		name = strings.TrimSpace(name)
		val, found := "", false
		if strings.HasPrefix(name, "--") {
			val, found = lookup(name)
		}
		if found && hasVar(val) {
			val, found = substitute(val, lookup, depth+1)
		}
		if !found {
			if !hasFallback {
				return "", false
			}
			val, found = substitute(strings.TrimSpace(fallback), lookup, depth+1)
			if !found {
				return "", false
			}
		}
		b.WriteString(val)
		i = p.i
	}
}

func isWide(v string) bool {
	switch strings.ToLower(v) {
	case "inherit", "initial", "unset", "revert", "revert-layer":
		return true
	}
	return false
}

// finish 处理 inherit / initial / unset，补上继承的属性，算 font-size、font-weight、line-height 和 currentcolor。
func finish(spec, vars map[string]string, parent *Style, rootPx float32) *Style {
	st := &Style{props: make(map[string]string, len(spec)+len(vars)+8)}
	parentPx := parent.FontPx()
	if parentPx <= 0 {
		parentPx = rootPx
	}
	for p, v := range spec {
		switch strings.ToLower(v) {
		case "inherit":
			if parent.Has(p) {
				st.props[p] = parent.Get(p)
			} else if iv, ok := initial[p]; ok {
				st.props[p] = iv
			}
			continue
		case "unset", "revert", "revert-layer":
			if inherited[p] {
				if parent.Has(p) {
					st.props[p] = parent.Get(p)
				}
				continue
			}
			fallthrough
		case "initial":
			if iv, ok := initial[p]; ok {
				st.props[p] = iv
			}
			continue
		}
		st.props[p] = v
	}
	if parent != nil {
		for p, v := range parent.props {
			if inherited[p] {
				if _, ok := spec[p]; !ok {
					st.props[p] = v
				}
			}
		}
	}
	for k, v := range vars {
		st.props[k] = v
	}

	// font-size：换成 px。
	px := parentPx
	if v, ok := spec["font-size"]; ok {
		px = fontPx(st.props["font-size"], v, parentPx, rootPx)
	}
	st.props["font-size"] = fmtPx(px)

	// font-weight：换成数。
	pw := 400
	if parent != nil {
		if n, err := strconv.Atoi(parent.Get("font-weight")); err == nil {
			pw = n
		}
	}
	w := pw
	if v, ok := st.props["font-weight"]; ok {
		w = fontWeight(v, pw)
	}
	st.props["font-weight"] = strconv.Itoa(w)

	// line-height：元素自己写的 em / % / rem 换成 px（不带单位的倍数原样继承）。
	if _, ok := spec["line-height"]; ok {
		if l, ok := ParseLength(st.props["line-height"]); ok {
			switch l.U {
			case Em, Rem, Percent:
				st.props["line-height"] = fmtPx(l.Resolve(px, rootPx, px))
			}
		}
	}

	// color: currentcolor 是父元素的 color。
	if strings.EqualFold(st.props["color"], "currentcolor") {
		if pc := parent.Get("color"); pc != "" && !strings.EqualFold(pc, "currentcolor") {
			st.props["color"] = pc
		} else {
			delete(st.props, "color")
		}
	}
	color, hasColor := st.props["color"]

	// 边框某一边写了宽或样式、没写颜色：用 color。
	for _, side := range sides {
		_, ws := st.props["border-"+side+"-width"]
		_, ss := st.props["border-"+side+"-style"]
		if (ws || ss) && !st.Has("border-"+side+"-color") {
			st.props["border-"+side+"-color"] = "currentcolor"
		}
	}
	// *-color 里的 currentcolor。
	if hasColor {
		for p, v := range st.props {
			if p != "color" && (strings.HasSuffix(p, "-color") || p == "fill" || p == "stroke") && strings.EqualFold(v, "currentcolor") {
				st.props[p] = color
			}
		}
	}
	return st
}

var sides = [4]string{"top", "right", "bottom", "left"}

func fmtPx(v float32) string {
	return strconv.FormatFloat(float64(v), 'f', -1, 32) + "px"
}

// fontScale 是字号关键字对着 medium 的倍数。
var fontScale = map[string]float32{
	"xx-small": 3.0 / 5, "x-small": 3.0 / 4, "small": 8.0 / 9, "medium": 1, "large": 6.0 / 5,
	"x-large": 3.0 / 2, "xx-large": 2, "xxx-large": 3,
}

// fontPx 算字号：v 是算好的值（inherit 已经换掉），raw 是写的值。读不出的跟父元素一样。
func fontPx(v, raw string, parentPx, rootPx float32) float32 {
	v = strings.ToLower(strings.TrimSpace(v))
	if f, ok := fontScale[v]; ok {
		return rootPx * f
	}
	switch v {
	case "smaller":
		return parentPx / 1.2
	case "larger":
		return parentPx * 1.2
	}
	l, ok := ParseLength(v)
	if !ok {
		return parentPx
	}
	var px float32
	switch l.U {
	case Px:
		px = l.V
	case Em, Percent:
		px = l.Resolve(parentPx, rootPx, parentPx)
	case Rem:
		px = l.V * rootPx
	default:
		return parentPx
	}
	if px < 0 || px != px {
		return parentPx
	}
	return min(px, 10000)
}

// fontWeight 把 font-weight 换成数；bolder / lighter 按父元素的算。
func fontWeight(v string, parent int) int {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "normal":
		return 400
	case "bold":
		return 700
	case "bolder":
		switch {
		case parent < 350:
			return 400
		case parent < 550:
			return 700
		}
		return 900
	case "lighter":
		switch {
		case parent < 550:
			return 100
		case parent < 750:
			return 400
		}
		return 700
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || f < 1 || f > 1000 {
		return parent
	}
	return int(f)
}

// longhands 是简写展开成的长写法（简写写 inherit / initial / unset 时每个都设成它）。
var longhands = map[string][]string{
	"margin":          {"margin-top", "margin-right", "margin-bottom", "margin-left"},
	"padding":         {"padding-top", "padding-right", "padding-bottom", "padding-left"},
	"inset":           {"top", "right", "bottom", "left"},
	"border-width":    {"border-top-width", "border-right-width", "border-bottom-width", "border-left-width"},
	"border-style":    {"border-top-style", "border-right-style", "border-bottom-style", "border-left-style"},
	"border-color":    {"border-top-color", "border-right-color", "border-bottom-color", "border-left-color"},
	"border-radius":   {"border-top-left-radius", "border-top-right-radius", "border-bottom-right-radius", "border-bottom-left-radius"},
	"border":          {"border-top-width", "border-right-width", "border-bottom-width", "border-left-width", "border-top-style", "border-right-style", "border-bottom-style", "border-left-style", "border-top-color", "border-right-color", "border-bottom-color", "border-left-color"},
	"border-top":      {"border-top-width", "border-top-style", "border-top-color"},
	"border-right":    {"border-right-width", "border-right-style", "border-right-color"},
	"border-bottom":   {"border-bottom-width", "border-bottom-style", "border-bottom-color"},
	"border-left":     {"border-left-width", "border-left-style", "border-left-color"},
	"outline":         {"outline-width", "outline-style", "outline-color"},
	"background":      {"background-color", "background-image"},
	"font":            {"font-style", "font-variant", "font-weight", "font-size", "line-height", "font-family"},
	"gap":             {"row-gap", "column-gap"},
	"grid-gap":        {"row-gap", "column-gap"},
	"flex":            {"flex-grow", "flex-shrink", "flex-basis"},
	"flex-flow":       {"flex-direction", "flex-wrap"},
	"place-items":     {"align-items", "justify-items"},
	"place-content":   {"align-content", "justify-content"},
	"place-self":      {"align-self", "justify-self"},
	"text-decoration": {"text-decoration-line", "text-decoration-style", "text-decoration-color", "text-decoration-thickness"},
	"list-style":      {"list-style-type", "list-style-position", "list-style-image"},
	"overflow":        {"overflow-x", "overflow-y"},
	"grid-column":     {"grid-column-start", "grid-column-end"},
	"grid-row":        {"grid-row-start", "grid-row-end"},
}

// alias 是旧名字 → 现在的名字。
var alias = map[string]string{
	"word-wrap": "overflow-wrap", "grid-row-gap": "row-gap", "grid-column-gap": "column-gap",
	"-webkit-text-decoration": "text-decoration",
}

// expand 把一条声明展开成长写法交给 set。读不出的简写整条不要（照 CSS 的规矩）。
func expand(prop, v string, set func(p, v string)) {
	if a, ok := alias[prop]; ok {
		prop = a
	}
	if lh := longhands[prop]; lh != nil && isWide(v) {
		for _, p := range lh {
			set(p, v)
		}
		return
	}
	switch prop {
	case "margin", "padding", "inset", "border-width", "border-style", "border-color":
		f := Fields(v)
		if len(f) < 1 || len(f) > 4 {
			return
		}
		t, r, b, l := boxSides(f)
		lh := longhands[prop]
		set(lh[0], t)
		set(lh[1], r)
		set(lh[2], b)
		set(lh[3], l)
	case "border-radius":
		h, vv, _ := strings.Cut(v, "/")
		hf, vf := Fields(h), Fields(vv)
		if len(hf) < 1 || len(hf) > 4 || len(vf) > 4 {
			return
		}
		corners := func(f []string) [4]string {
			// 顺序：左上、右上、右下、左下；缺的照 CSS 补。
			switch len(f) {
			case 1:
				return [4]string{f[0], f[0], f[0], f[0]}
			case 2:
				return [4]string{f[0], f[1], f[0], f[1]}
			case 3:
				return [4]string{f[0], f[1], f[2], f[1]}
			}
			return [4]string{f[0], f[1], f[2], f[3]}
		}
		hc := corners(hf)
		vc := hc
		if len(vf) > 0 {
			vc = corners(vf)
		}
		for i, p := range longhands["border-radius"] {
			if hc[i] == vc[i] {
				set(p, hc[i])
			} else {
				set(p, hc[i]+" "+vc[i])
			}
		}
	case "border", "border-top", "border-right", "border-bottom", "border-left", "outline":
		w, s, c, ok := parseBorder(v)
		if !ok {
			return
		}
		switch prop {
		case "border":
			for _, side := range sides {
				set("border-"+side+"-width", w)
				set("border-"+side+"-style", s)
				set("border-"+side+"-color", c)
			}
		case "outline":
			set("outline-width", w)
			set("outline-style", s)
			set("outline-color", c)
		default:
			set(prop+"-width", w)
			set(prop+"-style", s)
			set(prop+"-color", c)
		}
	case "background":
		color, image := parseBackground(v)
		set("background-color", color)
		set("background-image", image)
	case "font":
		expandFont(v, set)
	case "gap", "grid-gap":
		f := Fields(v)
		switch len(f) {
		case 1:
			set("row-gap", f[0])
			set("column-gap", f[0])
		case 2:
			set("row-gap", f[0])
			set("column-gap", f[1])
		}
	case "flex":
		expandFlex(v, set)
	case "flex-flow":
		for _, t := range Fields(v) {
			switch strings.ToLower(t) {
			case "row", "row-reverse", "column", "column-reverse":
				set("flex-direction", t)
			case "nowrap", "wrap", "wrap-reverse":
				set("flex-wrap", t)
			}
		}
	case "place-items", "place-content", "place-self":
		f := Fields(v)
		if len(f) < 1 || len(f) > 2 {
			return
		}
		lh := longhands[prop]
		set(lh[0], f[0])
		set(lh[1], f[len(f)-1])
	case "text-decoration":
		var lines []string
		style, color, thick := "solid", "currentcolor", "auto"
		for _, t := range Fields(v) {
			lt := strings.ToLower(t)
			switch lt {
			case "none", "underline", "overline", "line-through", "blink":
				if lt != "none" {
					lines = append(lines, lt)
				}
			case "solid", "double", "dotted", "dashed", "wavy":
				style = lt
			default:
				if _, ok := ParseColor(t); ok || lt == "currentcolor" {
					color = t
				} else if _, ok := ParseLength(t); ok || lt == "from-font" {
					thick = t
				} else {
					return
				}
			}
		}
		line := "none"
		if len(lines) > 0 {
			line = strings.Join(lines, " ")
		}
		set("text-decoration-line", line)
		set("text-decoration-style", style)
		set("text-decoration-color", color)
		set("text-decoration-thickness", thick)
	case "list-style":
		typ, pos, img := "", "outside", "none"
		nones := 0
		for _, t := range Fields(v) {
			lt := strings.ToLower(t)
			switch {
			case lt == "inside" || lt == "outside":
				pos = lt
			case lt == "none":
				nones++
			case strings.HasPrefix(lt, "url("):
				img = t
			default:
				typ = t
			}
		}
		if typ == "" {
			typ = "disc"
			if nones > 0 {
				typ = "none"
			}
		}
		set("list-style-type", typ)
		set("list-style-position", pos)
		set("list-style-image", img)
	case "overflow":
		f := Fields(v)
		if len(f) < 1 || len(f) > 2 {
			return
		}
		set("overflow-x", f[0])
		set("overflow-y", f[len(f)-1])
	case "grid-column", "grid-row":
		a, b, two := strings.Cut(v, "/")
		a = strings.TrimSpace(a)
		set(prop+"-start", a)
		if two {
			set(prop+"-end", strings.TrimSpace(b))
		} else {
			set(prop+"-end", "auto")
		}
	default:
		set(prop, v)
	}
}

// boxSides 按 CSS 的 1–4 个值补成上右下左。
func boxSides(f []string) (t, r, b, l string) {
	switch len(f) {
	case 1:
		return f[0], f[0], f[0], f[0]
	case 2:
		return f[0], f[1], f[0], f[1]
	case 3:
		return f[0], f[1], f[2], f[1]
	}
	return f[0], f[1], f[2], f[3]
}

func isBorderStyle(s string) bool {
	switch strings.ToLower(s) {
	case "none", "hidden", "dotted", "dashed", "solid", "double", "groove", "ridge", "inset", "outset":
		return true
	}
	return false
}

// parseBorder 读 border 简写：宽、样式、颜色，顺序随便，缺的是 medium、none、currentcolor。
func parseBorder(v string) (w, s, c string, ok bool) {
	w, s, c = "medium", "none", "currentcolor"
	var gotW, gotS, gotC bool
	for _, t := range Fields(v) {
		lt := strings.ToLower(t)
		switch {
		case !gotS && isBorderStyle(lt):
			s, gotS = lt, true
		case !gotW && (lt == "thin" || lt == "medium" || lt == "thick"):
			w, gotW = lt, true
		case !gotC && lt == "currentcolor":
			c, gotC = t, true
		default:
			if l, ok := ParseLength(t); !gotW && ok && (l.U != None || l.V == 0) && l.U != Percent && l.U != Auto && l.U != Fr {
				w, gotW = t, true
				continue
			}
			if _, ok := ParseColor(t); !gotC && ok {
				c, gotC = t, true
				continue
			}
			return "", "", "", false
		}
	}
	return w, s, c, true
}

// parseBackground 从 background 简写里拿颜色和图（渐变、url()）。没写颜色时：有渐变用渐变的第一个颜色，没有是 transparent。
func parseBackground(v string) (color, image string) {
	color, image = "transparent", "none"
	// 顶层逗号分开的是几层，颜色只能在最后一层。
	layers := SplitComma(v)
	for li, layer := range layers {
		for _, t := range Fields(layer) {
			lt := strings.ToLower(t)
			switch {
			case strings.Contains(lt, "gradient(") || strings.HasPrefix(lt, "url(") || strings.HasPrefix(lt, "image-set("):
				if image == "none" {
					image = t
				}
			case li == len(layers)-1:
				if _, ok := ParseColor(t); ok || lt == "currentcolor" {
					color = t
				}
			}
		}
	}
	if color == "transparent" && strings.Contains(strings.ToLower(image), "gradient(") {
		if c, ok := firstColor(image); ok {
			color = c
		}
	}
	return color, image
}

// firstColor 找渐变里第一个读得出的颜色。
func firstColor(g string) (string, bool) {
	i := strings.IndexByte(g, '(')
	if i < 0 || !strings.HasSuffix(g, ")") {
		return "", false
	}
	for _, stop := range SplitComma(g[i+1 : len(g)-1]) {
		for _, t := range Fields(stop) {
			if _, ok := ParseColor(t); ok {
				return t, true
			}
		}
	}
	return "", false
}

// expandFont 展开 font 简写：[style] [variant] [weight] [stretch] size[/line-height] family。没有字号或字体名的不认。
func expandFont(v string, set func(p, v string)) {
	f := Fields(v)
	style, variant, weight := "normal", "normal", "400"
	i := 0
	for ; i < len(f); i++ {
		lt := strings.ToLower(f[i])
		switch lt {
		case "normal":
			continue
		case "italic", "oblique":
			style = lt
			continue
		case "small-caps":
			variant = lt
			continue
		case "bold", "bolder", "lighter":
			weight = lt
			continue
		case "ultra-condensed", "extra-condensed", "condensed", "semi-condensed", "semi-expanded", "expanded",
			"extra-expanded", "ultra-expanded":
			continue
		}
		if n, err := strconv.Atoi(lt); err == nil && n >= 1 && n <= 1000 {
			weight = lt
			continue
		}
		break
	}
	if i >= len(f) {
		return
	}
	size, lh := f[i], "normal"
	i++
	if s, l, ok := strings.Cut(size, "/"); ok {
		size, lh = s, l
		if lh == "" {
			if i >= len(f) {
				return
			}
			lh = f[i]
			i++
		}
	} else if i < len(f) && strings.HasPrefix(f[i], "/") {
		lh = strings.TrimPrefix(f[i], "/")
		i++
		if lh == "" {
			if i >= len(f) {
				return
			}
			lh = f[i]
			i++
		}
	}
	if _, ok := fontScale[strings.ToLower(size)]; !ok {
		if l, ok := ParseLength(size); !ok || l.U == None || l.U == Auto || l.U == Fr {
			if !strings.EqualFold(size, "smaller") && !strings.EqualFold(size, "larger") {
				return
			}
		}
	}
	if i >= len(f) {
		return
	}
	set("font-style", style)
	set("font-variant", variant)
	set("font-weight", weight)
	set("font-size", size)
	set("line-height", lh)
	set("font-family", strings.Join(f[i:], " "))
}

// expandFlex 展开 flex 简写。
func expandFlex(v string, set func(p, v string)) {
	f := Fields(v)
	isNum := func(s string) bool {
		_, err := strconv.ParseFloat(s, 64)
		return err == nil
	}
	grow, shrink, basis := "", "1", "0%"
	switch len(f) {
	case 1:
		switch strings.ToLower(f[0]) {
		case "none":
			grow, shrink, basis = "0", "0", "auto"
		case "auto":
			grow, shrink, basis = "1", "1", "auto"
		default:
			if isNum(f[0]) {
				grow = f[0]
			} else if _, ok := ParseLength(f[0]); ok || strings.EqualFold(f[0], "content") {
				grow, basis = "1", f[0]
			} else {
				return
			}
		}
	case 2:
		if !isNum(f[0]) {
			return
		}
		grow = f[0]
		if isNum(f[1]) {
			shrink = f[1]
		} else if _, ok := ParseLength(f[1]); ok {
			basis = f[1]
		} else {
			return
		}
	case 3:
		if !isNum(f[0]) || !isNum(f[1]) {
			return
		}
		if _, ok := ParseLength(f[2]); !ok && !strings.EqualFold(f[2], "content") {
			return
		}
		grow, shrink, basis = f[0], f[1], f[2]
	default:
		return
	}
	set("flex-grow", grow)
	set("flex-shrink", shrink)
	set("flex-basis", basis)
}

// dump 是测试和调试用的：全部属性按名字排好的 "p: v" 列表。
func (s *Style) dump() []string {
	var out []string
	for _, p := range s.Props() {
		out = append(out, p+": "+s.props[p])
	}
	return slices.Clip(out)
}
