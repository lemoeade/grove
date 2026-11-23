package css

import (
	"strconv"
	"strings"
)

// Element 是调用方的一个 HTML 元素。Tag 是标签名（大小写都行），Attr 拿属性（名字给的是小写），
// Parent / Prev / Next 是父元素和前后的兄弟元素（只算元素，不算文字）。
//
// 没有的时候一定要返回无类型的 nil：把一个 nil 的 *node 当 Element 返回，接口就不是 nil 了，会被当成一个元素。
type Element interface {
	Tag() string
	Attr(name string) (string, bool)
	Parent() Element
	Prev() Element
	Next() Element
}

// selector 是一个复杂选择器：几个复合选择器用组合符连起来，combs[i] 在 parts[i] 和 parts[i+1] 之间（' ' '>' '+' '~'）。
type selector struct {
	parts []compound
	combs []byte
	never bool
}

// compound 是一个复合选择器：p.note#x[href]:first-child::after。
type compound struct {
	tag     string // "" 或 "*" 是任意标签
	ids     []string
	classes []string
	attrs   []attrSel
	pseudos []pseudoSel
	elem    string // 伪元素：before / after
	never   bool   // 用了认不出的伪类，永远不中
}

type attrSel struct {
	name, op, val string
	fold          bool // [a="x" i]
}

type pseudoSel struct {
	kind string // first-child、nth-child、not、is、root……
	a, b int    // :nth-*(an+b)
	list []*selector
}

// maxParts 是一个选择器最多几段；再长的不认（防病态的回溯）。
const maxParts = 32

// parseSelector 读一个选择器（选择器列表先按逗号拆开）。读不出的给 false；用了认不出的伪类的读得出，但 never。
func parseSelector(s string) (*selector, bool) {
	p := &selParser{s: strings.TrimSpace(s)}
	if p.s == "" {
		return nil, false
	}
	sel := &selector{}
	for {
		c, ok := p.compound()
		if !ok {
			return nil, false
		}
		sel.parts = append(sel.parts, c)
		if len(sel.parts) > maxParts {
			return nil, false
		}
		ws := p.skipSpace()
		if p.eof() {
			break
		}
		comb := byte(' ')
		switch ch := p.s[p.i]; ch {
		case '>', '+', '~':
			comb = ch
			p.i++
			p.skipSpace()
		default:
			if !ws {
				return nil, false
			}
		}
		if p.eof() {
			return nil, false
		}
		sel.combs = append(sel.combs, comb)
	}
	for i, c := range sel.parts {
		if c.never || c.elem != "" && i != len(sel.parts)-1 {
			sel.never = true
		}
	}
	return sel, true
}

type selParser struct {
	s string
	i int
}

func (p *selParser) eof() bool { return p.i >= len(p.s) }

func (p *selParser) skipSpace() bool {
	start := p.i
	for p.i < len(p.s) && isSpace(p.s[p.i]) {
		p.i++
	}
	return p.i > start
}

func isNameStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c >= 0x80 || c == '\\'
}

func isNameChar(c byte) bool {
	return isNameStart(c) || c >= '0' && c <= '9' || c == '-'
}

// ident 读一个标识符（认 \ 转义：\: 是 :，\31 是 1）。
func (p *selParser) ident() (string, bool) {
	start := p.i
	if p.i < len(p.s) && p.s[p.i] == '-' {
		p.i++
		if p.i < len(p.s) && p.s[p.i] == '-' {
			p.i++
		}
	}
	if p.i >= len(p.s) || !isNameStart(p.s[p.i]) && !(p.i > start+1) {
		p.i = start
		return "", false
	}
	var b strings.Builder
	b.WriteString(p.s[start:p.i])
	for p.i < len(p.s) && isNameChar(p.s[p.i]) {
		c := p.s[p.i]
		if c != '\\' {
			b.WriteByte(c)
			p.i++
			continue
		}
		p.i++
		if p.i >= len(p.s) {
			break
		}
		// 十六进制转义：最多 6 位，后面跟一个空白就吞掉。
		j := p.i
		for j < len(p.s) && j-p.i < 6 && isHex(p.s[j]) {
			j++
		}
		if j > p.i {
			n, _ := strconv.ParseUint(p.s[p.i:j], 16, 32)
			if n == 0 || n > 0x10FFFF {
				n = 0xFFFD
			}
			b.WriteRune(rune(n))
			p.i = j
			if p.i < len(p.s) && isSpace(p.s[p.i]) {
				p.i++
			}
			continue
		}
		b.WriteByte(p.s[p.i])
		p.i++
	}
	if b.Len() == 0 {
		p.i = start
		return "", false
	}
	return b.String(), true
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// paren 读一对配平的括号，返回里面的内容（p.i 在 '(' 上）。
func (p *selParser) paren() (string, bool) {
	depth := 0
	start := p.i + 1
	var quote byte
	for ; p.i < len(p.s); p.i++ {
		c := p.s[p.i]
		if quote != 0 {
			if c == '\\' {
				p.i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				arg := p.s[start:p.i]
				p.i++
				return arg, true
			}
		}
	}
	return "", false
}

func (p *selParser) compound() (compound, bool) {
	var c compound
	start := p.i
	if !p.eof() && p.s[p.i] == '*' {
		c.tag = "*"
		p.i++
	} else if !p.eof() && (isNameStart(p.s[p.i]) || p.s[p.i] == '-') {
		name, ok := p.ident()
		if !ok {
			return c, false
		}
		c.tag = strings.ToLower(name)
	}
	for !p.eof() {
		ch := p.s[p.i]
		if c.elem != "" && ch != ':' {
			// 伪元素后面只许跟伪类（:hover 这类），别的读不出。
			return c, false
		}
		switch ch {
		case '#':
			p.i++
			id, ok := p.ident()
			if !ok {
				return c, false
			}
			c.ids = append(c.ids, id)
		case '.':
			p.i++
			cls, ok := p.ident()
			if !ok {
				return c, false
			}
			c.classes = append(c.classes, cls)
		case '[':
			a, ok := p.attr()
			if !ok {
				return c, false
			}
			c.attrs = append(c.attrs, a)
		case ':':
			if !p.pseudo(&c) {
				return c, false
			}
		default:
			if p.i == start {
				return c, false
			}
			return c, true
		}
	}
	return c, p.i > start
}

func (p *selParser) attr() (attrSel, bool) {
	var a attrSel
	p.i++ // [
	p.skipSpace()
	name, ok := p.ident()
	if !ok {
		return a, false
	}
	a.name = strings.ToLower(name)
	p.skipSpace()
	if p.eof() {
		return a, false
	}
	if p.s[p.i] == ']' {
		p.i++
		return a, true
	}
	switch {
	case p.s[p.i] == '=':
		a.op = "="
		p.i++
	case p.i+1 < len(p.s) && p.s[p.i+1] == '=' && strings.IndexByte("~|^$*", p.s[p.i]) >= 0:
		a.op = p.s[p.i : p.i+2]
		p.i += 2
	default:
		return a, false
	}
	p.skipSpace()
	if p.eof() {
		return a, false
	}
	if q := p.s[p.i]; q == '"' || q == '\'' {
		end := p.i + 1
		var b strings.Builder
		for end < len(p.s) && p.s[end] != q {
			if p.s[end] == '\\' && end+1 < len(p.s) {
				end++
			}
			b.WriteByte(p.s[end])
			end++
		}
		if end >= len(p.s) {
			return a, false
		}
		a.val = b.String()
		p.i = end + 1
	} else {
		v, ok := p.ident()
		if !ok {
			// 不带引号的数（[colspan=2]）浏览器不认，这里宽松些也认。
			j := p.i
			for j < len(p.s) && isNameChar(p.s[j]) {
				j++
			}
			if j == p.i {
				return a, false
			}
			v, p.i = p.s[p.i:j], j
		}
		a.val = v
	}
	p.skipSpace()
	if !p.eof() && (p.s[p.i] == 'i' || p.s[p.i] == 'I' || p.s[p.i] == 's' || p.s[p.i] == 'S') {
		a.fold = p.s[p.i] == 'i' || p.s[p.i] == 'I'
		p.i++
		p.skipSpace()
	}
	if p.eof() || p.s[p.i] != ']' {
		return a, false
	}
	p.i++
	return a, true
}

// pseudo 读 :xxx / ::xxx（p.i 在第一个 ':' 上）。认不出的伪类读得出，但把 c 记成 never。
func (p *selParser) pseudo(c *compound) bool {
	p.i++
	double := false
	if !p.eof() && p.s[p.i] == ':' {
		double = true
		p.i++
	}
	name, ok := p.ident()
	if !ok {
		return false
	}
	name = strings.ToLower(name)
	arg, hasArg := "", false
	if !p.eof() && p.s[p.i] == '(' {
		if arg, hasArg = p.paren(); !hasArg {
			return false
		}
		hasArg = true
	}
	switch name {
	case "before", "after":
		if hasArg || c.elem != "" {
			return false
		}
		c.elem = name
		return true
	case "first-line", "first-letter", "marker", "placeholder", "selection", "backdrop":
		if c.elem != "" {
			return false
		}
		c.elem, c.never = name, true
		return true
	}
	if double {
		// 别的伪元素：读得出，永远不中。
		c.elem, c.never = name, true
		return true
	}
	ps := pseudoSel{kind: name}
	switch name {
	case "first-child", "last-child", "only-child", "first-of-type", "last-of-type", "only-of-type",
		"root", "link", "any-link", "checked", "disabled":
		if hasArg {
			return false
		}
	case "nth-child", "nth-last-child", "nth-of-type", "nth-last-of-type":
		a, b, ok := parseNth(arg)
		if !hasArg {
			return false
		}
		if !ok {
			c.never = true
		}
		ps.a, ps.b = a, b
	case "not", "is", "where", "matches", "-webkit-any":
		if !hasArg {
			return false
		}
		for _, one := range SplitComma(arg) {
			sub, ok := parseSelector(one)
			if !ok || sub.parts[len(sub.parts)-1].elem != "" {
				if name == "not" {
					c.never = true
				}
				continue
			}
			ps.list = append(ps.list, sub)
		}
		if name == "matches" || name == "-webkit-any" {
			ps.kind = "is"
		}
	default:
		// :hover、:focus、:visited、:lang() 这类：预览里不会成立。
		c.never = true
	}
	c.pseudos = append(c.pseudos, ps)
	return true
}

// parseNth 读 an+b：odd、even、3、2n+1、-n+3、n。
func parseNth(s string) (a, b int, ok bool) {
	s = strings.ToLower(strings.Join(strings.Fields(s), ""))
	switch s {
	case "odd":
		return 2, 1, true
	case "even":
		return 2, 0, true
	case "":
		return 0, 0, false
	}
	i := strings.IndexByte(s, 'n')
	if i < 0 {
		n, err := strconv.Atoi(s)
		return 0, n, err == nil
	}
	switch as := s[:i]; as {
	case "", "+":
		a = 1
	case "-":
		a = -1
	default:
		n, err := strconv.Atoi(as)
		if err != nil {
			return 0, 0, false
		}
		a = n
	}
	if bs := s[i+1:]; bs != "" {
		if bs[0] != '+' && bs[0] != '-' {
			return 0, 0, false
		}
		n, err := strconv.Atoi(bs)
		if err != nil {
			return 0, 0, false
		}
		b = n
	}
	return a, b, true
}

// spec 是选择器的优先级，(a, b, c) 压成 a<<20 | b<<10 | c。
func (s *selector) spec() int {
	n := 0
	for _, c := range s.parts {
		n += c.spec()
	}
	return n
}

func (c *compound) spec() int {
	a, b, cc := len(c.ids), len(c.classes)+len(c.attrs), 0
	if c.tag != "" && c.tag != "*" {
		cc++
	}
	if c.elem != "" {
		cc++
	}
	n := 0
	for _, p := range c.pseudos {
		switch p.kind {
		case "where":
		case "not", "is":
			best := 0
			for _, s := range p.list {
				best = max(best, s.spec())
			}
			n += best
		default:
			b++
		}
	}
	return min(a, 1023)<<20 | min(b, 1023)<<10 | min(cc, 1023) + n
}

// matchBudget 是一次匹配最多试几步；后代组合符多、树又深时到了就算不中，不卡住。
const matchBudget = 20000

// match 看元素中不中这个选择器（伪元素不在这里看，调用方按 elem 筛）。
func (s *selector) match(e Element) bool {
	if s == nil || s.never || e == nil {
		return false
	}
	budget := matchBudget
	return s.matchAt(len(s.parts)-1, e, &budget)
}

// elem 是选择器指的伪元素（before / after），没有是 ""。
func (s *selector) elem() string { return s.parts[len(s.parts)-1].elem }

func (s *selector) matchAt(i int, e Element, budget *int) bool {
	*budget--
	if *budget < 0 || !s.parts[i].match(e, budget) {
		return false
	}
	if i == 0 {
		return true
	}
	switch s.combs[i-1] {
	case '>':
		p := e.Parent()
		return p != nil && s.matchAt(i-1, p, budget)
	case '+':
		q := e.Prev()
		return q != nil && s.matchAt(i-1, q, budget)
	case '~':
		for q := e.Prev(); q != nil && *budget > 0; q = q.Prev() {
			if s.matchAt(i-1, q, budget) {
				return true
			}
		}
	default:
		for p := e.Parent(); p != nil && *budget > 0; p = p.Parent() {
			if s.matchAt(i-1, p, budget) {
				return true
			}
		}
	}
	return false
}

func (c *compound) match(e Element, budget *int) bool {
	if c.never {
		return false
	}
	if c.tag != "" && c.tag != "*" && !strings.EqualFold(e.Tag(), c.tag) {
		return false
	}
	if len(c.ids) > 0 {
		id, _ := e.Attr("id")
		for _, want := range c.ids {
			if id != want {
				return false
			}
		}
	}
	if len(c.classes) > 0 {
		cls, _ := e.Attr("class")
		have := strings.Fields(cls)
		for _, want := range c.classes {
			if !contains(have, want) {
				return false
			}
		}
	}
	for _, a := range c.attrs {
		if !a.match(e) {
			return false
		}
	}
	for _, p := range c.pseudos {
		if !p.match(e, budget) {
			return false
		}
	}
	return true
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (a attrSel) match(e Element) bool {
	v, ok := e.Attr(a.name)
	if !ok {
		return false
	}
	want := a.val
	if a.fold {
		v, want = strings.ToLower(v), strings.ToLower(want)
	}
	switch a.op {
	case "":
		return true
	case "=":
		return v == want
	case "~=":
		return want != "" && contains(strings.Fields(v), want)
	case "|=":
		return v == want || strings.HasPrefix(v, want+"-")
	case "^=":
		return want != "" && strings.HasPrefix(v, want)
	case "$=":
		return want != "" && strings.HasSuffix(v, want)
	case "*=":
		return want != "" && strings.Contains(v, want)
	}
	return false
}

func (p pseudoSel) match(e Element, budget *int) bool {
	switch p.kind {
	case "first-child":
		return e.Prev() == nil
	case "last-child":
		return e.Next() == nil
	case "only-child":
		return e.Prev() == nil && e.Next() == nil
	case "first-of-type":
		return siblingIndex(e, true, false) == 1
	case "last-of-type":
		return siblingIndex(e, true, true) == 1
	case "only-of-type":
		return siblingIndex(e, true, false) == 1 && siblingIndex(e, true, true) == 1
	case "nth-child":
		return nthMatch(p.a, p.b, siblingIndex(e, false, false))
	case "nth-last-child":
		return nthMatch(p.a, p.b, siblingIndex(e, false, true))
	case "nth-of-type":
		return nthMatch(p.a, p.b, siblingIndex(e, true, false))
	case "nth-last-of-type":
		return nthMatch(p.a, p.b, siblingIndex(e, true, true))
	case "root":
		return e.Parent() == nil
	case "link", "any-link":
		t := strings.ToLower(e.Tag())
		_, ok := e.Attr("href")
		return ok && (t == "a" || t == "area")
	case "checked", "disabled":
		_, ok := e.Attr(p.kind)
		return ok
	case "not":
		for _, s := range p.list {
			*budget--
			if s.match(e) {
				return false
			}
		}
		return true
	case "is", "where":
		for _, s := range p.list {
			*budget--
			if s.match(e) {
				return true
			}
		}
		return false
	}
	return false
}

// siblingIndex 是元素在兄弟里排第几（从 1 数）；ofType 只数同标签的，fromEnd 从后往前数。
func siblingIndex(e Element, ofType, fromEnd bool) int {
	n := 1
	step := Element.Prev
	if fromEnd {
		step = Element.Next
	}
	for q := step(e); q != nil; q = step(q) {
		if !ofType || strings.EqualFold(q.Tag(), e.Tag()) {
			n++
		}
		if n > 100000 {
			break
		}
	}
	return n
}

func nthMatch(a, b, k int) bool {
	if a == 0 {
		return k == b
	}
	d := k - b
	return d%a == 0 && d/a >= 0
}
