package ui

import (
	"testing"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/workspace"
)

// frostFill 画的浮层是磨砂的：底下一半黑一半白，面板盖在黑白交界上，交界那里透出来的是糊开的灰、盖着 80% 的底色，
// 不是纯的底色；系统开了“减少透明度”或“增强对比度”、不透明度设成 100 就是纯色。两种风格都一样。
func TestFrostFill(t *testing.T) {
	defer vsLook.Store(vsLook.Load())
	defer frostPct.Store(frostPct.Load())
	frostPct.Store(workspace.DefaultFrost)
	bg := wui.RGBA(255, 255, 255, 1)
	view := func(c *wui.Context) {
		wui.Box(c).Width(200).Height(100).Background(wui.RGBA(0, 0, 0, 1)).Children(func() {
			wui.Box(c).Absolute().Left(100).Top(0).Width(100).Height(100).Background(wui.RGBA(255, 255, 255, 1))
			frostFill(c, wui.Box(c).Absolute().Left(40).Top(20).Width(120).Height(60).Radius(dlgRadius), bg)
		})
	}
	for _, vs := range []bool{true, false} {
		vsLook.Store(vs)
		tt := wui.NewTester(view, 200, 100)
		at := func() (r, g, b uint8) {
			tt.Frame()
			img := tt.Image()
			s := float64(img.Bounds().Dx()) / 200
			c := img.RGBAAt(int(99*s), int(50*s))
			return c.R, c.G, c.B
		}
		tt.SetPreferences(wui.Preferences{TextScale: 1})
		r, _, _ := at()
		// 交界左边一点：没糊是 80% 白盖在黑上（204），糊开了白的那边渗过来，更亮；纯色是 255
		if r <= 168 || r >= 254 {
			t.Errorf("vs=%v：交界处该是糊开的灰盖 80%% 的白（169–253），量到 %d", vs, r)
		}
		frostPct.Store(100)
		if r, g, b := at(); r != 255 || g != 255 || b != 255 {
			t.Errorf("vs=%v：不透明度 100 该是纯色的底，量到 %d,%d,%d", vs, r, g, b)
		}
		frostPct.Store(workspace.DefaultFrost)
		for _, p := range []wui.Preferences{{TextScale: 1, ReduceTransparency: true}, {TextScale: 1, HighContrast: true}} {
			tt.SetPreferences(p)
			if r, g, b := at(); r != 255 || g != 255 || b != 255 {
				t.Errorf("vs=%v %+v：该是纯色的底，量到 %d,%d,%d", vs, p, r, g, b)
			}
		}
	}
}

// 默认的不透明度下 VS Code 风格的浮层字还读得清：底下是最坏的颜色（浅色底下全黑、深色底下全白）时，
// VS Code 的字色（#202020 / #BFBFBF）在菜单、命令面板、悬停提示的底上至少 2.5:1（
// 真的底下是糊开的、平均过的内容，到不了纯白一片，嫌淡可以把浮层不透明度调高）。
func TestFrostContrast(t *testing.T) {
	a := float32(workspace.DefaultFrost) / 100
	for _, dark := range []bool{false, true} {
		th, worst, text := wui.LightTheme(), wui.RGBA(0, 0, 0, 1), wui.Hex("#202020")
		if dark {
			th, worst, text = wui.DarkTheme(), wui.RGBA(255, 255, 255, 1), wui.Hex("#bfbfbf")
		}
		for name, bg := range map[string]wui.Color{"菜单": vsOverlayBg(th), "命令面板": vsQuickBg(th), "悬停提示": vsWidgetBg(th)} {
			panel := worst.Mix(bg, a)
			if r := contrast(text, panel); r < 2.5 {
				t.Errorf("dark=%v %s：最坏的底上面板 %v，字只有 %.2f:1", dark, name, panel, r)
			}
		}
	}
}
