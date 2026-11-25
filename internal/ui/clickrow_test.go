package ui

import (
	"testing"
	"time"

	"fmt"

	wui "github.com/egoist/mygo/ui"
)

// clickRow / clickRowWith / rightClickRow 点名字是 s 的那一格的中间，不走 Tester.Click 的"标签所在的元素自己接输入"校验。
//
// mygo 6842ad5e 起 Tester.Click 要求标签所在的元素（或它最近的接输入的祖先）就是点下去接输入的那个。
// 这几处标签挂在外层容器上，接点击的是容器里面的行：
//   - 列表的格子：ListOptions.Label 给的名字（打字跳行、读屏用）记在 List 的那一格上，接点击的是格子里面的行
//     （文件树行的 menuPress / treePress、Git 栏的段头、命令面板的结果行）；
//   - 历史右栏的标题行：名字"历史"在整行上，行的正中是搜索框。
//
// 真窗口里点那一格，落到的就是里面那一行（它铺满整格），这正是要的；所以这里只是换个点法，断言不变。
// 找不到名字、没有可见面积时照旧报错。
func clickRow(tt *wui.Tester, s string) error { return clickRowWith(tt, 0, s) }

func clickRowWith(tt *wui.Tester, mods wui.Modifiers, s string) error {
	x, y, err := rowCenter(tt, s)
	if err != nil {
		return err
	}
	tt.ClickAtWith(mods, x, y)
	return nil
}

func rightClickRow(tt *wui.Tester, s string) error {
	x, y, err := rowCenter(tt, s)
	if err != nil {
		return err
	}
	tt.RightClickAt(x, y)
	return nil
}

func rowCenter(tt *wui.Tester, s string) (float32, float32, error) {
	r, ok := tt.Find(s)
	if !ok {
		return 0, 0, fmt.Errorf("ui: no element shows %q", s)
	}
	if r.W <= 0 || r.H <= 0 {
		return 0, 0, fmt.Errorf("ui: %q has no visible area", s)
	}
	return r.X + r.W/2, r.Y + r.H/2, nil
}

// mustClickRow / mustRightClickRow 是 mustClick / mustRightClick 的 clickRow 版。
func mustClickRow(t *testing.T, tt *wui.Tester, s string) {
	t.Helper()
	if err := clickRow(tt, s); err != nil {
		t.Fatalf("点 %q: %v；texts=%q", s, err, tt.Texts())
	}
	tt.Frame()
}

func mustRightClickRow(t *testing.T, tt *wui.Tester, s string, item ...string) {
	t.Helper()
	if err := rightClickRow(tt, s); err != nil {
		t.Fatalf("右键 %q: %v", s, err)
	}
	tt.Frame()
	if err := chooseMenu(t, tt, item...); err != nil {
		t.Fatalf("右键 %q 选 %q: %v；菜单=%q", s, item, err, shownMenu(t, tt))
	}
	tt.Frame()
	tt.Frame()
}

// mustClickTitle 点历史里名字是 s 的提交那一格的第一行（标题）。展开的提交那一格往下还有作者、正文和文件，
// 点正中会落在下面（只选中，不收起）；点标题才是"展开 / 收起"。Tester 的钟不走，先拨一秒，
// 免得和上一下凑成双击。
func mustClickTitle(t *testing.T, tt *wui.Tester, s string) {
	t.Helper()
	tt.Advance(time.Second)
	r, ok := tt.Find(s)
	if !ok || r.W <= 0 || r.H <= 0 {
		t.Fatalf("历史里没有提交 %q；texts=%q", s, tt.Texts())
	}
	tt.ClickAt(r.X+r.W/2, r.Y+min(r.H, histRowH())/2)
	tt.Frame()
}
