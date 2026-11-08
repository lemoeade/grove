package highlight

import (
	"strings"
	"sync"

	"github.com/egoist/mygo/ui/editor"
)

// Vue 单文件组件（.vue）的高亮。mygo 把 .vue 当 HTML：<script> 一律按 JavaScript 分（lang="ts" 不管），
// 模板里的 {{ }} 是普通文字，v-if、:prop、@click、#slot 这些指令的值是普通字符串。这里自己分外层的标签，
// 里面的代码交给 mygo 的 TypeScript / JavaScript / CSS 分词器：<script lang="ts">（或 tsx）按 TypeScript，
// 别的 <script> 按 JavaScript，<style>（不管 lang）按 CSS，{{ }} 和指令的值按 TypeScript（JavaScript 的超集）。

// vueLexer 的状态：第 0–3 位是模式（下面的 v*），第 4–5 位是正在分的标签打开什么（vueEmbed*），第 6 位是 <script>
// 带了 lang="ts"，第 7–8 位是刚分完名字的属性是什么（vueAttr*），第 24–63 位是里面那段代码的分词器的状态
// （mygo 的分词器最多用 40 位：模式 8、括号深度 8、括号栈 24）。
type vueLexer struct {
	js, ts, css editor.Tokenizer
}

const (
	vText = iota
	vTag
	vComment
	vDouble     // 普通属性值，在 "" 里
	vSingle     // 在 '' 里
	vExprDouble // 指令的值（代码），在 "" 里
	vExprSingle
	vInterp // {{ }} 里
	vScript
	vStyle
)

const (
	vueEmbedNone = iota
	vueEmbedScript
	vueEmbedStyle
)

const (
	vueAttrPlain = iota
	vueAttrLang
	vueAttrDirective
)

const vueInnerShift = 24

// vueOut 攒一行的词：和 mygo 的一样，同类的接上，到不了更后面的不要。
type vueOut struct{ toks []editor.Token }

func (o *vueOut) add(end int, k editor.TokenKind) {
	if end <= 0 {
		return
	}
	if n := len(o.toks); n > 0 {
		last := &o.toks[n-1]
		if last.End >= end {
			return
		}
		if last.Kind == k {
			last.End = end
			return
		}
	}
	o.toks = append(o.toks, editor.Token{End: end, Kind: k})
}

// code 用 t 从 inner 分 line[from:to]，词挪到这一行里；剩下没分到的算普通文字。返回 t 分完的状态。
func (o *vueOut) code(t editor.Tokenizer, line string, from, to int, inner uint64, scratch *[]editor.Token) uint64 {
	if t != nil && to > from {
		*scratch, inner = t.Tokenize(line[from:to], inner, (*scratch)[:0])
		for _, tk := range *scratch {
			o.add(from+tk.End, tk.Kind)
		}
	}
	o.add(to, editor.TokenText)
	return inner
}

func (v *vueLexer) Tokenize(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	out := vueOut{toks: tokens}
	mode := int(state & 0xf)
	embed := int(state >> 4 & 3)
	ts := state>>6&1 == 1
	attr := int(state >> 7 & 3)
	inner := state >> vueInnerShift
	var scratch []editor.Token
	n := len(line)
	for i := 0; i < n; {
		switch mode {
		case vScript, vStyle:
			tag, t := "</script", v.js
			if mode == vStyle {
				tag, t = "</style", v.css
			} else if ts {
				t = v.ts
			}
			end := indexFold(line[i:], tag)
			stop := n
			if end >= 0 {
				stop = i + end
			}
			inner = out.code(t, line, i, stop, inner, &scratch)
			i = stop
			if end >= 0 {
				mode, inner, ts = vText, 0, false
			}
		case vInterp:
			end := strings.Index(line[i:], "}}")
			stop := n
			if end >= 0 {
				stop = i + end
			}
			inner = out.code(v.ts, line, i, stop, inner, &scratch)
			i = stop
			if end >= 0 {
				out.add(i+2, editor.TokenPunctuation)
				i += 2
				mode, inner = vText, 0
			}
		case vExprDouble, vExprSingle:
			q := byte('"')
			if mode == vExprSingle {
				q = '\''
			}
			end := strings.IndexByte(line[i:], q)
			stop := n
			if end >= 0 {
				stop = i + end
			}
			inner = out.code(v.ts, line, i, stop, inner, &scratch)
			i = stop
			if end >= 0 {
				out.add(i+1, editor.TokenString)
				i++
				mode, inner = vTag, 0
			}
		case vComment:
			end := strings.Index(line[i:], "-->")
			if end < 0 {
				out.add(n, editor.TokenComment)
				i = n
				continue
			}
			out.add(i+end+3, editor.TokenComment)
			i += end + 3
			mode = vText
		case vDouble, vSingle:
			q := byte('"')
			if mode == vSingle {
				q = '\''
			}
			end := strings.IndexByte(line[i:], q)
			if end < 0 {
				out.add(n, editor.TokenString)
				i = n
				continue
			}
			out.add(i+end+1, editor.TokenString)
			i += end + 1
			mode = vTag
		case vTag:
			c := line[i]
			switch {
			case c == '>':
				out.add(i+1, editor.TokenPunctuation)
				i++
				mode = vText
				switch embed {
				case vueEmbedScript:
					mode = vScript
				case vueEmbedStyle:
					mode = vStyle
				}
				if mode != vScript {
					ts = false
				}
				embed, attr = vueEmbedNone, vueAttrPlain
			case c == '/' && i+1 < n && line[i+1] == '>':
				out.add(i+2, editor.TokenPunctuation)
				i += 2
				mode, embed, attr, ts = vText, vueEmbedNone, vueAttrPlain, false
			case c == '"' || c == '\'':
				out.add(i+1, editor.TokenString)
				i++
				switch {
				case attr == vueAttrDirective && c == '"':
					mode = vExprDouble
				case attr == vueAttrDirective:
					mode = vExprSingle
				case c == '"':
					mode = vDouble
				default:
					mode = vSingle
				}
				if attr == vueAttrLang && embed == vueEmbedScript {
					val := line[i:]
					if j := strings.IndexByte(val, c); j >= 0 {
						val = val[:j]
					}
					switch strings.ToLower(strings.TrimSpace(val)) {
					case "ts", "tsx", "typescript":
						ts = true
					default:
						ts = false
					}
				}
				attr = vueAttrPlain
			case c == '=':
				out.add(i+1, editor.TokenOperator)
				i++
			case c == ' ' || c == '\t' || c == '\r':
				out.add(i+1, editor.TokenText)
				i++
			default:
				j := i
				for j < n && strings.IndexByte(" \t\r=>/\"'", line[j]) < 0 {
					j++
				}
				if j == i {
					j++
				}
				out.add(j, editor.TokenAttribute)
				attr = attrKind(line[i:j])
				i = j
			}
		default:
			c := line[i]
			switch {
			case strings.HasPrefix(line[i:], "<!--"):
				out.add(i+4, editor.TokenComment)
				i += 4
				mode = vComment
			case strings.HasPrefix(line[i:], "{{"):
				out.add(i+2, editor.TokenPunctuation)
				i += 2
				mode, inner = vInterp, 0
			case c == '<' && i+1 < n && (line[i+1] == '!' || line[i+1] == '?'):
				j := i + 2
				for j < n && line[j] != '>' && line[j] != ' ' {
					j++
				}
				out.add(j, editor.TokenMeta)
				i = j
				mode = vTag
			case c == '<' && i+1 < n && (line[i+1] == '/' || isLetter(line[i+1])):
				out.add(i+1, editor.TokenPunctuation)
				j := i + 1
				closing := line[j] == '/'
				if closing {
					out.add(j+1, editor.TokenPunctuation)
					j++
				}
				k := j
				for k < n && (isIdentByte(line[k]) || line[k] == '-' || line[k] == ':' || line[k] == '.') {
					k++
				}
				out.add(k, editor.TokenTag)
				embed, attr, ts = vueEmbedNone, vueAttrPlain, false
				if !closing {
					switch strings.ToLower(line[j:k]) {
					case "script":
						embed = vueEmbedScript
					case "style":
						embed = vueEmbedStyle
					}
				}
				i = k
				mode = vTag
			case c == '&':
				j := i + 1
				for j < n && j-i < 32 && (isIdentByte(line[j]) || line[j] == '#') {
					j++
				}
				if j < n && line[j] == ';' {
					out.add(j+1, editor.TokenEscape)
					i = j + 1
				} else {
					out.add(i+1, editor.TokenText)
					i++
				}
			default:
				j := i + 1
				for j < n && line[j] != '<' && line[j] != '&' && !strings.HasPrefix(line[j:], "{{") {
					j++
				}
				out.add(j, editor.TokenText)
				i = j
			}
		}
	}
	var b uint64
	if ts {
		b = 1
	}
	return out.toks, uint64(mode) | uint64(embed)<<4 | b<<6 | uint64(attr)<<7 | inner<<vueInnerShift
}

// attrKind 是叫 name 的属性的值怎么分：v-xxx、:xxx（v-bind）、@xxx（v-on）、#xxx（v-slot）的是代码，lang 要看是不是 ts。
func attrKind(name string) int {
	switch {
	case strings.HasPrefix(name, "v-") || strings.HasPrefix(name, ":") || strings.HasPrefix(name, "@") || strings.HasPrefix(name, "#"):
		return vueAttrDirective
	case strings.EqualFold(name, "lang"):
		return vueAttrLang
	}
	return vueAttrPlain
}

func isLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func isIdentByte(c byte) bool { return isLetter(c) || c >= '0' && c <= '9' || c == '_' }

// indexFold 是 sub（小写）在 s 里第一次出现的地方，不管 s 里 ASCII 字母的大小写；没有为 -1。
func indexFold(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if strings.EqualFold(s[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}

// vue 是 Vue 这种语言：缩进、括号、注释都照 mygo 的 HTML，分词换成 vueLexer。
var vue = sync.OnceValue(func() *editor.Language {
	html := editor.LanguageNamed("HTML")
	l := &editor.Language{Name: "Vue", Extensions: []string{".vue"}, BlockComment: [2]string{"<!--", "-->"}, TabSize: 2}
	if html != nil {
		c := *html
		c.Name, c.Extensions, c.Filenames = "Vue", []string{".vue"}, nil
		c.Highlighter = nil
		l = &c
	}
	tok := func(name string) editor.Tokenizer {
		if x := editor.LanguageNamed(name); x != nil {
			return x.Tokenizer
		}
		return nil
	}
	l.Tokenizer = &vueLexer{js: tok("JavaScript"), ts: tok("TypeScript"), css: tok("CSS")}
	return l
})

// Vue 是 Vue 单文件组件这种语言（同一个指针）。
func Vue() *editor.Language { return vue() }
