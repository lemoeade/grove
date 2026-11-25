package main

import (
	"testing"

	"github.com/egoist/mygo"

	"github.com/lemoeade/grove/internal/workspace"
)

// 主窗口的材质：窗口底是柔和（默认）、纯色时什么都不透；通透时默认侧栏，设置里选的那种，WORKBENCH_MATERIAL 压过设置。
func TestVibrancy(t *testing.T) {
	t.Setenv("WORKBENCH_MATERIAL", "")
	for _, c := range []struct {
		s    workspace.Settings
		env  string
		want mygo.Vibrancy
	}{
		{workspace.Settings{}, "", mygo.VibrancyNone}, // 柔和
		{workspace.Settings{Backdrop: "flat"}, "", mygo.VibrancyNone},
		{workspace.Settings{Material: "menu"}, "", mygo.VibrancyNone},
		{workspace.Settings{}, "menu", mygo.VibrancyNone},
		{workspace.Settings{Backdrop: "flat"}, "menu", mygo.VibrancyNone},
		{workspace.Settings{Look: "grove", Backdrop: "vibrancy"}, "", mygo.VibrancySidebar},
		{workspace.Settings{Look: "grove", Backdrop: "vibrancy", Material: "menu"}, "", mygo.VibrancyMenu},
		{workspace.Settings{Look: "grove", Backdrop: "vibrancy", Material: "glass"}, "", mygo.VibrancyGlass},
		{workspace.Settings{Look: "grove", Backdrop: "vibrancy", Material: "menu"}, "popover", mygo.VibrancyPopover},
		{workspace.Settings{Look: "grove", Backdrop: "vibrancy"}, "under-window", mygo.VibrancyUnderWindow},
		{workspace.Settings{Look: "grove", Backdrop: "vibrancy"}, "hud", mygo.VibrancyHUD},     // 设置里没有了，开发开关照样能试
		{workspace.Settings{Backdrop: "vibrancy", Material: "menu"}, "hud", mygo.VibrancyNone}, // VS Code 风格不透
	} {
		t.Setenv("WORKBENCH_MATERIAL", c.env)
		if got := vibrancy(&c.s); got != c.want {
			t.Errorf("%+v WORKBENCH_MATERIAL=%q：%q，应是 %q", c.s, c.env, got, c.want)
		}
	}
	// 设置里能选的都是 mygo 认的材质名
	t.Setenv("WORKBENCH_MATERIAL", "")
	for _, m := range workspace.Materials[1:] {
		if v := vibrancy(&workspace.Settings{Look: "grove", Backdrop: "vibrancy", Material: m}); string(v) != m {
			t.Errorf("材质 %q 成了 %q", m, v)
		}
	}
}

// 设置 / 关于窗口摆在工作台窗口正中，大小不变；工作台窗口靠着屏幕边时推回可用区里，比可用区大就缩小。
func TestCenterIn(t *testing.T) {
	area := mygo.Rectangle{X: 0, Y: 25, Width: 1920, Height: 1055}
	for _, c := range []struct {
		parent, r, want mygo.Rectangle
	}{
		{mygo.Rectangle{X: 0, Y: 25, Width: 1920, Height: 1055}, mygo.Rectangle{X: 660, Y: 157, Width: 780, Height: 560}, mygo.Rectangle{X: 570, Y: 272, Width: 780, Height: 560}},
		{mygo.Rectangle{X: 200, Y: 100, Width: 1000, Height: 700}, mygo.Rectangle{Width: 600, Height: 400}, mygo.Rectangle{X: 400, Y: 250, Width: 600, Height: 400}},
		{mygo.Rectangle{X: 1700, Y: 900, Width: 400, Height: 300}, mygo.Rectangle{Width: 780, Height: 560}, mygo.Rectangle{X: 1140, Y: 520, Width: 780, Height: 560}}, // 出了右下角：推回来
		{mygo.Rectangle{X: -300, Y: 0, Width: 400, Height: 300}, mygo.Rectangle{Width: 780, Height: 560}, mygo.Rectangle{X: 0, Y: 25, Width: 780, Height: 560}},       // 出了左上角
		{mygo.Rectangle{X: 0, Y: 25, Width: 1920, Height: 1055}, mygo.Rectangle{Width: 2400, Height: 1200}, area},                                                     // 比可用区大：缩成可用区
	} {
		if got := centerIn(c.parent, c.r, area); got != c.want {
			t.Errorf("centerIn(%v, %v) = %v, want %v", c.parent, c.r, got, c.want)
		}
	}
	// 拿不到屏幕（可用区为空）：只摆正中，不推不缩
	if got := centerIn(mygo.Rectangle{Width: 100, Height: 100}, mygo.Rectangle{Width: 200, Height: 200}, mygo.Rectangle{}); got != (mygo.Rectangle{X: -50, Y: -50, Width: 200, Height: 200}) {
		t.Errorf("no work area: %v", got)
	}
}
