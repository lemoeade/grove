package ui

import (
	"testing"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/testutil"
)

// 右键菜单"闪一下就消失"的几处（真窗口里才遇得到，这里一处一处喂）：
//   - 右键按下的事件里取不到窗口大小（c.Root().Bounds() 是 0×0），菜单按空窗口排成一小块：按下那一刻排的就要和下一帧一样；
//   - macOS 的右键、触摸板的相位事件会送来 dy=0 的滚动，不算滚文件树，菜单留着；
//   - 窗口里别的元素拿了键盘（列表在按下时自己会拿）不算窗口失焦，菜单留着；窗口不再是输入去的那个才关。
func TestGroveMenuNoFlash(t *testing.T) {
	alpha, beta, gamma, _ := testutil.ThreeRepos(t)
	gamma.Write("menu.txt", "m\n")
	gamma.Commit("加 menu.txt")
	k := newTreeKit(t, alpha.Root, beta.Root, gamma.Root)
	tt, a := k.tt, k.a
	x, y, err := rowCenter(tt, "menu.txt")
	if err != nil {
		t.Fatal(err)
	}

	// 右键按下（还没到下一帧）：菜单已经按真的窗口排好
	tt.PressButton(1, x, y)
	if !a.Menu.IsOpen() || len(a.Menu.Levels()) != 1 {
		t.Fatalf("右键按下没开菜单：开着 %v，%d 层", a.Menu.IsOpen(), len(a.Menu.Levels()))
	}
	at := a.Menu.Levels()[0].Rect
	tt.Frame()
	tt.Frame()
	if now := a.Menu.Levels()[0].Rect; now != at {
		t.Fatalf("按下那一刻排的菜单 %+v 和下一帧的 %+v 不一样（事件里窗口大小不对）", at, now)
	}
	if at.H < 200 || at.Y < y {
		t.Fatalf("菜单该整份开在那一行底下：%+v（行在 y=%.0f）", at, y)
	}
	// 0 的滚动：留着
	tt.Scroll(x, y, 0, 0)
	tt.Frame()
	if !a.Menu.IsOpen() {
		t.Fatal("dy=0 的滚动把菜单关了")
	}
	// 按住不动松开：留着，什么也没选
	tt.ReleaseButton(1, x, y)
	tt.Frame()
	tt.Frame()
	if !a.Menu.IsOpen() || a.Menu.Chosen() {
		t.Fatalf("松开右键：开着 %v，选了 %v", a.Menu.IsOpen(), a.Menu.Chosen())
	}
	// 窗口里别的元素拿键盘：留着
	a.FocusTree = true
	tt.Frame()
	tt.Frame()
	tt.Frame()
	if !a.Menu.IsOpen() {
		t.Fatal("窗口里别的元素拿了键盘，菜单不该关")
	}
	// 键盘离开窗口、窗口还是活跃的（系统的浮层拿了键盘）：留着
	tt.SetFocused(false)
	tt.Frame()
	tt.Frame()
	if !a.Menu.IsOpen() {
		t.Fatal("窗口还活跃，菜单不该关")
	}
	// 窗口不活跃了（切到别的窗口 / 应用）：关
	tt.SetWindowActive(false)
	tt.Frame()
	tt.Frame()
	if a.Menu.IsOpen() {
		t.Fatal("窗口不活跃了该关")
	}
	tt.SetWindowActive(true)
	tt.SetFocused(true)
	tt.Frame()

	// 键盘开的（⇧F10）也一样：按键那一刻排的和下一帧一样
	mustClickRow(t, tt, "menu.txt")
	tt.Frame()
	tt.Key(wui.Shift, wui.KeyF10)
	if !a.Menu.IsOpen() {
		t.Fatal("⇧F10 没开菜单")
	}
	at = a.Menu.Levels()[0].Rect
	tt.Frame()
	tt.Frame()
	if now := a.Menu.Levels()[0].Rect; now != at {
		t.Fatalf("⇧F10 那一刻排的菜单 %+v 和下一帧的 %+v 不一样", at, now)
	}
	closeMenu(t, tt)
}
