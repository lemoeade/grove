package ui

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/testutil"
)

// 文件树里按空格是系统的"快速查看"：看光标那一行（文件、目录都行），↓ 之后再按是下一行；选了多行也只看光标那行；
// 输入名字时空格归输入框；右键"更多"里也有一项；打不开时弹一句。
func TestTreeQuickLook(t *testing.T) {
	repo := testutil.NewRepo(t, "proj")
	repo.Write("a.txt", "a\n")
	repo.Write("b.txt", "b\n")
	repo.Write("docs/d.md", "d\n")
	repo.Commit("init")
	k := newTreeKit(t, repo.Root)
	a, tt := k.a, k.tt
	var seen []string
	var fail error
	a.Preview = func(p string) error {
		seen = append(seen, p)
		return fail
	}
	full := func(rel string) string { return filepath.Join(a.Repos[0].Root, filepath.FromSlash(rel)) }
	expect := func(what string, want ...string) {
		t.Helper()
		if !slices.Equal(seen, want) {
			t.Fatalf("%s：快速查看了 %q，该是 %q", what, seen, want)
		}
		seen = nil
	}
	space := func() {
		tt.Key(0, wui.KeySpace)
		tt.Frame()
	}

	k.click(0, "a.txt")
	space()
	expect("点 a.txt 再按空格", full("a.txt"))
	tt.Key(0, wui.KeyDown)
	tt.Frame()
	space()
	expect("↓ 之后再按", full("b.txt"))

	// 目录也能看；点目录是展开，光标在它上面
	k.click(0, "docs")
	space()
	expect("目录", full("docs"))

	// 选了两行：只看光标那行（最后点的）
	k.click(0, "a.txt")
	k.click(wui.Cmd, "b.txt")
	if n := len(a.ChosenRows()); n != 2 {
		t.Fatalf("选中 %d 行", n)
	}
	space()
	expect("选了两行", full("b.txt"))

	// 输入名字时空格是输入框的
	k.click(0, "a.txt")
	tt.Key(0, wui.KeyF2)
	k.wait("重命名输入框", func() bool { return tt.Focused("新名字") })
	space()
	expect("输入名字时")
	tt.Key(0, wui.KeyEscape)
	k.wait("输入框收起", func() bool { return a.Edit == nil })

	// 右键 → 更多 → 快速查看
	k.menu("b.txt", "更多", "快速查看")
	expect("右键菜单", full("b.txt"))

	// 打不开：弹一句，不闷着
	fail = errors.New("坏了")
	k.click(0, "a.txt")
	space()
	expect("打不开时", full("a.txt"))
	if !a.AlertOpen || a.AlertTitle != "没法快速查看" {
		t.Fatalf("打不开：alert=%v %q", a.AlertOpen, a.AlertTitle)
	}
}
