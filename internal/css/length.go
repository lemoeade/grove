package css

import (
	"strconv"
	"strings"
)

// Unit 是长度的单位。
type Unit uint8

const (
	None    Unit = iota // 不带单位的数（line-height: 1.45、flex-grow）
	Px                  // px；pt、pc、in、cm、mm 读的时候换成 px
	Em                  // 相对元素的字号；ex、ch 按半个 em 算
	Rem                 // 相对根字号
	Percent             // 百分数
	Fr                  // grid 的份数
	Auto                // auto（还有 min-content / max-content 这类，都当 auto）
	Vw                  // 视口宽的百分之几（预览没有视口，Resolve 按 pct 算）
	Vh                  // 视口高的百分之几（同上）
)

// Length 是一个长度。
type Length struct {
	V float32
	U Unit
}

// ParseLength 读一个长度："12px"、"1.5em"、"2rem"、"50%"、"1fr"、"auto"、"1.45"（不带单位）；"0" 是 0px。
// pt、pc、in、cm、mm 换成 px。calc() 这类不认。
func ParseLength(s string) (Length, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "":
		return Length{}, false
	case "auto", "min-content", "max-content", "fit-content":
		return Length{U: Auto}, true
	}
	n := numPrefix(s)
	if n == 0 {
		return Length{}, false
	}
	f, err := strconv.ParseFloat(s[:n], 32)
	if err != nil {
		return Length{}, false
	}
	v := float32(f)
	switch s[n:] {
	case "":
		if v == 0 {
			return Length{0, Px}, true
		}
		return Length{v, None}, true
	case "px":
		return Length{v, Px}, true
	case "pt":
		return Length{v * 4 / 3, Px}, true
	case "pc":
		return Length{v * 16, Px}, true
	case "in":
		return Length{v * 96, Px}, true
	case "cm":
		return Length{v * 96 / 2.54, Px}, true
	case "mm":
		return Length{v * 96 / 25.4, Px}, true
	case "q":
		return Length{v * 96 / 101.6, Px}, true
	case "em":
		return Length{v, Em}, true
	case "ex", "ch":
		return Length{v / 2, Em}, true
	case "rem":
		return Length{v, Rem}, true
	case "%":
		return Length{v, Percent}, true
	case "fr":
		return Length{v, Fr}, true
	case "vw", "vmin":
		return Length{v, Vw}, true
	case "vh", "vmax":
		return Length{v, Vh}, true
	}
	return Length{}, false
}

// numPrefix 是 s 开头的数有几个字节（[+-]数字.数字[e[+-]数字]），没有数是 0。
func numPrefix(s string) int {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return 0
	}
	if i+1 < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if s[j] == '+' || s[j] == '-' {
			j++
		}
		if j < len(s) && s[j] >= '0' && s[j] <= '9' {
			for j < len(s) && s[j] >= '0' && s[j] <= '9' {
				j++
			}
			i = j
		}
	}
	return i
}

// Resolve 换成 px：em 是元素的字号，rem 是根字号，pct 是百分之百对着多少（vw、vh 也按它）。
// 不带单位的数原样给出 V（是倍数还是 px 由调用方定），auto 是 0。
func (l Length) Resolve(em, rem, pct float32) float32 {
	switch l.U {
	case Em:
		return l.V * em
	case Rem:
		return l.V * rem
	case Percent, Vw, Vh:
		return l.V * pct / 100
	case Auto:
		return 0
	}
	return l.V
}

// Track 是 grid-template-columns 里的一列：minmax(Min, Max)。一个长度是 Min = Max，1fr 是 minmax(auto, 1fr)，
// auto 两头都是 auto，fit-content(x) 是 minmax(auto, x)。
type Track struct {
	Min, Max Length
}

// ParseTracks 读 grid-template-columns / rows：长度、fr、auto、minmax()、fit-content()、repeat(n, …)；
// 线名 [a] 跳过。repeat(auto-fill / auto-fit, …) 和读不出的给 false。
func ParseTracks(s string) ([]Track, bool) {
	var out []Track
	for _, t := range Fields(s) {
		lt := strings.ToLower(t)
		if strings.HasPrefix(lt, "[") {
			continue
		}
		if strings.HasPrefix(lt, "repeat(") && strings.HasSuffix(lt, ")") {
			args := SplitComma(t[len("repeat(") : len(t)-1])
			if len(args) != 2 {
				return nil, false
			}
			n, err := strconv.Atoi(strings.TrimSpace(args[0]))
			if err != nil || n < 1 || n > 1000 {
				return nil, false
			}
			inner, ok := ParseTracks(args[1])
			if !ok {
				return nil, false
			}
			for range n {
				out = append(out, inner...)
			}
			continue
		}
		tr, ok := parseTrack(t)
		if !ok {
			return nil, false
		}
		out = append(out, tr)
	}
	return out, len(out) > 0
}

func parseTrack(t string) (Track, bool) {
	lt := strings.ToLower(t)
	auto := Length{U: Auto}
	switch {
	case strings.HasPrefix(lt, "minmax(") && strings.HasSuffix(lt, ")"):
		args := SplitComma(t[len("minmax(") : len(t)-1])
		if len(args) != 2 {
			return Track{}, false
		}
		lo, ok1 := ParseLength(args[0])
		hi, ok2 := ParseLength(args[1])
		if !ok1 || !ok2 || lo.U == Fr {
			return Track{}, false
		}
		return Track{lo, hi}, true
	case strings.HasPrefix(lt, "fit-content(") && strings.HasSuffix(lt, ")"):
		hi, ok := ParseLength(t[len("fit-content(") : len(t)-1])
		if !ok {
			return Track{}, false
		}
		return Track{auto, hi}, true
	}
	l, ok := ParseLength(t)
	if !ok || l.U == None && l.V != 0 {
		return Track{}, false
	}
	if l.U == Fr {
		return Track{auto, l}, true
	}
	return Track{l, l}, true
}

// Fields 按顶层的空白切开一个值：括号、引号里的空白不切（"1px solid rgb(0 0 0)" 是三段）。
func Fields(v string) []string {
	var out []string
	depth, start := 0, -1
	var quote byte
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			if depth > 0 {
				depth--
			}
		case depth == 0 && isSpace(c):
			if start >= 0 {
				out = append(out, v[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, v[start:])
	}
	return out
}

// SplitComma 按顶层的逗号切开（括号、引号里的不切），每段去掉两头空白。
func SplitComma(v string) []string {
	return splitTop(v, ',')
}

// splitTop 按顶层的 sep 切开，每段去掉两头空白。
func splitTop(v string, sep byte) []string {
	var out []string
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '(' || c == '[':
			depth++
		case c == ')' || c == ']':
			if depth > 0 {
				depth--
			}
		case c == sep && depth == 0:
			out = append(out, strings.TrimSpace(v[start:i]))
			start = i + 1
		}
	}
	return append(out, strings.TrimSpace(v[min(start, len(v)):]))
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' }
