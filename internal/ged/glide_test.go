package ged

import (
	"fmt"
	"testing"
	"time"

	wui "github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// 鼠标滚轮一格在 150 ms 里缓出地滑过去（和 mygo 自己的滚动容器一样），连着几格接在一起；触控板一点点地来，照旧立刻跟手；
// 滑着时别的挪了它（触控板、键）就不滑了；系统开了“减少动态效果”一格一下跳过去。
func TestEditorWheelGlides(t *testing.T) {
	lines := make([]string, 400)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	d := plainEd(t, lines)
	lt := wui.LightTheme()
	th := editor.DefaultTheme(lt)
	view := func(c *wui.Context) {
		wui.Column(c).Width(600).Height(400).Children(func() {
			View(c, "ed", d, Style{Code: th, Font: wui.Font{Size: 13}})
		})
	}
	tt := wui.NewTester(view, 600, 400)
	tt.Frame()
	if d.lh <= 0 {
		t.Fatal("没画出来")
	}
	tt.Wheel(300, 200, 0, 120)
	if d.ScrollY <= 0 || d.ScrollY >= 120 {
		t.Fatalf("滚轮一格收到的这一帧该先走一段、没走完，ScrollY = %v", d.ScrollY)
	}
	at := d.ScrollY
	tt.Advance(50 * time.Millisecond)
	if d.ScrollY <= at || d.ScrollY >= 120 {
		t.Fatalf("50 ms 后该在半路上，ScrollY = %v（之前 %v）", d.ScrollY, at)
	}
	tt.Wheel(300, 200, 0, 120) // 再一格：接在没走完的后面
	tt.Advance(300 * time.Millisecond)
	if d.ScrollY != 240 {
		t.Fatalf("两格滑完该在 240，ScrollY = %v", d.ScrollY)
	}
	tt.Wheel(300, 200, 0, 120)
	tt.Scroll(300, 200, 0, -10) // 触控板接手：不滑了，立刻跟手
	y := d.ScrollY
	tt.Advance(300 * time.Millisecond)
	if d.ScrollY != y {
		t.Fatalf("触控板挪过以后滚轮还在滑：%v → %v", y, d.ScrollY)
	}
	tt.Wheel(300, 200, 0, -120) // 反方向：立刻掉头
	tt.Advance(300 * time.Millisecond)
	if d.ScrollY != y-120 {
		t.Fatalf("往回一格该在 %v，ScrollY = %v", y-120, d.ScrollY)
	}
	tt.Wheel(300, 200, 0, -100000) // 滚过头：停在顶上
	tt.Advance(300 * time.Millisecond)
	if d.ScrollY != 0 {
		t.Fatalf("滚过顶该停在 0，ScrollY = %v", d.ScrollY)
	}
	tt.SetPreferences(wui.Preferences{TextScale: 1, ReduceMotion: true})
	tt.Frame()
	tt.Wheel(300, 200, 0, 120)
	if d.ScrollY != 120 {
		t.Fatalf("减少动态效果：一格该一下跳过去，ScrollY = %v", d.ScrollY)
	}
}
