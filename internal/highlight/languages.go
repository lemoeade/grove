package highlight

import (
	"path"
	"strings"

	"github.com/egoist/mygo/ui/editor"
)

// Grove 认的语言是 mygo 的那些再加上自己的（Vue），Python 换成 Grove 改过分词的那份。要按文件名、名字找语言或者
// 列出所有语言的地方都走这里，不直接调 editor.LanguageFor / LanguageNamed / Languages，不然 .vue 又成了 HTML、
// Python 又满屏红色的冒号。

// LanguageFor 是 filename 的语言：.vue 是 Vue，别的照 mygo 的 editor.LanguageFor；不认识的为 nil。
func LanguageFor(filename string) *editor.Language {
	if strings.EqualFold(path.Ext(filename), ".vue") {
		return Vue()
	}
	return own(editor.LanguageFor(filename))
}

// own 是 mygo 的语言 l 在 Grove 里用的那份：Python 换成 Python()，别的原样。
func own(l *editor.Language) *editor.Language {
	if l != nil && l.Name == "Python" {
		if p := Python(); p != nil {
			return p
		}
	}
	return l
}

// LanguageNamed 是叫 name 的语言（不分大小写，扩展名和别名也行，和 editor.LanguageNamed 一样）；不认识的为 nil。
func LanguageNamed(name string) *editor.Language {
	if strings.EqualFold(strings.TrimSpace(name), "vue") {
		return Vue()
	}
	return own(editor.LanguageNamed(name))
}

// Languages 是所有认识的语言：mygo 的（Python 换成 Grove 的），后面是 Vue。
func Languages() []*editor.Language {
	var out []*editor.Language
	for _, l := range editor.Languages() {
		out = append(out, own(l))
	}
	return append(out, Vue())
}
