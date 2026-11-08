package highlight

import (
	"path"
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

// Grove 认的语言是 mygo 的那些再加上自己的（现在只有 Vue）。要按文件名、名字找语言或者列出所有语言的地方都走这里，
// 不直接调 editor.LanguageFor / LanguageNamed / Languages，不然 .vue 又成了 HTML。

// LanguageFor 是 filename 的语言：.vue 是 Vue，别的照 mygo 的 editor.LanguageFor；不认识的为 nil。
func LanguageFor(filename string) *editor.Language {
	if strings.EqualFold(path.Ext(filename), ".vue") {
		return Vue()
	}
	return editor.LanguageFor(filename)
}

// LanguageNamed 是叫 name 的语言（不分大小写，扩展名和别名也行，和 editor.LanguageNamed 一样）；不认识的为 nil。
func LanguageNamed(name string) *editor.Language {
	if strings.EqualFold(strings.TrimSpace(name), "vue") {
		return Vue()
	}
	return editor.LanguageNamed(name)
}

// Languages 是所有认识的语言：mygo 的，后面是 Vue。
func Languages() []*editor.Language {
	return append(append([]*editor.Language(nil), editor.Languages()...), Vue())
}
