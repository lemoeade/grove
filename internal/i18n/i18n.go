// Package i18n 是界面文案表。代码里文案直接写中文，显示前过一下 T / Tf（带数的 Tn）：界面是中文时原样返回，
// 是英文时查 en.json。zh-CN.json 是代码里的全部中文文案（catalog_test.go 用 -update 从代码里收集，
// 并检查 en.json 每条都有、占位符对得上）；en.json 先是机翻。
package i18n

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

// Langs 是设置里能选的界面语言："en" 英文、"zh" 中文。设置里存 "" 是没选过，按 Default 定的语言。
var Langs = []string{"en", "zh"}

//go:embed en.json
var enJSON []byte

var (
	lang   atomic.Value // string："" 中文、"en" 英文
	def    atomic.Value // string：Set("") 用的语言
	enOnce sync.Once
	en     map[string]string
)

// Default 定 Set("")（设置里没选过语言）用哪种语言："en" 英文，别的是中文。
// 不调时是中文（代码里文案的原文，测试都按它写）；应用启动时 cmd/workbench 先调 Default("en")，
// 新装的 Grove 默认是英文（老版本的 state.json 迁移时补上 "zh"，见 workspace.LoadState）。
func Default(l string) {
	def.Store(l)
}

// Set 换界面语言："en" 英文，"zh" 中文，""（或不认识的）按 Default。
func Set(l string) {
	if l != "en" && l != "zh" {
		l, _ = def.Load().(string)
	}
	if l != "en" {
		l = ""
	}
	lang.Store(l)
}

// Lang 是现在的界面语言："" 中文、"en" 英文。
func Lang() string {
	l, _ := lang.Load().(string)
	return l
}

// Locale 是交给 mygo 的语言（它自己的文案：编辑器的查找条、系统菜单、对话框按钮）。
func Locale() string {
	if Lang() == "en" {
		return "en"
	}
	return "zh-Hans"
}

// T 是中文文案 s 在当前语言里的样子；表里没有就原样返回。
func T(s string) string {
	if Lang() != "en" {
		return s
	}
	if t, ok := English()[s]; ok && t != "" {
		return t
	}
	return s
}

// List 把几项连成一句里的列举：中文用 "、"，英文用 ", "。
func List(items []string) string {
	return strings.Join(items, ListSep())
}

// 拼接用的标点。TestCatalog 只收带汉字的串，"%s：%s"、"，" 这种没有汉字的串不进翻译表，
// 里面的全角标点在英文界面会原样露出来（TestNoBareFullwidth 查）。拼文案时能写成带汉字的格式串
// （"已暂存：%s"）就那样写；拼的只是名字、路径、错误这类不翻译的片段时，标点从下面拿：
// 中文给全角，英文给半角（该带的空格也带上）。弯引号 “” 中英文都用，不在这里。

// Colon 是 "标签：内容" 里的冒号：中文 "："，英文 ": "。如 r.Name + i18n.Colon() + err.Error()。
func Colon() string {
	if Lang() == "en" {
		return ": "
	}
	return "："
}

// Comma 是分句的逗号：中文 "，"，英文 ", "。
func Comma() string {
	if Lang() == "en" {
		return ", "
	}
	return "，"
}

// Question 是问句末尾的问号：中文 "？"，英文 "?"。如对话框标题 label + i18n.Question()。
func Question() string {
	if Lang() == "en" {
		return "?"
	}
	return "？"
}

// ListSep 是列举的分隔（List 用的）：中文 "、"，英文 ", "。
func ListSep() string {
	if Lang() == "en" {
		return ", "
	}
	return "、"
}

// Paren 是跟在文字后面的括注：中文 "（s）"，英文 " (s)"（前面带空格）。如 text + i18n.Paren(key)。
func Paren(s string) string {
	if Lang() == "en" {
		return " (" + s + ")"
	}
	return "（" + s + "）"
}

// Tf 是 fmt.Sprintf(T(format), args...)。
func Tf(format string, args ...any) string {
	return fmt.Sprintf(T(format), args...)
}

// Tn 是带数的 Tf：英文里数是 1 时换单数的写法（"1 file"），中文同 Tf。
// 格式串里只有一个 %d 时，它是 1 就先找 en.json 里的 format+"|1"；有两个以上 %d 时看头两个：
// "|1|" 是第一个是 1、"||1" 是第二个是 1、"|1|1" 是都是 1（都是 1 而没有 "|1|1" 的依次退到 "|1|"、"||1"）。
// 没有对得上的变体就用 format 本身的译文。
func Tn(format string, args ...any) string {
	if Lang() != "en" {
		return fmt.Sprintf(format, args...)
	}
	for _, k := range PluralKeys(format, args...) {
		if t, ok := English()[format+k]; ok && t != "" {
			return fmt.Sprintf(t, args...)
		}
	}
	return Tf(format, args...)
}

// PluralKeys 是 Tn 按 args 要依次找的变体后缀（见 Tn）；数都不是 1 时为空。
func PluralKeys(format string, args ...any) []string {
	var ns []int64
	arg := 0
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		j := i + 1
		for j < len(format) && strings.IndexByte("+-# 0123456789.", format[j]) >= 0 {
			j++
		}
		if j == len(format) {
			break
		}
		i = j
		if format[j] == '%' {
			continue
		}
		if format[j] == 'd' && arg < len(args) {
			ns = append(ns, intOf(args[arg]))
		}
		arg++
	}
	switch {
	case len(ns) == 1 && ns[0] == 1:
		return []string{"|1"}
	case len(ns) < 2:
		return nil
	case ns[0] == 1 && ns[1] == 1:
		return []string{"|1|1", "|1|", "||1"}
	case ns[0] == 1:
		return []string{"|1|"}
	case ns[1] == 1:
		return []string{"||1"}
	}
	return nil
}

// intOf 是 %d 的参数的值（不是整数的当 0）。
func intOf(v any) int64 {
	switch x := v.(type) {
	case int:
		return int64(x)
	case int64:
		return x
	case int32:
		return int64(x)
	case uint:
		return int64(x)
	case uint64:
		return int64(x)
	case uint32:
		return int64(x)
	}
	return 0
}

// English 是英文表（中文 → 英文）。
func English() map[string]string {
	enOnce.Do(func() {
		if err := json.Unmarshal(enJSON, &en); err != nil {
			panic("i18n: en.json: " + err.Error())
		}
	})
	return en
}
