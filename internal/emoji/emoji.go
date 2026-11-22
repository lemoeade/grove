// Package emoji 把 Markdown 里 GitHub 的 emoji 短代码（:tada:、:+1:）换成 emoji 字符，照 GitHub 和 VS Code 的预览。
//
// 短代码表 gemoji.txt 是 GitHub 的 gemoji（github/gemoji，MIT，许可见 resources/licenses/gemoji.txt）的 db.json 里
// 每个别名一行（别名 TAB emoji）。围栏代码块、行内代码里的不换；冒号两边挨着字母、数字的（10:100:20、a:b:c）不当短代码。
// 只换一行里面的字，不增删换行：换完源码的第 k 行还是第 k 行（同步滚动、跳转按行对）。
package emoji

import (
	_ "embed"
	"strings"
	"sync"
)

//go:embed gemoji.txt
var table string

var (
	once  sync.Once
	names map[string]string
)

func load() {
	names = make(map[string]string, 1800)
	for ln := range strings.Lines(table) {
		k, v, ok := strings.Cut(strings.TrimRight(ln, "\n"), "\t")
		if ok {
			names[k] = v
		}
	}
}

// Lookup 返回短代码 name（不带冒号）对应的 emoji。
func Lookup(name string) (string, bool) {
	once.Do(load)
	e, ok := names[name]
	return e, ok
}

// Replace 换掉 src 里的短代码。没有可换的时候返回 src 本身（同一个字符串）。
func Replace(src string) string {
	if strings.IndexByte(src, ':') < 0 {
		return src
	}
	var b strings.Builder
	last := 0 // src[last:] 还没写进 b
	var fc byte
	var fn int
	for off := 0; off < len(src); {
		end := strings.IndexByte(src[off:], '\n')
		if end < 0 {
			end = len(src)
		} else {
			end += off + 1
		}
		ln := src[off:end]
		if c, n, rest := fence(ln); fc != 0 {
			if c == fc && n >= fn && strings.TrimSpace(rest) == "" {
				fc = 0
			}
		} else if c != 0 && (c == '~' || !strings.Contains(rest, "`")) {
			fc, fn = c, n
		} else if strings.IndexByte(ln, ':') >= 0 {
			last = line(&b, src, off, end, last)
		}
		off = end
	}
	if last == 0 {
		return src
	}
	b.WriteString(src[last:])
	return b.String()
}

// line 换 src[off:end] 这一行里的短代码，跳过行内代码；写进 b 的部分到哪儿了由 last 记着。
func line(b *strings.Builder, src string, off, end, last int) int {
	for i := off; i < end; i++ {
		switch src[i] {
		case '`': // 行内代码：找一样长的一串反引号收尾，找不到就当普通字
			n := 1
			for i+n < end && src[i+n] == '`' {
				n++
			}
			ticks := src[i : i+n]
			j := i + n
			for {
				k := strings.Index(src[j:end], ticks)
				if k < 0 {
					j = -1
					break
				}
				j += k
				m := len(ticks)
				for j+m < end && src[j+m] == '`' {
					m++
				}
				if m == len(ticks) {
					break
				}
				j += m
			}
			if j < 0 {
				i += n - 1
			} else {
				i = j + n - 1
			}
		case '\\': // \: 是转义的冒号
			i++
		case ':':
			if i > off && word(src[i-1]) {
				continue
			}
			j := i + 1
			for j < end && name(src[j]) {
				j++
			}
			if j == i+1 || j >= end || src[j] != ':' || (j+1 < end && word(src[j+1])) {
				continue
			}
			e, ok := Lookup(src[i+1 : j])
			if !ok {
				continue
			}
			if b.Len() == 0 {
				b.Grow(len(src))
			}
			b.WriteString(src[last:i])
			b.WriteString(e)
			last = j + 1
			i = j
		}
	}
	return last
}

func name(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '+' || c == '-'
}

func word(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// fence 认围栏代码块的开头、结尾一行（最多缩进 3 格，三个以上 ` 或 ~），返回字符、个数和后面的部分。
func fence(ln string) (byte, int, string) {
	s := strings.TrimRight(ln, "\r\n")
	i := 0
	for i < len(s) && i < 4 && s[i] == ' ' {
		i++
	}
	if i > 3 || i >= len(s) || (s[i] != '`' && s[i] != '~') {
		return 0, 0, ""
	}
	c, n := s[i], 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	if n < 3 {
		return 0, 0, ""
	}
	return c, n, s[i+n:]
}
