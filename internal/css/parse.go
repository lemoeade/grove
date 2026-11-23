package css

import "strings"

// Decl 是一条声明：prop: value [!important]。Prop 是小写（自定义属性 --x 保留原样），Value 去掉了两头空白和 !important。
type Decl struct {
	Prop, Value string
	Important   bool
}

// rule 是样式表里的一条规则：一个选择器（选择器列表拆成几条）和它的声明。
type rule struct {
	sel   *selector
	spec  int
	order int
	decls []Decl
}

// Sheet 是解析好的样式表。零值和 nil 都是空表。
type Sheet struct {
	rules []rule
}

// stripComments 去掉 /* … */ 注释（引号里的不算）。
func stripComments(s string) string {
	if !strings.Contains(s, "/*") {
		return s
	}
	var b strings.Builder
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				b.WriteByte(s[i])
			} else if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			b.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '*' {
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				break
			}
			i += 2 + end + 1
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// ParseDecls 读一串声明（style= 的值，或规则花括号里的）。读不出的那条跳过。
func ParseDecls(s string) []Decl {
	s = stripComments(s)
	var out []Decl
	for _, part := range splitTop(s, ';') {
		name, val, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if !isIdent(name) {
			continue
		}
		if !strings.HasPrefix(name, "--") {
			name = strings.ToLower(name)
		}
		val = strings.TrimSpace(val)
		imp := false
		if i := strings.LastIndexByte(val, '!'); i >= 0 && strings.EqualFold(strings.TrimSpace(val[i+1:]), "important") {
			imp = true
			val = strings.TrimSpace(val[:i])
		}
		if val == "" && !strings.HasPrefix(name, "--") || strings.ContainsAny(val, "{}") {
			continue
		}
		out = append(out, Decl{name, val, imp})
	}
	return out
}

// isIdent 看 s 是不是一个属性名：字母、数字、- 和 _，不以数字开头（-- 开头的自定义属性也算）。
func isIdent(s string) bool {
	if s == "" || s == "-" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '-', c == '_', c >= 0x80:
		case c >= '0' && c <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// ParseSheet 读一张样式表（<style> 的内容）。@media、@font-face 这类 @ 规则整块跳过；选择器列表里认不出的那个跳过，
// 别的照常用。
func ParseSheet(src string) *Sheet {
	s := &Sheet{}
	src = stripComments(src)
	i := 0
	for i < len(src) {
		// 选择器（或 @ 规则的头）到顶层的 { ; } 为止。
		start := i
		var quote byte
		depth := 0
		for i < len(src) {
			c := src[i]
			if quote != 0 {
				if c == '\\' {
					i++
				} else if c == quote {
					quote = 0
				}
				i++
				continue
			}
			if c == '"' || c == '\'' {
				quote = c
			} else if c == '(' || c == '[' {
				depth++
			} else if (c == ')' || c == ']') && depth > 0 {
				depth--
			} else if depth == 0 && (c == '{' || c == ';' || c == '}') {
				break
			}
			i++
		}
		if i >= len(src) {
			break
		}
		prelude := strings.TrimSpace(src[start:i])
		if src[i] != '{' {
			// @import …; 或多出来的 }：跳过。
			i++
			continue
		}
		bodyStart := i + 1
		i = skipBlock(src, i)
		bodyEnd := i
		if bodyEnd > bodyStart && src[bodyEnd-1] == '}' {
			bodyEnd--
		}
		body := src[bodyStart:max(bodyStart, bodyEnd)]
		if prelude == "" || prelude[0] == '@' {
			continue
		}
		decls := ParseDecls(body)
		if len(decls) == 0 {
			continue
		}
		for _, one := range SplitComma(prelude) {
			sel, ok := parseSelector(one)
			if !ok {
				continue
			}
			s.rules = append(s.rules, rule{sel: sel, spec: sel.spec(), order: len(s.rules), decls: decls})
		}
	}
	return s
}

// skipBlock 从 src[i] == '{' 起跳过配对的整块，返回块后面那个位置（没配对到时是 len(src)）。
func skipBlock(src string, i int) int {
	depth := 0
	var quote byte
	for ; i < len(src); i++ {
		c := src[i]
		if quote != 0 {
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(src)
}

// Add 把 o 的规则接在 s 后面（后来的同优先级压过先来的）。
func (s *Sheet) Add(o *Sheet) {
	if s == nil || o == nil {
		return
	}
	for _, r := range o.rules {
		r.order = len(s.rules)
		s.rules = append(s.rules, r)
	}
}

// Empty 是表里一条规则都没有。
func (s *Sheet) Empty() bool { return s == nil || len(s.rules) == 0 }
