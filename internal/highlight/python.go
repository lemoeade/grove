package highlight

import (
	"strings"
	"sync"

	"github.com/egoist/mygo/ui/editor"
)

// Python 的高亮在 mygo 分出来的词上改几处，免得满屏红蓝紫：
//   - 单独的 : = -> 是标点不是运算符（mygo 把它们和 + == 一样算运算符，GitHub 配色里运算符和关键字一样红，
//     注解 db: Session、默认值、关键字参数、行尾的 ): else: 全是红的）；
//   - 点后面的名字（属性链、导入路径 sqlalchemy.orm）是普通变量，不是蓝色的属性；
//   - self、cls 是斜体（mygo 算成函数，和函数调用一样紫）；
//   - 文件开头或者以 : 结尾的那行（def、class）下面单独成行的三引号字符串（docstring）画成注释，跨行的一直到字符串结束。

// 状态的高三位是 Grove 自己的（mygo 的分词器最多用 40 位）：pyDoc 正在一段 docstring 里，pyColon 上一行代码以 : 结尾，
// pyCode 前面有过代码（不算空行和注释）。
const (
	pyDoc   = uint64(1) << 63
	pyColon = uint64(1) << 62
	pyCode  = uint64(1) << 61
	pyBits  = pyDoc | pyColon | pyCode
)

type pyLexer struct{ base editor.Tokenizer }

func (p *pyLexer) Tokenize(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	doc := state&pyDoc != 0
	inner := state &^ pyBits
	start := len(tokens)
	tokens, inner = p.base.Tokenize(line, inner, tokens)
	toks := tokens[start:]

	// 这一行除了空白只有字符串（和里面的转义）：开头是三引号、前面是 def / class 那行或者什么代码都还没有的，
	// 或者接着上一行的 docstring（空行也接着）
	only := true
	prev := 0
	for _, tk := range toks {
		if s := strings.TrimSpace(line[prev:tk.End]); s != "" && tk.Kind != editor.TokenString && tk.Kind != editor.TokenEscape {
			only = false
			break
		}
		prev = tk.End
	}
	if !doc && only && state&^pyBits == 0 && (state&pyColon != 0 || state&pyCode == 0) {
		s := strings.TrimLeft(strings.TrimLeft(line, " \t"), "rRuUbB") // r"""…""" 也算
		doc = strings.HasPrefix(s, `"""`) || strings.HasPrefix(s, "'''")
	}
	if !only {
		doc = false
	}

	bits := state & (pyColon | pyCode)
	prev = 0
	for i := range toks {
		tk := &toks[i]
		w := strings.TrimSpace(line[prev:tk.End])
		prev = tk.End
		switch {
		case doc:
			tk.Kind = editor.TokenComment
		case tk.Kind == editor.TokenOperator && (w == ":" || w == "=" || w == "->"):
			tk.Kind = editor.TokenPunctuation
		case tk.Kind == editor.TokenProperty:
			tk.Kind = editor.TokenVariable
		case tk.Kind == editor.TokenFunction && (w == "self" || w == "cls"):
			tk.Kind = editor.TokenEmphasis
		}
		if w != "" && tk.Kind != editor.TokenComment {
			bits = pyCode
			if strings.HasSuffix(w, ":") && tk.Kind == editor.TokenPunctuation {
				bits |= pyColon
			}
		}
	}
	if doc {
		bits = pyCode
		if inner != 0 { // 字符串还没完：下一行还是 docstring
			bits |= pyDoc
		}
	}
	return mergeKinds(tokens, start), inner | bits
}

// mergeKinds 把 tokens[start:] 里挨着的同类词接成一个（改了类以后 ) 和 : 都是标点了）。
func mergeKinds(tokens []editor.Token, start int) []editor.Token {
	out := tokens[:start]
	for _, tk := range tokens[start:] {
		if n := len(out); n > start && out[n-1].Kind == tk.Kind {
			out[n-1].End = tk.End
			continue
		}
		out = append(out, tk)
	}
	return out
}

// python 是 Grove 的 Python：除了分词器都照 mygo 的。
var python = sync.OnceValue(func() *editor.Language {
	base := editor.LanguageNamed("Python")
	if base == nil || base.Tokenizer == nil {
		return base
	}
	l := *base
	l.Tokenizer = &pyLexer{base: base.Tokenizer}
	return &l
})

// Python 是 Grove 的 Python 这种语言（同一个指针）；mygo 没有 Python 时为 nil。
func Python() *editor.Language { return python() }
