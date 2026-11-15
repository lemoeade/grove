package app

import (
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/outline"
)

// 这个文件是按词补全：没有语言服务器，补全的候选是
//   - 这个文件里的词（光标附近 wordWindow 以内，离光标近的在前），在大纲里的带上种类（函数、类型……）；
//   - 开着的同一种语言的文件里的词（按标签和内容版本记着，wordCache）；
//   - 符号索引（symindex.go）建过的话，同一仓库同一种语言的文件里声明的名字，小字写在哪个文件。
// 和片段一起交给编辑器（Complete）。代码文件敲字就出（设置"敲字时提示补全"，默认开），Markdown、纯文本只在 ⌃Space 时给。
// 回车会接补全，所以敲完一个整词（"nil"）时列表里正好是它的那项要拿掉，不然回车换不了行：CompleteTick 每帧看一眼。

const (
	wordWindow   = 128 << 10 // 这个文件只扫光标前后这么多字节
	wordOtherMax = 1 << 20   // 别的开着的文件超过这么大不扫
	wordMinRunes = 3         // 短于这么多字的词不给
	wordMaxBytes = 64        // 长于这么多字节的不给（多半是哈希、base64）
	wordMaxItems = 400       // 一次最多给多少个词
	wordOtherDoc = 20        // 最多扫几个别的开着的文件
)

// wordCache 是别的开着的文件里的词（按标签、内容版本记），和这次给出去的那批词（offer）。
type wordCache struct {
	docs  map[*docs.Doc]wordList
	offer *wordOffer
}

type wordList struct {
	ver   uint64
	words []string
}

// wordOffer 是给编辑器的那批补全：从 start 开始的词，给的时候缓冲区的版本和光标。
type wordOffer struct {
	d     *docs.Doc
	start int
	ver   uint64
	cur   int
	items []editor.Completion
}

// Complete 是编辑器要补全时（敲字、⌃Space，State.OnCompletion）：片段在前、词在后，一起给。
func (a *App) Complete(d *docs.Doc, req editor.CompletionRequest) {
	snips, offer := a.snippetCompletions(d, req)
	d.SnipOffer = offer
	words := a.wordItems(d, req)
	items := append(snips, words...)
	a.words.offer = nil
	if len(items) == 0 {
		return
	}
	start := req.Offset - len(req.Prefix)
	if len(words) > 0 {
		a.words.offer = &wordOffer{d: d, start: start, ver: d.Buf.Version(), cur: req.Offset, items: items}
	}
	d.St.ShowCompletions(start, items)
}

// autoWords 是 d 敲字时就给词：设置开着、能改的代码文件（Markdown、纯文本这些只在 ⌃Space 时给）。
func (a *App) autoWords(d *docs.Doc) bool {
	if a.St.Settings.NoWordComplete || d.Buf.Language() == nil {
		return false
	}
	return !docs.IsProse(d.Name())
}

// wordItems 是补全里的词：匹配 req.Prefix（字按顺序出现、不分大小写）的，以它开头的在前；不给正好等于它的。
func (a *App) wordItems(d *docs.Doc, req editor.CompletionRequest) []editor.Completion {
	if !snipEditable(d) || d.Snip != nil || !req.Explicit && (req.Prefix == "" || !a.autoWords(d)) {
		return nil
	}
	start := req.Offset - len(req.Prefix)
	var first, rest []editor.Completion
	seen := map[string]bool{req.Prefix: true}
	add := func(w string, kind editor.CompletionKind, detail, doc string) bool {
		if seen[w] {
			return true
		}
		switch {
		case !strings.HasPrefix(strings.ToLower(w), strings.ToLower(req.Prefix)) && !subseq(w, req.Prefix):
			return true
		case req.Prefix != "" && strings.HasPrefix(strings.ToLower(w), strings.ToLower(req.Prefix)):
			seen[w] = true
			first = append(first, editor.Completion{Label: w, Kind: kind, Detail: detail, Documentation: doc})
		default:
			seen[w] = true
			rest = append(rest, editor.Completion{Label: w, Kind: kind, Detail: detail, Documentation: doc})
		}
		return len(first) < wordMaxItems
	}

	// 这个文件：光标附近的，近的在前；种类看上次解析的大纲（每个键都整篇解析太贵，见 TestTypingNoFullRecompute）
	kinds := map[string]outline.Kind{}
	if t := d.LastOutline(); t != nil {
		for _, f := range t.Flat {
			if isDefKind(f.Kind) {
				kinds[f.Name] = f.Kind
			}
		}
	}
	for _, w := range nearWords(d.Buf, req.Offset, start) {
		k := editor.CompletionText
		if sk, ok := kinds[w]; ok {
			k = completionKind(sk)
		}
		if !add(w, k, "", "") {
			break
		}
	}

	// 开着的同一种语言的文件
	fam := langFamily(d.Name())
	all := a.AllDocs()
	open := map[string]bool{d.Abs: true}
	n := 0
	for _, od := range all {
		if od == d || od.Kind != docs.File || od.Buf == nil || od.Abs == "" || open[od.Abs] || langFamily(od.Name()) != fam {
			continue
		}
		open[od.Abs] = true
		if n++; n > wordOtherDoc {
			break
		}
		for _, w := range a.docWords(od) {
			if !add(w, editor.CompletionText, od.Name(), "") {
				break
			}
		}
	}
	a.pruneWords(all)

	// 符号索引里同一仓库的（建过才用，不为补全去建）
	if r, _ := a.repoOfPath(d.Abs); r != nil && fam != "" {
		for abs, f := range a.syms.files {
			if f.r != r || open[abs] || langFamily(f.rel) != fam {
				continue
			}
			for _, s := range f.syms {
				if isDefKind(s.Kind) && !add(s.Name, completionKind(s.Kind), f.rel, s.Detail) {
					break
				}
			}
		}
	}
	out := append(first, rest...)
	return out[:min(len(out), wordMaxItems)]
}

// CompleteTick 每帧（画编辑器前）跟上 d：给过词、光标往后打了字，打出来的正好是列表里的一个词时把它拿掉重给一遍
// （回车就是换行，不是"接"一个一模一样的词）；拿完了列表就关。光标跑出那个词、换了行，这批作废。
func (a *App) CompleteTick(d *docs.Doc) {
	o := a.words.offer
	if o == nil || o.d != d {
		return
	}
	if d.Buf == nil {
		a.words.offer = nil
		return
	}
	ver := d.Buf.Version()
	if ver == o.ver {
		return
	}
	cur, n := d.St.Cursor(), d.Buf.Len()
	if cur < o.start || o.start > n || d.Buf.Pos(cur).Line != d.Buf.Pos(o.start).Line {
		a.words.offer = nil
		return
	}
	typed := d.Buf.Slice(o.start, cur)
	if !isIdentText(typed) {
		a.words.offer = nil
		return
	}
	forward := cur > o.cur
	o.ver, o.cur = ver, cur
	if !forward {
		return // 退格：编辑器自己的列表怎么样不知道，不去重新打开
	}
	i := slices.IndexFunc(o.items, func(c editor.Completion) bool { return c.Kind != editor.CompletionSnippet && c.Label == typed })
	if i < 0 {
		return
	}
	o.items = slices.Delete(slices.Clone(o.items), i, i+1)
	d.St.ShowCompletions(o.start, o.items)
}

// nearWords 是 b 里 off 前后 wordWindow 以内的词，离 off 近的在前；从 skip 开始的那个词（正在打的）不算。
func nearWords(b *editor.Buffer, off, skip int) []string {
	n := b.Len()
	lo, hi := max(0, off-wordWindow), min(n, off+wordWindow)
	text := b.Slice(lo, hi)
	dist := map[string]int{}
	var order []string
	scanWords(text, lo > 0, hi < n, func(w string, at int) {
		if lo+at == skip {
			return
		}
		dd := lo + at - off
		if dd < 0 {
			dd = -dd
		}
		if old, ok := dist[w]; !ok {
			dist[w] = dd
			order = append(order, w)
		} else if dd < old {
			dist[w] = dd
		}
	})
	slices.SortStableFunc(order, func(x, y string) int { return dist[x] - dist[y] })
	return order
}

// docWords 是别的开着的文件 d 里的词（不重复，按出现的先后），按内容版本记着；太大的不扫。
func (a *App) docWords(d *docs.Doc) []string {
	ver := d.Buf.Version()
	if l, ok := a.words.docs[d]; ok && l.ver == ver {
		return l.words
	}
	var words []string
	if d.Buf.Len() <= wordOtherMax {
		seen := map[string]bool{}
		scanWords(d.Buf.String(), false, false, func(w string, _ int) {
			if !seen[w] {
				seen[w] = true
				words = append(words, w)
			}
		})
	}
	if a.words.docs == nil {
		a.words.docs = map[*docs.Doc]wordList{}
	}
	a.words.docs[d] = wordList{ver: ver, words: words}
	return words
}

// pruneWords 丢掉已经关了的标签记着的词。
func (a *App) pruneWords(all []*docs.Doc) {
	for d := range a.words.docs {
		if !slices.Contains(all, d) {
			delete(a.words.docs, d)
		}
	}
	if o := a.words.offer; o != nil && !slices.Contains(all, o.d) {
		a.words.offer = nil
	}
}

// scanWords 对 text 里每个能补全的词（标识符，不以数字开头，wordMinRunes 到 wordMaxBytes）调 fn(词, 起点)；
// cutHead / cutTail 是 text 的头 / 尾是从中间切开的，挨着的那个词不完整、不要。
func scanWords(text string, cutHead, cutTail bool, fn func(w string, at int)) {
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		if !isIdentRune(r) {
			i += size
			continue
		}
		j, runes := i, 0
		for j < len(text) {
			r, sz := utf8.DecodeRuneInString(text[j:])
			if !isIdentRune(r) {
				break
			}
			j += sz
			runes++
		}
		switch {
		case i == 0 && cutHead, j == len(text) && cutTail:
		case runes < wordMinRunes || j-i > wordMaxBytes || unicode.IsDigit(r):
		default:
			fn(text[i:j], i)
		}
		i = j
	}
}

// subseq 是 q 的字按顺序出现在 w 里（不分大小写）。
func subseq(w, q string) bool {
	w, q = strings.ToLower(w), strings.ToLower(q)
	for _, c := range q {
		i := strings.IndexRune(w, c)
		if i < 0 {
			return false
		}
		w = w[i+utf8.RuneLen(c):]
	}
	return true
}

// isIdentText 是 s 全是标识符里的字（空的也算）。
func isIdentText(s string) bool {
	for _, r := range s {
		if !isIdentRune(r) {
			return false
		}
	}
	return true
}

// completionKind 是大纲里的种类在补全列表里的图标。
func completionKind(k outline.Kind) editor.CompletionKind {
	switch k {
	case outline.Func:
		return editor.CompletionFunction
	case outline.Method:
		return editor.CompletionMethod
	case outline.Type, outline.Class:
		return editor.CompletionType
	case outline.Const:
		return editor.CompletionConstant
	case outline.Var:
		return editor.CompletionVariable
	}
	return editor.CompletionText
}

// langFamily 是按扩展名分的一族语言：互相能用到名字的算一族（JS 和 TS、C 和 C++）；没扩展名的按文件名。
func langFamily(name string) string {
	ext := strings.ToLower(path.Ext(filepath.ToSlash(name)))
	switch ext {
	case ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts", ".vue", ".svelte":
		return "js"
	case ".c", ".h", ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx", ".m", ".mm":
		return "c"
	case ".py", ".pyw", ".pyi":
		return "py"
	case ".kt", ".kts":
		return "kt"
	case ".yaml", ".yml":
		return "yaml"
	case "":
		return strings.ToLower(path.Base(name))
	}
	return ext
}
