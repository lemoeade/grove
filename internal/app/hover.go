package app

import (
	"bytes"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/lemoeade/grove/internal/docs"
)

// 这个文件是悬停提示（指针停在名字上，或命令面板的“显示悬停提示”）：和跳到定义一样按名字找声明（definition.go），
// 找到了就给声明那一行（参数折行的连着几行）、它前面的注释（Python 是后面的文档字符串），和在哪个仓库哪个文件哪一行。
// 停在声明本身上、哪里都找不到的不出提示（检查报的问题照样由编辑器自己出）。设置"悬停时显示定义"关掉。
// 编辑器只认当场给的答复，所以是同步的：开着的文件读缓冲区，别的读磁盘上那个文件（超过 hoverMaxFile 不读，只给位置）。

const (
	hoverMaxFile = 1 << 20 // 不开着的文件超过这么大不读，悬停只写位置
	hoverDecl    = 6       // 声明最多给几行（参数折行的）
	hoverDoc     = 16      // 注释最多给几行
)

// Hover 是悬停提示：Start–End 是悬停的那个名字。
type Hover struct {
	Start, End int
	Code       string // 声明那几行（去掉了行首共同的缩进和末尾的 {）
	Doc        string // 声明前面的注释，去掉了注释符；没有为 ""
	Loc        string // "仓库/路径:行"
	More       int    // 别处还有几处同名的
	site       defSite
}

// HoverAt 是 d 里 off 处那个名字的悬停提示，没有为 nil。
func (a *App) HoverAt(d *docs.Doc, off int) *Hover {
	if a.St.Settings.NoHoverDef || !CanGoToDefinition(d) {
		return nil
	}
	word, from, qual := identAt(d.Buf, off)
	if word == "" {
		return nil
	}
	for _, r := range a.Repos { // 索引按需建（都读过、建过就什么都不做）；建着时先用已经有的
		a.EnsureFiles(r)
	}
	a.refreshSymbols()
	here := d.Buf.Pos(from).Line
	sites := a.definitions(word)
	for _, s := range sites {
		if s.abs == d.Abs && s.line == here {
			return nil // 这就是声明
		}
	}
	sites = narrowDefs(sites, d, qual)
	if len(sites) == 0 {
		return nil
	}
	s := sites[0]
	h := &Hover{Start: from, End: from + len(word), More: len(sites) - 1, site: s,
		Loc: s.r.Name + "/" + s.rel + ":" + strconv.Itoa(s.line+1)}
	if lines, at := a.siteLines(s); lines != nil {
		h.Code, h.Doc = declText(lines, at, path.Ext(s.rel))
	}
	if h.Code == "" {
		h.Code = word
	}
	return h
}

// OpenHoverSite 打开悬停提示里写的那一处（点了提示里的位置）。
func (a *App) OpenHoverSite(h *Hover) {
	s := h.site
	if s.r == nil || a.RepoFor(s.r.Root) != s.r {
		return
	}
	a.OpenAt(s.r, s.rel, docs.Jump{Line: s.line}, true)
	a.FocusDoc = true
}

// siteLines 是那处声明所在文件的行和声明在其中第几行：开着的读缓冲区（没存的改动也算，只取声明上下一段），
// 别的读磁盘；读不了、太大为 nil。
func (a *App) siteLines(s defSite) ([]string, int) {
	for _, od := range a.AllDocs() {
		if od.Kind == docs.File && od.Abs == s.abs && od.Buf != nil {
			lo, hi := max(0, s.line-2*hoverDoc), min(od.Buf.LineCount(), s.line+hoverDecl+hoverDoc)
			if s.line >= hi {
				return nil, 0
			}
			lines := make([]string, 0, hi-lo)
			for i := lo; i < hi; i++ {
				lines = append(lines, od.Buf.Line(i))
			}
			return lines, s.line - lo
		}
	}
	fi, err := os.Stat(s.abs)
	if err != nil || fi.Size() > hoverMaxFile {
		return nil, 0
	}
	data, err := os.ReadFile(s.abs)
	if err != nil || bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
		return nil, 0
	}
	return strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n"), s.line
}

// declText 是 lines 里第 line 行开始的声明（参数没写完接着往下，最多 hoverDecl 行）和它的注释。
func declText(lines []string, line int, ext string) (code, doc string) {
	if line < 0 || line >= len(lines) {
		return "", ""
	}
	// 括号外的第一个 { 是函数体、类型体的开始，从那里截掉（写在一行里的 `func A() int { return 1 }` 也只剩签名）
	var decl []string
	depth := 0
scan:
	for i := line; i < len(lines) && len(decl) < hoverDecl; i++ {
		l := lines[i]
		for j, ch := range l {
			switch ch {
			case '(', '[':
				depth++
			case ')', ']':
				depth--
			case '{':
				if depth <= 0 {
					decl = append(decl, strings.TrimRight(l[:j], " \t"))
					break scan
				}
			}
		}
		decl = append(decl, strings.TrimRight(l, " \t"))
		if depth <= 0 {
			break
		}
	}
	code = strings.Join(dedent(decl), "\n")
	if ext == ".py" || ext == ".pyi" {
		if d := pyDocstring(lines, line+len(decl)); d != "" {
			return code, d
		}
	}
	return code, commentAbove(lines, line)
}

// commentAbove 是 lines 里第 line 行上面紧挨着的注释（中间不隔空行；注解、装饰器跳过），去掉注释符。
func commentAbove(lines []string, line int) string {
	var out []string
	inBlock := false
	for i := line - 1; i >= 0 && len(out) < hoverDoc; i-- {
		t := strings.TrimSpace(lines[i])
		switch {
		case t == "" && !inBlock:
			i = -1
			continue
		case !inBlock && (strings.HasPrefix(t, "@") || strings.HasPrefix(t, "#[")):
			continue // 注解、装饰器、Rust 的属性
		case strings.HasSuffix(t, "*/"):
			inBlock = !strings.HasPrefix(t, "/*") || t == "*/"
			t = strings.TrimSpace(strings.TrimSuffix(t, "*/"))
			if !inBlock {
				t = strings.TrimSpace(strings.TrimLeft(t, "/*!"))
			}
		case inBlock:
			if strings.HasPrefix(t, "/*") {
				inBlock = false
				t = strings.TrimSpace(strings.TrimLeft(t, "/*!"))
			} else {
				t = strings.TrimPrefix(strings.TrimPrefix(t, "*"), " ")
			}
		default:
			p := commentPrefix(t)
			if p == "" {
				i = -1
				continue
			}
			t = strings.TrimPrefix(strings.TrimPrefix(t, p), " ")
		}
		out = append(out, strings.TrimRight(t, " \t"))
	}
	for l, r := 0, len(out)-1; l < r; l, r = l+1, r-1 {
		out[l], out[r] = out[r], out[l]
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// commentPrefix 是 t 开头的行注释符，不是注释为 ""。
func commentPrefix(t string) string {
	for _, p := range []string{"///", "//!", "//", "#", "--", ";;", ";"} {
		if strings.HasPrefix(t, p) && !strings.HasPrefix(t, "#!") && !strings.HasPrefix(t, "#include") && !strings.HasPrefix(t, "#define") {
			return p
		}
	}
	return ""
}

// pyDocstring 是 Python 声明后面第 at 行开始的文档字符串（三引号），没有为 ""。
func pyDocstring(lines []string, at int) string {
	if at >= len(lines) {
		return ""
	}
	t := strings.TrimSpace(lines[at])
	q := ""
	for _, p := range []string{`"""`, `'''`} {
		if strings.HasPrefix(t, p) {
			q = p
		}
	}
	if q == "" {
		return ""
	}
	t = t[len(q):]
	if i := strings.Index(t, q); i >= 0 {
		return strings.TrimSpace(t[:i])
	}
	out := []string{t}
	for i := at + 1; i < len(lines) && len(out) < hoverDoc; i++ {
		l := strings.TrimSpace(lines[i])
		if j := strings.Index(l, q); j >= 0 {
			out = append(out, l[:j])
			break
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// dedent 去掉 lines 共同的行首缩进。
func dedent(lines []string) []string {
	cut := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if cut < 0 || n < cut {
			cut = n
		}
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		if len(l) >= cut && cut > 0 {
			l = l[cut:]
		}
		out[i] = l
	}
	return out
}
