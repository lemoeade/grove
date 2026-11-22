// Package mdprep 在 Markdown 交给 mygo 解析之前先整理一遍源码，处理 mygo 不认、README 里又常见的几种写法：
//
//   - 文件开头的 front matter（--- YAML --- 或 +++ TOML +++）：mygo 会把它读成一条分隔线加一个 setext 标题。
//   - GitHub 的 emoji 短代码（:tada:）：换成 emoji 字符（internal/emoji），代码里的不换。
//   - 跨了几个块的 HTML 容器：<details> … </details>、<div align="center"> … </div>、<p align>、<center>。
//     开标签和闭标签之间隔着空行写 Markdown 时，CommonMark 把开、闭标签各读成一个 HTML 块，中间的照常是 Markdown，
//     容器的效果（折叠、居中）就没了。
//
// 做法是把这样的一段换成一个围栏代码块，信息串写成 "grove-region N"，第 N 段的内容另外记在 Regions 里；
// 预览按信息串认出它，自己画（front matter 画成一张键值卡，容器里的 Markdown 用嵌套的视图画）。
// 只换掉开头、结尾那一行，行数不变：源码的第 k 行在整理后还是第 k 行，大纲跳转、同步滚动按行对得上。
package mdprep

import (
	"strconv"
	"strings"

	"github.com/lemoeade/grove/internal/emoji"
)

// Lang 是整理出来的围栏代码块的语言。
const Lang = "grove-region"

// Kind 是一段的种类。
type Kind uint8

const (
	FrontMatter Kind = iota // 文件开头的 front matter
	Container               // 跨块的 HTML 容器
)

// KV 是 front matter 里的一项。
type KV struct{ Key, Value string }

// Region 是换成围栏代码块的一段。
type Region struct {
	Kind Kind
	// Line 是这一段在源码里从第几行开始（从 0 数），Lines 是一共几行（含开头、结尾那两行）
	Line, Lines int
	// Code 是围栏代码块里的内容（mygo 的 CodeBlockView.Code()）：拿它核对，正文里恰好也写了 grove-region 的代码块不认
	Code string

	// front matter：Format 是 "yaml" 或 "toml"，Raw 是两条分隔线之间的原文，Meta 是读出来的顶层键值（读不出来时为空，照原文显示）
	Format string
	Raw    string
	Meta   []KV

	// 容器：Tag 是标签名（小写），Open 是开标签所在的那个 HTML 块（可能带着 <summary>），Body 是中间的 Markdown
	Tag  string
	Open string
	Body string
}

// Doc 是整理过的源码。
type Doc struct {
	Src     string // 交给 mygo 的源码
	Regions []Region
	// Defs 是源码里的链接引用定义（[名字]: 地址 …），一行一个：分块画（每块单独解析）时接在每块后面
	Defs string
	// Footnotes 是有脚注定义（[^x]: …）：分块画时脚注编号和跳转会断，有它就不分块
	Footnotes bool
}

// Region 返回围栏代码块 info / code 对应的那一段；不是整理出来的返回 nil。
func (d *Doc) Region(info, code string) *Region {
	rest, ok := strings.CutPrefix(info, Lang+" ")
	if !ok {
		return nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(rest))
	if err != nil || n < 0 || n >= len(d.Regions) || d.Regions[n].Code != code {
		return nil
	}
	return &d.Regions[n]
}

// containerTags 是认的跨块容器。
var containerTags = []string{"details", "div", "center", "p", "section", "article", "figure", "blockquote"}

// Prepare 整理 src。没有要整理的地方时 Src 就是 src 本身（同一个字符串）。
func Prepare(src string) *Doc {
	d := prepare(src)
	d.Src = emoji.Replace(d.Src) // 整理出来的围栏块（front matter、容器）里的不换：容器里的 Markdown 另外整理时再换
	return d
}

func prepare(src string) *Doc {
	d := &Doc{Src: src}
	if !strings.Contains(src, "---") && !strings.Contains(src, "+++") && !strings.Contains(src, "<") && !strings.Contains(src, "]:") {
		return d
	}
	lines := strings.SplitAfter(src, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	out := lines
	copied := false
	replace := func(i int, s string) {
		if !copied {
			out = append([]string(nil), lines...)
			copied = true
		}
		nl := ""
		if strings.HasSuffix(lines[i], "\n") {
			nl = "\n"
		}
		out[i] = s + nl
	}
	var defs strings.Builder

	start := 0
	if r, end, ok := frontMatter(lines); ok {
		r.Line, r.Lines = 0, end+1
		r.Code = strings.Join(lines[1:end], "")
		fence := fenceFor(r.Code)
		replace(0, fence+Lang+" "+strconv.Itoa(len(d.Regions)))
		replace(end, fence)
		d.Regions = append(d.Regions, r)
		start = end + 1
	}

	var fc byte // 在围栏代码块里：围栏的字符和长度
	var fn int
	comment := false
	for i := start; i < len(lines); i++ {
		ln := strings.TrimRight(lines[i], "\r\n")
		if fc != 0 {
			if c, n, info := fenceLine(ln); c == fc && n >= fn && strings.TrimSpace(info) == "" {
				fc = 0
			}
			continue
		}
		if comment {
			if strings.Contains(ln, "-->") {
				comment = false
			}
			continue
		}
		if c, n, info := fenceLine(ln); c != 0 && (c == '~' || !strings.Contains(info, "`")) {
			fc, fn = c, n
			continue
		}
		if strings.HasPrefix(ln, "<!--") {
			comment = !strings.Contains(ln[4:], "-->")
			continue
		}
		if isDef(ln) {
			defs.WriteString(strings.TrimSpace(ln))
			defs.WriteByte('\n')
		}
		if !d.Footnotes && strings.HasPrefix(strings.TrimLeft(ln, " "), "[^") && strings.Contains(ln, "]:") {
			d.Footnotes = true
		}
		tag := openTag(ln)
		if tag == "" {
			continue
		}
		r, last, ok := container(lines, i, tag)
		if !ok {
			continue
		}
		r.Line, r.Lines = i, last-i+1
		r.Code = strings.Join(lines[i+1:last], "")
		fence := fenceFor(r.Code)
		replace(i, fence+Lang+" "+strconv.Itoa(len(d.Regions)))
		replace(last, fence)
		d.Regions = append(d.Regions, r)
		i = last
	}
	if copied {
		d.Src = strings.Join(out, "")
	}
	d.Defs = defs.String()
	return d
}

// frontMatter 认第一行起的 front matter，返回它和结尾那一行。
func frontMatter(lines []string) (Region, int, bool) {
	first := strings.TrimRight(lines[0], " \t\r\n")
	var closers []string
	format := ""
	switch first {
	case "---":
		closers, format = []string{"---", "..."}, "yaml"
	case "+++":
		closers, format = []string{"+++"}, "toml"
	default:
		return Region{}, 0, false
	}
	for i := 1; i < len(lines) && i < 400; i++ {
		ln := strings.TrimRight(lines[i], " \t\r\n")
		for _, c := range closers {
			if ln == c {
				if i == 1 {
					return Region{}, 0, false // 空的：--- 紧跟 --- 不当 front matter
				}
				raw := strings.Join(lines[1:i], "")
				r := Region{Kind: FrontMatter, Format: format, Raw: raw}
				if format == "yaml" {
					r.Meta = yamlMeta(raw)
				} else {
					r.Meta = tomlMeta(raw)
				}
				if format == "yaml" && r.Meta == nil {
					return Region{}, 0, false // 读不出一个键：多半是分隔线加 setext 标题，不是 front matter
				}
				return r, i, true
			}
		}
	}
	return Region{}, 0, false
}

// yamlMeta 读 YAML 的顶层键值：key: value；值是缩进的列表（- a）时用逗号连起来；块标量（| 和 >）和嵌套的映射
// 照原样拼成一行。读不出一个键时返回 nil。
func yamlMeta(raw string) []KV {
	var kv []KV
	for ln := range strings.Lines(raw) {
		ln = strings.TrimRight(ln, " \t\r\n")
		if ln == "" || strings.HasPrefix(strings.TrimSpace(ln), "#") {
			continue
		}
		if ln[0] == ' ' || ln[0] == '\t' || ln[0] == '-' {
			if len(kv) == 0 {
				return nil
			}
			last := &kv[len(kv)-1]
			item := strings.TrimSpace(ln)
			if s, ok := strings.CutPrefix(item, "- "); ok {
				item = unquote(strings.TrimSpace(s))
			} else if item == "-" {
				continue
			}
			if last.Value == "" || last.Value == "|" || last.Value == ">" || last.Value == "|-" || last.Value == ">-" {
				last.Value = item
			} else {
				sep := ", "
				if !strings.HasPrefix(strings.TrimSpace(ln), "- ") {
					sep = " "
				}
				last.Value += sep + item
			}
			continue
		}
		k, v, ok := strings.Cut(ln, ":")
		if !ok || strings.ContainsAny(k, " \t") && !strings.HasPrefix(k, `"`) {
			return nil
		}
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
			parts := strings.Split(v[1:len(v)-1], ",")
			for j := range parts {
				parts[j] = unquote(strings.TrimSpace(parts[j]))
			}
			v = strings.Join(parts, ", ")
		} else {
			v = unquote(v)
		}
		kv = append(kv, KV{unquote(strings.TrimSpace(k)), v})
	}
	return kv
}

// tomlMeta 读 TOML 的键值：key = value；[表] 下的键写成 表.key。
func tomlMeta(raw string) []KV {
	var kv []KV
	table := ""
	for ln := range strings.Lines(raw) {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		if strings.HasPrefix(ln, "[") && strings.HasSuffix(ln, "]") {
			table = strings.Trim(ln, "[] ")
			continue
		}
		k, v, ok := strings.Cut(ln, "=")
		if !ok {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if table != "" {
			k = table + "." + k
		}
		if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
			parts := strings.Split(v[1:len(v)-1], ",")
			for j := range parts {
				parts[j] = unquote(strings.TrimSpace(parts[j]))
			}
			v = strings.Join(parts, ", ")
		} else {
			v = unquote(v)
		}
		kv = append(kv, KV{unquote(k), v})
	}
	return kv
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' && s[len(s)-1] == '"' || s[0] == '\'' && s[len(s)-1] == '\'') {
		return s[1 : len(s)-1]
	}
	return s
}

// fenceLine 认围栏代码块的开头、结尾：最多 3 个空格的缩进，再 3 个以上 ` 或 ~。返回字符、个数和后面的信息串。
func fenceLine(ln string) (byte, int, string) {
	s := ln
	for k := 0; k < 3 && strings.HasPrefix(s, " "); k++ {
		s = s[1:]
	}
	if len(s) < 3 || s[0] != '`' && s[0] != '~' {
		return 0, 0, ""
	}
	c := s[0]
	n := 0
	for n < len(s) && s[n] == c {
		n++
	}
	if n < 3 {
		return 0, 0, ""
	}
	return c, n, s[n:]
}

// fenceFor 是包得住 code 的围栏：比里面最长的一串 ` 还长一个，至少 3 个。
func fenceFor(code string) string {
	longest, run := 0, 0
	for i := 0; i < len(code); i++ {
		if code[i] == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", max(3, longest+1))
}

// isDef 是一行链接引用定义：[名字]: 地址（不是脚注 [^x]:）。
func isDef(ln string) bool {
	s := strings.TrimLeft(ln, " ")
	if len(ln)-len(s) > 3 || !strings.HasPrefix(s, "[") || strings.HasPrefix(s, "[^") {
		return false
	}
	j := strings.Index(s, "]:")
	return j > 1 && !strings.Contains(s[1:j], "]")
}

// openTag 认第 0 列起的容器开标签，返回标签名（小写）。
func openTag(ln string) string {
	if len(ln) < 3 || ln[0] != '<' {
		return ""
	}
	low := strings.ToLower(ln[1:min(len(ln), 16)])
	for _, t := range containerTags {
		if strings.HasPrefix(low, t) {
			if len(low) == len(t) || low[len(t)] == '>' || low[len(t)] == ' ' || low[len(t)] == '\t' {
				return t
			}
		}
	}
	return ""
}

// container 认第 i 行开头的跨块容器：开标签所在的 HTML 块（到空行为止）里开、闭标签没配平，后面有一行只写着
// 配平它的闭标签。返回这一段和闭标签那一行。中间的围栏代码块里的标签不算。
func container(lines []string, i int, tag string) (Region, int, bool) {
	open, close := "<"+tag, "</"+tag+">"
	depth := 0
	count := func(ln string) {
		low := strings.ToLower(ln)
		for k := 0; k < len(low); {
			j := strings.IndexByte(low[k:], '<')
			if j < 0 {
				break
			}
			k += j
			switch {
			case strings.HasPrefix(low[k:], close):
				depth--
				k += len(close)
			case strings.HasPrefix(low[k:], open) && k+len(open) < len(low) && strings.IndexByte(" \t>", low[k+len(open)]) >= 0:
				depth++
				k += len(open)
			default:
				k++
			}
		}
	}
	// 开标签所在的 HTML 块
	end := i
	for end < len(lines) && strings.TrimSpace(lines[end]) != "" {
		count(lines[end])
		end++
	}
	if depth <= 0 {
		return Region{}, 0, false
	}
	var fc byte
	var fn int
	for k := end; k < len(lines); k++ {
		ln := strings.TrimRight(lines[k], "\r\n")
		if fc != 0 {
			if c, n, info := fenceLine(ln); c == fc && n >= fn && strings.TrimSpace(info) == "" {
				fc = 0
			}
			continue
		}
		if c, n, _ := fenceLine(ln); c != 0 {
			fc, fn = c, n
			continue
		}
		count(ln)
		if depth == 0 {
			if !strings.EqualFold(strings.TrimSpace(ln), close) {
				return Region{}, 0, false
			}
			return Region{
				Kind: Container,
				Tag:  tag,
				Open: strings.Join(lines[i:end], ""),
				Body: strings.Join(lines[end:k], ""),
			}, k, true
		}
		if depth < 0 {
			return Region{}, 0, false
		}
	}
	return Region{}, 0, false
}
