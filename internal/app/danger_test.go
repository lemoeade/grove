package app

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// 红的确认框（AskDanger）只给丢数据、改远程、改历史的那几样：
// 删分支 / 删远程 / 删远程分支 / 丢弃改动 / 重置 --hard / 强制推送 / 移除工作树 / 放弃变基·合并·拣选 / 删除贮藏；
// 还有：保存时覆盖别人在磁盘上的改动、换编码重新打开丢掉没保存的、撤销 / 修改 / 变基改写推送过的提交（StartRebase、RebaseOnto
// 有推送过的才红）、跳过停住的提交（它的改动不进结果）；
// 恢复默认设置、设置文档改过了还要换回现在的设置。
// 清除一个文件的 / 全部的本地历史、恢复删掉的文件时那个位置又有了文件（覆盖）。
// 其余的确认都是普通的 Ask。
// 按源码数：哪个函数里调了 AskDanger（重置是 ask = a.AskDanger 那一下），多了少了都报。
func TestConfirmDanger(t *testing.T) {
	want := []string{
		"AbortOngoing", "ApplyStashFiles", "ClearAllLocalHistory", "ClearLocal", "DeleteBranch", "DeleteRemote", "DeleteRemoteBranch",
		"Discard", "DiscardAll", "DiscardChanges", "DiscardHunk", "DiscardLines",
		"PushForce", "RebaseOnto", "RegenSettingsDoc", "ReopenWithEncoding", "ResetSettings", "ResetTo", "RestoreDeleted", "Save",
		"SkipOngoing", "StartRebase", "StashDrop", "UndoCommit",
		"commit", "deleteBranch", "removeWorktree",
	}
	got, asks := funcsUsing(t, ".", "AskDanger", "Ask")
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("调 AskDanger 的函数\n  有 %v\n  该 %v", got, want)
	}
	if asks < 30 {
		t.Errorf("普通的 Ask 只数到 %d 处，源码读错了？", asks)
	}
}

// 界面层自己画的红框（dlgDanger）：不经 AskDanger、带表单或说明的那几个对话框也要在名单里。
// 删标签（refDialog，没有输入框时）、移除工作树的第一问（removeWorktreeDlg）；
// confirmDialog 是 AskDanger 本身的画法、dlgIcon 只按种类挑图标，不算。
func TestConfirmDangerUI(t *testing.T) {
	want := []string{"refDialog", "removeWorktreeDlg"}
	got, _ := funcsUsing(t, "../ui", "dlgDanger", "")
	got = slices.DeleteFunc(got, func(s string) bool { return s == "confirmDialog" || s == "dlgIcon" })
	if !slices.Equal(got, want) {
		t.Errorf("界面里用 dlgDanger 的函数\n  有 %v\n  该 %v", got, want)
	}
}

// funcsUsing 扫 dir 里的非测试源码：哪些函数体里出现了名字 name（a.AskDanger 的 AskDanger、dlgDanger 都算），排好序；
// 另数 also 出现了几处（also 为空不数）。
func funcsUsing(t *testing.T, dir, name, also string) (got []string, n int) {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	if len(files) == 0 {
		t.Fatalf("%s 下没有源码", dir)
	}
	fs := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		af, err := parser.ParseFile(fs, f, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range af.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil || fd.Name.Name == name {
				continue
			}
			ast.Inspect(fd.Body, func(x ast.Node) bool {
				if id, ok := x.(*ast.Ident); ok {
					switch {
					case id.Name == name:
						if !slices.Contains(got, fd.Name.Name) {
							got = append(got, fd.Name.Name)
						}
					case also != "" && id.Name == also:
						n++
					}
				}
				return true
			})
		}
	}
	slices.Sort(got)
	return got, n
}
