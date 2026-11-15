package ui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// scanUI 解析 internal/ui 的非测试源码，每个调用表达式交给 fn。
func scanUI(t *testing.T, fn func(fset *token.FileSet, f *ast.File, call *ast.CallExpr)) {
	t.Helper()
	paths, _ := filepath.Glob("*.go")
	fset := token.NewFileSet()
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				fn(fset, f, call)
			}
			return true
		})
	}
}

// sizedButtons 是把按钮热区当参数往下传的函数：调用时在第几个参数（从 0 数）、一共几个参数。
var sizedButtons = map[string]struct {
	arg, nargs int
}{
	"squareButton":   {4, 8}, // squareButton(c, ic, label, tip, size, iconSize, disabled, fn)
	"iconActionSize": {1, 7}, // iconActionSize(c, size, ic, label, tip, why, fg)
}

// buttonTiers 是 widgets.go 里按钮热区的几档。
var buttonTiers = map[string]bool{
	"toolSize": true, "headToolSize": true, "midToolSize": true, "smallToolSize": true,
	"rowToolSize": true, "closeSize": true, "tinyToolSize": true,
}

// TestButtonSizes 扫源码：squareButton / iconActionSize 的热区、ButtonBase / ToggleBase 链上的 Size 只能是 widgets.go 的几档常量，
// 不写数字（commitBtnH 这种按旁边的东西算出来的不管，只要不是数字字面量）。
func TestButtonSizes(t *testing.T) {
	if toolSize != 28 || rowToolSize != 20 {
		t.Errorf("toolSize / rowToolSize 是 28 / 20，现在 %v / %v", toolSize, rowToolSize)
	}
	n := 0
	scanUI(t, func(fset *token.FileSet, f *ast.File, call *ast.CallExpr) {
		var args []ast.Expr
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			s, ok := sizedButtons[fn.Name]
			if !ok || len(call.Args) != s.nargs {
				return
			}
			if _, lit := call.Args[s.arg].(*ast.BasicLit); !lit {
				for name := range sizedButtons {
					if inFunc(f, call, name) {
						return // 那个函数自己的参数
					}
				}
			}
			args = []ast.Expr{call.Args[s.arg]}
		case *ast.SelectorExpr:
			if fn.Sel.Name != "Size" || len(call.Args) != 2 || !onBase(fn.X) {
				return
			}
			args = call.Args
		default:
			return
		}
		n++
		for _, arg := range args {
			if _, lit := arg.(*ast.BasicLit); lit {
				t.Errorf("%s：按钮热区写了数字 %s，用 widgets.go 的常量（toolSize / rowToolSize / …）",
					fset.Position(arg.Pos()), exprString(arg))
				continue
			}
			if _, isCall := call.Fun.(*ast.Ident); isCall {
				if id, ok := arg.(*ast.Ident); !ok || !buttonTiers[id.Name] {
					t.Errorf("%s：按钮热区 %s 不是 widgets.go 的几档之一", fset.Position(arg.Pos()), exprString(arg))
				}
			}
		}
	})
	if n < 20 {
		t.Errorf("只扫到 %d 处按钮热区，扫漏了？", n)
	}
}

// onBase 报 e 这条链是不是从 wui.ButtonBase(...) / wui.ToggleBase(...) 开始的。
func onBase(e ast.Expr) bool {
	for {
		switch x := e.(type) {
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "wui" {
					return sel.Sel.Name == "ButtonBase" || sel.Sel.Name == "ToggleBase"
				}
				e = sel.X
				continue
			}
			return false
		case *ast.SelectorExpr:
			e = x.X
		default:
			return false
		}
	}
}

// namedLits 是已经有常量的字号、圆角：写成字面量就报，该用哪个常量看用途。
// 只管收拢过的这几个值，别的字号、圆角照旧可以写数字。
var namedLits = map[string]map[string]string{
	"FontSize": {
		"12":   "sideSub / barFont / noteFont",
		"12.5": "bodyFont / sideMono",
		"14.5": "repoFont",
	},
	"Radius": {
		"5": "rowRadius / ctlRadius",
	},
}

// TestVisualConsts 扫源码：FontSize(12) / FontSize(12.5) / FontSize(14.5) / Radius(5) 不写数字，用 widgets.go、sidebar.go 里的常量。
func TestVisualConsts(t *testing.T) {
	if sideSub != 12 || barFont != 12 || noteFont != 12 || bodyFont != 12.5 || sideMono != 12.5 || repoFont != 14.5 ||
		rowRadius != 5 || ctlRadius != 5 {
		t.Error("字号、圆角常量的值变了：改外观的话把这个测试和 namedLits 一起改")
	}
	scanUI(t, func(fset *token.FileSet, _ *ast.File, call *ast.CallExpr) {
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || len(call.Args) != 1 {
			return
		}
		vals, ok := namedLits[sel.Sel.Name]
		if !ok {
			return
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			return
		}
		if use, ok := vals[lit.Value]; ok {
			t.Errorf("%s：%s(%s) 写了数字，用 %s", fset.Position(lit.Pos()), sel.Sel.Name, lit.Value, use)
		}
	})
}
