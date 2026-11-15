package app

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/lemoeade/grove/internal/docs"
	"github.com/lemoeade/grove/internal/i18n"
	"github.com/lemoeade/grove/internal/search"
	"github.com/lemoeade/grove/internal/workspace"
)

// 这个文件是查找引用（⇧F12、编辑器右键）：没有语言服务，就是拿光标处的名字在侧栏搜索里搜整个工作区——
// 整词、区分大小写、只搜同一种语言的文件（langFamily 那几种扩展名；没有扩展名的文件不限）。
// 结果就是普通的搜索结果：所有仓库都在里面，一个仓库一段，点哪一处开哪一处；框里的条件照样能改了再搜。

// familyExts 是一种语言（langFamily）的扩展名，只有一个扩展名的不在这里。
var familyExts = map[string][]string{
	"js":   {".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts", ".vue", ".svelte"},
	"c":    {".c", ".h", ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx", ".m", ".mm"},
	"py":   {".py", ".pyw", ".pyi"},
	"kt":   {".kt", ".kts"},
	"yaml": {".yaml", ".yml"},
}

// referenceGlobs 是查找引用时搜索的"包含"：和 name 同一种语言的扩展名，逗号隔开；没有扩展名为 ""（不限）。
func referenceGlobs(name string) string {
	if path.Ext(filepath.ToSlash(name)) == "" {
		return ""
	}
	fam := langFamily(name)
	exts, ok := familyExts[fam]
	if !ok {
		exts = []string{fam}
	}
	globs := make([]string, len(exts))
	for i, e := range exts {
		globs[i] = "*" + e
	}
	return strings.Join(globs, ",")
}

// FindReferences 在整个工作区里搜 d 里 off 处的名字（整词、区分大小写、同一种语言的文件），结果在侧栏的搜索里。
func (a *App) FindReferences(d *docs.Doc, off int) {
	if !CanGoToDefinition(d) {
		return
	}
	word, _, _ := identAt(d.Buf, off)
	if word == "" {
		a.Toast = i18n.T("光标不在名字上")
		return
	}
	a.Search.Scope = nil
	a.useQuery(search.Query{Text: word, CaseSensitive: true, WholeWord: true, Include: referenceGlobs(d.Name())})
	a.ShowSidebar(workspace.ModeSearch)
	a.SearchSubmitted()
}
