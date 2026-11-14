package ui

import (
	"image/color"
	"testing"

	wui "github.com/egoist/mygo/ui"

	"github.com/lemoeade/grove/internal/app"
	"github.com/lemoeade/grove/internal/testutil"
)

// 磨砂面板的底色是半透明的，底下是什么颜色字都得读得清：按最坏的底算（浅色主题底下全黑、深色主题底下全白），
// 字和面板、字和高亮项都至少 4.5:1。
func TestGroveMenuTintContrast(t *testing.T) {
	for _, dark := range []bool{false, true} {
		th, under := wui.LightTheme(), wui.RGBA(0, 0, 0, 1)
		if dark {
			th, under = wui.DarkTheme(), wui.RGBA(255, 255, 255, 1)
		}
		tint := gmenuTint(th)
		if tint.A == 0 || tint.A == 255 {
			t.Fatalf("dark=%v：底色该是半透明的：%v", dark, tint)
		}
		panel := under.Mix(gmenuBg(th), float32(tint.A)/255)
		if r := contrast(th.Text, panel); r < 4.5 {
			t.Errorf("dark=%v：最坏的底上面板是 %v，字 %v 只有 %.2f:1", dark, panel, th.Text, r)
		}
		bg, fg := gmenuHot(th)
		hot := panel.Mix(wui.RGBA(bg.R, bg.G, bg.B, 1), float32(bg.A)/255)
		if r := contrast(fg, hot); r < 4.5 {
			t.Errorf("dark=%v：最坏的底上高亮项是 %v，字 %v 只有 %.2f:1", dark, hot, fg, r)
		}
	}
}

// 菜单面板是磨砂的：底下的内容糊开、盖一层半透明的底色，所以面板上没有字的地方不是 gmenuBg 那个纯色；
// 系统开了“减少透明度”或“增强对比度”就退回不透明的 gmenuBg。浅色深色都量。
func TestGroveMenuFrosted(t *testing.T) {
	alpha, beta, gamma, _ := testutil.ThreeRepos(t)
	gamma.Write("menu.txt", "m\n")
	gamma.Commit("加 menu.txt")
	k := newTreeKit(t, alpha.Root, beta.Root, gamma.Root)
	tt := k.tt
	const winW = 1200
	tt.SetSize(winW, 800)
	tt.Frame()
	if err := rightClickRow(tt, "menu.txt"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	ms := popMenus(t, tt)
	if len(ms) != 1 {
		t.Fatalf("右键 menu.txt 该开一层菜单：%q", shownMenu(t, tt))
	}
	defer closeMenu(t, tt)
	p := ms[0].Panel
	// 面板顶上那条内边距的正中：边框里面、第一项上面，没有字
	at := func() color.RGBA {
		tt.Frame()
		img := tt.Image()
		s := float64(img.Bounds().Dx()) / winW
		return img.RGBAAt(int((p.X+p.W/2)*s), int((p.Y+1+app.MenuPad/2)*s))
	}
	same := func(c color.RGBA, w wui.Color) bool {
		d := func(a, b uint8) int { return max(int(a), int(b)) - min(int(a), int(b)) }
		return d(c.R, w.R) <= 1 && d(c.G, w.G) <= 1 && d(c.B, w.B) <= 1
	}
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		th := wui.LightTheme()
		if dark {
			th = wui.DarkTheme()
		}
		bg := gmenuBg(th)
		tt.SetPreferences(wui.Preferences{TextScale: 1})
		if c := at(); same(c, bg) {
			t.Errorf("dark=%v：面板该是磨砂的，量到的却是不透明的底色 %v", dark, c)
		}
		for _, pref := range []wui.Preferences{{TextScale: 1, ReduceTransparency: true}, {TextScale: 1, HighContrast: true}} {
			tt.SetPreferences(pref)
			if c := at(); !same(c, bg) {
				t.Errorf("dark=%v %+v：面板该是不透明的 %v，量到 %v", dark, pref, bg, c)
			}
		}
	}
	tt.SetPreferences(wui.Preferences{TextScale: 1})
	tt.SetDark(false)
}
