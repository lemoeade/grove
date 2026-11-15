package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// 系统剪贴板和系统对话框只经几个能在测试里换掉的口子：写文字 CopyText、读文字 ReadClipboard、放文件 CopyFiles、
// 打开 / 存储面板 a.OpenDialog / a.SaveDialog（defaults 里填默认值）。报错一律走 a.Fail（应用里的提示），
// 不直接弹 mygo.Dialog.Error。internal/app 和 internal/ui 里别处直接用 mygo.Clipboard / mygo.Dialog 就报。
func TestSystemCallsStubbed(t *testing.T) {
	allowed := []string{"CopyFiles", "CopyText", "ReadClipboard", "defaults"}
	var bad []string
	fs := token.NewFileSet()
	for _, dir := range []string{".", "../ui"} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		if len(files) == 0 {
			t.Fatalf("%s 下没有源码", dir)
		}
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") {
				continue
			}
			af, err := parser.ParseFile(fs, f, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range af.Decls {
				owner := ""
				switch d := d.(type) {
				case *ast.FuncDecl:
					owner = d.Name.Name
				case *ast.GenDecl:
					for _, s := range d.Specs {
						if vs, ok := s.(*ast.ValueSpec); ok && len(vs.Names) > 0 {
							owner = vs.Names[0].Name
						}
					}
				}
				ast.Inspect(d, func(x ast.Node) bool {
					sel, ok := x.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					pkg, ok := sel.X.(*ast.Ident)
					if ok && pkg.Name == "mygo" && (sel.Sel.Name == "Clipboard" || sel.Sel.Name == "Dialog") && !slices.Contains(allowed, owner) {
						bad = append(bad, fs.Position(sel.Pos()).String()+" "+owner)
					}
					return true
				})
			}
		}
	}
	if len(bad) > 0 {
		t.Errorf("直接用了系统剪贴板 / 对话框（换成 CopyText / ReadClipboard / CopyFiles / a.OpenDialog / a.SaveDialog / a.Fail）：\n  %s", strings.Join(bad, "\n  "))
	}
}
