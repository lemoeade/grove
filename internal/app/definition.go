package app

import (
	"cmp"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui/editor"

	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/outline"
)

// 这个文件是跳到定义（⌘点击、F12、编辑器右键"跳到定义"，命令 editor.GoToDefinition）。没有语言服务器：
// 按光标下的名字在大纲里找同名的声明——这个文件、开着的文件（没存的改动也算）和工作区符号索引（symindex.go）里的，
// 所以所有仓库都找。找到好几个时先缩：`pkg.Name` 的限定词 pkg 和所在目录名一样的、这个文件里的、同一目录的；
// 剩一个就打开到那一行（固定标签，‹ 回到点的地方），还剩好几个就开面板只列同名的（"#名字"），一个没有提示一下。
// 文件列表还没读、索引还没建好时先记下（defWait），都好了再接着找。只读；Markdown 的标题、Go 的 const ( … ) 这类分组不算定义。

// defSite 是一处同名的声明。
type defSite struct {
	r      *Repo
	rel    string
	abs    string
	detail string
	kind   outline.Kind
	line   int
}

// defWait 是索引建着时等着的那次跳转：建完时标签、内容、光标都没变才接着跳。
type defWait struct {
	d          *docs.Doc
	word, qual string
	ver        uint64
	off        int
}

// identAt 是 off 处（在词里、或紧挨着词的末尾）的标识符、它的起点，和前面紧挨着 "." 的那个限定词（`pkg.Name` 的 pkg，没有为 ""）。
// 只看 off 所在的那一行、前后各 256 字节；数字开头的不算。
func identAt(b *editor.Buffer, off int) (word string, from int, qual string) {
	snap := b.Snapshot()
	line := snap.LineOf(off)
	ls, le := snap.LineStart(line), snap.LineEnd(line)
	lo, hi := max(ls, off-256), min(le, off+256)
	s := snap.Slice(lo, hi)
	k := off - lo
	if k < 0 || k > len(s) {
		return "", 0, ""
	}
	i, j := identBounds(s, k)
	if i == j || (i == 0 && lo > ls) || (j == len(s) && hi < le) {
		return "", 0, ""
	}
	if r, _ := utf8.DecodeRuneInString(s[i:]); unicode.IsDigit(r) {
		return "", 0, ""
	}
	if i > 0 && s[i-1] == '.' {
		if qi, qj := identBounds(s, i-1); qi < qj {
			qual = s[qi:qj]
		}
	}
	return s[i:j], lo + i, qual
}

// identBounds 是 s 里包着下标 k（或在 k 前面结束）的那个标识符的 [i, j)。
func identBounds(s string, k int) (i, j int) {
	i, j = k, k
	for i > 0 {
		r, size := utf8.DecodeLastRuneInString(s[:i])
		if !isIdentRune(r) {
			break
		}
		i -= size
	}
	for j < len(s) {
		r, size := utf8.DecodeRuneInString(s[j:])
		if !isIdentRune(r) {
			break
		}
		j += size
	}
	return i, j
}

// isIdentRune 是标识符里的字：字母、数字、下划线，JS 的 $。
func isIdentRune(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isDefKind 是能当定义的符号：标题、分组不算。
func isDefKind(k outline.Kind) bool { return k != outline.Heading && k != outline.Section }

// CanGoToDefinition 是 d 能跳到定义：磁盘上的文件（只读的也行），不是十六进制、大文件。
func CanGoToDefinition(d *docs.Doc) bool {
	return d != nil && d.Kind == docs.File && d.Buf != nil && d.Abs != "" && !d.Hex && !d.Big
}

// GoToDefinition 跳到 d 里 off 处那个名字的定义（编辑器的 OnDefinition）。
func (a *App) GoToDefinition(d *docs.Doc, off int) {
	a.defWait = nil
	if !CanGoToDefinition(d) {
		return
	}
	word, _, qual := identAt(d.Buf, off)
	if word == "" {
		return
	}
	for _, r := range a.Repos {
		a.EnsureFiles(r)
	}
	a.refreshSymbols()
	if a.defPending() {
		a.defWait = &defWait{d: d, word: word, qual: qual, ver: d.Buf.Version(), off: off}
		a.Toast = i18n.Tf("正在建符号索引，建好了跳到 %s 的定义…", word)
		return
	}
	a.resolveDefinition(d, word, qual, off)
}

// defPending 是索引还不全：还在建，或者有仓库的文件列表还在读（读完了才能往索引里加）。
func (a *App) defPending() bool {
	if a.syms.building {
		return true
	}
	for _, r := range a.Repos {
		if !r.Files.Loaded && r.Files.Loading {
			return true
		}
	}
	return false
}

// defFilesLoaded 是一个仓库的文件列表读到了（LoadFiles）：有等着的跳转就把它加进索引。
func (a *App) defFilesLoaded() {
	if a.defWait != nil {
		a.refreshSymbols()
		a.defIndexed()
	}
}

// defIndexed 是索引建完一批、或文件列表读到时调：有等着的跳转、索引全了、而且标签和光标都没动，就接着找。
func (a *App) defIndexed() {
	w := a.defWait
	if w == nil || a.defPending() {
		return
	}
	a.defWait = nil
	d := a.CurrentDoc()
	if d != w.d || d.Buf == nil || d.Buf.Version() != w.ver || d.St.Cursor() != w.off {
		return
	}
	a.resolveDefinition(d, w.word, w.qual, w.off)
}

// resolveDefinition 找 word 的定义并跳过去（或开面板、提示）。
func (a *App) resolveDefinition(d *docs.Doc, word, qual string, off int) {
	sites := a.definitions(word)
	here := d.Buf.Pos(off).Line
	self := false
	sites = slices.DeleteFunc(sites, func(s defSite) bool {
		if s.abs == d.Abs && s.line == here {
			self = true
			return true
		}
		return false
	})
	if len(sites) == 0 {
		if self {
			a.Toast = i18n.Tf("这里就是 %s 的定义", word)
		} else {
			a.Toast = i18n.Tf("没找到 %s 的定义", word)
		}
		return
	}
	sites = narrowDefs(sites, d, qual)
	if len(sites) == 1 {
		s := sites[0]
		if a.RepoFor(s.r.Root) != s.r {
			return
		}
		a.OpenAt(s.r, s.rel, docs.Jump{Line: s.line}, true)
		a.FocusDoc = true
		return
	}
	a.OpenPalette("#" + word)
	a.Palette.defs = word
	a.Palette.defFrom = d
	a.Palette.defQual = qual
}

// definitions 是所有仓库里名字正好是 word 的声明：开着的文件按标签的大纲，别的按索引。
func (a *App) definitions(word string) []defSite {
	var out []defSite
	open := map[string]bool{}
	for _, od := range a.AllDocs() {
		if od.Kind != docs.File || od.Buf == nil || od.Abs == "" || open[od.Abs] {
			continue
		}
		r, rel := a.repoOfPath(od.Abs)
		if r == nil {
			continue
		}
		open[od.Abs] = true
		t := od.Outline()
		if t == nil {
			continue
		}
		for _, f := range t.Flat {
			if f.Name == word && isDefKind(f.Kind) {
				out = append(out, defSite{r: r, rel: rel, abs: od.Abs, detail: f.Detail, kind: f.Kind, line: f.Line})
			}
		}
	}
	live := map[*Repo]bool{}
	for _, r := range a.Repos {
		live[r] = true
	}
	for abs, f := range a.syms.files {
		if open[abs] || !live[f.r] {
			continue
		}
		for _, s := range f.syms {
			if s.Name == word && isDefKind(s.Kind) {
				out = append(out, defSite{r: f.r, rel: f.rel, abs: abs, detail: s.Detail, kind: s.Kind, line: int(s.Line)})
			}
		}
	}
	slices.SortFunc(out, func(x, y defSite) int {
		return cmp.Or(strings.Compare(x.r.Name, y.r.Name), strings.Compare(x.rel, y.rel), x.line-y.line)
	})
	return out
}

// narrowDefs 在好几处同名的里先挑可能是的：限定词和目录名一样的（Go 的包、Python 的模块目录），
// 没限定词时这个文件里的、再是同一目录的；挑不出来原样返回（按离 d 近排好，面板照这个顺序列）。
func narrowDefs(sites []defSite, d *docs.Doc, qual string) []defSite {
	if len(sites) < 2 {
		return sites
	}
	dir := path.Dir(d.Rel)
	pick := func(keep func(s defSite) bool) []defSite {
		var out []defSite
		for _, s := range sites {
			if keep(s) {
				out = append(out, s)
			}
		}
		return out
	}
	if qual != "" {
		if got := pick(func(s defSite) bool { return path.Base(path.Dir(s.rel)) == qual }); len(got) > 0 {
			return got
		}
		// 限定词是变量（db.Users()）：还是本仓库的靠前，别的照常排
	} else {
		if got := pick(func(s defSite) bool { return s.abs == d.Abs }); len(got) > 0 {
			return got
		}
		if got := pick(func(s defSite) bool { return s.r.Root == d.Root && path.Dir(s.rel) == dir }); len(got) > 0 {
			return got
		}
	}
	out := slices.Clone(sites)
	slices.SortStableFunc(out, func(x, y defSite) int { return defDistance(x, d) - defDistance(y, d) })
	return out
}

// defDistance 是一处声明离 d 有多远：同一文件、同一目录、同一仓库、别的仓库。
func defDistance(s defSite, d *docs.Doc) int {
	switch {
	case s.abs == d.Abs:
		return 0
	case s.r.Root == d.Root && path.Dir(s.rel) == path.Dir(d.Rel):
		return 1
	case s.r.Root == d.Root:
		return 2
	}
	return 3
}

// definitionItems 是面板里 "#名字" 在跳到定义之后（Palette.defs）的列表：只列名字正好是它的，离来的文件近的在前。
func (a *App) definitionItems(word string) ([]PaletteItem, string) {
	p := &a.Palette
	d := p.defFrom
	sites := a.definitions(word)
	if d != nil && d.Buf != nil {
		here := d.Buf.Pos(d.St.Cursor()).Line
		sites = slices.DeleteFunc(sites, func(s defSite) bool { return s.abs == d.Abs && s.line == here })
		sites = narrowDefs(sites, d, p.defQual)
	}
	items := make([]PaletteItem, 0, len(sites))
	for _, s := range sites {
		hits := make([]int, 0, len(word))
		for i := range word {
			hits = append(hits, i)
		}
		items = append(items, PaletteItem{Kind: ItemSymbol, Text: word, Hits: hits, Detail: s.detail, Sym: s.kind,
			Loc: s.r.Name + "/" + s.rel + ":" + strconv.Itoa(s.line+1), repo: s.r, path: s.rel, jump: docs.Jump{Line: s.line}, at: true})
	}
	note := i18n.Tn("%s 有 %d 处定义；改了输入就是按名字模糊找", word, len(items))
	if a.syms.building {
		note = i18n.Tf("正在建索引 %d / %d…", a.syms.done, a.syms.total)
	}
	return items, note
}
